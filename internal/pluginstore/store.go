// Package pluginstore keeps plugin artifacts on disk, keyed by their digest.
//
// A plugin is third-party code that runs when someone types "godo test", so
// the digest is not a cache key that happens to be a hash — it is the identity.
// Two artifacts with the same digest are the same artifact, and one whose bytes
// stop matching is not the artifact that was reviewed.
package pluginstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Store is a directory of artifacts named by digest.
type Store struct {
	// Dir defaults to <user cache>/godo/plugins.
	Dir string
	// HTTP defaults to http.DefaultClient.
	HTTP *http.Client
}

// Dirname returns the directory this store writes to.
func (s Store) Dirname() (string, error) {
	if s.Dir != "" {
		return s.Dir, nil
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "godo", "plugins"), nil
}

// Path is where an artifact with this digest lives.
//
// The digest becomes a filename, so it is validated rather than trusted: 64
// lowercase hex characters and nothing else, which cannot contain a separator
// or climb out of the directory.
func (s Store) Path(digest string) (string, error) {
	if !validDigest(digest) {
		return "", fmt.Errorf("not a sha256 digest: %q", digest)
	}
	dir, err := s.Dirname()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, digest+".wasm"), nil
}

// Has reports whether the artifact is already stored.
func (s Store) Has(digest string) bool {
	p, err := s.Path(digest)
	if err != nil {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// Fetch reads an artifact from source and stores it, returning its digest.
//
// It always reads: the digest is what source turns out to contain, and there is
// no way to know that without reading it. Ensure is the call that can skip.
//
// A relative source resolves against base. Callers pass the directory the
// source was written in — the catalog's for a declared plugin, the caller's own
// for one typed on a command line — so that "./x.wasm" means the same thing
// wherever godo is run from.
func (s Store) Fetch(ctx context.Context, base, source string) (digest, path string, err error) {
	data, err := s.read(ctx, base, source)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(data)
	digest = hex.EncodeToString(sum[:])
	path, err = s.write(digest, data)
	if err != nil {
		return "", "", err
	}
	return digest, path, nil
}

// Ensure returns the stored artifact for digest, fetching from source only if
// it is missing. A fetch that produces different bytes is refused: source is
// where an artifact comes from, digest is which artifact it must be.
func (s Store) Ensure(ctx context.Context, base, digest, source string) (string, error) {
	if s.Has(digest) {
		return s.Path(digest)
	}
	got, path, err := s.Fetch(ctx, base, source)
	if err != nil {
		return "", err
	}
	if got != digest {
		_ = os.Remove(path)
		return "", fmt.Errorf("%s: sha256 mismatch\n  declared %s\n  actual   %s", source, digest, got)
	}
	return path, nil
}

// Verify re-hashes a stored artifact, catching a cache that was corrupted or
// tampered with after it was written.
func (s Store) Verify(digest string) error {
	p, err := s.Path(digest)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != digest {
		return fmt.Errorf("%s: stored bytes no longer match\n  expected %s\n  actual   %s", p, digest, got)
	}
	return nil
}

// Add stores bytes whose digest the caller already computed.
func (s Store) Add(digest string, data []byte) (string, error) {
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != digest {
		return "", fmt.Errorf("sha256 mismatch\n  declared %s\n  actual   %s", digest, got)
	}
	return s.write(digest, data)
}

func (s Store) read(ctx context.Context, base, source string) ([]byte, error) {
	switch {
	case strings.HasPrefix(source, "http://"):
		// An artifact is code. Its integrity cannot rest on a transport that
		// anyone on the path can rewrite, digest or not: a wrong digest is a
		// failure you see, and a silently swapped download plus a swapped
		// catalog is not.
		return nil, fmt.Errorf("%s: refusing http; use https or a local path", source)
	case strings.HasPrefix(source, "https://"):
		client := s.HTTP
		if client == nil {
			client = http.DefaultClient
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", source, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s: %s", source, resp.Status)
		}
		return io.ReadAll(resp.Body)
	default:
		p := strings.TrimPrefix(source, "file://")
		if !filepath.IsAbs(p) && base != "" {
			p = filepath.Join(base, p)
		}
		return os.ReadFile(p)
	}
}

// write stores data atomically: a temp file in the same directory, then a
// rename. An interrupted fetch must not leave a partial file that a later Has
// would accept as the real artifact.
func (s Store) write(digest string, data []byte) (string, error) {
	p, err := s.Path(digest)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".partial-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), p); err != nil {
		return "", err
	}
	return p, nil
}

func validDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f':
		default:
			return false
		}
	}
	return true
}
