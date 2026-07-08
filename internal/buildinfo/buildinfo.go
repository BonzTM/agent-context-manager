// Package buildinfo carries identifying metadata stamped into the binary at
// build time. Release builds override the ldflags variables; module-installed
// binaries (`go install ...@vX.Y.Z`) fall back to the module build info the Go
// toolchain embeds, so `acm version` is meaningful on every install path.
package buildinfo

import "runtime/debug"

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

// Info resolves the effective version, commit, and build date: ldflags values
// when stamped, otherwise the toolchain-embedded module version and VCS
// metadata (which `go install module@version` and `-buildvcs` builds carry).
func Info() (version, commit, date string) {
	version, commit, date = Version, Commit, Date
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return version, commit, date
	}
	if version == "dev" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		version = bi.Main.Version
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if commit == "none" {
				commit = s.Value
			}
		case "vcs.time":
			if date == "unknown" {
				date = s.Value
			}
		}
	}
	return version, commit, date
}
