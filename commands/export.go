package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"ric/dispatcher"
)

// Export builds production assets and exports the container as a .tar file.
func Export(inputs []string, flagArgs []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current directory: %w", err)
	}
	name := filepath.Base(cwd)

	checkCmd := exec.Command("docker", "inspect", name)
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("container %s not found — are you inside a ric project directory?", name)
	}

	// 1. Precompile assets
	fmt.Println("Precompiling assets for production...")
	precompileCmd := exec.Command(
		"docker", "exec",
		"-w", "/workspace/"+name,
		name,
		"bin/rails", "assets:precompile", "-e", "production",
	)
	precompileCmd.Stdout = os.Stdout
	precompileCmd.Stderr = os.Stderr
	if err := precompileCmd.Run(); err != nil {
		return fmt.Errorf("assets precompile failed: %w", err)
	}

	// 2. Stop the container
	fmt.Println("Stopping container...")
	stopCmd := exec.Command("docker", "stop", name)
	stopCmd.Stdout = os.Stdout
	stopCmd.Stderr = os.Stderr
	if err := stopCmd.Run(); err != nil {
		return fmt.Errorf("stop failed: %w", err)
	}

	// 3. Commit to a new image
	fmt.Println("Committing container to image...")
	commitCmd := exec.Command("docker", "commit", name, name+":latest")
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	// 4. Save to .tar
	tarFile := name + ".tar"
	fmt.Printf("Exporting image to %s...\n", tarFile)
	saveCmd := exec.Command("docker", "save", "-o", tarFile, name+":latest")
	saveCmd.Stdout = os.Stdout
	saveCmd.Stderr = os.Stderr
	if err := saveCmd.Run(); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}

	fmt.Printf("Export complete: %s\n", tarFile)
	return nil
}

func init() {
	dispatcher.Register("export", Export)
}
