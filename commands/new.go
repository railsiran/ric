// commands/new.go
package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"ric/dispatcher"
)

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

	if err := ensureRicDir(); err != nil {
		return err
	}

	fmt.Printf("Creating new project: %s\n", name)
	if flags.css != "" {
		fmt.Printf("  css: %s\n", flags.css)
	}
	if flags.local {
		fmt.Println("  using local image")
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

func init() {
	dispatcher.Register("new", New)
}
