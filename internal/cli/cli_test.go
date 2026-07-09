package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"version"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute version: %v", err)
	}
	if got := out.String(); !strings.HasPrefix(got, "acm ") {
		t.Fatalf("version output = %q, want prefix %q", got, "acm ")
	}
}

func TestDoctorCommandReportsOK(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dbPath := filepath.Join(t.TempDir(), "acm.db")
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--db", dbPath, "doctor"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute doctor: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "status:         ok") {
		t.Fatalf("doctor output missing ok status:\n%s", got)
	}
	if !strings.Contains(got, dbPath) {
		t.Fatalf("doctor output missing db path %q:\n%s", dbPath, got)
	}
}
