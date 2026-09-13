package catalog

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindFile walks from start (or cwd) up to root looking for godo.yaml.
func FindFile(start string) (string, error) {
	dir := start
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = cwd
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, FileName)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%s not found from %s", FileName, start)
		}
		dir = parent
	}
}
