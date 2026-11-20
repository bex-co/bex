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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Render reads a Blueprint from a GitHub, GitLab or Bitbucket repository, and
// a public one needs no connected account: "Paste your GitHub/GitLab/Bitbucket
// repo URL", "Public repos are supported out of the box"
// (https://render.com/docs/deploy-to-render). bex read every Blueprint through
// the GitHub API under the URL's last two path segments, so
// https://gitlab.com/render-examples/elasticsearch pinned and applied
// github.com/render-examples/elasticsearch, another repository (w8/344).
//
// A gitlab.com or bitbucket.org Blueprint now resolves its branch from that
// repository's own public advertisement and reads its file from that host's
// raw endpoint at the pinned commit. Any other host is refused by name. None
// of it reaches the GitHub client or mints a token.

// CodeBlueprintHostUnsupported refuses a Blueprint repository that bex does
// not read: one on a host Render reads no Blueprint from, or a URL form its
// host is not read in. Its message names the host from the caller's own URL.
const CodeBlueprintHostUnsupported = "BLUEPRINT_HOST_UNSUPPORTED"

// publicBlueprintRawInfix is, per host, what follows the repository path in
// its raw-file URL: <repo><infix><commit>/<file>.
var publicBlueprintRawInfix = map[string]string{
	"gitlab.com":    "/-/raw/",
	"bitbucket.org": "/raw/",
}

// publicBlueprintSource is a Blueprint repository on gitlab.com or
// bitbucket.org, as the anonymous reads address it.
type publicBlueprintSource struct {
	host          string
	advertisement string // its upload-pack advertisement URL
	rawPrefix     string // https://<host>/<repo path><infix>
	path          string // the repository path, unescaped: group/sub/repo
}

// onGitHub is the one test of whether a repository is GitHub's, shared by
// every read that picks the GitHub API or another host's public Git.
func onGitHub(repoURL string) bool {
	return strings.HasPrefix(core.CanonicalRepo(repoURL), "github.com/")
}

// BlueprintHost names the host a failed Blueprint read's wording names
// (w8/346): "" for a repository the GitHub API reads, whose wording names
// bex's GitHub app, else the host its URL names, read or refused.
func BlueprintHost(repoURL string) string {
	if onGitHub(repoURL) {
		return ""
	}
	return blueprintURLHost(repoURL)
}

// RefuseBlueprintHost is the refusal a Blueprint read of repoURL gives
// before any request (CodeBlueprintHostUnsupported), or nil for a repository
// bex reads. A default-branch lookup asks it first, so a host bex reads no
// Blueprint from is refused by name with no branch too, never probed (w8/346).
func RefuseBlueprintHost(repoURL string) error {
	if onGitHub(repoURL) {
		return nil
	}
	_, err := parsePublicBlueprintSource(repoURL)
	var coded *core.CodedError
	if errors.As(err, &coded) && coded.Code == CodeBlueprintHostUnsupported {
		return err
	}
	return nil
}

// blueprintURLHost is the host a repository URL names, lowercased: its
// parsed hostname, or else the first segment of its canonical form.
func blueprintURLHost(repoURL string) string {
	host, _, _ := strings.Cut(core.CanonicalRepo(repoURL), "/")
	if u, err := url.Parse(strings.TrimSpace(repoURL)); err == nil && u.Hostname() != "" {
		host = strings.ToLower(u.Hostname())
	}
	return host
}

// parsePublicBlueprintSource accepts a plain https URL of a gitlab.com or
// bitbucket.org repository and refuses anything else with the host named.
func parsePublicBlueprintSource(repoURL string) (publicBlueprintSource, error) {
	repoURL = strings.TrimSpace(repoURL)
	host := blueprintURLHost(repoURL)
	u, parseErr := url.Parse(repoURL)
	infix, supported := publicBlueprintRawInfix[host]
	if !supported {
		return publicBlueprintSource{}, core.NewBadRequestError(CodeBlueprintHostUnsupported, fmt.Sprintf(
			"bex reads a Blueprint from a public repository on github.com, gitlab.com or bitbucket.org, as Render does; %s is not one of them", host),
			map[string]any{"host": host})
	}
	advertisement, ok := publicGitAdvertisementURL(repoURL)
	if !ok || parseErr != nil || !strings.EqualFold(u.Host, host) {
		return publicBlueprintSource{}, core.NewBadRequestError(CodeBlueprintHostUnsupported, fmt.Sprintf(
			"a Blueprint repository on %s is read over HTTPS: use its https://%s/<owner>/<repo> URL", host, host),
			map[string]any{"host": host})
	}
	path := strings.TrimRight(u.Path, "/")
	escaped := strings.TrimRight(u.EscapedPath(), "/")
	if strings.HasSuffix(strings.ToLower(path), ".git") && strings.HasSuffix(strings.ToLower(escaped), ".git") {
		path, escaped = path[:len(path)-4], escaped[:len(escaped)-4]
	}
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	// GitLab reserves "-" for its routes (/-/raw/, /-/tree/), so no project
	// path holds it; Bitbucket's is exactly workspace/repo.
	if len(segments) < 2 || (host == "bitbucket.org" && len(segments) != 2) || slices.Contains(segments, "-") {
		return publicBlueprintSource{}, fmt.Errorf("%w: repo URL %q is not an owner/repo URL", core.ErrBadRequest, repoURL)
	}
	return publicBlueprintSource{host: host, advertisement: advertisement, rawPrefix: "https://" + host + escaped + infix, path: strings.Join(segments, "/")}, nil
}

// resolvePublicBlueprintCommit pins branch from the repository's own
// advertisement, read fresh (a sync pins the tip, so no cache), classified as
// ValidateRepo classifies it (publicGitRefused): an answer no clone gets past
// is ErrRepoNotFoundOrNoAccess, an advertisement without the branch
// ErrBranchNotFound, and a host that did not answer an unavailable error.
func (s *Service) resolvePublicBlueprintCommit(ctx context.Context, src publicBlueprintSource, branch string) (string, error) {
	if !validPublicGitRef(branch) {
		return "", ErrBranchNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, publicCommitResolveTimeout)
	defer cancel()
	answer, err := s.getPublicGitAdvertisement(ctx, src.advertisement)
	switch {
	case publicGitRefused(answer, err):
		return "", fmt.Errorf("%w: %s has no public Git repository at this URL", ErrRepoNotFoundOrNoAccess, src.host)
	case err != nil:
		return "", fmt.Errorf("github: blueprint repository on %s did not answer: %w", src.host, err)
	case !answer.advertisement():
		return "", fmt.Errorf("github: blueprint repository on %s answered status %d", src.host, answer.status)
	}
	ref := "refs/heads/" + branch
	if len(answer.body) > maxPublicGitAdvertisement {
		// Only HEAD's branch is readable past the bound (oversizedPublicGitHead).
		if sha := oversizedPublicGitHead(answer.body, ref); sha != "" {
			return sha, nil
		}
		return "", fmt.Errorf("github: blueprint repository on %s advertises too many refs to find branch %q", src.host, branch)
	}
	refs, _, ok := parsePublicGitAdvertisement(answer.body)
	if !ok {
		return "", fmt.Errorf("github: blueprint repository on %s answered a malformed advertisement", src.host)
	}
	if sha := refs[ref]; validCommitSHA(sha) {
		return sha, nil
	}
	return "", ErrBranchNotFound
}

// readPublicBlueprintFile reads one file at a resolved commit from the host's
// raw endpoint, anonymously, with the Blueprint size limit. It follows no
// redirect: GitLab sends a repository it does not show to its sign-in page.
// A 404 answers as the contents API's not-found does.
func (s *Service) readPublicBlueprintFile(ctx context.Context, src publicBlueprintSource, commitSHA, filePath string) (string, error) {
	noRedirect := func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	status, body, err := getRawBlueprintFile(ctx, s.anonymousGitClient(noRedirect), src.rawPrefix+commitSHA+"/"+escapeRepoPath(filePath))
	switch {
	case err != nil:
		return "", fmt.Errorf("github: %s raw file: %w", src.host, err)
	case status == http.StatusNotFound:
		return "", fmt.Errorf("github: file %q not found at ref %q: %w", filePath, commitSHA, &APIError{Status: status})
	case status == http.StatusUnauthorized, status == http.StatusForbidden, status >= 300 && status < 400:
		return "", fmt.Errorf("%w: %s did not serve the file publicly", ErrRepoNotFoundOrNoAccess, src.host)
	case status != http.StatusOK:
		return "", fmt.Errorf("github: %s raw file answered status %d", src.host, status)
	case len(body) > maxBlueprintFileBytes:
		return "", fmt.Errorf("%w: file %q at ref %q", errFileTooLarge, filePath, commitSHA)
	}
	return string(body), nil
}
