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
	"net"
	"net/http"
	"strings"

	boundedhttp "github.com/bex-co/bex/lego/operator/internal/httpclient"
	"github.com/bex-co/bex/lego/operator/internal/identity"
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

// ClusterLocal reports whether a registry host is the in-cluster or local-dev
// endpoint that legitimately speaks plain HTTP. Everything else is treated as a
// real registry whose traffic must be encrypted.
//
// The signal is the hostname rather than a scheme because BEX_REGISTRY carries
// no scheme (it is a host:port such as "zot.bex-registry.svc:5000"). This is the
// operator's single answer to "may this host be reached in the clear" — the
// build plane's skopeo TLS flags and the digest client's base URL below both
// read it, so the two halves cannot disagree about the same BEX_REGISTRY value.
func ClusterLocal(registryHost string) bool {
	host := strings.TrimPrefix(strings.TrimPrefix(registryHost, "http://"), "https://")
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(strings.Trim(host, "[]"))
	switch host {
	case "", "localhost", "127.0.0.1", "::1":
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	for _, suffix := range []string{".svc", ".svc.cluster.local", ".cluster.local", ".local", ".internal"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	// A single-label host (no dot) cannot be a public DNS name; it is a
	// cluster-local Service short name or a dev alias.
	return !strings.Contains(host, ".")
}

// NormalizeBase turns a configured registry host into a request base URL. An
// explicit scheme is honored as given; a bare host defaults to http:// only
// when it is cluster-local (the in-cluster Zot default) and to https://
// otherwise. Shared so the digest read and the repo teardown cannot disagree on
// what a bare host means.
func NormalizeBase(registryHost string) string {
	if strings.HasPrefix(registryHost, "http://") || strings.HasPrefix(registryHost, "https://") {
		return registryHost
	}
	if ClusterLocal(registryHost) {
		return "http://" + registryHost
	}
	return "https://" + registryHost
}

// CredentialedBase is NormalizeBase for a request that will carry registry
// credentials. It fails closed rather than silently putting an Authorization
// header on the wire in the clear: a plaintext base is allowed only for a
// cluster-local host, which is reached over cluster networking. The only way to
// reach this error is an explicit `http://` BEX_REGISTRY naming an off-cluster
// registry, which is a configuration mistake with no safe interpretation.
func CredentialedBase(registryHost string) (string, error) {
	base := NormalizeBase(registryHost)
	if strings.HasPrefix(base, "http://") && !ClusterLocal(registryHost) {
		return "", fmt.Errorf("registry %q is not cluster-local: refusing to send credentials over plaintext HTTP "+
			"(set BEX_REGISTRY to an https:// URL)", registryHost)
	}
	return base, nil
}

// baseFor picks the request base URL for a call that carries credentials only
// when username is set — an anonymous read (the dev default) discloses nothing,
// so it keeps NormalizeBase's plain answer.
func baseFor(registryHost, username string) (string, error) {
	if username == "" {
		return NormalizeBase(registryHost), nil
	}
	return CredentialedBase(registryHost)
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
	base, err := baseFor(registryHost, username)
	if err != nil {
		return "", err
	}
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

// ListTags returns the tags currently published on repo. A missing repository
// (404) is an empty list so a dry-run of an App with no legacy images still
// prints a plan.
func ListTags(ctx context.Context, httpClient *http.Client, registryHost, repo, username, password string) ([]string, error) {
	requestCtx, cancel := boundedhttp.WithTimeout(ctx)
	defer cancel()
	ctx = requestCtx
	if httpClient == nil {
		httpClient = defaultHTTPClient
	}
	base, err := baseFor(registryHost, username)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v2/%s/tags/list", base, repo), nil)
	if err != nil {
		return nil, err
	}
	if username != "" {
		req.SetBasicAuth(username, password)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("tags list %s/%s: status %d", registryHost, repo, resp.StatusCode)
	}
	var body struct {
		Tags []string `json:"tags"`
	}
	if err := boundedhttp.DecodeJSON(resp.Body, &body); err != nil {
		return nil, fmt.Errorf("tags list %s/%s: %w", registryHost, repo, err)
	}
	return body.Tags, nil
}

// ResolveBuiltDigestFor resolves id's freshly pushed tag with the App's own
// per-App credential — the same identity EnsureActiveFor just proved the
// registry accepts, so a resolution cannot fail on auth that pulls would pass.
func (c *Creds) ResolveBuiltDigestFor(ctx context.Context, id identity.Identity, appNS, tag string) (string, error) {
	password, err := c.readPasswordFor(ctx, id, appNS)
	if err != nil {
		return "", fmt.Errorf("read credential for digest resolution: %w", err)
	}
	return ResolveDigest(ctx, c.HTTPClient, c.Registry, id.Repo(), tag, id.ZotUsername(), password)
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
