package dispatcher

type commandFunc func(inputs []string, flagArgs []string) error

var listOfCommands = map[string]commandFunc{}

func Register(name string, fn commandFunc) {
    if _, exists := listOfCommands[name]; exists {
        panic("command already registered: " + name)
    }
    listOfCommands[name] = fn
}

func Dispatch(operation string, inputs []string, flagArgs []string) error {
    cmd, ok := listOfCommands[operation]
    if !ok {
        return &UnknownOperationError{Operation: operation}
    }
    return cmd(inputs, flagArgs)
}

type UnknownOperationError struct {
    Operation string
}

func (e *UnknownOperationError) Error() string {
    return "unknown operation: " + e.Operation
}

// Operations returns a copy of all registered operation names.
func Operations() []string {
    names := make([]string, 0, len(listOfCommands))
    for name := range listOfCommands {
        names = append(names, name)
    }
    return names
}


