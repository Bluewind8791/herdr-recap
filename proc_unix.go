//go:build !windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// setProcessGroup puts cmd in a process group of its own, so that it and
// whatever it starts can be ended together (killCommandGroup).
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killCommandGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

// startDetached starts cmd in a session of its own, so it outlives its asker.
func startDetached(cmd *exec.Cmd) (*exec.Cmd, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd, cmd.Start()
}

// tryLock takes an exclusive lock on f without waiting: busy is true when
// another process holds it.
func tryLock(f *os.File) (busy bool, err error) {
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
		return true, nil
	}
	return false, err
}

func unlock(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

// replaceFile renames from over to.
func replaceFile(from, to string) error { return os.Rename(from, to) }

// claudeProcess is the Claude process for pid: on Unix, the pid the app
// gives is Claude's own.
func claudeProcess(pid int, argv0 string) (int, string) { return pid, argv0 }
