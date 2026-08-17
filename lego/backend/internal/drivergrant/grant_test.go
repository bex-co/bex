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

func TestSnapshotGrantIsActionBound(t *testing.T) {
	secret := []byte("gateway-ticket-secret")
	token, err := MintAction(secret, "ags-one", ActionSnapshot, time.Unix(1000, 0), 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	body, _, _ := strings.Cut(token, ".")
	payload, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"act":"snapshot"`) {
		t.Fatalf("snapshot action missing from claims: %s", payload)
	}
	if _, err := MintAction(secret, "ags-one", "arbitrary", time.Unix(1000, 0), time.Second); err == nil {
		t.Fatal("arbitrary driver action was accepted")
	}
}
