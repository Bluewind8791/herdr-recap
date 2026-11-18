package main

import "testing"

func TestTheAgentsClaudeIsTheOneNearestTheShell(t *testing.T) {
	// The shell (11636) runs the agent's claude (21000), whose Bash tool ran a
	// `claude -p` (30628): the higher pid, but the agent's is wanted.
	procs := map[int]proc{
		11636: {1, "pwsh.exe"},
		21000: {11636, "claude.exe"},
		52000: {21000, "bash.exe"},
		30628: {52000, "claude.exe"},
		99999: {1, "claude.exe"}, // someone else's
	}
	if got := outerClaude(procs, 11636); got != 21000 {
		t.Errorf("got %d", got)
	}
	// Two as near: the lower pid, always.
	procs[20000] = proc{11636, "Claude.exe"}
	if got := outerClaude(procs, 11636); got != 20000 {
		t.Errorf("got %d", got)
	}
	// None, and parent ids that loop (reused pids): 0.
	if got := outerClaude(map[int]proc{5: {6, "claude.exe"}, 6: {5, "bash.exe"}}, 11636); got != 0 {
		t.Errorf("got %d", got)
	}
}
