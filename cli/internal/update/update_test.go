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

package update

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

func rootFlags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("root", pflag.ContinueOnError)
	flags.StringP("output", "o", "", "")
	flags.Bool("confirm", false, "")
	return flags
}

func TestIsRootVersionRequest(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"-v"}, true},
		{[]string{"--version"}, true},
		{[]string{"--version=true"}, true},
		{[]string{"-o", "text", "--version"}, true}, // global flag value skipped
		{[]string{"--output=json", "-v"}, true},     // inline value
		{[]string{"--confirm", "--version"}, true},  // bool flag consumes no value
		{[]string{"services", "-v"}, false},         // after first subcommand token
		{[]string{"-o", "text", "services", "-v"}, false},
		{[]string{"--", "-v"}, false},
		{[]string{"--unknown", "--version"}, false}, // unknown flag ends root scan
		{[]string{}, false},
	}
	flags := rootFlags()
	for _, c := range cases {
		if got := IsRootVersionRequest(c.args, flags); got != c.want {
			t.Errorf("IsRootVersionRequest(%v) = %v, want %v", c.args, got, c.want)
		}
	}
}

func TestAllowedGates(t *testing.T) {
	lookup := func(env map[string]string) func(string) (string, bool) {
		return func(name string) (string, bool) { v, ok := env[name]; return v, ok }
	}
	tty := func() bool { return true }
	noTTY := func() bool { return false }

	if Allowed("dev", lookup(nil), nil) {
		t.Error("dev build must never check")
	}
	if Allowed("", lookup(nil), nil) {
		t.Error("empty version must never check")
	}
	if Allowed("1.0.0", lookup(map[string]string{"BEX_NO_UPDATE_NOTIFIER": "1"}), nil) {
		t.Error("opt-out must silence the check")
	}
	if Allowed("1.0.0", lookup(map[string]string{"CI": "true"}), nil) {
		t.Error("CI must silence the check")
	}
	if Allowed("1.0.0", lookup(nil), noTTY) {
		t.Error("a TTY gate without a TTY must silence the check")
	}
	if !Allowed("1.0.0", lookup(nil), tty) {
		t.Error("all gates open: check must be allowed")
	}
	if !Allowed("1.0.0", lookup(nil), nil) {
		t.Error("explicit path (nil gate) must not require a TTY")
	}
}

const releasesJSON = `[
	{"tag_name": "bex-cli/v1.4.0", "html_url": "https://example.test/r/1.4.0", "draft": false, "prerelease": false},
	{"tag_name": "bex-cli/v2.0.0", "html_url": "https://example.test/r/2.0.0-draft", "draft": true, "prerelease": false},
	{"tag_name": "bex-cli/v1.9.0-rc.1", "html_url": "https://example.test/r/rc", "draft": false, "prerelease": true},
	{"tag_name": "operator/v9.9.9", "html_url": "https://example.test/r/operator", "draft": false, "prerelease": false},
	{"tag_name": "bex-cli/v1.5.2", "html_url": "https://example.test/r/1.5.2", "draft": false, "prerelease": false}
]`

func TestLatestFiltersToNewestStableBexCliRelease(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/repos/bex-co/bex/releases" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(releasesJSON))
	}))
	t.Cleanup(server.Close)

	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	c := &Checker{
		APIBase:   server.URL,
		Repo:      "bex-co/bex",
		CachePath: filepath.Join(t.TempDir(), "cache.json"),
		Now:       func() time.Time { return now },
	}
	release, err := c.Latest()
	if err != nil {
		t.Fatal(err)
	}
	if release.Version != "1.5.2" || release.URL != "https://example.test/r/1.5.2" {
		t.Errorf("release = %+v, want 1.5.2 (drafts, prereleases, and other tags skipped)", release)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}

	// Fresh cache: no second network call.
	if _, err := c.Latest(); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Errorf("cache hit still made a request (requests = %d)", requests)
	}

	// Stale cache: refetch.
	now = now.Add(25 * time.Hour)
	if _, err := c.Latest(); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Errorf("stale cache did not refetch (requests = %d)", requests)
	}
}

func TestLatestCachesFailuresForTheFullWindow(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "rate limited", http.StatusForbidden)
	}))
	t.Cleanup(server.Close)
	c := &Checker{
		APIBase:   server.URL,
		Repo:      "bex-co/bex",
		CachePath: filepath.Join(t.TempDir(), "cache.json"),
		Now:       time.Now,
	}
	if _, err := c.Latest(); err == nil {
		t.Fatal("HTTP failure must return an error for the caller to silence")
	}
	// The failure is negatively cached: within the window the empty result
	// comes back without a retry, so a bad network costs one attempt per day.
	release, err := c.Latest()
	if err != nil || release.Version != "" {
		t.Errorf("cached failure: release=%+v err=%v, want empty and nil", release, err)
	}
	if requests != 1 {
		t.Errorf("failed fetch retried within the cache window (requests = %d)", requests)
	}
}

func TestLatestWithoutCachePathStillFetches(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(releasesJSON))
	}))
	t.Cleanup(server.Close)
	c := &Checker{APIBase: server.URL, Repo: "bex-co/bex", Now: time.Now}
	release, err := c.Latest()
	if err != nil || release.Version != "1.5.2" {
		t.Fatalf("release=%+v err=%v", release, err)
	}
	if _, err := c.Latest(); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Errorf("cacheless checker must fetch each time (requests = %d)", requests)
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"1.0.0", "1.0.1", true},
		{"1.0.0", "2.0.0", true},
		{"1.5.2", "1.5.2", false},
		{"2.0.0", "1.9.9", false},
		{"dev", "1.0.0", false},
		{"1.0.0", "garbage", false},
	}
	for _, c := range cases {
		if got := Newer(c.current, c.latest); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}
