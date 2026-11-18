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

// Command oidc-verify checks a Managed OIDC token the way a relying party
// does, given only an issuer URL (docs/ADR116-managed-oidc.md). It is the
// independent half of scripts/oidc-issuer-verify.sh: go-oidc and go-jose,
// sharing no code with bex-api's signer.
//
//	oidc-verify --issuer https://oidc.bex.co/tea-… --audience bex-oidc-verify < token
//
// It fetches the issuer's discovery document (whose issuer must equal the
// URL given), then its JWKS, verifies the token's ES256 signature, iss, aud
// and exp, checks that every published kid is its key's RFC 7638 thumbprint,
// and prints the verified claims and the published kids as JSON. Any failure
// exits 1.
package main

import (
	"bufio"
	"context"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
)

type report struct {
	Issuer   string   `json:"iss"`
	Subject  string   `json:"sub"`
	Audience []string `json:"aud"`
	IssuedAt int64    `json:"iat"`
	Expiry   int64    `json:"exp"`
	KeyID    string   `json:"kid"`
	Kids     []string `json:"jwksKids"`
}

func main() {
	issuer := flag.String("issuer", "", "the workspace's issuer URL, e.g. https://oidc.bex.co/tea-…")
	audience := flag.String("audience", "", "the audience the token must carry")
	flag.Parse()
	if *issuer == "" || *audience == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: oidc-verify --issuer <url> --audience <aud> < token")
		os.Exit(2)
	}
	raw, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && raw == "" {
		fmt.Fprintln(os.Stderr, "oidc-verify: no token on stdin")
		os.Exit(2)
	}
	out, err := verify(*issuer, *audience, strings.TrimSpace(raw))
	if err != nil {
		fmt.Fprintln(os.Stderr, "oidc-verify:", err)
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(out)
}

func verify(issuer, audience, raw string) (report, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return report{}, err
	}
	idt, err := provider.Verifier(&oidc.Config{ClientID: audience, SupportedSigningAlgs: []string{oidc.ES256}}).Verify(ctx, raw)
	if err != nil {
		return report{}, err
	}
	var meta struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := provider.Claims(&meta); err != nil {
		return report{}, err
	}
	kids, err := publishedKids(ctx, meta.JWKSURI)
	if err != nil {
		return report{}, err
	}
	sig, err := jose.ParseSigned(raw, []jose.SignatureAlgorithm{jose.ES256})
	if err != nil {
		return report{}, err
	}
	return report{
		Issuer: idt.Issuer, Subject: idt.Subject, Audience: idt.Audience,
		IssuedAt: idt.IssuedAt.Unix(), Expiry: idt.Expiry.Unix(),
		KeyID: sig.Signatures[0].Header.KeyID, Kids: kids,
	}, nil
}

// publishedKids reads the JWKS and refuses a key that is not a P-256 ES256
// signing key named by its RFC 7638 thumbprint.
func publishedKids(ctx context.Context, uri string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", uri, resp.Status)
	}
	var set jose.JSONWebKeySet
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return nil, err
	}
	if len(set.Keys) == 0 {
		return nil, errors.New("the JWKS publishes no key")
	}
	kids := make([]string, 0, len(set.Keys))
	for _, k := range set.Keys {
		tp, err := k.Thumbprint(crypto.SHA256)
		if err != nil {
			return nil, err
		}
		if want := base64.RawURLEncoding.EncodeToString(tp); k.KeyID != want {
			return nil, fmt.Errorf("kid %q is not the key's RFC 7638 thumbprint %q", k.KeyID, want)
		}
		if k.Algorithm != string(jose.ES256) || k.Use != "sig" || !k.IsPublic() || !k.Valid() {
			return nil, fmt.Errorf("key %s is not a public ES256 signing key", k.KeyID)
		}
		kids = append(kids, k.KeyID)
	}
	return kids, nil
}
