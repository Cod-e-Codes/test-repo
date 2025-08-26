package shared

import "fmt"

// Version variables that can be set at build time using ldflags
var (
	// ClientVersion is the version of the MarChat client
	ClientVersion = "dev"

	// ServerVersion is the version of the MarChat server
	ServerVersion = "dev"

	// BuildTime is the time when the binary was built
	BuildTime = "unknown"

	// GitCommit is the git commit hash
	GitCommit = "unknown"
)

// GetVersionInfo returns a formatted version string
func GetVersionInfo() string {
	return fmt.Sprintf("%s (build: %s, commit: %s)", ClientVersion, BuildTime, GitCommit)
}

// GetServerVersionInfo returns a formatted server version string
func GetServerVersionInfo() string {
	return fmt.Sprintf("%s (build: %s, commit: %s)", ServerVersion, BuildTime, GitCommit)
}
