package expand_test

import (
	"strings"
	"testing"

	"github.com/my-rv/godo/internal/expand"
)

func TestExpand_argsAndCaptures(t *testing.T) {
	got, err := expand.Expand("go test ./${godo:argv[MODULE]}/... ${godo:args}", map[string]string{"MODULE": "pay"}, []string{"-count=1"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "go test ./pay/... -count=1" {
		t.Fatalf("%q", got)
	}
	got, err = expand.Expand("x ${godo:args[0]} ${godo:args[1..3]}", nil, []string{"a", "b", "c", "d"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "x a b c" {
		t.Fatalf("%q", got)
	}
}

func TestExpand_oobIndexErrors(t *testing.T) {
	_, err := expand.Expand("${godo:args[2]}", nil, []string{"a"})
	if err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("%v", err)
	}
}

func TestExpand_invalidCaptureFailClosed(t *testing.T) {
	_, err := expand.Expand("echo ${godo:argv[foo-bar]}", map[string]string{"foo-bar": "x"}, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid capture") {
		t.Fatalf("%v", err)
	}
	_, err = expand.Expand("echo ${godo:argv[MISSING]}", map[string]string{}, nil)
	if err == nil || !strings.Contains(err.Error(), "unknown capture") {
		t.Fatalf("%v", err)
	}
}

// --- the namespace split: bodies are shell space ---

func TestExpand_bareBracesBelongToTheShell(t *testing.T) {
	// godo claims ${godo:…} and nothing else, so every one of these reaches the
	// shell byte for byte. No escape, no error, no capture lookup.
	for _, in := range []string{
		"echo ${HOME}",
		"echo $HOME",
		"echo $$",
		"echo ${MODULE}",
		`echo ${VAR:-default}`,
		`echo ${#arr[@]}`,
	} {
		got, err := expand.Expand(in, map[string]string{"MODULE": "pay"}, nil)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if got != in {
			t.Errorf("%q became %q", in, got)
		}
	}
}

func TestExpand_argvIndexesCapturesByName(t *testing.T) {
	_, err := expand.Expand("echo ${godo:argv[0]}", nil, []string{"a"})
	if err == nil || !strings.Contains(err.Error(), "${godo:args[0]}") {
		t.Fatalf("want a pointer to args[0], got %v", err)
	}
	_, err = expand.Expand("echo ${godo:argv}", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "argv[NAME]") {
		t.Fatalf("%v", err)
	}
}

func TestExpand_unterminatedPlaceholder(t *testing.T) {
	_, err := expand.Expand("echo ${godo:oops", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "unterminated") {
		t.Fatalf("%v", err)
	}
}

// --- quoting ---

func TestExpand_argsAreOneArgumentEach(t *testing.T) {
	got, err := expand.Expand("echo hello ${godo:args}", nil, []string{"a; touch PWNED"})
	if err != nil {
		t.Fatal(err)
	}
	if got != `echo hello 'a; touch PWNED'` {
		t.Fatalf("%q", got)
	}
	got, err = expand.Expand("echo ${godo:args[0]}", nil, []string{"it's"})
	if err != nil {
		t.Fatal(err)
	}
	if got != `echo 'it'\''s'` {
		t.Fatalf("%q", got)
	}
}

func TestExpand_plainTokensStayUnquoted(t *testing.T) {
	// Previews have to stay readable, so ordinary flags and paths are not quoted.
	got, err := expand.Expand("go test ${godo:args}", nil, []string{"-v", "./...", "-run=TestX"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "go test -v ./... -run=TestX" {
		t.Fatalf("%q", got)
	}
}

func TestExpand_capturesAreQuotedToo(t *testing.T) {
	got, err := expand.Expand("deploy ${godo:argv[env]}", map[string]string{"env": "a; rm -rf /"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != `deploy 'a; rm -rf /'` {
		t.Fatalf("%q", got)
	}
}

func TestExpand_rawOptsOutOfQuoting(t *testing.T) {
	got, err := expand.Expand("ls ${godo:args:raw}", nil, []string{"*.go"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "ls *.go" {
		t.Fatalf("%q", got)
	}
	got, err = expand.Expand("cd ${godo:argv[dir]:raw}", map[string]string{"dir": "$HOME/x"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "cd $HOME/x" {
		t.Fatalf("%q", got)
	}
}

func TestCommandsNeedArgs(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"echo hi", false},
		{"echo ${godo:args}", true},
		{"echo ${godo:args:raw}", true},
		{"echo ${godo:args[0]}", true},
		{"echo ${godo:args[0..2]:raw}", true},
		{"echo ${godo:argv[NAME]}", false},
		{"echo ${args}", false}, // bare braces are the shell's
	} {
		if got := expand.CommandsNeedArgs([]string{tc.in}); got != tc.want {
			t.Errorf("%q: got %v want %v", tc.in, got, tc.want)
		}
	}
}

// --- @deps is godo space ---

func TestExpandInvocation_bareCapturesAndTokens(t *testing.T) {
	got, err := expand.ExpandInvocation("lint ${MODULE}", map[string]string{"MODULE": "pay"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "|") != "lint|pay" {
		t.Fatalf("%q", got)
	}
}

func TestExpandInvocation_captureWithSpaceStaysOneToken(t *testing.T) {
	// No shell is involved, so the value is not quoted — and because the entry
	// resolves to tokens instead of a line, a space cannot fragment it.
	got, err := expand.ExpandInvocation("lint ${MODULE}", map[string]string{"MODULE": "my module"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1] != "my module" {
		t.Fatalf("%q", got)
	}
}

func TestExpandInvocation_rejectsGodoNamespace(t *testing.T) {
	_, err := expand.ExpandInvocation("lint ${godo:args}", nil)
	if err == nil || !strings.Contains(err.Error(), "not available in @deps") {
		t.Fatalf("%v", err)
	}
	_, err = expand.ExpandInvocation("lint ${godo:argv[M]}", map[string]string{"M": "x"})
	if err == nil || !strings.Contains(err.Error(), "not available in @deps") {
		t.Fatalf("%v", err)
	}
}

func TestExpandInvocation_unknownCaptureFailsClosed(t *testing.T) {
	_, err := expand.ExpandInvocation("lint ${NOPE}", map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "unknown capture") {
		t.Fatalf("%v", err)
	}
}
