//go:build !windows

package cli

import (
	"os/exec"
	"syscall"
)

// detachProcess gives the command its own session, so it is not signalled
// along with tap's process group and keeps running after tap exits.
func detachProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
