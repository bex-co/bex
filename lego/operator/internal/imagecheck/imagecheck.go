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

// Package imagecheck verifies cosign private-key signatures on OCI images.
// It contacts the OCI registry directly using the Distribution Spec v1 HTTP
// API; no cosign binary or sigstore Go library is needed. Only private-key
// (non-keyless) signatures are supported — keyless Rekor-based signatures are
// out of scope because in-cluster tenant images are signed with an injected key
// (BEX_TENANT_SIGNING_KEY_SECRET, w6/006) and the Zot registry is internal.
package imagecheck

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// cosignSigAnnotation is the manifest-layer annotation that holds the
// base64-encoded DER ECDSA signature created by cosign sign --key.
const cosignSigAnnotation = "dev.cosignproject.cosign/signature"

// Verifier verifies cosign private-key signatures on OCI images.
// It is safe for concurrent use.
type Verifier struct {
	pub    *ecdsa.PublicKey
	client *http.Client
	scheme string // "http" or "https"
	auth   string // "username:password" or "" for unauthenticated
}

// NewVerifier parses pemPublicKey (PEM-encoded EC public key — the cosign.pub
// file produced by cosign generate-key-pair) and returns a Verifier.
//
// scheme is "http" or "https" for registry connections; defaults to "https".
// auth is "username:password" for Basic auth, or "" for unauthenticated.
func NewVerifier(pemPublicKey []byte, scheme, auth string) (*Verifier, error) {
	block, _ := pem.Decode(pemPublicKey)
	if block == nil {
		return nil, fmt.Errorf("imagecheck: no PEM block in public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("imagecheck: parse public key: %w", err)
	}
	ecPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("imagecheck: expected EC public key, got %T", pub)
	}
	if scheme == "" {
		scheme = "https"
	}
	return &Verifier{pub: ecPub, client: &http.Client{}, scheme: scheme, auth: auth}, nil
}

// Verify returns nil if imageRef has at least one valid cosign signature under
// the verifier's public key. imageRef must include the registry host
// (e.g. "zot.bex-registry.svc:5000/myapp:rev-abc").
func (v *Verifier) Verify(ctx context.Context, imageRef string) error {
	host, repo, ref, err := parseRef(imageRef)
	if err != nil {
		return fmt.Errorf("imagecheck: %w", err)
	}

	var digest string
	if isDigest(ref) {
		digest = ref
	} else {
		digest, err = v.resolveDigest(ctx, host, repo, ref)
		if err != nil {
			return fmt.Errorf("imagecheck: resolve %q: %w", imageRef, err)
		}
	}

	// cosign stores signatures at <repo>:<sha256-hexdigest>.sig
	sigTag := strings.ReplaceAll(digest, ":", "-") + ".sig"
	layers, err := v.fetchSigLayers(ctx, host, repo, sigTag)
	if err != nil {
		return fmt.Errorf("imagecheck: fetch signature for %q: %w", imageRef, err)
	}
	if len(layers) == 0 {
		return fmt.Errorf("imagecheck: no cosign signature layers for %q", imageRef)
	}

	var lastErr error
	for _, l := range layers {
		payload, err := v.fetchBlob(ctx, host, repo, l.digest)
		if err != nil {
			lastErr = err
			continue
		}
		if err := v.verifyECDSA(l.sig, payload); err != nil {
			lastErr = err
			continue
		}
		return nil // at least one valid signature found
	}
	if lastErr != nil {
		return fmt.Errorf("imagecheck: no valid signature for %q: %w", imageRef, lastErr)
	}
	return fmt.Errorf("imagecheck: no valid signature for %q", imageRef)
}

// RegistryAuth parses a Docker config.json blob (from a kubernetes.io/dockerconfigjson
// or generic Secret key) and returns the Basic Auth credentials ("user:pass")
// for the given registry host, or "" if the host is not present.
func RegistryAuth(configJSON []byte, host string) (string, error) {
	var cfg struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return "", fmt.Errorf("imagecheck: parse docker config: %w", err)
	}
	a, ok := cfg.Auths[host]
	if !ok {
		return "", nil
	}
	decoded, err := base64.StdEncoding.DecodeString(a.Auth)
	if err != nil {
		return "", fmt.Errorf("imagecheck: decode auth for %s: %w", host, err)
	}
	return string(decoded), nil
}

// --- internal types & helpers -------------------------------------------------

type sigLayer struct {
	digest string
	sig    []byte
}

func isDigest(ref string) bool {
	return strings.HasPrefix(ref, "sha256:")
}

// parseRef splits an OCI image reference into host, repository, and reference.
//
// Supported formats:
//
//	host/name:tag              → ref = "tag"
//	host/name@sha256:hex       → ref = "sha256:hex"
//	host/name:tag@sha256:hex   → ref = "sha256:hex" (digest wins)
//	host/org/name:tag          → host = "host", repo = "org/name", ref = "tag"
func parseRef(imageRef string) (host, repo, ref string, err error) {
	var rest string
	host, rest, _ = strings.Cut(imageRef, "/")
	if rest == "" {
		return "", "", "", fmt.Errorf("invalid image ref (no host): %q", imageRef)
	}

	// Digest reference takes precedence over any tag.
	if atIdx := strings.LastIndex(rest, "@"); atIdx >= 0 {
		repo = rest[:atIdx]
		if colonIdx := strings.Index(repo, ":"); colonIdx >= 0 {
			repo = repo[:colonIdx] // strip tag component from repo
		}
		ref = rest[atIdx+1:] // "sha256:hex"
		return
	}

	// Tag-only reference.
	if colonIdx := strings.LastIndex(rest, ":"); colonIdx >= 0 {
		repo = rest[:colonIdx]
		ref = rest[colonIdx+1:]
		return
	}

	// No tag — default to "latest".
	repo = rest
	ref = "latest"
	return
}

func (v *Verifier) baseURL(host string) string {
	return v.scheme + "://" + host
}

func (v *Verifier) newRequest(ctx context.Context, method, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	if v.auth != "" {
		req.Header.Set("Authorization",
			"Basic "+base64.StdEncoding.EncodeToString([]byte(v.auth)))
	}
	return req, nil
}

// resolveDigest fetches the manifest for ref and returns the content digest
// from the Docker-Content-Digest (or OCI-Digest) response header.
func (v *Verifier) resolveDigest(ctx context.Context, host, repo, ref string) (string, error) {
	url := fmt.Sprintf("%s/v2/%s/manifests/%s", v.baseURL(host), repo, ref)
	req, err := v.newRequest(ctx, http.MethodGet, url)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept",
		"application/vnd.oci.image.manifest.v1+json,"+
			"application/vnd.docker.distribution.manifest.v2+json")
	resp, err := v.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()        //nolint:errcheck
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry %s returned HTTP %d for %s/%s:%s",
			host, resp.StatusCode, host, repo, ref)
	}
	if d := resp.Header.Get("Docker-Content-Digest"); d != "" {
		return d, nil
	}
	if d := resp.Header.Get("OCI-Digest"); d != "" {
		return d, nil
	}
	return "", fmt.Errorf("registry %s did not return a content digest for %s/%s:%s",
		host, host, repo, ref)
}

type ociManifest struct {
	Layers []struct {
		Digest      string            `json:"digest"`
		Annotations map[string]string `json:"annotations"`
	} `json:"layers"`
}

// fetchSigLayers fetches the cosign signature OCI manifest at sigTag and
// returns layers that carry a cosign signature annotation.
func (v *Verifier) fetchSigLayers(ctx context.Context, host, repo, sigTag string) ([]sigLayer, error) {
	url := fmt.Sprintf("%s/v2/%s/manifests/%s", v.baseURL(host), repo, sigTag)
	req, err := v.newRequest(ctx, http.MethodGet, url)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.oci.image.manifest.v1+json")
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("signature tag %q not found in %s/%s (image is unsigned)",
			sigTag, host, repo)
	}
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		return nil, fmt.Errorf("registry %s returned HTTP %d for sig manifest %s/%s:%s",
			host, resp.StatusCode, host, repo, sigTag)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var m ociManifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("parse sig manifest: %w", err)
	}
	var out []sigLayer
	for _, l := range m.Layers {
		rawSig, ok := l.Annotations[cosignSigAnnotation]
		if !ok {
			continue
		}
		sigBytes, err := base64.StdEncoding.DecodeString(rawSig)
		if err != nil {
			continue // skip malformed annotation
		}
		out = append(out, sigLayer{digest: l.Digest, sig: sigBytes})
	}
	return out, nil
}

// fetchBlob retrieves a content-addressed blob from the registry.
func (v *Verifier) fetchBlob(ctx context.Context, host, repo, digest string) ([]byte, error) {
	url := fmt.Sprintf("%s/v2/%s/blobs/%s", v.baseURL(host), repo, digest)
	req, err := v.newRequest(ctx, http.MethodGet, url)
	if err != nil {
		return nil, err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		return nil, fmt.Errorf("registry %s returned HTTP %d for blob %s", host, resp.StatusCode, digest)
	}
	return io.ReadAll(resp.Body)
}

// verifyECDSA checks that sig is a valid DER-encoded ECDSA signature over the
// SHA-256 hash of payload using the verifier's public key.
func (v *Verifier) verifyECDSA(sig, payload []byte) error {
	hash := sha256.Sum256(payload)
	if !ecdsa.VerifyASN1(v.pub, hash[:], sig) {
		return fmt.Errorf("ECDSA signature verification failed")
	}
	return nil
}
