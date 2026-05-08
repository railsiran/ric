package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"ric/dispatcher"
)

// Migrate runs rails db:migrate inside the container.
func Migrate(inputs []string, flagArgs []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current directory: %w", err)
	}
	name := filepath.Base(cwd)

	checkCmd := exec.Command("docker", "inspect", name)
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("container %s not found — are you inside a ric project directory?", name)
	}

	fmt.Printf("Running migrations for %s...\n", name)

	cmd := exec.Command(
		"docker", "exec",
		"-w", "/workspace/"+name,
		name,
		"bin/rails", "db:migrate",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("migrate failed: %w", err)
	}

	fmt.Println("Migrations complete.")
	return nil
}

func init() {
	dispatcher.Register("migrate", Migrate)
}
