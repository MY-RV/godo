package godo

import "testing"

func TestTaggedVersion(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"v0.3.0", true},
		{"v0.3.0-preview.1", true},
		{"", false},
		{"(devel)", false},
		// What `go build` in a working tree records.
		{"v0.2.1-0.20260921040944-a467864ac83c+dirty", false},
		// The same without the dirty marker: still nothing anyone tagged.
		{"v0.2.1-0.20260921040944-a467864ac83c", false},
		{"v2.0.0+incompatible", false},
	}
	for _, c := range cases {
		if got := taggedVersion(c.in); got != c.want {
			t.Errorf("taggedVersion(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// A stamped build wins over anything the build info says: the release is the
// authority on what it released.
func TestReleasePrefersStampedVersion(t *testing.T) {
	old := Version
	defer func() { Version = old }()

	Version = "v0.3.0"
	if got := Release(); got != "v0.3.0" {
		t.Fatalf("Release() = %q, want the stamped version", got)
	}
}

// Unstamped, in a test binary, there is no tag to find, so the default stands
// rather than a pseudo-version pretending to be a release.
func TestReleaseFallsBackToDefault(t *testing.T) {
	if got := Release(); got != devVersion {
		t.Fatalf("Release() = %q, want %q", got, devVersion)
	}
}
