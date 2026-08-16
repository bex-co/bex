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

package registry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	boundedhttp "github.com/bex-co/bex/lego/operator/internal/httpclient"
)

// Tag→digest resolution (w9/013). The build plane tags tenant images with the
// mutable per-generation `gen-N`, which a delete+recreate of the same App name
// RESETS — so the same tag can name two different images over time, and a node
// that cached the old `gen-1` can silently run stale code. After a build
// completes, the operator resolves the tag it just pushed to the immutable
// manifest digest the registry now serves and references the image as
// `<repo>:<tag>@<digest>` everywhere (Deployment, CronJob, static-site publish
// Job), so kubelet pulls exactly the built bytes regardless of tag history.

// manifestAccept lists the manifest media types the digest HEAD accepts —
// without these the registry answers 404 for OCI/list manifests.
const manifestAccept = "application/vnd.oci.image.manifest.v1+json, " +
	"application/vnd.oci.image.index.v1+json, " +
	"application/vnd.docker.distribution.manifest.v2+json, " +
	"application/vnd.docker.distribution.manifest.list.v2+json"

// NormalizeBase turns a configured registry host into a request base URL,
// defaulting to plain HTTP when no scheme is given (the in-cluster Zot default).
// Shared so the digest read and the repo teardown cannot disagree on what a
// bare host means.
func NormalizeBase(registryHost string) string {
	if strings.HasPrefix(registryHost, "http://") || strings.HasPrefix(registryHost, "https://") {
		return registryHost
	}
	return "http://" + registryHost
}

// ResolveDigest asks the registry which immutable manifest digest it currently
// serves for repo:tag (HEAD /v2/<repo>/manifests/<tag> → Docker-Content-Digest).
// username may be empty for an unauthenticated registry (the dev default).
// Callers invoke this immediately after a successful build push, so the answer
// is the digest of exactly that push.
func ResolveDigest(ctx context.Context, httpClient *http.Client, registryHost, repo, tag, username, password string) (string, error) {
	requestCtx, cancel := boundedhttp.WithTimeout(ctx)
	defer cancel()
	ctx = requestCtx
	if httpClient == nil {
		httpClient = defaultHTTPClient
	}
	base := NormalizeBase(registryHost)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead,
		fmt.Sprintf("%s/v2/%s/manifests/%s", base, repo, tag), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", manifestAccept)
	if username != "" {
		req.SetBasicAuth(username, password)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("manifest HEAD %s/%s:%s: status %d", registryHost, repo, tag, resp.StatusCode)
	}
	digest := resp.Header.Get("Docker-Content-Digest")
	if !strings.HasPrefix(digest, "sha256:") {
		return "", fmt.Errorf("manifest HEAD %s/%s:%s: missing or malformed Docker-Content-Digest %q", registryHost, repo, tag, digest)
	}
	return digest, nil
}

// ResolveBuiltDigest resolves appName's freshly pushed tag with the App's own
// per-App credential — the same identity EnsureActive just proved the registry
// accepts, so a resolution cannot fail on auth that pulls would pass.
func (c *Creds) ResolveBuiltDigest(ctx context.Context, appName, appNS, tag string) (string, error) {
	password, err := c.readPassword(ctx, appName, appNS)
	if err != nil {
		return "", fmt.Errorf("read credential for digest resolution: %w", err)
	}
	return ResolveDigest(ctx, c.HTTPClient, c.Registry, appName, tag, ZotUsername(appName), password)
}

// BasicAuthFromDockerConfig extracts the basic-auth pair a dockerconfigjson
// holds for host. ok is false when the config has no entry for host — callers
// then resolve anonymously (the dev default).
func BasicAuthFromDockerConfig(configJSON []byte, host string) (username, password string, ok bool) {
	var cfg struct {
		Auths map[string]struct {
			Auth     string `json:"auth"`
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return "", "", false
	}
	entry, found := cfg.Auths[host]
	if !found {
		return "", "", false
	}
	if entry.Auth != "" {
		decoded, err := base64.StdEncoding.DecodeString(entry.Auth)
		if err == nil {
			if u, p, split := strings.Cut(string(decoded), ":"); split {
				return u, p, true
			}
		}
	}
	if entry.Username != "" {
		return entry.Username, entry.Password, true
	}
	return "", "", false
}
