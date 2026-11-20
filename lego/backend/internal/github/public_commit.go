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

package github

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Render deploys "either a full or short commit SHA"
// (https://render.com/docs/deploy-a-commit). An advertisement lists only ref
// tips, and a clone's upload-pack takes ref names and full object names, never
// a prefix: on GitLab and Bitbucket a short SHA missed, so its build fetched
// the raw prefix and failed with "couldn't find remote ref", and a SHA the
// repository lacks opened a deploy that failed at checkout (w8/345).
//
// gitlab.com and bitbucket.org each answer an anonymous commit lookup that
// takes a prefix and returns the full SHA, message and time, as GitHub's
// commits API does, and says so when the repository has no such commit.

// minCommitPrefix is Git's shortest abbreviated object name (MINIMUM_ABBREV);
// both hosts answer a shorter one "not found", which says nothing.
const minCommitPrefix = 4

// commitNameRef reports whether ref can name a commit by its object name: 4
// to 64 hex digits, a full SHA-1 or SHA-256 name or a prefix of one.
func commitNameRef(ref string) bool {
	if len(ref) < minCommitPrefix || len(ref) > 64 {
		return false
	}
	return strings.Trim(strings.ToLower(ref), "0123456789abcdef") == ""
}

// publicHostCommitURL is the host's anonymous commit lookup for ref, false for
// a repository on any other host.
func publicHostCommitURL(repoURL, ref string) (host, endpoint string, ok bool) {
	src, err := parsePublicBlueprintSource(repoURL)
	if err != nil {
		return "", "", false
	}
	ref = strings.ToLower(ref)
	if src.host == "gitlab.com" {
		return src.host, "https://gitlab.com/api/v4/projects/" + url.PathEscape(src.path) + "/repository/commits/" + ref, true
	}
	return src.host, "https://api.bitbucket.org/2.0/repositories/" + escapeRepoPath(src.path) + "/commit/" + ref, true
}

// lookupPublicHostCommit asks the repository's host for the commit ref names,
// cached and collapsed like the advertisement reads. Only the host's "no such
// commit" is notFound. A missing or private repository, 401, 403, 429, 5xx, a
// redirect, a timeout or a malformed answer is a miss, as the advertisement's
// failures are, so the lookup is no existence oracle.
func (s *Service) lookupPublicHostCommit(ctx context.Context, repoURL, ref string) cachedPublicCommit {
	host, endpoint, ok := publicHostCommitURL(repoURL, ref)
	if !ok {
		return cachedPublicCommit{}
	}
	return s.memoPublicCommit(ctx, "commit-api:"+endpoint, func(ctx context.Context) (cachedPublicCommit, time.Duration) {
		noRedirect := func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		status, body, err := getRawBlueprintFile(ctx, s.anonymousGitClient(noRedirect), endpoint)
		switch {
		case err != nil:
			log.Printf("github: %s commit lookup had no answer: %v", host, err)
		case len(body) > maxBlueprintFileBytes: // a miss
		case status == http.StatusOK:
			if info, ok := decodePublicHostCommit(host, body, ref); ok {
				return cachedPublicCommit{info: info, ok: true}, publicCommitCacheTTL
			}
		case status == http.StatusNotFound && publicHostNoSuchCommit(host, body):
			return cachedPublicCommit{notFound: true}, publicCommitNotFoundTTL
		}
		return cachedPublicCommit{}, publicCommitMissTTL
	})
}

// publicHostCommit is the fields each host's commit names: GitLab's id,
// message and authored_date, Bitbucket's hash, message and date (the
// author's), the ones GitHub's path records.
type publicHostCommit struct {
	ID           string `json:"id"`
	Hash         string `json:"hash"`
	Message      string `json:"message"`
	AuthoredDate string `json:"authored_date"`
	Date         string `json:"date"`
}

// decodePublicHostCommit reads a 200's commit, which must be a full object
// name that ref abbreviates: anything else is a miss, never a guess.
func decodePublicHostCommit(host string, body []byte, ref string) (store.CommitInfo, bool) {
	var c publicHostCommit
	if json.Unmarshal(body, &c) != nil {
		return store.CommitInfo{}, false
	}
	hash, at := c.Hash, c.Date
	if host == "gitlab.com" {
		hash, at = c.ID, c.AuthoredDate
	}
	hash = strings.ToLower(hash)
	if !core.ValidCommitSHA(hash) || !strings.HasPrefix(hash, strings.ToLower(ref)) {
		return store.CommitInfo{}, false
	}
	info := store.CommitInfo{Hash: hash, Message: strings.TrimRight(c.Message, "\n")}
	if t, err := time.Parse(time.RFC3339, at); err == nil {
		utc := t.UTC()
		info.AuthorAt = &utc
	}
	return info, true
}

// publicHostNoSuchCommit is the host's 404 for a repository it shows that has
// no such commit, told apart from its 404 for a missing or private one:
// GitLab's "404 Commit Not Found" (not "404 Project Not Found"), Bitbucket's
// error "Commit not found" (a missing repository is an HTML page).
func publicHostNoSuchCommit(host string, body []byte) bool {
	var answer struct {
		Message any `json:"message"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &answer) != nil {
		return false
	}
	if host == "gitlab.com" {
		return answer.Message == "404 Commit Not Found"
	}
	return answer.Error.Message == "Commit not found"
}
