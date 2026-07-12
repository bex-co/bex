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
