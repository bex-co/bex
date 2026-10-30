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

package build

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/registry"
)

// Exact runtime versions (w8/175). Render installs the exact version a service
// pins (PYTHON_VERSION=3.11.9, NODE_VERSION=18.18.0); the reviewed lines
// (docs/ADR060 D7) pin one patch each. When a service pins another patch of a
// line, the resolve phase looks the official image for that exact tag up and
// pins its digest for this build, so the toolchain is the one the service asked
// for and the digest is the one the build log records. Only the four official
// Docker Hub repositories the lines already use are ever consulted, and only a
// fully qualified version of the requested line; anything else, or any lookup
// failure, keeps the line's reviewed image and says so in the build log.
//
// An exact Erlang/OTP (w8/197) also reads the official erlang repository and
// hexpm's Elixir builds, which are named by Elixir and OTP release; see
// pinErlangVersion.

// ExactImageResolver resolves Docker Hub tags of the reviewed repositories.
// A repo is an official image name ("python", "erlang") or hexpm's
// "hexpm/elixir".
type ExactImageResolver interface {
	// ResolveTag returns the manifest digest ("sha256:…") of
	// docker.io/<repo>:<tag>; ok is false when the tag does not exist.
	ResolveTag(ctx context.Context, repo, tag string) (digest string, ok bool, err error)
	// ListTags returns the repo's tags that begin with prefix.
	ListTags(ctx context.Context, repo, prefix string) ([]string, error)
	// ImageMetadata returns the pinned image's verified Linux/amd64 environment and build history.
	ImageMetadata(ctx context.Context, image string) (env, history []string, err error)
}

const (
	// erlangRepo is the official Erlang/OTP images, one tag per release.
	erlangRepo = "erlang"
	// hexpmElixirRepo publishes Elixir built on each Erlang/OTP release.
	hexpmElixirRepo = "hexpm/elixir"
)

// exactRepos are the repositories an exact version may name: the official
// toolchain repositories, the official erlang images and hexpm's Elixir.
var exactRepos = map[string]bool{nativePythonRuntime: true, nativeNodeRuntime: true, nativeRubyRuntime: true, nativeElixirRuntime: true,
	erlangRepo: true, hexpmElixirRepo: true}

// repoPath is a repo's Docker Hub path: official images live under library/.
func repoPath(repo string) string {
	if strings.Contains(repo, "/") {
		return repo
	}
	return "library/" + repo
}

var tagPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// DockerHubResolver reads digests from Docker Hub's registry API and tag
// listings from its Hub API, anonymously.
type DockerHubResolver struct {
	Client      *http.Client
	AuthURL     string // token endpoint; defaults to Docker Hub's
	RegistryURL string // registry base; defaults to Docker Hub's
	HubURL      string // Hub API base, for tag listings; defaults to Docker Hub's
}

// NewDockerHubResolver is the production resolver.
func NewDockerHubResolver() *DockerHubResolver {
	return &DockerHubResolver{
		Client:      &http.Client{Timeout: 10 * time.Second},
		AuthURL:     "https://auth.docker.io/token",
		RegistryURL: "https://registry-1.docker.io",
		HubURL:      "https://hub.docker.com",
	}
}

const maxTokenBytes = 64 << 10

// ResolveTag implements ExactImageResolver.
func (d *DockerHubResolver) ResolveTag(ctx context.Context, repo, tag string) (string, bool, error) {
	if !exactRepos[repo] || !tagPattern.MatchString(tag) {
		return "", false, fmt.Errorf("%s:%s is not a reviewed runtime image tag", repo, tag)
	}
	token, err := d.token(ctx, repo)
	if err != nil {
		return "", false, err
	}
	u := fmt.Sprintf("%s/v2/%s/manifests/%s", d.RegistryURL, repoPath(repo), tag)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json")
	resp, err := d.Client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
		digest := resp.Header.Get("Docker-Content-Digest")
		if !digestPattern.MatchString(digest) {
			return "", false, errors.New("registry answered without a content digest")
		}
		return digest, true, nil
	case http.StatusNotFound:
		return "", false, nil
	default:
		return "", false, fmt.Errorf("registry answered %d", resp.StatusCode)
	}
}

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func (d *DockerHubResolver) ImageMetadata(ctx context.Context, image string) (env, history []string, err error) {
	ref, digest, pinned := strings.Cut(image, "@")
	repo, _, tagged := strings.Cut(ref, ":")
	if !pinned || !tagged || !exactRepos[repo] || !digestPattern.MatchString(digest) {
		return nil, nil, fmt.Errorf("%s is not a pinned reviewed runtime image", image)
	}
	env, history, err = registry.ReadImageRuntimeMetadata(ctx, d.Client, d.RegistryURL, repoPath(repo), digest, "")
	if err == nil && len(env) == 0 {
		return nil, nil, fmt.Errorf("pinned runtime image %s has no environment", image)
	}
	return env, history, err
}

const (
	// tagPageSize is the Hub API's largest page; maxTagPages bounds a listing.
	// A prefix as narrow as an Elixir and OTP release (1.19.5-erlang-28.3)
	// names a few hundred hexpm builds across every OS and build date.
	tagPageSize  = 100
	maxTagPages  = 30
	maxPageBytes = 4 << 20
)

// ListTags implements ExactImageResolver through the Hub API's tag search,
// which answers in about a second where the registry's own sorted listing
// takes tens of seconds to seek into a repository as large as hexpm/elixir.
// The search matches a substring, so only the tags that begin with prefix
// are kept.
func (d *DockerHubResolver) ListTags(ctx context.Context, repo, prefix string) ([]string, error) {
	if !exactRepos[repo] || !tagPattern.MatchString(prefix) {
		return nil, fmt.Errorf("%s:%s is not a reviewed runtime image tag prefix", repo, prefix)
	}
	path := repoPath(repo)
	var tags []string
	for page := 1; page <= maxTagPages; page++ {
		q := url.Values{"name": {prefix}, "page_size": {strconv.Itoa(tagPageSize)}, "page": {strconv.Itoa(page)}}
		names, more, err := d.tagPage(ctx, fmt.Sprintf("%s/v2/repositories/%s/tags?%s", d.HubURL, path, q.Encode()))
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			if strings.HasPrefix(name, prefix) {
				tags = append(tags, name)
			}
		}
		if !more {
			return tags, nil
		}
	}
	return nil, fmt.Errorf("%s has more than %d tags matching %s", repo, maxTagPages*tagPageSize, prefix)
}

// tagPage reads one page of a Hub tag search; more says another page follows.
func (d *DockerHubResolver) tagPage(ctx context.Context, u string) (names []string, more bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, false, err
	}
	resp, err := d.Client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("tag listing answered %d for %s", resp.StatusCode, u)
	}
	var body struct {
		Next    *string `json:"next"`
		Results []struct {
			Name string `json:"name"`
		} `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxPageBytes)).Decode(&body); err != nil {
		return nil, false, fmt.Errorf("tag listing: %w", err)
	}
	for _, r := range body.Results {
		names = append(names, r.Name)
	}
	return names, body.Next != nil && *body.Next != "" && len(body.Results) > 0, nil
}

func (d *DockerHubResolver) token(ctx context.Context, repo string) (string, error) {
	q := url.Values{"service": {"registry.docker.io"}, "scope": {"repository:" + repoPath(repo) + ":pull"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.AuthURL+"?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := d.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry token endpoint answered %d", resp.StatusCode)
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxTokenBytes)).Decode(&body); err != nil || body.Token == "" {
		return "", errors.New("registry token endpoint answered without a token")
	}
	return body.Token, nil
}
