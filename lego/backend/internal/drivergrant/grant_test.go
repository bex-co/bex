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
