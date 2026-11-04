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

import (
	"net/url"
	"strings"
)

// CommitURL builds the repo's web commit page URL from an App's spec.repo and a
// commit SHA — the deploy email's "View commit" link and a build's `buildRun.commitUrl`. It normalizes the common
// clone-URL shapes (https, scp-like SSH `git@host:owner/repo`, ssh://, a
// trailing .git) to an https web URL and picks the host's commit path segment
// (GitHub/Gitea `/commit/`, GitLab `/-/commit/`, Bitbucket `/commits/`).
// Returns "" when either input is empty or the repo URL can't be parsed to a
// host — an image-backed deploy (no repo) then simply carries no commit link.
func CommitURL(repo, sha string) string {
	repo, sha = strings.TrimSpace(repo), strings.TrimSpace(sha)
	if repo == "" || sha == "" {
		return ""
	}
	// scp-like SSH (git@github.com:owner/repo) → URL form the parser accepts.
	if strings.HasPrefix(repo, "git@") {
		if i := strings.Index(repo, ":"); i > len("git@") {
			repo = "https://" + repo[len("git@"):i] + "/" + repo[i+1:]
		}
	}
	repo = strings.TrimSuffix(repo, ".git")
	u, err := url.Parse(repo)
	if err != nil || u.Host == "" {
		return ""
	}
	// The web link is always https, without any embedded credentials.
	u.Scheme, u.User = "https", nil
	seg := "/commit/"
	switch {
	case strings.Contains(u.Host, "gitlab"):
		seg = "/-/commit/"
	case strings.Contains(u.Host, "bitbucket"):
		seg = "/commits/"
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + seg + sha
	return u.String()
}
