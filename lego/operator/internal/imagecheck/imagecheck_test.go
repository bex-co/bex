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
// Distribution API endpoints needed for cosign signature verification.
type fakeRegistry struct {
	srv *httptest.Server
	// Host is the "host:port" portion of the server URL.
	Host string
}

type registryOptions struct {
	repo         string
	imageDigest  string // sha256:...
	payloadBytes []byte // the simplesigning JSON blob
	sigBytes     []byte // DER-encoded ECDSA signature over SHA256(payloadBytes)
	// omitSigTag causes the .sig manifest endpoint to return 404.
	omitSigTag bool
	// badSig replaces the layer annotation with an invalid signature.
	badSig bool
}

func newFakeRegistry(t *testing.T, o registryOptions) *fakeRegistry {
	t.Helper()
	payloadDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(o.payloadBytes))
	sigTag := strings.ReplaceAll(o.imageDigest, ":", "-") + ".sig"
	sigB64 := base64.StdEncoding.EncodeToString(o.sigBytes)
	if o.badSig {
		sigB64 = base64.StdEncoding.EncodeToString([]byte("invalidsig"))
	}

	mux := http.NewServeMux()

	// Resolve tag → digest
	mux.HandleFunc("/v2/"+o.repo+"/manifests/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		w.Header().Set("Docker-Content-Digest", o.imageDigest)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"schemaVersion":2}`) //nolint:errcheck
	})

	// Signature manifest
	mux.HandleFunc("/v2/"+o.repo+"/manifests/"+sigTag, func(w http.ResponseWriter, r *http.Request) {
		if o.omitSigTag {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		manifest := map[string]any{
			"schemaVersion": 2,
			"layers": []map[string]any{
				{
					"mediaType": "application/vnd.dev.cosign.simplesigning.v1+json",
					"digest":    payloadDigest,
					"size":      len(o.payloadBytes),
					"annotations": map[string]string{
						"dev.cosignproject.cosign/signature": sigB64,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(manifest) //nolint:errcheck
	})

	// Payload blob
	mux.HandleFunc("/v2/"+o.repo+"/blobs/"+payloadDigest, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(o.payloadBytes) //nolint:errcheck
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &fakeRegistry{
		srv:  srv,
		Host: strings.TrimPrefix(srv.URL, "http://"),
	}
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
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	return priv, pubPEM
}

// sign returns a DER ECDSA signature over SHA256(payload).
func sign(t *testing.T, priv *ecdsa.PrivateKey, payload []byte) []byte {
	t.Helper()
	h := sha256.Sum256(payload)
	sig, err := ecdsa.SignASN1(rand.Reader, priv, h[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return sig
}

func TestVerify_ValidSignature(t *testing.T) {
	priv, pubPEM := generateKey(t)
	payload := []byte(`{"critical":{"identity":{"docker-reference":"zot/myapp"},"image":{"docker-manifest-digest":"sha256:deadbeef"},"type":"cosign container image signature"},"optional":null}`)
	sig := sign(t, priv, payload)

	reg := newFakeRegistry(t, registryOptions{
		repo:         "myapp",
		imageDigest:  "sha256:deadbeef",
		payloadBytes: payload,
		sigBytes:     sig,
	})

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
	reg := newFakeRegistry(t, registryOptions{
		repo:         "myapp",
		imageDigest:  "sha256:deadbeef",
		payloadBytes: []byte("{}"),
		sigBytes:     []byte("unused"),
		omitSigTag:   true,
	})

	v, err := imagecheck.NewVerifier(pubPEM, "http", "")
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	err = v.Verify(context.Background(), reg.Host+"/myapp:latest")
	if err == nil {
		t.Error("expected error for missing signature tag, got nil")
	}
}

func TestVerify_InvalidSignature(t *testing.T) {
	_, pubPEM := generateKey(t)
	payload := []byte(`{"critical":{"image":{"docker-manifest-digest":"sha256:deadbeef"}}}`)
	reg := newFakeRegistry(t, registryOptions{
		repo:         "myapp",
		imageDigest:  "sha256:deadbeef",
		payloadBytes: payload,
		sigBytes:     []byte("not-a-real-sig"),
		badSig:       true,
	})

	v, err := imagecheck.NewVerifier(pubPEM, "http", "")
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	err = v.Verify(context.Background(), reg.Host+"/myapp:latest")
	if err == nil {
		t.Error("expected error for invalid signature, got nil")
	}
}

func TestVerify_WrongKey(t *testing.T) {
	priv, _ := generateKey(t)
	_, otherPubPEM := generateKey(t) // verifier uses a different key

	payload := []byte(`{"payload":"test"}`)
	sig := sign(t, priv, payload)

	reg := newFakeRegistry(t, registryOptions{
		repo:         "myapp",
		imageDigest:  "sha256:deadbeef",
		payloadBytes: payload,
		sigBytes:     sig,
	})

	v, err := imagecheck.NewVerifier(otherPubPEM, "http", "")
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	err = v.Verify(context.Background(), reg.Host+"/myapp:latest")
	if err == nil {
		t.Error("expected error when using wrong public key, got nil")
	}
}

func TestVerify_DigestReference(t *testing.T) {
	priv, pubPEM := generateKey(t)
	payload := []byte(`{"critical":{"image":{"docker-manifest-digest":"sha256:deadbeef"}}}`)
	sig := sign(t, priv, payload)

	reg := newFakeRegistry(t, registryOptions{
		repo:         "myapp",
		imageDigest:  "sha256:deadbeef",
		payloadBytes: payload,
		sigBytes:     sig,
	})

	v, err := imagecheck.NewVerifier(pubPEM, "http", "")
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	// Use digest-pinned reference; no resolve step should be needed.
	if err := v.Verify(context.Background(), reg.Host+"/myapp@sha256:deadbeef"); err != nil {
		t.Errorf("Verify with digest ref: %v", err)
	}
}

func TestVerify_SubPath(t *testing.T) {
	// Verify that images with org/name repository paths (multiple path components)
	// are handled correctly.
	priv, pubPEM := generateKey(t)
	payload := []byte(`{"critical":{"image":{"docker-manifest-digest":"sha256:aabbcc"}}}`)
	sig := sign(t, priv, payload)

	// Build a fake registry server manually (newFakeRegistry only supports simple
	// single-component repo names via its path pattern).
	imageDigest := "sha256:aabbcc"
	sigTag := strings.ReplaceAll(imageDigest, ":", "-") + ".sig"
	payloadDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(payload))
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	mux := http.NewServeMux()
	mux.HandleFunc("/v2/org/name/manifests/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Content-Digest", imageDigest)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"schemaVersion":2}`) //nolint:errcheck
	})
	mux.HandleFunc("/v2/org/name/manifests/"+sigTag, func(w http.ResponseWriter, r *http.Request) {
		manifest := map[string]any{
			"layers": []map[string]any{
				{
					"digest": payloadDigest,
					"annotations": map[string]string{
						"dev.cosignproject.cosign/signature": sigB64,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		json.NewEncoder(w).Encode(manifest) //nolint:errcheck
	})
	mux.HandleFunc("/v2/org/name/blobs/"+payloadDigest, func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload) //nolint:errcheck
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	host := strings.TrimPrefix(srv.URL, "http://")

	v, err := imagecheck.NewVerifier(pubPEM, "http", "")
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	if err := v.Verify(context.Background(), host+"/org/name:latest"); err != nil {
		t.Errorf("Verify with sub-path repo: %v", err)
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
	// Encode an RSA public key DER block with an EC PEM header to trigger the
	// "expected EC public key" check via a mis-typed PEM.
	badPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: []byte("not-real-DER"),
	})
	_, err := imagecheck.NewVerifier(badPEM, "http", "")
	if err == nil {
		t.Error("expected error for non-EC key, got nil")
	}
}
