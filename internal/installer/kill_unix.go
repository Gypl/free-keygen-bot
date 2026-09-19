//go:build !windows

package installer

import (
	"os/exec"
	"syscall"
)

// killProcessGroup sends SIGKILL to the entire process group.
// On Unix, pty.Start sets Setsid: true, making the process a session leader
// with PGID == PID. All child processes (curl, bash, podman) inherit the same
// PGID, so killing -PID terminates the whole tree.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
