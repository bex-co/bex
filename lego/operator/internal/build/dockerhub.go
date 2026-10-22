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
	"time"
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

// ExactImageResolver resolves an official-image tag to its content digest.
type ExactImageResolver interface {
	// ResolveOfficialTag returns the manifest digest ("sha256:…") of
	// docker.io/library/<repo>:<tag>; ok is false when the tag does not exist.
	ResolveOfficialTag(ctx context.Context, repo, tag string) (digest string, ok bool, err error)
}

// exactRepos are the official repositories an exact version may name.
var exactRepos = map[string]bool{nativePythonRuntime: true, nativeNodeRuntime: true, nativeRubyRuntime: true, nativeElixirRuntime: true}

var tagPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// DockerHubResolver reads digests from Docker Hub's registry API anonymously.
type DockerHubResolver struct {
	Client      *http.Client
	AuthURL     string // token endpoint; defaults to Docker Hub's
	RegistryURL string // registry base; defaults to Docker Hub's
}

// NewDockerHubResolver is the production resolver.
func NewDockerHubResolver() *DockerHubResolver {
	return &DockerHubResolver{
		Client:      &http.Client{Timeout: 10 * time.Second},
		AuthURL:     "https://auth.docker.io/token",
		RegistryURL: "https://registry-1.docker.io",
	}
}

const maxTokenBytes = 64 << 10

// ResolveOfficialTag implements ExactImageResolver.
func (d *DockerHubResolver) ResolveOfficialTag(ctx context.Context, repo, tag string) (string, bool, error) {
	if !exactRepos[repo] || !tagPattern.MatchString(tag) {
		return "", false, fmt.Errorf("%s:%s is not an official runtime image tag", repo, tag)
	}
	token, err := d.token(ctx, repo)
	if err != nil {
		return "", false, err
	}
	u := fmt.Sprintf("%s/v2/library/%s/manifests/%s", d.RegistryURL, repo, tag)
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

func (d *DockerHubResolver) token(ctx context.Context, repo string) (string, error) {
	q := url.Values{"service": {"registry.docker.io"}, "scope": {"repository:library/" + repo + ":pull"}}
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
