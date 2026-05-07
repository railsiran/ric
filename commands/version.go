package commands

import (
    "fmt"
    "ric/dispatcher"
)

// Version prints the CLI version.
func Version(inputs []string, flagArgs []string) error {
    fmt.Println("ri v0.1.0")
    return nil
}

func init() {
    dispatcher.Register("version", Version)
}
