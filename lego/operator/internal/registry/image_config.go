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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"

	boundedhttp "github.com/bex-co/bex/lego/operator/internal/httpclient"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ImageConfig is verified metadata for an immutable Linux/amd64 manifest.
// The hosting node pool and its source builds currently target this platform.
type ImageConfig struct {
	Digest   string
	TCPPorts []int32
}

var imageDigestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

// IsDigest reports whether s is a sha256 content digest, "sha256:<64 hex>".
func IsDigest(s string) bool { return imageDigestPattern.MatchString(s) }

var imageTagPattern = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,127}$`)
var imageRepoPattern = regexp.MustCompile(`^[a-z0-9]+(?:[._-]+[a-z0-9]+)*(?:/[a-z0-9]+(?:[._-]+[a-z0-9]+)*)*$`)

// imageRepoBase validates the reference before any request and refuses to
// carry credentials over plaintext HTTP to an off-cluster registry.
func imageRepoBase(registryHost, repo, reference, authorization string) (string, error) {
	if !imageRepoPattern.MatchString(repo) || (!imageDigestPattern.MatchString(reference) && !imageTagPattern.MatchString(reference)) {
		return "", fmt.Errorf("invalid image metadata reference")
	}
	base, err := baseFor(registryHost, authorization)
	if err != nil {
		return "", err
	}
	return base + "/v2/" + repo + "/", nil
}

// ReadImageConfig reads only the authorized repository, verifies both manifest
// and config digests, and never downloads layers. Redirects are refused so the
// registry cannot redirect the operator's credentials or fetches elsewhere.
func ReadImageConfig(ctx context.Context, httpClient *http.Client, registryHost, repo, reference, authorization string) (ImageConfig, error) {
	digest, document, err := readImageConfigDocument(ctx, httpClient, registryHost, repo, reference, authorization)
	if err != nil {
		return ImageConfig{}, err
	}
	ports, err := privateTCPPorts(document.Config.ExposedPorts)
	if err != nil {
		return ImageConfig{}, err
	}
	return ImageConfig{Digest: digest, TCPPorts: ports}, nil
}

// RevisionLabel is the OCI annotation bex's builds stamp with the commit the
// image was built from (w5/m148).
const RevisionLabel = "org.opencontainers.image.revision"

// ReadImageRevision is ReadImageConfig for the image's RevisionLabel: the
// same authorized, digest-verified, layer-free read, without the port rules,
// which say nothing about provenance. "" when the image carries no label.
func ReadImageRevision(ctx context.Context, httpClient *http.Client, registryHost, repo, reference, authorization string) (string, error) {
	_, document, err := readImageConfigDocument(ctx, httpClient, registryHost, repo, reference, authorization)
	if err != nil {
		return "", err
	}
	return document.Config.Labels[RevisionLabel], nil
}

type imageConfigDocument struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Config       struct {
		ExposedPorts map[string]json.RawMessage `json:"ExposedPorts"`
		Labels       map[string]string          `json:"Labels"`
	} `json:"config"`
}

func fetchOnce(ctx context.Context, client *http.Client, url, authorization, bearer string) ([]byte, int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, "", err
	}
	req.Header.Set("Accept", manifestAccept)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, resp.Header.Get("WWW-Authenticate"), nil
	}
	data, err := boundedhttp.ReadAll(resp.Body)
	return data, resp.StatusCode, "", err
}

// anonymousBearerToken answers a registry's Bearer challenge without any
// credential (public images, w4/m243): GET the realm with the challenge's
// service/scope and return the issued token. The realm must be https, or
// http on a loopback host (local registries in tests).
func anonymousBearerToken(ctx context.Context, client *http.Client, challenge string) (string, error) {
	if !strings.HasPrefix(challenge, "Bearer ") {
		return "", fmt.Errorf("image metadata request returned status 401 without a Bearer challenge")
	}
	params := map[string]string{}
	for part := range strings.SplitSeq(strings.TrimPrefix(challenge, "Bearer "), ",") {
		k, v, ok := strings.Cut(part, "=")
		if ok {
			params[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	realm := params["realm"]
	if realm == "" {
		return "", fmt.Errorf("registry Bearer challenge has no realm")
	}
	if !strings.HasPrefix(realm, "https://") && !strings.HasPrefix(realm, "http://127.0.0.1") && !strings.HasPrefix(realm, "http://localhost") {
		return "", fmt.Errorf("registry Bearer realm is not https: %q", realm)
	}
	sep := "?"
	if strings.Contains(realm, "?") {
		sep = "&"
	}
	url := realm
	if params["service"] != "" {
		url += sep + "service=" + params["service"]
		sep = "&"
	}
	if params["scope"] != "" {
		url += sep + "scope=" + params["scope"]
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry token request returned status %d", resp.StatusCode)
	}
	data, err := boundedhttp.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var body struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return "", fmt.Errorf("decode registry token: %w", err)
	}
	if body.Token != "" {
		return body.Token, nil
	}
	if body.AccessToken != "" {
		return body.AccessToken, nil
	}
	return "", fmt.Errorf("registry token response has no token")
}

func readImageConfigDocument(ctx context.Context, httpClient *http.Client, registryHost, repo, reference, authorization string) (string, imageConfigDocument, error) {
	base, err := imageRepoBase(registryHost, repo, reference, authorization)
	if err != nil {
		return "", imageConfigDocument{}, err
	}
	ctx, cancel := boundedhttp.WithTimeout(ctx)
	defer cancel()
	if httpClient == nil {
		httpClient = defaultHTTPClient
	}
	bounded := *httpClient
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	fetch := func(path, expectedDigest string) ([]byte, string, error) {
		bearer := ""
		for attempt := 0; ; attempt++ {
			data, status, challenge, err := fetchOnce(ctx, &bounded, base+path, authorization, bearer)
			if err != nil {
				return nil, "", err
			}
			if status == http.StatusOK {
				digest := fmt.Sprintf("sha256:%x", sha256.Sum256(data))
				if expectedDigest != "" && digest != expectedDigest {
					return nil, "", fmt.Errorf("image metadata digest mismatch")
				}
				return data, digest, nil
			}
			// An anonymous read of a public registry (w4/m243) may be asked
			// for a bearer token: fetch one without credentials and retry
			// once. Credentialed reads never take this path.
			if status == http.StatusUnauthorized && authorization == "" && bearer == "" && attempt == 0 {
				token, terr := anonymousBearerToken(ctx, &bounded, challenge)
				if terr != nil {
					return nil, "", terr
				}
				bearer = token
				continue
			}
			return nil, "", fmt.Errorf("image metadata request returned status %d", status)
		}
	}
	for range 3 {
		expected := ""
		if imageDigestPattern.MatchString(reference) {
			expected = reference
		}
		body, digest, err := fetch("manifests/"+reference, expected)
		if err != nil {
			return "", imageConfigDocument{}, err
		}
		var manifest struct {
			SchemaVersion int    `json:"schemaVersion"`
			MediaType     string `json:"mediaType"`
			Config        struct {
				Digest string `json:"digest"`
			} `json:"config"`
			Manifests []struct {
				Digest   string `json:"digest"`
				Platform struct {
					OS           string `json:"os"`
					Architecture string `json:"architecture"`
				} `json:"platform"`
			} `json:"manifests"`
		}
		if err := json.Unmarshal(body, &manifest); err != nil {
			return "", imageConfigDocument{}, fmt.Errorf("decode image manifest: %w", err)
		}
		if manifest.SchemaVersion != 2 {
			return "", imageConfigDocument{}, fmt.Errorf("unsupported image manifest schema")
		}
		switch manifest.MediaType {
		case "application/vnd.oci.image.index.v1+json", "application/vnd.docker.distribution.manifest.list.v2+json":
			reference = ""
			for _, child := range manifest.Manifests {
				if child.Platform.OS == "linux" && child.Platform.Architecture == "amd64" {
					if !imageDigestPattern.MatchString(child.Digest) {
						return "", imageConfigDocument{}, fmt.Errorf("invalid image manifest digest")
					}
					reference = child.Digest
					break
				}
			}
			if reference == "" {
				return "", imageConfigDocument{}, fmt.Errorf("image has no Linux/amd64 manifest")
			}
			continue
		case "application/vnd.oci.image.manifest.v1+json", "application/vnd.docker.distribution.manifest.v2+json":
		default:
			return "", imageConfigDocument{}, fmt.Errorf("unsupported image manifest media type")
		}
		if !imageDigestPattern.MatchString(manifest.Config.Digest) {
			return "", imageConfigDocument{}, fmt.Errorf("invalid image config digest")
		}
		config, _, err := fetch("blobs/"+manifest.Config.Digest, manifest.Config.Digest)
		if err != nil {
			return "", imageConfigDocument{}, err
		}
		var document imageConfigDocument
		if err := json.Unmarshal(config, &document); err != nil {
			return "", imageConfigDocument{}, fmt.Errorf("decode image config: %w", err)
		}
		if document.OS != "linux" || document.Architecture != "amd64" {
			return "", imageConfigDocument{}, fmt.Errorf("image config is not Linux/amd64")
		}
		return digest, document, nil
	}
	return "", imageConfigDocument{}, fmt.Errorf("image index nesting exceeds the supported limit")
}

func privateTCPPorts(exposed map[string]json.RawMessage) ([]int32, error) {
	ports := make([]int32, 0, len(exposed))
	for key := range exposed {
		number, protocol, ok := strings.Cut(key, "/")
		if !ok || (protocol != "tcp" && protocol != "udp") {
			return nil, fmt.Errorf("invalid exposed port declaration")
		}
		port, err := strconv.ParseInt(number, 10, 32)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("exposed port is outside 1-65535")
		}
		if protocol == "udp" {
			continue
		}
		if appv1alpha1.IsReservedImagePort(int32(port)) {
			return nil, fmt.Errorf("exposed TCP port %d is reserved", port)
		}
		ports = append(ports, int32(port))
	}
	slices.Sort(ports)
	ports = slices.Compact(ports)
	if len(ports) > appv1alpha1.MaxImagePorts {
		return nil, fmt.Errorf("private services support at most %d exposed TCP ports", appv1alpha1.MaxImagePorts)
	}
	return ports, nil
}
