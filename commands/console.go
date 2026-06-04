// commands/console.go
package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"ric/dispatcher"
)

// Console opens a Rails console inside the project's container.
// Without flags: the local dev container.
// With --remote: the production container on the host configured in
// .ric/deploy.yml, reached over the same multiplexed SSH used by deploy.
func Console(inputs []string, flagArgs []string) error {
	remote := false
	for _, f := range flagArgs {
		switch f {
		case "--remote":
			remote = true
		default:
			return fmt.Errorf("unknown flag: %s", f)
		}
	}
	if len(inputs) > 0 {
		return fmt.Errorf("ric console takes no positional arguments (got %q)", inputs[0])
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current directory: %w", err)
	}
	name := filepath.Base(cwd)

	if remote {
		return remoteConsole(name)
	}
	return localConsole(name)
}

func localConsole(name string) error {
	if err := exec.Command("docker", "inspect", name).Run(); err != nil {
		return fmt.Errorf("container %s not found — are you inside a ric project directory?", name)
	}
	fmt.Printf("Opening Rails console for %s...\n", name)
	cmd := exec.Command("docker", "exec", "-it",
		"-w", "/workspace/"+name,
		name,
		"bin/rails", "console",
	)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("console failed: %w", err)
	}
	return nil
}

// remoteConsole opens a Rails console inside the production container on the
// deployed host. RAILS_ENV is inherited from the container's primary process
// (set to production at deploy time), so this is a production console.
func remoteConsole(name string) error {
	cfg, err := loadDeployConfig()
	if err != nil {
		return err
	}
	sess := &deploySession{
		cfg:         cfg,
		name:        name,
		controlPath: fmt.Sprintf("/tmp/ric-cm-%s-%d", name, os.Getpid()),
	}
	if err := sess.openMaster(); err != nil {
		return err
	}
	defer sess.closeMaster()
	if err := sess.detectPrivilege(); err != nil {
		return err
	}

	fmt.Printf("Opening production Rails console for %s on %s...\n", name, cfg.Host)
	cmd := fmt.Sprintf("docker exec -it %s bin/rails console", name)
	return sess.sshInteractive("rails console", sess.priv(cmd))
}

func init() {
	dispatcher.Register("console", Console)
}
