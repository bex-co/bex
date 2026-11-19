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
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// AnnotationGitHubRepoID carries, on a store-managed App, GitHub's numeric id
// for the repository spec.repo names (w10/m490). The id survives a rename or a
// transfer, so the push webhook matches the App by it instead of by URL. The
// projector stamps it from the apps row (github_repo_id), which is its source
// of truth. An annotation, not a spec field: writing it bumps no generation,
// so stamping it never reads as a new release, and the operator never reads it.
const AnnotationGitHubRepoID = "app.bex.co/github-repo-id"

// GitHubRepoID is the GitHub repository id the App carries, 0 when it carries
// none or an unreadable one.
func GitHubRepoID(meta metav1.Object) int64 {
	id, err := strconv.ParseInt(meta.GetAnnotations()[AnnotationGitHubRepoID], 10, 64)
	if err != nil || id < 0 {
		return 0
	}
	return id
}

// SetGitHubRepoID stamps id on meta, or removes the annotation for 0, and
// reports whether that changed it.
func SetGitHubRepoID(meta metav1.Object, id int64) bool {
	annotations := meta.GetAnnotations()
	current, has := annotations[AnnotationGitHubRepoID]
	if id <= 0 {
		if !has {
			return false
		}
		delete(annotations, AnnotationGitHubRepoID)
		meta.SetAnnotations(annotations)
		return true
	}
	want := strconv.FormatInt(id, 10)
	if has && current == want {
		return false
	}
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[AnnotationGitHubRepoID] = want
	meta.SetAnnotations(annotations)
	return true
}

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

// GitBacked reports whether a deploy with commit was built from source: it
// carries a commit, or its service a builds from a repo (a commit-less row of
// a repo-backed service is still not an image deploy). Render's deploy has no
// image for one ("Not present for Git-backed deploys"), so deploy views and
// deploy events both omit it (w4/621). a may be nil when the service is gone.
func GitBacked(a *appv1alpha1.App, commit string) bool {
	return commit != "" || (a != nil && a.Spec.Repo != "")
}
