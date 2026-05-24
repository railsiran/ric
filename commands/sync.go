package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ric/dispatcher"
)

func Sync(inputs []string, flagArgs []string) error {
	reverse := false
	for _, flag := range flagArgs {
		if flag == "--reverse" {
			reverse = true
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current directory: %w", err)
	}
	name := filepath.Base(cwd)

	if err := exec.Command("docker", "inspect", name).Run(); err != nil {
		return fmt.Errorf("container %s not found — are you inside a ric project directory?", name)
	}

	if reverse {
		return syncContainerToHost(name, cwd)
	}
	return syncHostToContainer(name, cwd)
}

func syncHostToContainer(name, cwd string) error {
	entries, err := os.ReadDir(cwd)
	if err != nil {
		return fmt.Errorf("cannot read project directory: %w", err)
	}

	for _, entry := range entries {
		entryName := entry.Name()
		if entryName == "tmp" || entryName == "storage" {
			continue
		}
		hostPath := filepath.Join(cwd, entryName)
		containerPath := fmt.Sprintf("/workspace/%s/%s", name, entryName)

		exec.Command("docker", "exec", name, "rm", "-rf", containerPath).Run()

		cpCmd := exec.Command("docker", "cp", "-a", hostPath, fmt.Sprintf("%s:/workspace/%s/", name, name))
		cpCmd.Stdout = nil
		cpCmd.Stderr = os.Stderr
		if err := cpCmd.Run(); err != nil {
			return fmt.Errorf("sync %s failed: %w", entryName, err)
		}
	}
	fmt.Println("Sync complete – host → container.")
	return nil
}

func syncContainerToHost(name, cwd string) error {
	containerPath := "/workspace/" + name
	listCmd := exec.Command("docker", "exec", name, "sh", "-c",
		fmt.Sprintf("find %s -maxdepth 1 -not -name tmp -not -name storage -not -path %s", containerPath, containerPath))
	out, err := listCmd.Output()
	if err != nil {
		return fmt.Errorf("list container files: %w", err)
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		entryName := strings.TrimPrefix(line, containerPath+"/")
		if entryName == "" || entryName == containerPath {
			continue
		}
		hostPath := filepath.Join(cwd, entryName)
		containerSrc := fmt.Sprintf("%s:%s/%s", name, containerPath, entryName)

		os.RemoveAll(hostPath)

		cpCmd := exec.Command("docker", "cp", "-a", containerSrc, cwd)
		cpCmd.Stdout = nil
		cpCmd.Stderr = os.Stderr
		if err := cpCmd.Run(); err != nil {
			return fmt.Errorf("pull %s failed: %w", entryName, err)
		}
	}
	fmt.Println("Sync complete – container → host.")
	return nil
}

func init() {
	dispatcher.Register("sync", Sync)
}
