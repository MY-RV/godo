package pluginstore_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/my-rv/godo/internal/pluginstore"
)

func digestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestFetch_fromALocalFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.wasm")
	data := []byte("artifact bytes")
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatal(err)
	}
	s := pluginstore.Store{Dir: filepath.Join(dir, "store")}

	got, path, err := s.Fetch(context.Background(), "", src)
	if err != nil {
		t.Fatal(err)
	}
	if got != digestOf(data) {
		t.Fatalf("digest=%s", got)
	}
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != string(data) {
		t.Fatalf("stored %q", stored)
	}
	if !s.Has(got) {
		t.Fatal("Has says no after a Fetch")
	}
}

func TestEnsure_downloadsOnceAndReusesAfter(t *testing.T) {
	data := []byte("artifact over the wire")
	var hits int
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write(data)
	}))
	defer srv.Close()

	s := pluginstore.Store{Dir: t.TempDir(), HTTP: srv.Client()}
	want := digestOf(data)

	for i := 0; i < 3; i++ {
		if _, err := s.Ensure(context.Background(), "", want, srv.URL); err != nil {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	if hits != 1 {
		t.Fatalf("hit the server %d times, want 1", hits)
	}
}

// The source says where to look; the digest says what must come back.
func TestEnsure_refusesBytesThatDoNotMatch(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("something else entirely"))
	}))
	defer srv.Close()

	s := pluginstore.Store{Dir: t.TempDir(), HTTP: srv.Client()}
	want := digestOf([]byte("what the catalog declared"))

	_, err := s.Ensure(context.Background(), "", want, srv.URL)
	if err == nil {
		t.Fatal("stored an artifact the catalog did not declare")
	}
	if !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("err=%v", err)
	}
	if s.Has(want) {
		t.Fatal("the mismatched download was left in the store")
	}
}

// An artifact is code; its integrity cannot rest on a transport anyone can
// rewrite.
func TestFetch_refusesPlainHTTP(t *testing.T) {
	s := pluginstore.Store{Dir: t.TempDir()}
	_, _, err := s.Fetch(context.Background(), "", "http://example.test/a.wasm")
	if err == nil || !strings.Contains(err.Error(), "refusing http") {
		t.Fatalf("err=%v", err)
	}
}

// The digest becomes a filename, so it is validated rather than trusted.
func TestPath_rejectsAnythingThatIsNotADigest(t *testing.T) {
	s := pluginstore.Store{Dir: t.TempDir()}
	for _, bad := range []string{
		"", "short", strings.Repeat("g", 64),
		"../" + strings.Repeat("a", 61),
		strings.Repeat("a", 32) + "/" + strings.Repeat("b", 31),
		strings.ToUpper(strings.Repeat("a", 64)),
		strings.Repeat("a", 65),
	} {
		if _, err := s.Path(bad); err == nil {
			t.Fatalf("accepted %q as a digest", bad)
		}
	}
}

func TestVerify_catchesAStoreChangedAfterwards(t *testing.T) {
	dir := t.TempDir()
	s := pluginstore.Store{Dir: dir}
	data := []byte("original")
	d := digestOf(data)
	path, err := s.Add(d, data)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Verify(d); err != nil {
		t.Fatalf("fresh store does not verify: %v", err)
	}
	if err := os.WriteFile(path, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Verify(d); err == nil {
		t.Fatal("Verify accepted bytes that changed")
	}
}

func TestAdd_refusesBytesThatDoNotHashToTheDigest(t *testing.T) {
	s := pluginstore.Store{Dir: t.TempDir()}
	if _, err := s.Add(digestOf([]byte("a")), []byte("b")); err == nil {
		t.Fatal("stored bytes under someone else's digest")
	}
}

// An interrupted write must not leave something a later Has would accept.
func TestWrite_leavesNoPartialFileBehind(t *testing.T) {
	dir := t.TempDir()
	s := pluginstore.Store{Dir: dir}
	data := []byte("complete")
	d := digestOf(data)
	if _, err := s.Add(d, data); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".partial") {
			t.Fatalf("left a temp file: %s", e.Name())
		}
	}
}
