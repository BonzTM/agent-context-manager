//go:build !windows

package summarize

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// configureCommand gives each model invocation its own process group so a
// deadline terminates descendants as well as the immediate agent CLI process.
func configureCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}
