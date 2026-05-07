package commands

import (
    "fmt"
    "ric/dispatcher"
)

func Help(inputs []string, flagArgs []string) error {
    fmt.Println("Usage: ri <operation> [inputs] -- [flags]")
    fmt.Println("Available operations:")
    for _, name := range dispatcher.Operations() {
        fmt.Println(" ", name)
    }
    return nil
}

func init() {
    dispatcher.Register("help", Help)
}
