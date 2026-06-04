// commands/upgrade.go
package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"ric/dispatcher"
)

// Upgrade pushes a new version of an existing deployment. The persistent
// state (database, production credentials, shared nginx, registry mirror
// config) is preserved — only the app image and site config are refreshed.
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
		sess.provisionSharedLayout,
		func() error { return sess.uploadArtifacts(tarPath, sha) },
		func() error { return sess.loadAndTagImage(sha) },
		sess.ensureNetwork,
		sess.syncMasterKeyLocal,
		func() error { return sess.runDBMigrate(sha) },
		func() error { return sess.startApp(sha) },
		sess.ensureSharedNginx,
		sess.reloadNginx,
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

// ensureExistingDeploy is the inverse of ensureFirstDeploy: upgrade requires
// the success marker (current_sha) to be present.
func (s *deploySession) ensureExistingDeploy() error {
	root := s.root()
	if _, ok := s.sshQuiet("test -f " + shellQuote(root+"/current_sha") + " && echo yes"); !ok {
		return fmt.Errorf("no existing deployment found for %q on %s. "+
			"Use 'ric deploy' to set up the application for the first time.",
			s.name, s.cfg.Host)
	}
	return nil
}

// runDBMigrate runs migrations against the existing persistent storage. No
// seeds — those are first-deploy only; running them on upgrade would either
// no-op or risk duplicating data.
func (s *deploySession) runDBMigrate(sha string) error {
	step("Running db:migrate against persistent database")
	cmd := fmt.Sprintf("docker run --rm --network ric-net %s %s:%s bin/rails db:migrate",
		s.appMounts(), s.name, sha)
	if err := s.ssh("db:migrate", s.priv(cmd)); err != nil {
		return fmt.Errorf("db:migrate failed on %s — database at %s/storage is untouched, previous container still running: %w",
			s.cfg.Host, s.root(), err)
	}
	done("db:migrate complete")
	return nil
}

func init() {
	dispatcher.Register("upgrade", Upgrade)
}
