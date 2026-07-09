package buildinfo

import (
	"strings"
	"testing"
)

func TestInfoStripsVPrefixFromDisplayVersion(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	Version = "v9.9.9"
	got, _, _ := Info()
	if got != "9.9.9" {
		t.Fatalf("version = %q, want v-less %q", got, "9.9.9")
	}

	Version = "9.9.9"
	got, _, _ = Info()
	if got != "9.9.9" {
		t.Fatalf("version = %q, want unchanged %q", got, "9.9.9")
	}
}

func TestInfoModuleFallbackIsVLess(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	// Under `go test` the main module version is (devel), so the ldflags
	// default survives — but whatever comes back must never carry a v.
	Version = "dev"
	got, _, _ := Info()
	if strings.HasPrefix(got, "v") {
		t.Fatalf("displayed version %q must not carry a v prefix", got)
	}
}
