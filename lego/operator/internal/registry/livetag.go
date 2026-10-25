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

// TagManifest pushes repo's manifest digest under tag, server-side: the
// manifest is read by digest and PUT back unchanged, so no blob moves and the
// digest stays the same. authorization is the full Authorization value (""
// for an anonymous registry); like ReadImageConfig it is never sent over
// plaintext to an off-cluster host, and redirects are refused so it cannot be
// forwarded elsewhere.
func TagManifest(ctx context.Context, httpClient *http.Client, registryHost, repo, digest, tag, authorization string) error {
	base, err := baseFor(registryHost, authorization)
	if err != nil {
		return err
	}
	ctx, cancel := boundedhttp.WithTimeout(ctx)
	defer cancel()
	if httpClient == nil {
		httpClient = defaultHTTPClient
	}
	bounded := *httpClient
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	do := func(method, ref, contentType string, body []byte) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s/v2/%s/manifests/%s", base, repo, ref), bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		if method == http.MethodGet {
			req.Header.Set("Accept", manifestAccept)
		} else {
			req.Header.Set("Content-Type", contentType)
		}
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		return bounded.Do(req)
	}
	resp, err := do(http.MethodGet, digest, "", nil)
	if err != nil {
		return fmt.Errorf("get manifest %s: %w", digest, err)
	}
	manifest, err := boundedhttp.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return fmt.Errorf("read manifest %s: %w", digest, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get manifest %s: status %d: %s", digest, resp.StatusCode, manifest)
	}
	resp, err = do(http.MethodPut, tag, resp.Header.Get("Content-Type"), manifest)
	if err != nil {
		return fmt.Errorf("put manifest %s as %s: %w", digest, tag, err)
	}
	body, _ := boundedhttp.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("put manifest %s as %s: status %d: %s", digest, tag, resp.StatusCode, body)
	}
	return nil
}
