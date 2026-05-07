// main.go
package main

import (
    "fmt"
    "os"

    _ "ric/commands"     // blank import to trigger init() registration
    "ric/dispatcher"
)

func main() {
    if len(os.Args) < 2 {
				dispatcher.Dispatch("help", nil, nil)
        os.Exit(1)
    }

    operation := os.Args[1]
    rest := os.Args[2:]

    var inputs, flagArgs []string
    for i := 0; i < len(rest); i++ {
        if rest[i] == "--" {
            inputs = rest[:i]
            flagArgs = rest[i+1:]
            break
        }
    }
    if flagArgs == nil {
        inputs = rest
    }

    if err := dispatcher.Dispatch(operation, inputs, flagArgs); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        if _, ok := err.(*dispatcher.UnknownOperationError); ok {
            dispatcher.Dispatch("help", nil, nil)
        }
        os.Exit(1)
    }
}
