package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// readProcessEnv reads a process's environment from its PEB: readable for
// your own processes.
func readProcessEnv(pid int) (map[string]string, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_VM_READ, false, uint32(pid))
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(h)

	var pbi windows.PROCESS_BASIC_INFORMATION
	if err := windows.NtQueryInformationProcess(h, windows.ProcessBasicInformation, unsafe.Pointer(&pbi), uint32(unsafe.Sizeof(pbi)), nil); err != nil {
		return nil, err
	}
	var peb windows.PEB
	if err := readMem(h, uintptr(unsafe.Pointer(pbi.PebBaseAddress)), unsafe.Pointer(&peb), unsafe.Sizeof(peb)); err != nil {
		return nil, err
	}
	var params windows.RTL_USER_PROCESS_PARAMETERS
	if err := readMem(h, uintptr(unsafe.Pointer(peb.ProcessParameters)), unsafe.Pointer(&params), unsafe.Sizeof(params)); err != nil {
		return nil, err
	}
	size := params.EnvironmentSize
	if size == 0 || size > 1<<20 {
		size = 64 << 10
	}
	buf := make([]uint16, size/2)
	var n uintptr
	err = windows.ReadProcessMemory(h, uintptr(params.Environment), (*byte)(unsafe.Pointer(&buf[0])), uintptr(len(buf))*2, &n)
	if err != nil && n == 0 {
		return nil, err
	}
	buf = buf[:n/2]

	env := map[string]string{}
	for len(buf) > 0 {
		end := 0
		for end < len(buf) && buf[end] != 0 {
			end++
		}
		if end == 0 {
			break
		}
		kv := string(utf16.Decode(buf[:end]))
		// Entries such as "=C:=C:\dir" start with "=": not variables.
		if k, v, ok := strings.Cut(kv[1:], "="); ok {
			env[strings.ToUpper(kv[:1]+k)] = v
		}
		if end+1 >= len(buf) {
			break
		}
		buf = buf[end+1:]
	}
	// Names are upper-cased: Windows' are case-insensitive, and the callers
	// ask for PATH, CLAUDE_CONFIG_DIR and CODEX_HOME.
	return env, nil
}

func readMem(h windows.Handle, addr uintptr, dst unsafe.Pointer, size uintptr) error {
	var n uintptr
	return windows.ReadProcessMemory(h, addr, (*byte)(dst), size, &n)
}

// processExe is the program a process runs.
func processExe(pid int) (string, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf[:size]), nil
}

// openFiles: Windows has no cheap way to list another process's open files,
// so Codex sessions (found by them) aren't recapped here.
func openFiles(pid int) []string { return nil }

// detached: no systemd; spawn starts its copy itself.
func detached(self string, args []string, logPath string) *exec.Cmd { return nil }

// claudeProcess is the Claude process for pid. On Windows herdr-core reports
// the terminal's shell as its foreground process, not the claude it runs, so
// it's the claude.exe among pid and its descendants (the highest pid, should
// there be more); its path is what the recap runs. Without one, pid and argv0
// as given.
func claudeProcess(pid int, argv0 string) (int, string) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return pid, argv0
	}
	defer windows.CloseHandle(snap)
	type proc struct {
		parent int
		name   string
	}
	procs := map[int]proc{}
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		procs[int(e.ProcessID)] = proc{int(e.ParentProcessID), windows.UTF16ToString(e.ExeFile[:])}
	}
	// Descends from pid: its parent chain reaches pid (bounded, as parent ids
	// are reused on Windows and can loop).
	descends := func(p int) bool {
		for range 32 {
			if p == pid {
				return true
			}
			q, ok := procs[p]
			if !ok || q.parent == p || q.parent == 0 {
				return false
			}
			p = q.parent
		}
		return false
	}
	found := 0
	for p, q := range procs {
		if strings.EqualFold(q.name, "claude.exe") && descends(p) && p > found {
			found = p
		}
	}
	if found == 0 {
		return pid, argv0
	}
	if exe, err := processExe(found); err == nil {
		return found, filepath.Clean(exe)
	}
	return found, argv0
}
