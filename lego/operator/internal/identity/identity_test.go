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

package identity

import (
	"strings"
	"testing"
)

func TestLegacyByteIdentity(t *testing.T) {
	id := ForApp("hello", "")
	if id.Scoped() {
		t.Fatal("unlabeled App must not be scoped")
	}
	if got, want := id.Repo(), "hello"; got != want {
		t.Errorf("Repo = %q, want %q", got, want)
	}
	if got, want := id.ZotUsername(), "app-hello"; got != want {
		t.Errorf("ZotUsername = %q, want %q", got, want)
	}
	if got, want := id.PullSecretName(), "reg-pull-hello"; got != want {
		t.Errorf("PullSecretName = %q, want %q", got, want)
	}
	if got, want := id.StaticPrefix("rev-7"), "hello/rev-7/"; got != want {
		t.Errorf("StaticPrefix = %q, want %q", got, want)
	}
	if got, want := id.CacheRepo(), "hello-cache"; got != want {
		t.Errorf("CacheRepo = %q, want %q", got, want)
	}
	if id.DualRead() {
		t.Error("unlabeled App must not dual-read")
	}
	if got, want := id.Key(), "hello"; got != want {
		t.Errorf("Key = %q, want %q", got, want)
	}
}

func TestWorkspaceScopedFormats(t *testing.T) {
	const ws = "tea-c185th5c2rvvnhbfiltg"
	id := ForApp("hello", ws)
	if !id.Scoped() {
		t.Fatal("labeled App must be scoped")
	}
	if got, want := id.Repo(), ws+"/hello"; got != want {
		t.Errorf("Repo = %q, want %q", got, want)
	}
	if got, want := id.ZotUsername(), "app-"+ws+"-hello"; got != want {
		t.Errorf("ZotUsername = %q, want %q", got, want)
	}
	if got, want := id.PullSecretName(), "reg-pull-"+ws+"-hello"; got != want {
		t.Errorf("PullSecretName = %q, want %q", got, want)
	}
	if got, want := id.StaticPrefix("rev-7"), ws+"/hello/rev-7/"; got != want {
		t.Errorf("StaticPrefix = %q, want %q", got, want)
	}
	if got, want := id.CacheRepo(), ws+"/hello-cache"; got != want {
		t.Errorf("CacheRepo = %q, want %q — last-component -cache, not a third segment", got, want)
	}
	if got, want := id.LegacyRepo(), "hello"; got != want {
		t.Errorf("LegacyRepo = %q, want %q", got, want)
	}
	if !id.DualRead() {
		t.Error("labeled untombstoned App must dual-read")
	}
	id.Tombstoned = true
	if id.DualRead() {
		t.Error("tombstoned App must not dual-read")
	}
}

func TestSameNameDifferentWorkspacesAreDisjoint(t *testing.T) {
	a := ForApp("web", "tea-aaaaaaaaaaaaaaaaaaaa")
	b := ForApp("web", "tea-bbbbbbbbbbbbbbbbbbbb")
	if a.Repo() == b.Repo() || a.ZotUsername() == b.ZotUsername() || a.PullSecretName() == b.PullSecretName() {
		t.Fatalf("same App name in two workspaces collided: a=%+v b=%+v", a, b)
	}
	if a.StaticPrefix("rev-1") == b.StaticPrefix("rev-1") {
		t.Fatal("static prefixes collided")
	}
	if a.CacheRepo() == b.CacheRepo() {
		t.Fatal("cache repos collided")
	}
}

func TestPullSecretNameTruncatesOverlong(t *testing.T) {
	long := strings.Repeat("n", 250)
	id := ForApp(long, "tea-c185th5c2rvvnhbfiltg")
	got := id.PullSecretName()
	if len(got) > maxSecretName {
		t.Errorf("PullSecretName length %d exceeds %d: %q", len(got), maxSecretName, got)
	}
	other := ForApp(long+"x", "tea-c185th5c2rvvnhbfiltg")
	if got == other.PullSecretName() {
		t.Error("truncated names for distinct identities must not collide")
	}
}

func TestAppLevelPrefix(t *testing.T) {
	cases := []struct{ in, want string }{
		{"web/rev-7/", "web/"},
		{"tea-x/web/rev-7/", "tea-x/web/"},
		{"web/", "web/"},
		{"", ""},
	}
	for _, c := range cases {
		if got := AppLevelPrefix(c.in); got != c.want {
			t.Errorf("AppLevelPrefix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPurgePrefixesTombstoneAddsLegacy(t *testing.T) {
	id := ForApp("web", "tea-aaaaaaaaaaaaaaaaaaaa")
	got := id.PurgePrefixes(id.StaticPrefix("rev-7"))
	if len(got) != 1 || got[0] != "tea-aaaaaaaaaaaaaaaaaaaa/web/" {
		t.Errorf("untombstoned purge = %v", got)
	}
	id.Tombstoned = true
	got = id.PurgePrefixes(id.StaticPrefix("rev-7"))
	want := map[string]bool{"tea-aaaaaaaaaaaaaaaaaaaa/web/": true, "web/": true}
	if len(got) != 2 {
		t.Fatalf("tombstoned purge = %v, want 2 prefixes", got)
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("unexpected purge prefix %q", p)
		}
	}
}

func TestCharsetNoColonOrUnderscore(t *testing.T) {
	id := ForApp("hello-go", "tea-c185th5c2rvvnhbfiltg")
	for _, s := range []string{id.Repo(), id.ZotUsername(), id.PullSecretName(), id.CacheRepo()} {
		if strings.ContainsAny(s, ":_") {
			t.Errorf("%q contains illegal ':' or '_'", s)
		}
	}
}
