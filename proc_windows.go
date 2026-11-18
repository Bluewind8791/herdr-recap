//go:build windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"

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

// replaceFile renames from over to. Windows refuses while another process
// has that file open (the mod reads recaps every few seconds), so it's tried
// again for about a second before giving up.
func replaceFile(from, to string) error {
	err := os.Rename(from, to)
	for range 10 {
		if !errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return err
		}
		time.Sleep(100 * time.Millisecond)
		err = os.Rename(from, to)
	}
	return err
}
