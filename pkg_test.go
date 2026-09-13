package godo_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/my-rv/godo"
)

func TestFacade_LoadAndPreview(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, godo.FileName)
	body := "version: \"0.1\"\ndialect: package\nscripts:\n  hi: echo hi\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cat, err := godo.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines, err := godo.NewEngine(cat, nil).PreviewLines([]string{"hi"})
	if err != nil || len(lines) != 1 || lines[0] != "echo hi" {
		t.Fatalf("%v %v", lines, err)
	}
}
