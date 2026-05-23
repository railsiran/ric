// commands/serve.go
package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ric/dispatcher"
)

// Serve starts the Rails server inside the container.
func Serve(inputs []string, flagArgs []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current directory: %w", err)
	}
	name := filepath.Base(cwd)

	// Verify container exists
	if err := exec.Command("docker", "inspect", name).Run(); err != nil {
		return fmt.Errorf("container %s not found — are you inside a ric project directory?", name)
	}

	// Check if container is running; if not, offer to start it
	if !isContainerRunning(name) {
		fmt.Printf("Container %s is not running.\n", name)
		fmt.Printf("Start it with: ric container start\n")
		return fmt.Errorf("container not running")
	}

	port, err := getContainerPort(name)
	if err != nil {
		return err
	}

	hostPort := strings.Split(port, ":")[1] // "0.0.0.0:3001" → "3001"
	url := "http://localhost:" + hostPort

	fmt.Printf("Starting server for %s...\n", name)
	fmt.Printf("Container port 3000 → %s\n", url)

	// openBrowser(url) // optional, already discussed

	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	cmd := exec.Command(
		"docker", "exec", "-it",
		"-w", "/workspace/"+name,
		name,
		"bin/dev",
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("serve failed: %w", err)
	}

	return nil
}

// getContainerPort returns the host port mapping for container port 3000.
func getContainerPort(name string) (string, error) {
	cmd := exec.Command("docker", "port", name, "3000/tcp")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("cannot get port for %s (is the container running?): %w", name, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// isContainerRunning returns true if the container is in "running" status.
func isContainerRunning(name string) bool {
	cmd := exec.Command("docker", "inspect", "-f", "{{.State.Status}}", name)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "running"
}

func init() {
	dispatcher.Register("serve", Serve)
}
