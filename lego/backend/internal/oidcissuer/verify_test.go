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

package oidcissuer_test

import (
	"context"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"

	"github.com/bex-co/bex/lego/backend/internal/oidcissuer"
)

// The verifier in these tests is go-oidc with go-jose, which shares no code
// with the signer (golang-jwt) or the hand-written JWKS: what passes here is
// what an independent relying party accepts, given only the issuer URL.

// serve starts the issuer on a real listener and returns its base URL.
func serve(t *testing.T, start time.Time) (*fixture, string) {
	t.Helper()
	var f *fixture
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { f.issuer.ServeHTTP(w, r) }))
	t.Cleanup(srv.Close)
	f = newFixture(t, srv.URL, start)
	return f, srv.URL
}

// verifier builds a go-oidc verifier from discovery alone: NewProvider fetches
// <issuer>/.well-known/openid-configuration and requires its issuer to equal
// the URL it was given, then the verifier fetches the JWKS it names.
func verifier(t *testing.T, issuer, audience string, now func() time.Time) *oidc.IDTokenVerifier {
	t.Helper()
	p, err := oidc.NewProvider(context.Background(), issuer)
	if err != nil {
		t.Fatalf("NewProvider(%s): %v", issuer, err)
	}
	return p.Verifier(&oidc.Config{ClientID: audience, SupportedSigningAlgs: []string{oidc.ES256}, Now: now})
}

// TestATokenVerifiesThroughItsWorkspacesDiscoveryOnly is the definition of
// done's core claim: a minted token verifies at its own workspace's issuer,
// and fails at another workspace's issuer, for another audience, and after
// it expires.
func TestATokenVerifiesThroughItsWorkspacesDiscoveryOnly(t *testing.T) {
	f, base := serve(t, time.Now())
	sub := oidcissuer.Subject{Workspace: wsA, Service: srvA}
	tok, err := f.issuer.Mint(sub, "sts.amazonaws.com")
	if err != nil {
		t.Fatal(err)
	}
	idt, err := verifier(t, base+"/"+wsA, "sts.amazonaws.com", nil).Verify(context.Background(), tok.Raw)
	if err != nil {
		t.Fatalf("verify at its own issuer: %v", err)
	}
	if idt.Subject != sub.String() || idt.Issuer != base+"/"+wsA || idt.Expiry.Sub(idt.IssuedAt) != time.Hour {
		t.Errorf("verified token = sub %q iss %q lifetime %v", idt.Subject, idt.Issuer, idt.Expiry.Sub(idt.IssuedAt))
	}

	// Another workspace's issuer is another identity provider: its verifier
	// refuses the token even with a matching audience and no subject check.
	if _, err := verifier(t, base+"/"+wsB, "sts.amazonaws.com", nil).Verify(context.Background(), tok.Raw); err == nil ||
		!strings.Contains(err.Error(), "different provider") {
		t.Fatalf("verify at workspace B's issuer = %v, want an issuer mismatch", err)
	}
	if _, err := verifier(t, base+"/"+wsA, "api.openai.com", nil).Verify(context.Background(), tok.Raw); err == nil {
		t.Fatal("a token for sts.amazonaws.com verified for api.openai.com")
	}
	later := func() time.Time { return tok.ExpiresAt.Add(time.Second) }
	if _, err := verifier(t, base+"/"+wsA, "sts.amazonaws.com", later).Verify(context.Background(), tok.Raw); err == nil ||
		!strings.Contains(err.Error(), "expired") {
		t.Fatalf("verify after exp = %v, want expired", err)
	}
}

// TestARotationHasNoVerificationGap walks one rotation with the verifier's
// clock tied to the issuer's: a token signed before it keeps verifying until
// it expires, the next key is published before it signs, and tokens signed
// after carry the new kid.
func TestARotationHasNoVerificationGap(t *testing.T) {
	start := time.Now().Add(-6 * time.Hour)
	f, base := serve(t, start)
	ctx := context.Background()
	sub := oidcissuer.Subject{Workspace: wsA, Environment: evmA, Service: srvA}
	v := verifier(t, base+"/"+wsA, "api.anthropic.com", f.clock.Now)
	reload := func() {
		t.Helper()
		if err := f.keys.Load(ctx); err != nil {
			t.Fatal(err)
		}
	}
	mint := func() oidcissuer.Token {
		t.Helper()
		tok, err := f.issuer.Mint(sub, "api.anthropic.com")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := v.Verify(ctx, tok.Raw); err != nil {
			t.Fatalf("a freshly minted token failed to verify: %v", err)
		}
		return tok
	}

	before := mint()
	report, err := oidcissuer.Rotate(ctx, f.store, f.clock.Now(), false, true)
	if err != nil {
		t.Fatal(err)
	}
	reload()
	if got := publishedKids(t, base); len(got) != 2 || got[1] != report.NewKey.KID {
		t.Fatalf("JWKS after the rotation = %v, want the current key and %s", got, report.NewKey.KID)
	}
	if mint().KeyID != before.KeyID {
		t.Fatal("the rotated-in key signed before its publish lead elapsed")
	}

	f.clock.Advance(oidcissuer.PublishLead - 30*time.Minute)
	reload()
	lastOld := mint() // signed 30 minutes before the switch
	f.clock.Advance(30 * time.Minute)
	reload()
	after := mint()
	if after.KeyID != report.NewKey.KID || lastOld.KeyID != before.KeyID {
		t.Fatalf("kids: last old %s, first new %s; want %s then %s", lastOld.KeyID, after.KeyID, before.KeyID, report.NewKey.KID)
	}
	if _, err := v.Verify(ctx, lastOld.Raw); err != nil {
		t.Fatalf("a token signed with the superseded key stopped verifying before it expired: %v", err)
	}

	// Past the retention the old key is gone, by which time every token it
	// signed has expired.
	f.clock.Advance(oidcissuer.RetireAfter)
	reload()
	if got := publishedKids(t, base); len(got) != 1 || got[0] != report.NewKey.KID {
		t.Fatalf("JWKS after the retention = %v, want only %s", got, report.NewKey.KID)
	}
	if f.clock.Now().Before(lastOld.ExpiresAt) {
		t.Fatal("the old key was removed while a token it signed was still valid")
	}
	mint()
}

// publishedKids reads the JWKS with go-jose and checks every kid is the key's
// RFC 7638 thumbprint, as go-jose computes it.
func publishedKids(t *testing.T, base string) []string {
	t.Helper()
	resp, err := http.Get(base + "/" + wsA + "/.well-known/jwks.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var set jose.JSONWebKeySet
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		t.Fatal(err)
	}
	var kids []string
	for _, k := range set.Keys {
		tp, err := k.Thumbprint(crypto.SHA256)
		if err != nil {
			t.Fatal(err)
		}
		if got := base64.RawURLEncoding.EncodeToString(tp); got != k.KeyID {
			t.Errorf("kid %s is not the key's RFC 7638 thumbprint %s", k.KeyID, got)
		}
		if k.Algorithm != "ES256" || k.Use != "sig" || !k.Valid() || !k.IsPublic() {
			t.Errorf("published key %s: alg %q use %q valid %v public %v", k.KeyID, k.Algorithm, k.Use, k.Valid(), k.IsPublic())
		}
		kids = append(kids, k.KeyID)
	}
	return kids
}

type answer struct {
	status int
	header http.Header
	body   string
}

func get(t *testing.T, h http.Handler, method, path string) answer {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	body, _ := io.ReadAll(w.Result().Body)
	return answer{w.Code, w.Result().Header, string(body)}
}

// TestDocumentsAnswerAsRendersDo pins the two documents and every refusal to
// Render's answers, probed 2026-11-18 (ADR116 Context).
func TestDocumentsAnswerAsRendersDo(t *testing.T) {
	f := newFixture(t, "https://oidc.bex.co", time.Now())
	h := f.issuer

	d := get(t, h, http.MethodGet, "/"+wsA+"/.well-known/openid-configuration")
	wantDiscovery := `{"issuer":"https://oidc.bex.co/` + wsA + `","jwks_uri":"https://oidc.bex.co/` + wsA +
		`/.well-known/jwks.json","response_types_supported":["id_token"],"subject_types_supported":["public"],` +
		`"id_token_signing_alg_values_supported":["ES256"]}` + "\n"
	if d.status != http.StatusOK || d.body != wantDiscovery {
		t.Errorf("discovery = %d %q\nwant 200 %q", d.status, d.body, wantDiscovery)
	}
	j := get(t, h, http.MethodGet, "/"+wsA+"/.well-known/jwks.json")
	if j.status != http.StatusOK || !strings.HasPrefix(j.body, `{"keys":[{"use":"sig","kty":"EC","kid":"`+f.store.kids()[0]+`","crv":"P-256","alg":"ES256","x":"`) ||
		strings.HasSuffix(j.body, "\n") {
		t.Errorf("jwks = %d %q", j.status, j.body)
	}
	for _, a := range []answer{d, j} {
		if a.header.Get("Content-Type") != "application/json" || a.header.Get("Cache-Control") != "public, max-age=300" {
			t.Errorf("headers = %v", a.header)
		}
	}
	// Every workspace shares the key set, as on Render.
	if other := get(t, h, http.MethodGet, "/"+wsB+"/.well-known/jwks.json"); other.body != j.body {
		t.Errorf("workspace B's JWKS differs: %q", other.body)
	}

	for _, path := range []string{
		"/", "/" + wsA, "/" + wsA + "/", "/" + wsA + "/.well-known/openid-configuration/",
		"/tea-abc/.well-known/openid-configuration", "/tea-d0000000000000000001/.well-known/jwks.json",
		"/TEA-c0000000000000000000/.well-known/jwks.json", "/" + srvA + "/.well-known/jwks.json",
		"/" + wsA + "/.well-known/oauth-authorization-server", "/healthz",
	} {
		if a := get(t, h, http.MethodGet, path); a.status != http.StatusNotFound || a.body != "404 page not found\n" ||
			a.header.Get("Content-Type") != "text/plain; charset=utf-8" || a.header.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("GET %s = %d %q %v, want Go's 404", path, a.status, a.body, a.header)
		}
	}
	// A document's path with another method is a 405 even for a malformed id,
	// and a path of no document's shape stays a 404, as on Render.
	for _, method := range []string{http.MethodPost, http.MethodHead, http.MethodPut} {
		for _, path := range []string{"/" + wsA + "/.well-known/jwks.json", "/tea-abc/.well-known/openid-configuration"} {
			if a := get(t, h, method, path); a.status != http.StatusMethodNotAllowed || a.body != "" || a.header.Get("Allow") != "GET" {
				t.Errorf("%s %s = %d %q %v, want 405 Allow: GET with no body", method, path, a.status, a.body, a.header)
			}
		}
		if a := get(t, h, method, "/"); a.status != http.StatusNotFound {
			t.Errorf("%s / = %d, want 404", method, a.status)
		}
	}

	// Before this replica has loaded the key set, discovery still answers and
	// the JWKS says it cannot.
	unloaded, err := oidcissuer.New("https://oidc.bex.co", &oidcissuer.KeySet{Store: &memStore{now: time.Now}})
	if err != nil {
		t.Fatal(err)
	}
	if a := get(t, unloaded, http.MethodGet, "/"+wsA+"/.well-known/jwks.json"); a.status != http.StatusServiceUnavailable {
		t.Errorf("unloaded jwks = %d", a.status)
	}
	if a := get(t, unloaded, http.MethodGet, "/"+wsA+"/.well-known/openid-configuration"); a.status != http.StatusOK {
		t.Errorf("unloaded discovery = %d", a.status)
	}
}

// TestParseBaseURLTakesAnOriginOnly covers BEX_OIDC_ISSUER_URL's shape.
func TestParseBaseURLTakesAnOriginOnly(t *testing.T) {
	for raw, want := range map[string][2]string{
		"https://oidc.bex.co":            {"https://oidc.bex.co", "oidc.bex.co"},
		"https://oidc.bex.co/":           {"https://oidc.bex.co", "oidc.bex.co"},
		"http://oidc.localhost:54020":    {"http://oidc.localhost:54020", "oidc.localhost"},
		"https://OIDC.Example.com:8443/": {"https://OIDC.Example.com:8443", "oidc.example.com"},
	} {
		base, host, err := oidcissuer.ParseBaseURL(raw)
		if err != nil || base != want[0] || host != want[1] {
			t.Errorf("ParseBaseURL(%q) = %q %q %v, want %q %q", raw, base, host, err, want[0], want[1])
		}
	}
	for _, raw := range []string{"", "oidc.bex.co", "ftp://oidc.bex.co", "https://oidc.bex.co/oidc", "https://oidc.bex.co?x=1", "https://u:p@oidc.bex.co", "https://"} {
		if _, _, err := oidcissuer.ParseBaseURL(raw); err == nil {
			t.Errorf("ParseBaseURL(%q) accepted", raw)
		}
	}
}

// TestAnEmergencyRotationCutsTheOldKeyAtOnce: after `oidc-rotate-key
// --emergency --apply` and a reload, the old key is gone from the JWKS, a
// verifier that fetches it refuses a token the old key signed, and new tokens
// are signed with the new key straight away.
func TestAnEmergencyRotationCutsTheOldKeyAtOnce(t *testing.T) {
	f, base := serve(t, time.Now())
	ctx := context.Background()
	sub := oidcissuer.Subject{Workspace: wsA, Service: srvA}
	old, err := f.issuer.Mint(sub, "api.openai.com")
	if err != nil {
		t.Fatal(err)
	}
	report, err := oidcissuer.Rotate(ctx, f.store, f.clock.Now(), true, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.keys.Load(ctx); err != nil {
		t.Fatal(err)
	}
	if got := publishedKids(t, base); len(got) != 1 || got[0] != report.NewKey.KID {
		t.Fatalf("JWKS after an emergency rotation = %v, want only %s", got, report.NewKey.KID)
	}
	v := verifier(t, base+"/"+wsA, "api.openai.com", nil)
	if _, err := v.Verify(ctx, old.Raw); err == nil {
		t.Fatal("a token signed with the replaced key still verifies")
	}
	fresh, err := f.issuer.Mint(sub, "api.openai.com")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.KeyID != report.NewKey.KID {
		t.Fatalf("after an emergency rotation tokens carry %s, want %s", fresh.KeyID, report.NewKey.KID)
	}
	if _, err := v.Verify(ctx, fresh.Raw); err != nil {
		t.Fatalf("a token signed with the new key fails: %v", err)
	}
}
