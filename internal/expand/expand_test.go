package expand_test

import (
	"strings"
	"testing"

	"github.com/my-rv/godo/internal/expand"
)

func TestExpand_argsAndCaptures(t *testing.T) {
	got, err := expand.Expand("go test ./${MODULE}/... ${godo:args}", map[string]string{"MODULE": "pay"}, []string{"-count=1"})
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
	_, err := expand.Expand("echo ${foo-bar}", map[string]string{"foo-bar": "x"}, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid capture") {
		t.Fatalf("%v", err)
	}
	_, err = expand.Expand("echo ${MISSING}", map[string]string{}, nil)
	if err == nil || !strings.Contains(err.Error(), "unknown capture") {
		t.Fatalf("%v", err)
	}
}
