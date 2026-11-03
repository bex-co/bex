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
	"bytes"
	"context"
	"fmt"
	"net/http"

	boundedhttp "github.com/bex-co/bex/lego/operator/internal/httpclient"
)

// manifestRequester issues registry API requests against registryHost with
// authorization (the full Authorization value, "" for an anonymous registry).
// Like ReadImageConfig it never sends credentials over plaintext to an
// off-cluster host, and redirects are refused so they cannot be forwarded.
// Each request gets its own bounded deadline, so a multi-request operation
// (a copy mounts every blob) is not starved by one shared budget.
type manifestRequester struct {
	ctx           context.Context
	client        http.Client
	base          string
	authorization string
}

func newManifestRequester(ctx context.Context, httpClient *http.Client, registryHost, authorization string) (*manifestRequester, error) {
	base, err := baseFor(registryHost, authorization)
	if err != nil {
		return nil, err
	}
	if httpClient == nil {
		httpClient = defaultHTTPClient
	}
	bounded := *httpClient
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &manifestRequester{ctx: ctx, client: bounded, base: base, authorization: authorization}, nil
}

// registryResponse is a fully read response.
type registryResponse struct {
	status int
	header http.Header
	body   []byte
}

func (m *manifestRequester) do(method, path, contentType string, body []byte) (registryResponse, error) {
	ctx, cancel := boundedhttp.WithTimeout(m.ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, m.base+path, bytes.NewReader(body))
	if err != nil {
		return registryResponse{}, err
	}
	if method == http.MethodGet {
		req.Header.Set("Accept", manifestAccept)
	} else if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if m.authorization != "" {
		req.Header.Set("Authorization", m.authorization)
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return registryResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	read, err := boundedhttp.ReadAll(resp.Body)
	if err != nil {
		return registryResponse{}, fmt.Errorf("read response: %w", err)
	}
	return registryResponse{status: resp.StatusCode, header: resp.Header, body: read}, nil
}

// getManifest reads repo's manifest at ref, returning its bytes and media type.
func (m *manifestRequester) getManifest(repo, ref string) ([]byte, string, error) {
	resp, err := m.do(http.MethodGet, fmt.Sprintf("/v2/%s/manifests/%s", repo, ref), "", nil)
	if err != nil {
		return nil, "", fmt.Errorf("get manifest %s: %w", ref, err)
	}
	if resp.status != http.StatusOK {
		return nil, "", fmt.Errorf("get manifest %s: status %d: %s", ref, resp.status, resp.body)
	}
	return resp.body, resp.header.Get("Content-Type"), nil
}

// putManifest stores manifest in repo under ref.
func (m *manifestRequester) putManifest(repo, ref, mediaType string, manifest []byte) error {
	resp, err := m.do(http.MethodPut, fmt.Sprintf("/v2/%s/manifests/%s", repo, ref), mediaType, manifest)
	if err != nil {
		return fmt.Errorf("put manifest %s: %w", ref, err)
	}
	if resp.status != http.StatusCreated && resp.status != http.StatusOK {
		return fmt.Errorf("put manifest %s: status %d: %s", ref, resp.status, resp.body)
	}
	return nil
}

// TagManifest pushes repo's manifest digest under tag, server-side: the
// manifest is read by digest and PUT back unchanged, so no blob moves and the
// digest stays the same.
func TagManifest(ctx context.Context, httpClient *http.Client, registryHost, repo, digest, tag, authorization string) error {
	m, err := newManifestRequester(ctx, httpClient, registryHost, authorization)
	if err != nil {
		return err
	}
	manifest, mediaType, err := m.getManifest(repo, digest)
	if err != nil {
		return err
	}
	if err := m.putManifest(repo, tag, mediaType, manifest); err != nil {
		return fmt.Errorf("tag %s as %s: %w", digest, tag, err)
	}
	return nil
}
