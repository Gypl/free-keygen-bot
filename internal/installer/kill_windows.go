//go:build windows

package installer

import "os/exec"

// killProcessGroup is a no-op on Windows.
// The creack/pty package does not support Windows, so this code path
// is only needed for cross-compilation. The individual process kill
// via cmd.Process.Kill() still works.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
