// commands/new.go
package commands

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"ric/dispatcher"
)

// BaseURL is the server base URL for downloading Docker images.
// It is set at build time via ldflags.
var BaseURL = "http://localhost:3000"

// loadImage downloads and loads a Docker image, or checks local if --local.
// The variant is picked from the JS choice (three / p5 / react — each already
// includes tailwind), falling back to tailwind, falling back to base.
func loadImage(css, js string, local bool) (string, error) {
	imageName := "ri_base"
	switch {
	case js == "three":
		imageName = "ri_three"
	case js == "p5":
		imageName = "ri_p5"
	case js == "react":
		imageName = "ri_react"
	case css == "tailwind":
		imageName = "ri_tailwind"
	}

	if local {
		fmt.Printf("Checking for local image %s...\n", imageName)
		cmd := exec.Command("docker", "image", "inspect", imageName+":latest")
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("local image %s not found: %w", imageName, err)
		}
		fmt.Println("Local image found.")
		return imageName, nil
	}

	// Download
	url := fmt.Sprintf("%s/package?name=%s", BaseURL, imageName)
	tarPath := filepath.Join(os.TempDir(), imageName+".tar")

	fmt.Printf("Downloading %s...\n", imageName)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	file, err := os.Create(tarPath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", fmt.Errorf("save download: %w", err)
	}
	file.Close()

	// Load into Docker
	fmt.Printf("Loading %s into Docker...\n", imageName)
	loadCmd := exec.Command("docker", "load", "-i", tarPath)
	loadCmd.Stdout = os.Stdout
	loadCmd.Stderr = os.Stderr
	if err := loadCmd.Run(); err != nil {
		return "", fmt.Errorf("docker load failed: %w", err)
	}

	// Clean up
	os.Remove(tarPath)

	return imageName, nil
}

// findPort finds an available port starting from the given port.
func findPort(start int) (int, error) {
	for port := start; port < start+100; port++ {
		addr := fmt.Sprintf(":%d", port)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			listener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports found between %d and %d", start, start+100)
}

// createContainer runs the Docker container with the renamed project.
func createContainer(name, imageName string) (int, error) {
	port, err := findPort(3000)
	if err != nil {
		return 0, err
	}

	fmt.Printf("Creating container %s on port %d...\n", name, port)

	renameCmd := fmt.Sprintf(
		"if [ -d /workspace/railsiran ]; then mv /workspace/railsiran /workspace/%s; fi && tail -f /dev/null",
		name,
	)

	cmd := exec.Command(
		"docker", "run", "-d",
		"--name", name,
		"-p", fmt.Sprintf("%d:3000", port),
		imageName+":latest",
		"sh", "-c", renameCmd,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("docker run failed: %w", err)
	}

	fmt.Println("Waiting for container to be ready...")
	time.Sleep(4 * time.Second)

	fmt.Println("Container created.")
	return port, nil
}

// copyProject copies the project from the container to the current directory.
func copyProject(name string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current directory: %w", err)
	}

	targetDir := filepath.Join(cwd, name)

	// Check if directory already exists
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("directory %s already exists", targetDir)
	}

	fmt.Printf("Copying project to %s...\n", targetDir)

	source := fmt.Sprintf("%s:/workspace/%s", name, name)
	cmd := exec.Command("docker", "cp", "-a", source, targetDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker cp failed: %w", err)
	}

	fmt.Println("Project copied.")
	return nil
}

// regenerateMasterKey creates a new master.key and credentials.yml.enc
// inside the container and copies them back to the host project.
func regenerateMasterKey(name string) error {
	// Remove old files and generate new ones inside the container
	genCmd := exec.Command("docker", "exec",
		"-w", "/workspace/"+name,
		name,
		"sh", "-c",
		"rm -f config/master.key config/credentials.yml.enc && EDITOR=true bin/rails credentials:edit",
	)
	genCmd.Stdout = os.Stdout
	genCmd.Stderr = os.Stderr
	if err := genCmd.Run(); err != nil {
		return fmt.Errorf("regenerate master key: %w", err)
	}

	// Copy the new files back to the host project
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}
	for _, file := range []string{"config/master.key", "config/credentials.yml.enc"} {
		src := fmt.Sprintf("%s:/workspace/%s/%s", name, name, file)
		dst := filepath.Join(cwd, name, file)
		cpCmd := exec.Command("docker", "cp", src, dst)
		cpCmd.Stdout = os.Stdout
		cpCmd.Stderr = os.Stderr
		if err := cpCmd.Run(); err != nil {
			return fmt.Errorf("copy %s back: %w", file, err)
		}
	}
	return nil
}

// parseName extracts and validates the project name from inputs.
func parseName(inputs []string) (string, error) {
	if len(inputs) != 1 {
		return "", fmt.Errorf("ric new expects exactly one argument: the project name")
	}
	name := inputs[0]

	if len(name) == 0 {
		return "", fmt.Errorf("project name is required")
	}
	if name[0] >= '0' && name[0] <= '9' {
		return "", fmt.Errorf("invalid project name: %s (cannot start with a number)", name)
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.') {
			return "", fmt.Errorf("invalid project name: %s (use letters, numbers, hyphens, underscores, and dots)", name)
		}
	}
	return name, nil
}

// newFlags holds the parsed flag values for the new command.
type newFlags struct {
	css   string // "" or "tailwind"
	js    string // "" or "three" / "p5" / "react"
	local bool
}

// parseFlags extracts flags from the -- segment.
func parseFlags(flagArgs []string) (newFlags, error) {
	var flags newFlags
	for i := 0; i < len(flagArgs); i++ {
		switch flagArgs[i] {
		case "--css":
			i++
			if i >= len(flagArgs) {
				return flags, fmt.Errorf("--css requires a value")
			}
			if flagArgs[i] != "tailwind" {
				return flags, fmt.Errorf("unsupported css: %s (only 'tailwind' is supported)", flagArgs[i])
			}
			flags.css = flagArgs[i]
		case "--js":
			i++
			if i >= len(flagArgs) {
				return flags, fmt.Errorf("--js requires a value (three, p5, or react)")
			}
			switch flagArgs[i] {
			case "three", "p5", "react":
				flags.js = flagArgs[i]
			default:
				return flags, fmt.Errorf("unsupported js: %s (expected three, p5, or react)", flagArgs[i])
			}
		case "--local":
			flags.local = true
		default:
			return flags, fmt.Errorf("unknown flag: %s", flagArgs[i])
		}
	}
	// A JS variant already bundles tailwind, so specifying both is redundant
	// and almost always a misunderstanding — reject it with a clear hint.
	if flags.js != "" && flags.css != "" {
		return flags, fmt.Errorf("--js %s already includes tailwind; drop --css", flags.js)
	}
	return flags, nil
}

// New creates a new project from a Docker image.
func New(inputs []string, flagArgs []string) error {
	name, err := parseName(inputs)
	if err != nil {
		return err
	}

	flags, err := parseFlags(flagArgs)
	if err != nil {
		return err
	}

	fmt.Printf("Creating new project: %s\n", name)
	if flags.css != "" {
		fmt.Printf("  css: %s\n", flags.css)
	}
	if flags.js != "" {
		fmt.Printf("  js:  %s\n", flags.js)
	}

	imageName, err := loadImage(flags.css, flags.js, flags.local)
	if err != nil {
		return err
	}

	port, err := createContainer(name, imageName)
	if err != nil {
		return err
	}

	if err := copyProject(name); err != nil {
		return err
	}

	// Generate a unique master key for this project
	if err := regenerateMasterKey(name); err != nil {
		return err
	}

	fmt.Printf("Project %s is ready at ./%s (port %d)\n", name, name, port)
	return nil
}

func init() {
	dispatcher.Register("new", New)
}
