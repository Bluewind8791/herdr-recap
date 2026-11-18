//go:build !windows

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestBoundedCommandKillsDescendantsOnOverflow(t *testing.T) {
	assertBoundedCommandKillsDescendant(t, true)
}

func TestBoundedCommandKillsDescendantsOnCancellation(t *testing.T) {
	assertBoundedCommandKillsDescendant(t, false)
}

func assertBoundedCommandKillsDescendant(t *testing.T, overflow bool) {
	t.Helper()
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	script := `sleep 30 & child=$!; printf %s "$child" > "$1"; `
	if overflow {
		script += `while :; do printf '%032768d' 0; done`
	} else {
		script += `wait "$child"`
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", script, "sh", pidFile)
	_, _, err := runBoundedCommand(cmd)
	if overflow && !errors.Is(err, errOutputLimit) {
		t.Fatalf("got %v", err)
	}
	if !overflow && err == nil {
		t.Fatal("canceled command succeeded")
	}
	data, readErr := os.ReadFile(pidFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	pid, convErr := strconv.Atoi(string(data))
	if convErr != nil {
		t.Fatal(convErr)
	}
	for range 50 {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	t.Fatalf("descendant %d survived command cleanup", pid)
}
