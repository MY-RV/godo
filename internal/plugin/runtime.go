package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// Runtime compiles and holds plugins for one godo invocation.
type Runtime struct {
	rt   wazero.Runtime
	once sync.Once
}

// NewRuntime returns a runtime with WASI available and nothing else.
func NewRuntime(ctx context.Context) *Runtime {
	rt := wazero.NewRuntime(ctx)
	wasi_snapshot_preview1.MustInstantiate(ctx, rt)
	return &Runtime{rt: rt}
}

// Close releases every compiled module.
func (r *Runtime) Close(ctx context.Context) error {
	var err error
	r.once.Do(func() { err = r.rt.Close(ctx) })
	return err
}

// Plugin is one verified, compiled artifact.
type Plugin struct {
	Source   string
	Provides []string
	Config   map[string]any

	rt       wazero.Runtime
	compiled wazero.CompiledModule
}

// Load reads an artifact, checks it against its digest, and compiles it.
//
// The digest is checked before the bytes reach a compiler, not after: a
// plugin is third-party code, and "this is the artifact that was reviewed" is
// the one thing a catalog can actually assert about it.
//
// source is resolved relative to root when it is not absolute. Only local
// paths are understood today; fetching is a separate concern and belongs with
// a lockfile, not here.
func (r *Runtime) Load(ctx context.Context, source, digest, root string, provides []string, config map[string]any) (*Plugin, error) {
	path, err := localPath(source, root)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("plugin %s: %w", source, err)
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, strings.TrimSpace(digest)) {
		return nil, fmt.Errorf("plugin %s: sha256 mismatch\n  declared %s\n  actual   %s", source, digest, got)
	}
	compiled, err := r.rt.CompileModule(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("plugin %s: not a usable wasm module: %w", source, err)
	}
	return &Plugin{
		Source:   source,
		Provides: provides,
		Config:   config,
		rt:       r.rt,
		compiled: compiled,
	}, nil
}

// localPath resolves a source that names a file on this machine.
func localPath(source, root string) (string, error) {
	p := strings.TrimPrefix(source, "file://")
	if p == source && strings.Contains(source, "://") {
		return "", fmt.Errorf("plugin %s: only local paths are supported in this build", source)
	}
	if filepath.IsAbs(p) {
		return p, nil
	}
	return filepath.Join(root, p), nil
}

// Digest returns the sha256 of a file, for writing into a catalog.
func Digest(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
