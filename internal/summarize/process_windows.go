//go:build windows

package summarize

import "os/exec"

// configureCommand keeps CommandContext's default process termination on
// Windows; WaitDelay still bounds inherited output pipes.
func configureCommand(_ *exec.Cmd) {}
