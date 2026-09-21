package execshell

import "testing"

func TestShellAncestor(t *testing.T) {
	// explorer → WindowsTerminal → powershell → godo-shim → godo
	table := map[uint32]proc{
		4:   {parent: 0, name: "System"},
		100: {parent: 4, name: "explorer.exe"},
		200: {parent: 100, name: "WindowsTerminal.exe"},
		300: {parent: 200, name: "powershell.exe"},
		400: {parent: 300, name: "godo.exe"}, // the scoop shim
		500: {parent: 400, name: "godo.exe"}, // godo itself
	}

	t.Run("through a package manager shim", func(t *testing.T) {
		// The bug this fixes: reading only the parent found godo.exe, decided
		// no shell was there, and reported cmd.exe to a PowerShell user.
		if got := shellAncestor(table, 500); got != "powershell.exe" {
			t.Fatalf("got %q, want powershell.exe", got)
		}
	})

	t.Run("directly from the shell", func(t *testing.T) {
		if got := shellAncestor(table, 400); got != "powershell.exe" {
			t.Fatalf("got %q, want powershell.exe", got)
		}
	})

	t.Run("the nearest shell wins", func(t *testing.T) {
		// A cmd started from PowerShell is the shell you are in.
		table := map[uint32]proc{
			300: {parent: 0, name: "powershell.exe"},
			310: {parent: 300, name: "cmd.exe"},
			320: {parent: 310, name: "godo.exe"},
		}
		if got := shellAncestor(table, 320); got != "cmd.exe" {
			t.Fatalf("got %q, want cmd.exe", got)
		}
	})

	t.Run("no shell up there", func(t *testing.T) {
		table := map[uint32]proc{
			4:   {parent: 0, name: "System"},
			600: {parent: 4, name: "services.exe"},
			610: {parent: 600, name: "godo.exe"},
		}
		if got := shellAncestor(table, 610); got != "" {
			t.Fatalf("got %q, want the caller to fall back", got)
		}
	})

	t.Run("a pid missing from the table", func(t *testing.T) {
		if got := shellAncestor(table, 9999); got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})
}

// Windows reuses pids, so a table can point a process at a "parent" that is
// really its own descendant. The walk has to end anyway.
func TestShellAncestorTerminatesOnACycle(t *testing.T) {
	table := map[uint32]proc{
		10: {parent: 20, name: "a.exe"},
		20: {parent: 10, name: "b.exe"},
	}
	if got := shellAncestor(table, 10); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

// A chain of wrappers deeper than the bound is not worth walking, but it must
// not hang either.
func TestShellAncestorStopsAtTheBound(t *testing.T) {
	table := map[uint32]proc{1: {parent: 0, name: "powershell.exe"}}
	var pid uint32 = 1
	for i := 0; i < maxAncestry+5; i++ {
		next := pid + 1
		table[next] = proc{parent: pid, name: "wrapper.exe"}
		pid = next
	}
	if got := shellAncestor(table, pid); got != "" {
		t.Fatalf("got %q, want empty past the bound", got)
	}
}
