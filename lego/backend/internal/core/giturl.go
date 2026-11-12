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

package core

import "strings"

// CanonicalRepo normalizes a git URL to a comparable key "host/owner/repo":
// lowercased, scheme (https/http/ssh/git) and any "user@" stripped, scp-style
// "host:owner/repo" flattened to "host/owner/repo", and trailing "/" and ".git"
// removed. So the https, ssh, and scp forms of one repository compare equal.
//
// Shared by the push webhook's repo match (apps) and the GitHub clone-token
// grant derivation (github) so both derive the same key from the same URL — a
// drift there would let a private repo redeploy but fail to clone (or vice
// versa).
func CanonicalRepo(u string) string {
	s := strings.ToLower(strings.TrimSpace(u))
	for _, scheme := range []string{"https://", "http://", "ssh://", "git://"} {
		s = strings.TrimPrefix(s, scheme)
	}
	if at := strings.Index(s, "@"); at != -1 {
		s = s[at+1:] // drop user@ (git@github.com:… / user@host/…)
	}
	s = strings.ReplaceAll(s, ":", "/") // scp-style host:owner/repo -> host/owner/repo
	return strings.TrimSuffix(strings.TrimRight(s, "/"), ".git")
}

// ValidGitRefName reports whether ref is a ref name Git accepts that cannot
// read as revision syntax or a refspec: no space, ASCII control character or
// any of ~^:?*[\, no ".." or "@{", and no empty, dot-led, dot-ended or
// ".lock" path component, as Git's check-ref-format refuses. Callers bound
// the length. Shared by the public-Git ref check and the agent-session branch
// gate, which refuses non-ASCII whitespace and controls on top.
func ValidGitRefName(ref string) bool {
	if ref == "" || strings.Contains(ref, "..") || strings.Contains(ref, "@{") {
		return false
	}
	for _, r := range ref {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(" ~^:?*[\\", r) {
			return false
		}
	}
	for _, part := range strings.Split(ref, "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".") || strings.HasSuffix(part, ".lock") {
			return false
		}
	}
	return true
}

// ValidCommitSHA accepts Git object IDs GitHub returns for a branch HEAD:
// 40 hex chars (SHA-1) or 64 (SHA-256 repos). Anything else is malformed
// provenance a Blueprint pin must refuse (w8/m36 / w8/m41).
func ValidCommitSHA(sha string) bool {
	if len(sha) != 40 && len(sha) != 64 {
		return false
	}
	for i := 0; i < len(sha); i++ {
		c := sha[i]
		if c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' {
			continue
		}
		return false
	}
	return true
}
