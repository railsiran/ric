package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ric/dispatcher"
)

// DeployConfig holds the parsed .ric/deploy.yml settings.
type DeployConfig struct {
	Host           string
	User           string
	SSHKey         string
	Port           int
	Domain         string
	SSLCert        string
	SSLKey         string
	RegistryMirror string
}

type deployFlags struct {
	exportOnly bool
}

func parseDeployFlags(flagArgs []string) (deployFlags, error) {
	var f deployFlags
	for i := 0; i < len(flagArgs); i++ {
		switch flagArgs[i] {
		case "--export-only":
			f.exportOnly = true
		default:
			return f, fmt.Errorf("unknown deploy flag: %s", flagArgs[i])
		}
	}
	return f, nil
}

// loadDeployConfig reads and parses .ric/deploy.yml from the current project.
func loadDeployConfig() (*DeployConfig, error) {
	data, err := os.ReadFile(".ric/deploy.yml")
	if err != nil {
		return nil, fmt.Errorf("cannot read .ric/deploy.yml: %w", err)
	}

	cfg := &DeployConfig{Port: 22} // default SSH port
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "host":
			cfg.Host = value
		case "user":
			cfg.User = value
		case "ssh_key":
			cfg.SSHKey = value
		case "port":
			port, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("invalid port: %s", value)
			}
			cfg.Port = port
		case "domain":
			cfg.Domain = value
		case "ssl_cert":
			cfg.SSLCert = value
		case "ssl_key":
			cfg.SSLKey = value
		case "registry_mirror":
			cfg.RegistryMirror = value
		}
	}

	// Expand ~ in SSH key path
	if strings.HasPrefix(cfg.SSHKey, "~/") {
		home, _ := os.UserHomeDir()
		cfg.SSHKey = filepath.Join(home, cfg.SSHKey[2:])
	}

	// Validate required fields
	if cfg.Host == "" {
		return nil, fmt.Errorf("host is required in .ric/deploy.yml")
	}
	if cfg.User == "" {
		return nil, fmt.Errorf("user is required in .ric/deploy.yml")
	}

	return cfg, nil
}

// Deploy handles the full deployment pipeline.
func Deploy(inputs []string, flagArgs []string) error {
	flags, err := parseDeployFlags(flagArgs)
	if err != nil {
		return err
	}

	// Load config
	cfg, err := loadDeployConfig()
	if err != nil {
		return err
	}

	// For now, just print the config to verify parsing
	fmt.Println("Deploy configuration:")
	fmt.Printf("  Host:       %s\n", cfg.Host)
	fmt.Printf("  User:       %s\n", cfg.User)
	fmt.Printf("  SSH Key:    %s\n", cfg.SSHKey)
	fmt.Printf("  Port:       %d\n", cfg.Port)
	fmt.Printf("  Domain:     %s\n", cfg.Domain)
	fmt.Printf("  SSL Cert:   %s\n", cfg.SSLCert)
	fmt.Printf("  SSL Key:    %s\n", cfg.SSLKey)
	fmt.Printf("  Registry:   %s\n", cfg.RegistryMirror)
	fmt.Printf("  ExportOnly: %v\n", flags.exportOnly)

	return nil
}

func init() {
	dispatcher.Register("deploy", Deploy)
}
