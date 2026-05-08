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

func getContainerPort(name string) (string, error) {
	cmd := exec.Command("docker", "port", name, "3000/tcp")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("cannot get port for %s: %w", name, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func Serve(inputs []string, flagArgs []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current directory: %w", err)
	}
	name := filepath.Base(cwd)

	checkCmd := exec.Command("docker", "inspect", name)
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("container %s not found — are you inside a ric project directory?", name)
	}

	port, err := getContainerPort(name)
	if err != nil {
		return err
	}

	fmt.Printf("Starting server for %s on http://%s\n", name, port)

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

func init() {
	dispatcher.Register("serve", Serve)
}
