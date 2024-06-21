//go:build !windows

package backend

import "os/exec"

func sysProcAttr(cmd *exec.Cmd) {
	// do nothing
}
