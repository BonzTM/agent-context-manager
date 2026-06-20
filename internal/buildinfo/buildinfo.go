// Package buildinfo carries identifying metadata stamped into the binary at
// build time via -ldflags. The defaults keep `go run` and tests working without
// any flags; release builds override Version/Commit/Date.
package buildinfo

// Name is the binary/tool name. It is a constant because it never varies by
// build, unlike the version metadata below.
const Name = "acm"

// Build metadata, overridable with, e.g.:
//
//	go build -ldflags "-X github.com/bonztm/agent-context-manager/internal/buildinfo.Version=1.2.3"
var (
	// Version is the release version (semver) or "dev" for local builds.
	Version = "dev"
	// Commit is the git commit the binary was built from.
	Commit = "none"
	// Date is the build timestamp (RFC3339) or "unknown".
	Date = "unknown"
)
