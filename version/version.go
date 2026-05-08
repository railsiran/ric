// version/version.go
package version

import (
	"fmt"
	"runtime"
)

var (
	Version   = "0.0.0-dev"
	Codename  = "mystery-kebab"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

func Info() string {
	return fmt.Sprintf("ric v%s (%s)\n  commit: %s\n  built:  %s\n  go:     %s %s/%s",
		Version,
		Codename,
		GitCommit,
		BuildDate,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
	)
}
