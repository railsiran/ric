package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"ric/dispatcher"
)

// Sync copies files from host to container.
func Sync(inputs []string, flagArgs []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current directory: %w", err)
	}
	name := filepath.Base(cwd)

	checkCmd := exec.Command("docker", "inspect", name)
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("container %s not found — are you inside a ric project directory?", name)
	}

	fmt.Printf("Syncing %s to container...\n", name)

	source := cwd + "/."
	dest := fmt.Sprintf("%s:/workspace/%s", name, name)

	cmd := exec.Command("docker", "cp", "-a", source, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	fmt.Println("Sync complete.")
	return nil
}

func init() {
	dispatcher.Register("sync", Sync)
}
