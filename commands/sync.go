package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"ric/dispatcher"
)

// Sync copies host project files to the container.
// It mirrors app/, db/, and config/ exactly, so deleted files are removed.
// storage/ is never touched – your databases and uploads stay safe.
func Sync(inputs []string, flagArgs []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current directory: %w", err)
	}
	name := filepath.Base(cwd)

	// Verify container exists
	if err := exec.Command("docker", "inspect", name).Run(); err != nil {
		return fmt.Errorf("container %s not found — are you inside a ric project directory?", name)
	}

	// Directories to mirror from host → container
	dirs := []string{"app", "db", "config"}

	for _, dir := range dirs {
		hostDir := filepath.Join(cwd, dir)
		if _, err := os.Stat(hostDir); os.IsNotExist(err) {
			// Skip if the host doesn't have this directory yet
			continue
		}

		// Remove the existing directory inside the container
		containerPath := fmt.Sprintf("/workspace/%s/%s", name, dir)
		rmCmd := exec.Command("docker", "exec", name, "rm", "-rf", containerPath)
		rmCmd.Run() // ignore error if it doesn't exist

		// Copy host directory into the container (creates the directory)
		fmt.Printf("Syncing %s/...\n", dir)
		cpCmd := exec.Command("docker", "cp", "-a", hostDir, fmt.Sprintf("%s:/workspace/%s/", name, name))
		cpCmd.Stdout = os.Stdout
		cpCmd.Stderr = os.Stderr
		if err := cpCmd.Run(); err != nil {
			return fmt.Errorf("sync %s failed: %w", dir, err)
		}
	}

	fmt.Println("Sync complete – app/, db/, config/ mirrored. storage/ untouched.")
	return nil
}

func init() {
	dispatcher.Register("sync", Sync)
}
