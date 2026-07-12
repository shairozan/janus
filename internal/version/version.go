package version

import (
	"fmt"
	"runtime"
)

// Build-time variables set via ldflags by goreleaser.
var (
	Version = "dev"     // Version is the git tag version
	Commit  = "unknown" // Commit is the git commit hash
	Date    = "unknown" // Date is the build date
	BuiltBy = "unknown" // BuiltBy indicates who/what built the binary
)

// Get returns the application version.
func Get() string {
	return Version
}

// GetFull returns detailed version information.
func GetFull() string {
	return fmt.Sprintf("Janus %s (commit: %s, built: %s, by: %s, go: %s)",
		Version, Commit, Date, BuiltBy, runtime.Version())
}

// GetBuildInfo returns build information as a map.
func GetBuildInfo() map[string]string {
	return map[string]string{
		"version":   Version,
		"commit":    Commit,
		"date":      Date,
		"builtBy":   BuiltBy,
		"goVersion": runtime.Version(),
	}
}
