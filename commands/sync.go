// commands/sync.go
package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"ric/dispatcher"
)

// Sync mirrors your host project into the container.
// Everything is synced except the `tmp` and `storage` directories.
// Deleted files on the host are also removed from the container.
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

	// List top-level entries in the host project
	entries, err := os.ReadDir(cwd)
	if err != nil {
		return fmt.Errorf("cannot read project directory: %w", err)
	}

	for _, entry := range entries {
		entryName := entry.Name()

		// Never touch tmp/ or storage/ – those contain databases and uploads
		if entryName == "tmp" || entryName == "storage" {
			continue
		}

		hostPath := filepath.Join(cwd, entryName)
		containerPath := fmt.Sprintf("/workspace/%s/%s", name, entryName)

		// Remove the old version inside the container (if it exists)
		rmCmd := exec.Command("docker", "exec", name, "rm", "-rf", containerPath)
		rmCmd.Run() // ignore error if path doesn't exist

		// Copy the current version from host to container
		fmt.Printf("Syncing %s/...\n", entryName)
		cpCmd := exec.Command("docker", "cp", "-a", hostPath, fmt.Sprintf("%s:/workspace/%s/", name, name))
		cpCmd.Stdout = nil
		cpCmd.Stderr = os.Stderr
		if err := cpCmd.Run(); err != nil {
			return fmt.Errorf("sync %s failed: %w", entryName, err)
		}
	}

	fmt.Println("Sync complete – tmp/ and storage/ left untouched.")
	return nil
}

func init() {
	dispatcher.Register("sync", Sync)
}
