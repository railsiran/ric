package commands

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
	"net"

	"ric/dispatcher"
)

const baseURL = "http://localhost:3000"

// downloadImage pulls the .tar file from the server.
// loadImage downloads and loads a Docker image, or checks local if --local.
func loadImage(css string, local bool) (string, error) {
	imageName := "ri_base"
	if css == "tailwind" {
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
	url := fmt.Sprintf("%s/package?name=%s", baseURL, imageName)
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

// copyProject copies the project from the container to ~/ric/<name>.
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
	css   string // empty or "tailwind"
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
		case "--local":
			flags.local = true
		default:
			return flags, fmt.Errorf("unknown flag: %s", flagArgs[i])
		}
	}
	return flags, nil
}

// ensureRicDir creates ~/ric if it doesn't exist.
func ensureRicDir() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot find home directory: %w", err)
	}
	ricDir := filepath.Join(homeDir, "ric")
	if err := os.MkdirAll(ricDir, 0755); err != nil {
		return fmt.Errorf("cannot create ~/ric: %w", err)
	}
	return nil
}

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

	imageName, err := loadImage(flags.css, flags.local)
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

	fmt.Printf("Project %s is ready at ./%s (port %d)\n", name, name, port)
	return nil
}

func init() {
	dispatcher.Register("new", New)
}
