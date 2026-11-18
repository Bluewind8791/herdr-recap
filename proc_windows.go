//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// setProcessGroup: Windows has no process groups to kill by; killCommandGroup
// ends the process itself.
func setProcessGroup(cmd *exec.Cmd) {}

func killCommandGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	return cmd.Process.Kill()
}

// startDetached starts cmd without a window and in a process group of its
// own, out of the asker's job so it outlives the asker; inside the job when
// the job forbids breaking away.
func startDetached(cmd *exec.Cmd) (*exec.Cmd, error) {
	flags := uint32(windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP)
	retry := exec.Command(cmd.Path, cmd.Args[1:]...)
	retry.Dir, retry.Env, retry.Stdout, retry.Stderr = cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: flags | windows.CREATE_BREAKAWAY_FROM_JOB, HideWindow: true}
	if err := cmd.Start(); err == nil {
		return cmd, nil
	}
	retry.SysProcAttr = &syscall.SysProcAttr{CreationFlags: flags, HideWindow: true}
	return retry, retry.Start()
}

// tryLock takes an exclusive lock on f without waiting: busy is true when
// another process holds it.
func tryLock(f *os.File) (busy bool, err error) {
	ol := new(windows.Overlapped)
	err = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol)
	if err == windows.ERROR_LOCK_VIOLATION || err == windows.ERROR_IO_PENDING {
		return true, nil
	}
	return false, err
}

func unlock(f *os.File) {
	_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, new(windows.Overlapped))
}
