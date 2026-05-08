package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"ric/dispatcher"
)

// Generate runs rails generate inside the container and copies new files back.
func Generate(inputs []string, flagArgs []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current directory: %w", err)
	}
	name := filepath.Base(cwd)

	checkCmd := exec.Command("docker", "inspect", name)
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("container %s not found — are you inside a ric project directory?", name)
	}

	// Build the rails generate command
	args := append([]string{"exec", "-w", "/workspace/" + name, name, "bin/rails", "generate"}, inputs...)

	fmt.Printf("Running rails generate %s...\n", name)

	cmd := exec.Command("docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("generate failed: %w", err)
	}

	// Copy generated files back to host
	fmt.Println("Syncing files back to host...")
	source := fmt.Sprintf("%s:/workspace/%s/.", name, name)
	cpCmd := exec.Command("docker", "cp", "-a", source, cwd)

	if err := cpCmd.Run(); err != nil {
		return fmt.Errorf("copy failed: %w", err)
	}

	fmt.Println("Done.")
	return nil
}

func init() {
	dispatcher.Register("generate", Generate)
}
