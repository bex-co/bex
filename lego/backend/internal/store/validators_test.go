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

package store

import (
	"strings"
	"testing"
)

// validators_test.go pins the build-from-git input validators added in w6/m6
// (t003 injection audit): repo URL scheme, git ref format, and root-directory
// path traversal. Each case is an adversarial input an untrusted caller might
// send; the validators are the API boundary that keeps it out of the BuildKit
// context string. A regression here is a security regression, not just a bug.

func TestValidRepo(t *testing.T) {
	cases := []struct {
		name string
		repo string
		want bool
	}{
		{"https", "https://github.com/bex-co/bex.git", true},
		{"http", "http://gitea.internal:3000/x/y", true},
		{"ssh", "ssh://git@github.com/bex-co/bex.git", true},
		{"scp-form", "git@github.com:bex-co/bex.git", true},
		{"empty-rejected", "", false},
		{"file-scheme-rejected", "file:///etc/passwd", false},
		{"local-path-rejected", "/etc/passwd", false},
		{"bare-host-rejected", "github.com/x/y", false},
		{"git-scheme-rejected", "git://github.com/x/y", false},
		{"newline-injected", "https://github.com/x/y\n--upload-pack=malicious", false},
		{"crlf-injected", "https://github.com/x/y\r\nX", false},
		{"space-injected", "https://github.com/x/y evil", false},
		{"control-char", "https://github.com/x/\x00y", false},
		// Repo, ref, and rootDir are separately validated intent. A URL-fragment
		// ref/subdir would create a second, ambiguous source for the latter two.
		{"fragment-bypass-rejected", "https://github.com/x/y.git#--evil-ref:../../escape", false},
		// Embedded credentials (round-6 #13): the stored URL is echoed to every
		// workspace viewer, so userinfo would leak a member's token. Only the
		// password-less ssh:// username (git@, matched above) is legitimate.
		{"https-user-pass-rejected", "https://user:ghp_token@github.com/x/y.git", false},
		{"https-token-as-user-rejected", "https://ghp_token@github.com/x/y.git", false},
		{"http-userinfo-rejected", "http://user:pw@gitea.internal:3000/x/y", false},
		{"ssh-password-rejected", "ssh://git:secret@github.com/x/y.git", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidRepo(tc.repo); got != tc.want {
				t.Errorf("ValidRepo(%q) = %v, want %v", tc.repo, got, tc.want)
			}
		})
	}
}

func TestValidImage(t *testing.T) {
	cases := []struct {
		name  string
		image string
		want  bool
	}{
		{"registry-tag", "zot.bex-registry.svc:5000/myapp:rev-abc", true},
		{"public-tag", "docker.io/library/nginx:1.25", true},
		{"digest", "ghcr.io/org/repo@sha256:deadbeefcafebabe", true},
		{"short", "nginx", true},
		{"empty-rejected", "", false},
		// w1/m53: the SSRF-adjacent shapes — whitespace/control/shell-meta chars —
		// must be refused so the operator never gets a weaponizable image string.
		{"space-rejected", "nginx latest", false},
		{"newline-rejected", "nginx\nevil", false},
		{"control-char-rejected", "nginx\x00", false},
		{"backtick-rejected", "nginx`whoami`", false},
		{"substitution-rejected", "nginx$(whoami)", false},
		{"leading-slash-rejected", "/etc/passwd", false},
		{"leading-dash-rejected", "-flag", false},
		{"too-long-rejected", strings.Repeat("a", 513), false},
		// codex round-7 F6: the kubelet pulls tenant images from node network
		// context, outside every pod egress policy — a private/loopback/link-local/
		// CGNAT/metadata IP-literal host or localhost turns spec.Image into a
		// node-origin probe with tenant-visible pull errors. No public registry
		// is affected.
		{"loopback-host-rejected", "127.0.0.1/myapp:latest", false},
		{"loopback-host-port-rejected", "127.0.0.1:5000/myapp", false},
		{"localhost-rejected", "localhost/myapp", false},
		{"localhost-port-rejected", "localhost:5000/myapp", false},
		{"uppercase-localhost-rejected", "LOCALHOST/myapp", false},
		{"private-host-rejected", "10.0.0.5:5000/myapp", false},
		{"private-host-noport-rejected", "192.168.1.4/myapp", false},
		{"metadata-host-rejected", "169.254.169.254/latest/meta-data", false},
		{"cgnat-host-rejected", "100.64.0.9:5000/myapp", false},
		{"linklocal-v6-rejected", "[fe80::1]:5000/myapp", false}, // bracket forms are regex-refused anyway
		{"public-ip-literal-allowed", "1.2.3.4:5000/myapp", true},
		{"internal-dns-name-residual", "zot.bex-registry.svc:5000/myapp", true}, // documented residual: names are indistinguishable
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidImage(tc.image); got != tc.want {
				t.Errorf("ValidImage(%q) = %v, want %v", tc.image, got, tc.want)
			}
		})
	}
}

func TestValidGitRef(t *testing.T) {
	cases := []struct {
		name string
		ref  string
		want bool
	}{
		{"branch", "main", true},
		{"feature-slash", "feature/add-thing", true},
		{"refsprefix", "refs/heads/main", true},
		{"tag", "v1.2.3", true},
		{"sha", "abcdef1234567890abcdef1234567890abcdef12", true},
		{"empty-rejected", "", false},
		// argument-injection class: a leading dash could be read as a git flag.
		{"leading-dash-rejected", "--upload-pack=malicious", false},
		{"shell-semicolon-rejected", "main; rm -rf /", false},
		{"pipe-rejected", "main | cat", false},
		{"backtick-rejected", "main`whoami`", false},
		{"substitution-rejected", "main$(whoami)", false},
		{"newline-rejected", "main\nrefs/heads/evil", false},
		{"space-rejected", "main evil", false},
		{"leading-dot-rejected", ".git/refs", false},
		{"leading-slash-rejected", "/refs/heads/main", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidGitRef(tc.ref); got != tc.want {
				t.Errorf("ValidGitRef(%q) = %v, want %v", tc.ref, got, tc.want)
			}
		})
	}
}

func TestValidRootDir(t *testing.T) {
	cases := []struct {
		name    string
		rootDir string
		want    bool
	}{
		{"empty-repo-root", "", true},
		{"simple", "apps/web", true},
		{"nested", "apps/web/dist", true},
		{"dot-segment", "./apps", true},
		{"parent-traversal-rejected", "../escape", false},
		{"mid-traversal-rejected", "apps/../escape", false},
		{"absolute-rejected", "/etc/passwd", false},
		{"double-separator-rejected", "apps//web", false},
		{"newline-rejected", "apps\n/web", false},
		{"control-char-rejected", "apps/\x00web", false},
		{"backslash-rejected", "apps\\web", false},
		{"too-long-rejected", "a/" + strings.Repeat("x", 600), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidRootDir(tc.rootDir); got != tc.want {
				t.Errorf("ValidRootDir(%q) = %v, want %v", tc.rootDir, got, tc.want)
			}
		})
	}
}

func TestValidGlob(t *testing.T) {
	cases := []struct {
		name string
		glob string
		want bool
	}{
		{"empty-rejected", "", false},
		{"simple-star", "src/*", true},
		{"globstar", "src/**", true},
		{"double-globstar-suffix", "**/*.md", true},
		{"single-char", "src/main.?s", true},
		{"char-class", "src/[abc].go", true},
		{"negated-class", "src/[^abc].go", true},
		{"range-class", "src/[a-z].go", true},
		{"unclosed-class-rejected", "src/[", false},
		{"parent-traversal-rejected", "../escape/**", false},
		{"mid-traversal-rejected", "src/../escape", false},
		{"absolute-rejected", "/etc/passwd", false},
		{"backslash-rejected", "src\\**", false},
		{"newline-rejected", "src\n/**", false},
		{"control-char-rejected", "src/\x00", false},
		{"too-long-rejected", "a/" + strings.Repeat("x", 600), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidGlob(tc.glob); got != tc.want {
				t.Errorf("ValidGlob(%q) = %v, want %v", tc.glob, got, tc.want)
			}
		})
	}
}

// TestRedactRepoURL pins round-6 #13's read side: legacy rows that embedded
// credentials in the repo URL are redacted before any viewer sees them, while
// clean URLs (and the legitimate ssh:// username) pass through unchanged.
func TestRedactRepoURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://github.com/x/y.git", "https://github.com/x/y.git"},
		{"ssh://git@github.com/x/y.git", "ssh://git@github.com/x/y.git"},
		{"git@github.com:x/y.git", "git@github.com:x/y.git"},
		{"https://user:ghp_token@github.com/x/y.git", "https://github.com/x/y.git"},
		{"https://ghp_token@github.com/x/y.git", "https://github.com/x/y.git"},
		{"ssh://git:secret@github.com/x/y.git", "ssh://github.com/x/y.git"},
	}
	for _, tc := range cases {
		if got := RedactRepoURL(tc.in); got != tc.want {
			t.Errorf("RedactRepoURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
