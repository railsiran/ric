package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"ric/dispatcher"
)

// Usage: ric container start | stop
func Container(inputs []string, flagArgs []string) error {
	if len(inputs) < 1 {
		return fmt.Errorf("expected 'start' or 'stop'")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current directory: %w", err)
	}
	name := filepath.Base(cwd)

	// Verify container exists
	if err := exec.Command("docker", "inspect", name).Run(); err != nil {
		return fmt.Errorf("container %s not found — are you inside a ric project directory?", name)
	}

	action := inputs[0]
	switch action {
	case "start":
		return startContainer(name)
	case "stop":
		return stopContainer(name)
	default:
		return fmt.Errorf("unknown container action: %s (expected 'start' or 'stop')", action)
	}
}

func startContainer(name string) error {
	fmt.Printf("Starting container %s...\n", name)
	cmd := exec.Command("docker", "start", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("start failed: %w", err)
	}
	fmt.Println("Container started.")
	return nil
}

func stopContainer(name string) error {
	fmt.Printf("Stopping container %s...\n", name)
	cmd := exec.Command("docker", "stop", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stop failed: %w", err)
	}
	fmt.Println("Container stopped.")
	return nil
}

func init() {
	dispatcher.Register("container", Container)
}
