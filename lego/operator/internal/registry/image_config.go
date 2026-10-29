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
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	boundedhttp "github.com/bex-co/bex/lego/operator/internal/httpclient"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ImageConfig is verified metadata for an immutable image reference, with its
// Linux/amd64 config selected for the current hosting node pool.
type ImageConfig struct {
	// Digest identifies the requested reference: the outer index for a multiarch
	// image, or the manifest itself for a single-platform image.
	Digest string
	// PlatformDigest identifies the verified Linux/amd64 manifest whose config
	// supplied TCPPorts. It equals Digest for a single-platform reference.
	PlatformDigest string
	TCPPorts       []int32
}

// TransientError is a registry answer a later read may not repeat: a rate
// limit (429), a server error (5xx) or a failed request (Status 0). A deploy
// waits on it rather than failing (w4/m243 t008). RetryAfter is the
// registry's Retry-After, zero when it named none.
type TransientError struct {
	Status     int
	RetryAfter time.Duration
	err        error
}

func (e *TransientError) Error() string { return e.err.Error() }
func (e *TransientError) Unwrap() error { return e.err }

// statusFailure classifies a non-200 registry answer: 429 and 5xx are
// transient, anything else is the registry's definitive refusal.
func statusFailure(what string, status int, header http.Header) error {
	err := fmt.Errorf("%s returned status %d", what, status)
	if status != http.StatusTooManyRequests && status < 500 {
		return err
	}
	retryAfter := time.Duration(0)
	value := strings.TrimSpace(header.Get("Retry-After"))
	if seconds, perr := strconv.Atoi(value); perr == nil && seconds > 0 {
		retryAfter = time.Duration(seconds) * time.Second
	} else if at, perr := http.ParseTime(value); perr == nil {
		retryAfter = max(time.Until(at), 0)
	}
	return &TransientError{Status: status, RetryAfter: retryAfter, err: err}
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

// maxBlobRedirects bounds the storage hops a config blob read may take.
const maxBlobRedirects = 3

// ReadImageConfig reads only the authorized repository, verifies both manifest
// and config digests, and never downloads layers. Manifest redirects are
// refused so the registry cannot redirect the operator's credentials or
// fetches elsewhere; a config blob may follow https storage redirects without
// the credential.
func ReadImageConfig(ctx context.Context, httpClient *http.Client, registryHost, repo, reference, authorization string) (ImageConfig, error) {
	digest, platformDigest, document, err := readImageConfigDocument(ctx, httpClient, registryHost, repo, reference, authorization)
	if err != nil {
		return ImageConfig{}, err
	}
	ports, err := privateTCPPorts(document.Config.ExposedPorts)
	if err != nil {
		return ImageConfig{}, err
	}
	return ImageConfig{Digest: digest, PlatformDigest: platformDigest, TCPPorts: ports}, nil
}

// RevisionLabel is the OCI annotation bex's builds stamp with the commit the
// image was built from (w5/m148).
const RevisionLabel = "org.opencontainers.image.revision"

// ReadImageRevision is ReadImageConfig for the image's RevisionLabel: the
// same authorized, digest-verified, layer-free read, without the port rules,
// which say nothing about provenance. "" when the image carries no label.
func ReadImageRevision(ctx context.Context, httpClient *http.Client, registryHost, repo, reference, authorization string) (string, error) {
	_, _, document, err := readImageConfigDocument(ctx, httpClient, registryHost, repo, reference, authorization)
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

func fetchOnce(ctx context.Context, client *http.Client, url, authorization, bearer string) ([]byte, int, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, nil, err
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
		return nil, 0, nil, &TransientError{err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, resp.Header, nil
	}
	data, err := boundedhttp.ReadAll(resp.Body)
	return data, resp.StatusCode, nil, err
}

// bearerToken answers a registry's Bearer challenge: GET the realm with the
// challenge's service/scope and return the issued token. basic is "" for a
// public image (w4/m243), or the workspace credential's Basic header for a
// private one (w6/095); like the registry request itself, it travels only over
// https. The realm must be https, or http on a loopback host (local registries
// in tests).
func bearerToken(ctx context.Context, client *http.Client, challenge, basic string) (string, error) {
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
	if basic != "" {
		req.Header.Set("Authorization", basic)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", &TransientError{err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", statusFailure("registry token request", resp.StatusCode, resp.Header)
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

// blobRedirectPolicy bounds blob-storage redirects (w8/201): a short https
// chain, and the registry credential never travels to the storage host.
func blobRedirectPolicy(req *http.Request, via []*http.Request) error {
	if len(via) > maxBlobRedirects {
		return fmt.Errorf("image metadata blob redirected more than %d times", maxBlobRedirects)
	}
	if req.URL.Scheme != "https" {
		return errors.New("image metadata blob redirected to a non-https URL")
	}
	req.Header.Del("Authorization")
	return nil
}

// exchangesForBearer reports whether a 401 is answered with a token: always
// for an anonymous read, and for a Basic credential only when the registry
// asks for a Bearer token instead.
func exchangesForBearer(authorization, challenge string) bool {
	return authorization == "" || strings.HasPrefix(authorization, "Basic ") && strings.HasPrefix(challenge, "Bearer ")
}

func readImageConfigDocument(ctx context.Context, httpClient *http.Client, registryHost, repo, reference, authorization string) (string, string, imageConfigDocument, error) {
	base, err := imageRepoBase(registryHost, repo, reference, authorization)
	if err != nil {
		return "", "", imageConfigDocument{}, err
	}
	ctx, cancel := boundedhttp.WithTimeout(ctx)
	defer cancel()
	if httpClient == nil {
		httpClient = defaultHTTPClient
	}
	bounded := *httpClient
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	// Registries serve blobs from storage: Docker Hub, GHCR, ECR and GCR all
	// answer a blob GET with a redirect to a CDN or presigned URL (w8/201).
	// A blob is content-addressed and digest-verified below, so following an
	// https redirect cannot substitute content; the registry credential never
	// travels with it (presigned URLs carry their own authorization).
	blobs := bounded
	blobs.CheckRedirect = blobRedirectPolicy
	// One token serves every manifest and blob read of this repository.
	bearer := ""
	fetch := func(client *http.Client, path, expectedDigest string) ([]byte, string, error) {
		exchanged := false
		for {
			data, status, header, err := fetchOnce(ctx, client, base+path, authorization, bearer)
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
			// A registry may answer with a Bearer challenge: an anonymous read
			// (w4/m243) fetches a token without credentials, a Basic-credentialed
			// read (w6/095) exchanges its credential at the realm, then retries
			// once. A registry that accepts Basic directly never challenges.
			challenge := header.Get("WWW-Authenticate")
			if status == http.StatusUnauthorized && !exchanged && exchangesForBearer(authorization, challenge) {
				token, terr := bearerToken(ctx, &bounded, challenge, authorization)
				if terr != nil {
					return nil, "", terr
				}
				bearer, exchanged = token, true
				continue
			}
			return nil, "", statusFailure("image metadata request", status, header)
		}
	}
	// The first verified response is the digest users see when pinning a tag.
	// Keep it even while traversing nested indexes to the platform manifest.
	requestedDigest := ""
	for range 3 {
		expected := ""
		if imageDigestPattern.MatchString(reference) {
			expected = reference
		}
		body, digest, err := fetch(&bounded, "manifests/"+reference, expected)
		if err != nil {
			return "", "", imageConfigDocument{}, err
		}
		requestedDigest = cmp.Or(requestedDigest, digest)
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
			return "", "", imageConfigDocument{}, fmt.Errorf("decode image manifest: %w", err)
		}
		if manifest.SchemaVersion != 2 {
			return "", "", imageConfigDocument{}, fmt.Errorf("unsupported image manifest schema")
		}
		switch manifest.MediaType {
		case "application/vnd.oci.image.index.v1+json", "application/vnd.docker.distribution.manifest.list.v2+json":
			reference = ""
			for _, child := range manifest.Manifests {
				if child.Platform.OS == "linux" && child.Platform.Architecture == "amd64" {
					if !imageDigestPattern.MatchString(child.Digest) {
						return "", "", imageConfigDocument{}, fmt.Errorf("invalid image manifest digest")
					}
					reference = child.Digest
					break
				}
			}
			if reference == "" {
				return "", "", imageConfigDocument{}, fmt.Errorf("image has no Linux/amd64 manifest")
			}
			continue
		case "application/vnd.oci.image.manifest.v1+json", "application/vnd.docker.distribution.manifest.v2+json":
		default:
			return "", "", imageConfigDocument{}, fmt.Errorf("unsupported image manifest media type")
		}
		if !imageDigestPattern.MatchString(manifest.Config.Digest) {
			return "", "", imageConfigDocument{}, fmt.Errorf("invalid image config digest")
		}
		config, _, err := fetch(&blobs, "blobs/"+manifest.Config.Digest, manifest.Config.Digest)
		if err != nil {
			return "", "", imageConfigDocument{}, err
		}
		var document imageConfigDocument
		if err := json.Unmarshal(config, &document); err != nil {
			return "", "", imageConfigDocument{}, fmt.Errorf("decode image config: %w", err)
		}
		if document.OS != "linux" || document.Architecture != "amd64" {
			return "", "", imageConfigDocument{}, fmt.Errorf("image config is not Linux/amd64")
		}
		return requestedDigest, digest, document, nil
	}
	return "", "", imageConfigDocument{}, fmt.Errorf("image index nesting exceeds the supported limit")
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
