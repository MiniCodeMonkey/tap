//go:build windows

package cli

import "os/exec"

func detachProcess(*exec.Cmd) {}
