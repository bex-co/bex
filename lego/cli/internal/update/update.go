/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package update owns bex's release identity and update discovery. The
// upstream Render CLI's own update check compares against render-oss/cli
// releases behind the const cfg.RepoURL, which cannot be repointed at build
// time — so the launcher intercepts the version path before upstream code
// runs (IsRootVersionRequest) and performs its own check against this repo's
// `bex-cli/v*` GitHub releases, gh-style: passive, cached, and silent on any
// failure.
package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/spf13/pflag"
)

// TagPrefix is the release-tag namespace of the bex CLI inside the bex
// monorepo; /releases/latest is unusable because other components release
// from the same repository.
const TagPrefix = "bex-cli/v"

const (
	// defaultAPIBase is the GitHub API origin; NewChecker lets
	// BEX_UPDATE_API_URL override it for tests.
	defaultAPIBase = "https://api.github.com"
	apiBaseEnv     = "BEX_UPDATE_API_URL"
	repo           = "bex-co/bex"

	cacheMaxAge = 24 * time.Hour
)

// httpClient is deliberately short-fused: the explicit `bex -v` path blocks
// on it, so a bad network costs at most this once per cache window.
var httpClient = &http.Client{Timeout: 3 * time.Second}

// IsRootVersionRequest mirrors the upstream Render CLI's unexported detector
// byte for byte: --version / -v counts only before the first subcommand
// token, and values consumed by global flags (e.g. `-o text`) are skipped.
// Diverging here would change which invocations upstream handles.
func IsRootVersionRequest(args []string, rootFlags *pflag.FlagSet) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--version" || arg == "-v" || strings.HasPrefix(arg, "--version=") {
			return true
		}
		if !strings.HasPrefix(arg, "-") || arg == "--" {
			return false
		}
		if strings.Contains(arg, "=") {
			continue
		}

		var flag *pflag.Flag
		if strings.HasPrefix(arg, "--") {
			flag = rootFlags.Lookup(strings.TrimPrefix(arg, "--"))
		} else {
			short := strings.TrimPrefix(arg, "-")
			if len(short) != 1 {
				return false
			}
			flag = rootFlags.ShorthandLookup(short)
		}
		if flag == nil {
			return false
		}
		if flag.Value.Type() != "bool" && flag.NoOptDefVal == "" {
			i++
		}
	}
	return false
}

// IsReleaseBuild reports whether version is a real release identity rather
// than a local/dev build. A "dev" (or empty) build has nothing to compare or
// upgrade to, so both the passive notice and `bex upgrade` gate on this.
func IsReleaseBuild(version string) bool {
	return version != "" && version != "dev"
}

// Allowed reports whether an update check may run at all. A non-nil isTTY
// adds the interactive gate used by the post-command notice; the explicit
// version path passes nil (the user asked).
func Allowed(version string, lookup func(string) (string, bool), isTTY func() bool) bool {
	if !IsReleaseBuild(version) {
		return false
	}
	if v, _ := lookup("BEX_NO_UPDATE_NOTIFIER"); v != "" {
		return false
	}
	if v, _ := lookup("CI"); v != "" {
		return false
	}
	if isTTY != nil && !isTTY() {
		return false
	}
	return true
}

// A Release is the newest published bex-cli release.
type Release struct {
	Version string `json:"version"`
	URL     string `json:"url"`
}

// An Asset is a file attached to a GitHub release (the per-target archive,
// checksums.txt, or the cosign signature bundle).
type Asset struct {
	Name string
	URL  string
}

// A FullRelease carries a release's downloadable assets. The passive notice
// only needs Release's version+URL, but the explicit `bex upgrade` path needs
// the asset download URLs, so it queries this uncached.
type FullRelease struct {
	Version string
	Tag     string
	URL     string
	Assets  []Asset
}

// Asset returns the named asset's download URL and whether it was found.
func (r FullRelease) Asset(name string) (string, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a.URL, true
		}
	}
	return "", false
}

// LatestRelease returns the newest matching release with its assets, always
// from the network — the explicit upgrade path must see fresh asset URLs, not
// the passive notice's day-old cache.
func (c *Checker) LatestRelease() (FullRelease, error) {
	releases, err := c.fetchReleases()
	if err != nil {
		return FullRelease{}, err
	}
	best, ok := newest(releases)
	if !ok {
		return FullRelease{}, fmt.Errorf("no %s* release found", TagPrefix)
	}
	return best, nil
}

// Checker finds the newest `bex-cli/v*` release, remembering the answer —
// including a failed or empty one — for cacheMaxAge so at most one network
// call happens per day.
type Checker struct {
	APIBase   string
	Repo      string // "owner/name"
	CachePath string // "" disables the cache
	Now       func() time.Time
}

// NewChecker assembles the production checker: bex's release channel, the
// test-only BEX_UPDATE_API_URL override, and the on-disk cache (dropped when
// the home directory is unresolvable).
func NewChecker(lookup func(string) (string, bool)) *Checker {
	apiBase := defaultAPIBase
	if v, _ := lookup(apiBaseEnv); v != "" {
		apiBase = v
	}
	cachePath := ""
	if home, err := os.UserHomeDir(); err == nil {
		cachePath = filepath.Join(home, ".bex", "cache", "bex-cli-update.json")
	}
	return &Checker{APIBase: apiBase, Repo: repo, CachePath: cachePath, Now: time.Now}
}

type cacheEntry struct {
	CheckedAt time.Time `json:"checked_at"`
	Release   Release   `json:"release"`
}

// Latest returns the newest matching release, from cache when fresh. A
// fetch failure is also cached (as an empty Release) so an offline or
// misbehaving network is retried at most once per cache window instead of on
// every invocation; a cached empty Release means "nothing to report".
func (c *Checker) Latest() (Release, error) {
	if entry, err := c.readCache(); err == nil && c.Now().Sub(entry.CheckedAt) < cacheMaxAge {
		return entry.Release, nil
	}
	release, err := c.fetch()
	c.writeCache(cacheEntry{CheckedAt: c.Now(), Release: release})
	if err != nil {
		return Release{}, err
	}
	return release, nil
}

func (c *Checker) readCache() (cacheEntry, error) {
	var entry cacheEntry
	if c.CachePath == "" {
		return entry, fmt.Errorf("no cache path")
	}
	data, err := os.ReadFile(c.CachePath)
	if err != nil {
		return entry, err
	}
	if err := json.Unmarshal(data, &entry); err != nil {
		return entry, err
	}
	return entry, nil
}

// writeCache is best-effort: a read-only home must not break the CLI.
func (c *Checker) writeCache(entry cacheEntry) {
	if c.CachePath == "" {
		return
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(c.CachePath), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(c.CachePath, data, 0o600)
}

func (c *Checker) fetch() (Release, error) {
	best, err := c.LatestRelease()
	if err != nil {
		return Release{}, err
	}
	return Release{Version: best.Version, URL: best.URL}, nil
}

// fetchReleases lists the published, non-draft/prerelease bex-cli releases,
// normalizing each tag to a semver and capturing its assets. Malformed or
// out-of-namespace releases are skipped, never fatal.
func (c *Checker) fetchReleases() ([]FullRelease, error) {
	url := strings.TrimRight(c.APIBase, "/") + "/repos/" + c.Repo + "/releases?per_page=30"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list releases: HTTP %d", resp.StatusCode)
	}
	var releases []struct {
		TagName    string `json:"tag_name"`
		HTMLURL    string `json:"html_url"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}
	var out []FullRelease
	for _, r := range releases {
		if r.Draft || r.Prerelease || !strings.HasPrefix(r.TagName, TagPrefix) {
			continue
		}
		v, err := semver.ParseTolerant(strings.TrimPrefix(r.TagName, TagPrefix))
		if err != nil {
			continue
		}
		assets := make([]Asset, 0, len(r.Assets))
		for _, a := range r.Assets {
			assets = append(assets, Asset{Name: a.Name, URL: a.URL})
		}
		out = append(out, FullRelease{Version: v.String(), Tag: r.TagName, URL: r.HTMLURL, Assets: assets})
	}
	return out, nil
}

// newest returns the highest-semver release. ok is false for an empty slice.
func newest(releases []FullRelease) (FullRelease, bool) {
	var best FullRelease
	var bestVersion semver.Version
	found := false
	for _, r := range releases {
		v, err := semver.ParseTolerant(r.Version)
		if err != nil {
			continue
		}
		if !found || v.GT(bestVersion) {
			best, bestVersion, found = r, v, true
		}
	}
	return best, found
}

// Newer reports whether latest is a strictly newer version than current.
// Unparseable versions are never "newer" — the notice stays silent rather
// than nagging on malformed data.
func Newer(current, latest string) bool {
	cur, err := semver.ParseTolerant(current)
	if err != nil {
		return false
	}
	lat, err := semver.ParseTolerant(latest)
	if err != nil {
		return false
	}
	return lat.GT(cur)
}
