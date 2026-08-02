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

package imagecheck_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/operator/internal/imagecheck"
)

// fakeRegistry builds a minimal httptest.Server that simulates the OCI
// Distribution API endpoints needed for cosign signature verification. The
// payload + signature are set after the server starts (setSignedPayload) so the
// signed docker-reference can carry the server's real host — the binding check
// (codex-security #8) compares it against the image reference being verified.
type fakeRegistry struct {
	srv           *httptest.Server
	Host          string
	repo          string
	imageDigest   string
	omitSigTag    bool
	payload       []byte
	payloadDigest string
	sigB64        string
}

func newFakeRegistry(t *testing.T, repo, imageDigest string, omitSigTag bool) *fakeRegistry {
	t.Helper()
	r := &fakeRegistry{repo: repo, imageDigest: imageDigest, omitSigTag: omitSigTag}
	sigTag := strings.ReplaceAll(imageDigest, ":", "-") + ".sig"
	mux := http.NewServeMux()

	mux.HandleFunc("/v2/"+repo+"/manifests/latest", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		w.Header().Set("Docker-Content-Digest", r.imageDigest)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"schemaVersion":2}`) //nolint:errcheck
	})

	mux.HandleFunc("/v2/"+repo+"/manifests/"+sigTag, func(w http.ResponseWriter, req *http.Request) {
		if r.omitSigTag {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		manifest := map[string]any{
			"schemaVersion": 2,
			"layers": []map[string]any{
				{
					"mediaType": "application/vnd.dev.cosign.simplesigning.v1+json",
					"digest":    r.payloadDigest,
					"size":      len(r.payload),
					"annotations": map[string]string{
						"dev.cosignproject.cosign/signature": r.sigB64,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(manifest) //nolint:errcheck
	})

	// Single payload blob regardless of the requested digest.
	mux.HandleFunc("/v2/"+repo+"/blobs/", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(r.payload) //nolint:errcheck
	})

	r.srv = httptest.NewServer(mux)
	t.Cleanup(r.srv.Close)
	r.Host = strings.TrimPrefix(r.srv.URL, "http://")
	return r
}

// setSignedPayload builds + signs a Simple Signing payload claiming (ref, digest)
// for this registry's repo. Empty ref/digest default to the registry's real
// host/repo and imageDigest — the values a correctly-signed image carries. badSig
// stores an invalid signature instead (for the invalid-signature negative).
func (r *fakeRegistry) setSignedPayload(t *testing.T, priv *ecdsa.PrivateKey, digest, ref string, badSig bool) {
	t.Helper()
	if ref == "" {
		ref = r.Host + "/" + r.repo
	}
	if digest == "" {
		digest = r.imageDigest
	}
	type critical struct {
		Identity struct {
			DockerReference string `json:"docker-reference"`
		} `json:"identity"`
		Image struct {
			DockerManifestDigest string `json:"docker-manifest-digest"`
		} `json:"image"`
		Type string `json:"type"`
	}
	pl := struct {
		Critical critical `json:"critical"`
		Optional any      `json:"optional"`
	}{
		Critical: func() critical {
			c := critical{Type: "cosign container image signature"}
			c.Identity.DockerReference = ref
			c.Image.DockerManifestDigest = digest
			return c
		}(),
	}
	var err error
	if r.payload, err = json.Marshal(pl); err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	r.payloadDigest = fmt.Sprintf("sha256:%x", sha256.Sum256(r.payload))
	if badSig {
		r.sigB64 = base64.StdEncoding.EncodeToString([]byte("invalidsig"))
		return
	}
	h := sha256.Sum256(r.payload)
	sig, err := ecdsa.SignASN1(rand.Reader, priv, h[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	r.sigB64 = base64.StdEncoding.EncodeToString(sig)
}

// generateKey returns a test ECDSA P-256 key pair and its PEM-encoded public key.
func generateKey(t *testing.T) (*ecdsa.PrivateKey, []byte) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(priv.Public())
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	return priv, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
}

func TestVerify_ValidSignature(t *testing.T) {
	priv, pubPEM := generateKey(t)
	reg := newFakeRegistry(t, "myapp", "sha256:deadbeef", false)
	reg.setSignedPayload(t, priv, "", "", false)

	v, err := imagecheck.NewVerifier(pubPEM, "http", "")
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	if err := v.Verify(context.Background(), reg.Host+"/myapp:latest"); err != nil {
		t.Errorf("Verify: unexpected error: %v", err)
	}
}

func TestVerify_MissingSignatureTag(t *testing.T) {
	_, pubPEM := generateKey(t)
	reg := newFakeRegistry(t, "myapp", "sha256:deadbeef", true)
	priv, _ := generateKey(t)
	reg.setSignedPayload(t, priv, "", "", false)

	v, err := imagecheck.NewVerifier(pubPEM, "http", "")
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	if err := v.Verify(context.Background(), reg.Host+"/myapp:latest"); err == nil {
		t.Error("expected error for missing signature tag, got nil")
	}
}

func TestVerify_InvalidSignature(t *testing.T) {
	priv, pubPEM := generateKey(t)
	reg := newFakeRegistry(t, "myapp", "sha256:deadbeef", false)
	reg.setSignedPayload(t, priv, "", "", true) // badSig

	v, err := imagecheck.NewVerifier(pubPEM, "http", "")
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	if err := v.Verify(context.Background(), reg.Host+"/myapp:latest"); err == nil {
		t.Error("expected error for invalid signature, got nil")
	}
}

func TestVerify_WrongKey(t *testing.T) {
	priv, _ := generateKey(t)
	_, otherPubPEM := generateKey(t)
	reg := newFakeRegistry(t, "myapp", "sha256:deadbeef", false)
	reg.setSignedPayload(t, priv, "", "", false) // signed by priv, verified by other

	v, err := imagecheck.NewVerifier(otherPubPEM, "http", "")
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	if err := v.Verify(context.Background(), reg.Host+"/myapp:latest"); err == nil {
		t.Error("expected error when using wrong public key, got nil")
	}
}

func TestVerify_DigestReference(t *testing.T) {
	priv, pubPEM := generateKey(t)
	reg := newFakeRegistry(t, "myapp", "sha256:deadbeef", false)
	reg.setSignedPayload(t, priv, "", "", false)

	v, err := imagecheck.NewVerifier(pubPEM, "http", "")
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	// Digest-pinned reference; no resolve step should be needed.
	if err := v.Verify(context.Background(), reg.Host+"/myapp@sha256:deadbeef"); err != nil {
		t.Errorf("Verify with digest ref: %v", err)
	}
}

func TestVerify_SubPath(t *testing.T) {
	priv, pubPEM := generateKey(t)
	reg := newFakeRegistry(t, "org/name", "sha256:aabbcc", false)
	reg.setSignedPayload(t, priv, "", "", false)

	v, err := imagecheck.NewVerifier(pubPEM, "http", "")
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	if err := v.Verify(context.Background(), reg.Host+"/org/name:latest"); err != nil {
		t.Errorf("Verify with sub-path repo: %v", err)
	}
}

// TestVerify_PayloadDigestMismatch is the codex-security #8 regression: a
// cryptographically valid signature over a payload that claims a DIFFERENT
// manifest digest must be rejected, so a registry writer can't replay a valid
// signature onto another image's signature tag.
func TestVerify_PayloadDigestMismatch(t *testing.T) {
	priv, pubPEM := generateKey(t)
	reg := newFakeRegistry(t, "myapp", "sha256:deadbeef", false)
	reg.setSignedPayload(t, priv, "sha256:different-digest", "", false) // valid sig, wrong digest

	v, err := imagecheck.NewVerifier(pubPEM, "http", "")
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	err = v.Verify(context.Background(), reg.Host+"/myapp:latest")
	if err == nil || !strings.Contains(err.Error(), "does not match image digest") {
		t.Fatalf("expected digest-binding error, got %v", err)
	}
}

// TestVerify_PayloadReferenceMismatch is the codex-security #8 regression: a
// valid signature whose payload names a different repository must be rejected.
func TestVerify_PayloadReferenceMismatch(t *testing.T) {
	priv, pubPEM := generateKey(t)
	reg := newFakeRegistry(t, "myapp", "sha256:deadbeef", false)
	reg.setSignedPayload(t, priv, "", "evil.example/other", false) // valid sig, wrong repo

	v, err := imagecheck.NewVerifier(pubPEM, "http", "")
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	err = v.Verify(context.Background(), reg.Host+"/myapp:latest")
	if err == nil || !strings.Contains(err.Error(), "does not match image reference") {
		t.Fatalf("expected reference-binding error, got %v", err)
	}
}

func TestRegistryAuth(t *testing.T) {
	cfg := []byte(`{"auths":{"myregistry:5000":{"auth":"` +
		base64.StdEncoding.EncodeToString([]byte("user:secret")) + `"}}}`)

	got, err := imagecheck.RegistryAuth(cfg, "myregistry:5000")
	if err != nil {
		t.Fatalf("RegistryAuth: %v", err)
	}
	if got != "user:secret" {
		t.Errorf("RegistryAuth = %q, want %q", got, "user:secret")
	}
}

func TestRegistryAuth_MissingHost(t *testing.T) {
	cfg := []byte(`{"auths":{}}`)
	got, err := imagecheck.RegistryAuth(cfg, "notpresent:5000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string for missing host, got %q", got)
	}
}

func TestNewVerifier_InvalidPEM(t *testing.T) {
	_, err := imagecheck.NewVerifier([]byte("not-pem"), "http", "")
	if err == nil {
		t.Error("expected error for invalid PEM, got nil")
	}
}

func TestNewVerifier_NonECKey(t *testing.T) {
	badPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: []byte("not-real-DER"),
	})
	_, err := imagecheck.NewVerifier(badPEM, "http", "")
	if err == nil {
		t.Error("expected error for non-EC key, got nil")
	}
}
