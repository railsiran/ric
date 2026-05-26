// commands/upgrade.go
package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"ric/dispatcher"
)

// Upgrade updates an existing deployment on the remote server.
// It preserves the production database and credentials.
func Upgrade(inputs []string, flagArgs []string) error {
	if len(flagArgs) > 0 {
		return fmt.Errorf("ric upgrade takes no flags (got %s)", flagArgs[0])
	}
	if len(inputs) > 0 {
		return fmt.Errorf("ric upgrade takes no positional arguments (got %q)", inputs[0])
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current directory: %w", err)
	}
	name := filepath.Base(cwd)

	cfg, err := loadDeployConfig()
	if err != nil {
		return err
	}
	if err := validateLocalArtifacts(cfg); err != nil {
		return err
	}
	if err := exec.Command("docker", "inspect", name).Run(); err != nil {
		return fmt.Errorf("container %s not found — are you inside a ric project directory?", name)
	}

	step("Precompiling assets for production")
	if err := dockerExec(name, "/workspace/"+name,
		[]string{"RAILS_ENV=production"},
		"bin/rails", "assets:precompile"); err != nil {
		return fmt.Errorf("assets:precompile failed in dev container %s: %w", name, err)
	}
	done("Assets precompiled")

	step("Building deployment image")
	tarPath, sha, err := buildImage(name)
	if err != nil {
		return err
	}
	defer os.Remove(tarPath)
	done(fmt.Sprintf("Built %s @ %s", filepath.Base(tarPath), shortSHA(sha)))

	sess := &deploySession{
		cfg:         cfg,
		name:        name,
		controlPath: fmt.Sprintf("/tmp/ric-cm-%s-%d", name, os.Getpid()),
	}

	step(fmt.Sprintf("Opening SSH connection to %s@%s", cfg.User, cfg.Host))
	if err := sess.openMaster(); err != nil {
		return err
	}
	defer sess.closeMaster()
	done("SSH connection ready")

	for _, step := range []func() error{
		sess.detectPrivilege,
		sess.ensureDocker,
		sess.configureDockerMirror,
		sess.verifyRegistryMirror,
		sess.ensureExistingDeploy,
		func() error { return sess.uploadArtifacts(tarPath, sha) },
		func() error { return sess.loadAndTagImage(sha) },
		sess.ensureNetwork,
		func() error { return sess.runDBMigrate(sha) },
		func() error { return sess.rotateAppContainer(sha) },
		func() error { return sess.rotateNginxContainer(sha) },
		func() error { return sess.writeCurrentSHA(sha) },
	} {
		if err := step(); err != nil {
			return err
		}
	}

	fmt.Printf("\nUpgraded %s @ %s on %s. Live at https://%s\n",
		name, shortSHA(sha), cfg.Host, cfg.Domain)
	return nil
}

// --- upgrade-specific remote steps ---

// ensureExistingDeploy verifies the server already has a completed deploy.
func (s *deploySession) ensureExistingDeploy() error {
	root := s.root()
	if _, ok := s.sshQuiet("test -f " + shellQuote(root+"/current_sha") + " && echo yes"); !ok {
		return fmt.Errorf("no existing deployment found for %q on %s. "+
			"Use 'ric deploy' to set up the application for the first time.",
			s.name, s.cfg.Host)
	}
	return nil
}

// runDBMigrate runs migrations and (if fresh tables) seeds against the
// existing persistent storage volume.
func (s *deploySession) runDBMigrate(sha string) error {
	step("Running db:migrate against existing database")
	cmd := fmt.Sprintf("docker run --rm --network ric-net %s %s:%s bin/rails db:migrate",
		s.appMounts(), s.name, sha)
	if err := s.ssh("db:migrate", s.priv(cmd)); err != nil {
		return fmt.Errorf("db:migrate failed on %s — see rails output above. Database at %s/storage is preserved; the previous container is still running: %w",
			s.cfg.Host, s.root(), err)
	}
	done("db:migrate complete")
	return nil
}

// rotateAppContainer stops the old app container, starts the new one.
func (s *deploySession) rotateAppContainer(sha string) error {
	step("Replacing app container (zero-downtime restart)")
	// docker rm -f stops and removes in one step
	cmd := fmt.Sprintf(
		"docker rm -f %[1]s >/dev/null 2>&1; true && "+
			"docker run -d --name %[1]s --network ric-net --restart unless-stopped "+
			"%[2]s "+
			"%[1]s:%[3]s bin/rails server -b 0.0.0.0 -p 3000",
		s.name, s.appMounts(), sha)
	if err := s.ssh("restart app container", s.priv(cmd)); err != nil {
		return fmt.Errorf("could not restart app container %s on %s. Run 'docker logs %s' on the server to see why. The previous container may have been removed: %w",
			s.name, s.cfg.Host, s.name, err)
	}
	done("App container restarted")
	return nil
}

// rotateNginxContainer uploads the updated nginx config (which might have
// changed if domain/ssl were adjusted) and restarts the nginx container so
// it picks up the new config.
func (s *deploySession) rotateNginxContainer(sha string) error {
	_ = sha // unused here but kept for consistency with other methods

	step("Replacing nginx container")
	root := s.root()
	confLocal := filepath.Join(os.TempDir(), s.name+"-nginx.conf")
	if err := os.WriteFile(confLocal, []byte(renderNginxConf(s.name, s.cfg.Domain)), 0644); err != nil {
		return fmt.Errorf("write nginx config to %s: %w", confLocal, err)
	}
	defer os.Remove(confLocal)
	if err := s.scp("upload nginx.conf", confLocal, root+"/nginx/default.conf"); err != nil {
		return err
	}

	containerName := s.name + "-nginx"
	cmd := fmt.Sprintf(
		"docker rm -f %[1]s >/dev/null 2>&1; true && "+
			"docker run -d --name %[1]s --network ric-net --restart unless-stopped "+
			"-p 80:80 -p 443:443 "+
			"-v %[2]s/ssl:/etc/nginx/ssl:ro "+
			"-v %[2]s/nginx/default.conf:/etc/nginx/conf.d/default.conf:ro "+
			"nginx:stable",
		containerName, root)
	if err := s.ssh("restart nginx container", s.priv(cmd)); err != nil {
		return fmt.Errorf("nginx container failed to restart on %s — ports 80/443 may be in use: %w",
			s.cfg.Host, err)
	}
	done("Nginx container restarted")
	return nil
}

func init() {
	dispatcher.Register("upgrade", Upgrade)
}
