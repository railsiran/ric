// commands/deploy.go
package commands

import (
	"fmt"

	"ric/dispatcher"
)

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

func Deploy(inputs []string, flagArgs []string) error {
	flags, err := parseDeployFlags(flagArgs)
	if err != nil {
		return err
	}
	_ = flags

	fmt.Println("Deploy command starting...")
	return nil
}

func init() {
	dispatcher.Register("deploy", Deploy)
}
