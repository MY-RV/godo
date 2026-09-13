// Package update checks GitHub Releases and can replace the running binary.
//
// Until github.com/my-rv/godo publishes releases, Latest fails with a clear error
// unless APIBase is overridden (tests / future mirrors).
package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DefaultAPIBase is the GitHub API root for this project when it has its own repo.
const DefaultAPIBase = "https://api.github.com/repos/my-rv/godo"

// Client talks to a GitHub-like releases API.
type Client struct {
	HTTP    *http.Client
	APIBase string // e.g. DefaultAPIBase; override in tests
	GOOS    string
	GOARCH  string
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c *Client) apiBase() string {
	if c.APIBase != "" {
		return strings.TrimRight(c.APIBase, "/")
	}
	if v := strings.TrimSpace(os.Getenv("GODO_RELEASES_API")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return DefaultAPIBase
}

func (c *Client) goos() string {
	if c.GOOS != "" {
		return c.GOOS
	}
	return runtime.GOOS
}

func (c *Client) goarch() string {
	if c.GOARCH != "" {
		return c.GOARCH
	}
	return runtime.GOARCH
}

// Release is a trimmed GitHub release payload.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Asset is a release binary.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Latest fetches /releases/latest and picks the asset for this OS/arch.
func (c *Client) Latest() (tag, downloadURL string, err error) {
	url := c.apiBase() + "/releases/latest"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := c.http().Do(req)
	if err != nil {
		return "", "", fmt.Errorf("check releases: %w (repo may not be published yet)", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode == http.StatusNotFound {
		return "", "", fmt.Errorf("no GitHub releases at %s (publish the repo + a release, or set GODO_RELEASES_API)", c.apiBase())
	}
	if res.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("releases API %s: %s", res.Status, truncate(body, 200))
	}
	var rel Release
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", "", err
	}
	if rel.TagName == "" {
		return "", "", fmt.Errorf("latest release has no tag_name")
	}
	want := assetName(rel.TagName, c.goos(), c.goarch())
	for _, a := range rel.Assets {
		if a.Name == want || matchAsset(a.Name, c.goos(), c.goarch()) {
			return rel.TagName, a.BrowserDownloadURL, nil
		}
	}
	return rel.TagName, "", fmt.Errorf("release %s has no asset for %s/%s (want %q)", rel.TagName, c.goos(), c.goarch(), want)
}

// assetName is the convention used by scripts/release-local.sh.
func assetName(tag, goos, goarch string) string {
	tag = strings.TrimPrefix(tag, "v")
	return fmt.Sprintf("godo_%s_%s_%s", tag, goos, goarch)
}

func matchAsset(name, goos, goarch string) bool {
	n := strings.ToLower(name)
	if strings.Contains(n, ".tar.gz") || strings.Contains(n, ".zip") || strings.HasSuffix(n, ".txt") {
		return false
	}
	return strings.Contains(n, goos) && strings.Contains(n, goarch) && strings.Contains(n, "godo")
}

// Newer reports whether latest is a newer semver than current (v-prefix optional).
func Newer(current, latest string) (bool, error) {
	c, err := parseSemver(current)
	if err != nil {
		return false, fmt.Errorf("current version %q: %w", current, err)
	}
	l, err := parseSemver(latest)
	if err != nil {
		return false, fmt.Errorf("latest version %q: %w", latest, err)
	}
	return cmpSemver(l, c) > 0, nil
}

type semver struct{ major, minor, patch int }

func parseSemver(s string) (semver, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i] // drop -dev / +build
	}
	parts := strings.Split(s, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return semver{}, fmt.Errorf("want major.minor.patch")
	}
	var v semver
	var err error
	if v.major, err = atoi(parts[0]); err != nil {
		return semver{}, err
	}
	if len(parts) > 1 {
		if v.minor, err = atoi(parts[1]); err != nil {
			return semver{}, err
		}
	}
	if len(parts) > 2 {
		if v.patch, err = atoi(parts[2]); err != nil {
			return semver{}, err
		}
	}
	return v, nil
}

func atoi(s string) (int, error) {
	n := 0
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a number %q", s)
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

func cmpSemver(a, b semver) int {
	if a.major != b.major {
		return a.major - b.major
	}
	if a.minor != b.minor {
		return a.minor - b.minor
	}
	return a.patch - b.patch
}

// Download replaces destPath with the file at url (atomic rename when possible).
func Download(httpClient *http.Client, url, destPath string) error {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	res, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, res.Status)
	}
	dir := filepath.Dir(destPath)
	tmp, err := os.CreateTemp(dir, ".godo-update-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := io.Copy(tmp, res.Body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, destPath)
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
