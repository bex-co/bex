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

// Allowed reports whether an update check may run at all. A non-nil isTTY
// adds the interactive gate used by the post-command notice; the explicit
// version path passes nil (the user asked). A "dev" build has no release
// identity to compare.
func Allowed(version string, lookup func(string) (string, bool), isTTY func() bool) bool {
	if version == "" || version == "dev" {
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
	url := strings.TrimRight(c.APIBase, "/") + "/repos/" + c.Repo + "/releases?per_page=30"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("list releases: HTTP %d", resp.StatusCode)
	}
	var releases []struct {
		TagName    string `json:"tag_name"`
		HTMLURL    string `json:"html_url"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return Release{}, err
	}
	var best Release
	var bestVersion semver.Version
	found := false
	for _, r := range releases {
		if r.Draft || r.Prerelease || !strings.HasPrefix(r.TagName, TagPrefix) {
			continue
		}
		v, err := semver.ParseTolerant(strings.TrimPrefix(r.TagName, TagPrefix))
		if err != nil {
			continue
		}
		if !found || v.GT(bestVersion) {
			best = Release{Version: v.String(), URL: r.HTMLURL}
			bestVersion = v
			found = true
		}
	}
	if !found {
		return Release{}, fmt.Errorf("no %s* release found", TagPrefix)
	}
	return best, nil
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
