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

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// R25-12 (w21/074): a per-source budget that keys on the exact IPv6 address gives
// a client a fresh budget for every address of its own /64, which one subscriber
// holds 2^64 of. The budget, the credential fingerprint and the pre-auth caps
// now count an IPv6 client under its /64, as the operator's proxies do.

func bearerFrom(token, remoteAddr string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	r.RemoteAddr = remoteAddr
	return r
}

func TestInvalidCredentialBudgetIsPerIPv6Slash64(t *testing.T) {
	p := newCountingIdentityProvider(t, "good", "")
	mw := gateWithAdmission(p, NewAuthAdmission(3, 3, 0, 0))

	// A flood of bad bearers, each from another address of one /64.
	for i := range 40 {
		mw.ServeHTTP(httptest.NewRecorder(), bearerFrom("bad-"+string(rune('a'+i%26))+string(rune('a'+i/26)),
			"[2001:db8:1:2::"+string(rune('1'+i%9))+string(rune('a'+i/9))+"]:40000"))
	}
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, bearerFrom("good", "[2001:db8:1:2:ffff::9]:40000"))
	if w.Code == http.StatusOK {
		t.Error("a client rotating through its /64 got a fresh failure budget for every address")
	}

	w = httptest.NewRecorder()
	mw.ServeHTTP(w, bearerFrom("good", "[2001:db8:1:3::1]:40000"))
	if w.Code != http.StatusOK {
		t.Errorf("a different /64: status = %d, want 200 — budgets are per /64, not global", w.Code)
	}
}

func TestCredentialFingerprintOfAnonymousRequestsIsPerIPv6Slash64(t *testing.T) {
	adm := NewAuthAdmission(0, 0, 0, 1)
	fingerprint := func(remote string) string {
		r := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
		r.RemoteAddr = remote
		return adm.credentialFingerprint(r)
	}
	a := fingerprint("[2001:db8:1:2::1]:40000")
	if b := fingerprint("[2001:db8:1:2:ffff::9]:40000"); a != b {
		t.Error("two addresses of one /64 are two sources for the per-credential cap")
	}
	if c := fingerprint("[2001:db8:1:3::1]:40000"); a == c {
		t.Error("two /64s share a source")
	}
	if d, e := fingerprint("203.0.113.1:40000"), fingerprint("203.0.113.2:40000"); d == e {
		t.Error("two IPv4 addresses share a source")
	}
}
