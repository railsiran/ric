// commands/console.go
package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"ric/dispatcher"
)

// Console opens a Rails console inside the container.
func Console(inputs []string, flagArgs []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current directory: %w", err)
	}
	name := filepath.Base(cwd)

	checkCmd := exec.Command("docker", "inspect", name)
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("container %s not found — are you inside a ric project directory?", name)
	}

	fmt.Printf("Opening Rails console for %s...\n", name)

	cmd := exec.Command(
		"docker", "exec", "-it",
		"-w", "/workspace/"+name,
		name,
		"bin/rails", "console",
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("console failed: %w", err)
	}

	return nil
}

func init() {
	dispatcher.Register("console", Console)
}
