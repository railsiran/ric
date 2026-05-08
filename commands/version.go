// commands/version.go
package commands

import (
	"fmt"
	"ric/dispatcher"
	"ric/version"
)

func Version(inputs []string, flagArgs []string) error {
	fmt.Println(version.Info())
	return nil
}

func init() {
	dispatcher.Register("version", Version)
}
