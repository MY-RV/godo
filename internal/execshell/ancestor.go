package execshell

// proc is one row of the process table: who started it, and what it is.
type proc struct {
	parent uint32
	name   string
}

// maxAncestry bounds the walk. A shell is a handful of hops away at most; a
// longer chain means the answer is not up there, and the bound is also what
// keeps a corrupt table from spinning.
const maxAncestry = 16

// shellAncestor returns the nearest ancestor of pid that is a shell godo
// knows, or "".
//
// It walks rather than reading the parent alone because something is usually
// in between. A package manager's shim is the common one: scoop installs
// godo as shims\godo.exe, which starts the real godo.exe as a child, so the
// parent of the process asking this question is *godo* — and godo is not a
// shell, so reading one level up answered "no shell here" and fell back to
// %ComSpec%, telling every PowerShell user they were in cmd. The same shape
// appears with npm, bun and make wrappers, and inside an editor's terminal.
//
// Walking up is a wider question than "who started me" — it is "what shell am
// I under". That is the question the runner is trying to answer, and it is
// the same looseness $SHELL already has on Unix, where the login shell answers
// even when another shell is running right now.
//
// A pid is visited once. Windows reuses pids, so a table can point a process
// at a "parent" that is really its own descendant, and a cycle would otherwise
// never end.
func shellAncestor(table map[uint32]proc, pid uint32) string {
	seen := make(map[uint32]bool, maxAncestry)
	for i := 0; i < maxAncestry; i++ {
		p, ok := table[pid]
		if !ok || p.parent == 0 || seen[p.parent] {
			return ""
		}
		seen[pid] = true
		parent, ok := table[p.parent]
		if !ok {
			return ""
		}
		if isKnownShell(shellBase(parent.name)) {
			return parent.name
		}
		pid = p.parent
	}
	return ""
}
