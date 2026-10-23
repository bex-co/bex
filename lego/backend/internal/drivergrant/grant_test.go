package drivergrant

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestGrantUsesDerivedPublicKeyAndBindsAction(t *testing.T) {
	secret := []byte("gateway-ticket-secret")
	token, err := Mint(secret, "ags-one", time.Unix(1000, 0), 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	body, signature, ok := strings.Cut(token, ".")
	if !ok {
		t.Fatalf("malformed token %q", token)
	}
	publicText, err := PublicKey(secret)
	if err != nil {
		t.Fatal(err)
	}
	public, _ := base64.RawURLEncoding.DecodeString(publicText)
	sig, _ := base64.RawURLEncoding.DecodeString(signature)
	if !ed25519.Verify(public, []byte(body), sig) {
		t.Fatal("derived public key did not verify grant")
	}
	if other, _ := PublicKey([]byte("other")); other == publicText {
		t.Fatal("different ticket secrets derived the same grant key")
	}
}

func TestGrantIsBoundToTheTurnAction(t *testing.T) {
	secret := []byte("gateway-ticket-secret")
	token, err := Mint(secret, "ags-one", time.Unix(1000, 0), 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	body, _, _ := strings.Cut(token, ".")
	payload, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"act":"turn"`) || !strings.Contains(string(payload), `"ses":"ags-one"`) {
		t.Fatalf("turn action or session missing from claims: %s", payload)
	}
	for _, bad := range []struct {
		secret  []byte
		session string
		ttl     time.Duration
	}{{nil, "ags-one", time.Second}, {secret, "", time.Second}, {secret, "ags-one", 0}} {
		if _, err := Mint(bad.secret, bad.session, time.Unix(1000, 0), bad.ttl); err == nil {
			t.Fatalf("invalid grant configuration %+v was accepted", bad)
		}
	}
}
