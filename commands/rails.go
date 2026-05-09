// commands/rails.go
package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"ric/dispatcher"
)

// Rails runs any rails command inside the container and syncs files back.
func Rails(inputs []string, flagArgs []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current directory: %w", err)
	}
	name := filepath.Base(cwd)

	// Check container exists
	checkCmd := exec.Command("docker", "inspect", name)
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("container %s not found — are you inside a ric project directory?", name)
	}

	// Build the rails command
	args := append([]string{"exec", "-w", "/workspace/" + name, name, "bin/rails"}, inputs...)

	fmt.Printf("Running rails %s...\n", name)

	cmd := exec.Command("docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rails command failed: %w", err)
	}

	// Sync files back to host
	// First remove old files (preserving .git)
	fmt.Println("Syncing files back to host...")

	gitDir := filepath.Join(cwd, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		// .git exists, remove everything else
		entries, err := os.ReadDir(cwd)
		if err != nil {
			return fmt.Errorf("cannot read directory: %w", err)
		}
		for _, entry := range entries {
			if entry.Name() == ".git" {
				continue
			}
			path := filepath.Join(cwd, entry.Name())
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("cannot remove %s: %w", path, err)
			}
		}
	} else {
		// No .git, remove everything
		entries, err := os.ReadDir(cwd)
		if err != nil {
			return fmt.Errorf("cannot read directory: %w", err)
		}
		for _, entry := range entries {
			path := filepath.Join(cwd, entry.Name())
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("cannot remove %s: %w", path, err)
			}
		}
	}

	// Full copy from container
	source := fmt.Sprintf("%s:/workspace/%s/.", name, name)
	cpCmd := exec.Command("docker", "cp", "-a", source, cwd)
	cpCmd.Stdout = os.Stdout
	cpCmd.Stderr = os.Stderr

	if err := cpCmd.Run(); err != nil {
		return fmt.Errorf("copy failed: %w", err)
	}

	fmt.Println("Sync complete.")
	return nil
}

func init() {
	dispatcher.Register("rails", Rails)
}
