package cliauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// The device flow must request the granular capabilities, not identity alone
// (w7/037).
//
// 9081fbdb deliberately removed core.CapabilityExempt's platform-client branch
// ("Human OAuth delegations are never exempt, including first-party public
// clients"), which the CLI had been relying on to carry its authority with an
// identity-only grant. Nothing updated this request, so every CLI login produced
// a token that 403'd on every operation with INSUFFICIENT_SCOPE. Nothing covered
// the pairing, which is how it shipped.
func TestDeviceAuthRequestsGranularCapabilities(t *testing.T) {
	var deviceForm url.Values
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse upstream form: %v", err)
		}
		if r.URL.Path == "/oauth2/device/auth" {
			deviceForm = r.Form
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code": "d", "user_code": "U",
				"verification_uri": "https://dashboard.bex.co/auth/device",
				"expires_in":       600, "interval": 5,
			})
			return
		}
		http.Error(w, "unexpected", http.StatusBadRequest)
	}))
	defer upstream.Close()

	svc := New(upstream.URL, "", nil, nil)
	mux := http.NewServeMux()
	svc.RegisterPublic(mux, noMiddleware)

	body := strings.NewReader(`{"client_id":"` + RenderCLIClientID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/device-grant", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("device-grant returned %d, body %s", rec.Code, rec.Body.String())
	}
	got := deviceForm.Get("scope")
	for _, want := range []string{core.ScopeRead, core.ScopeWrite, core.ScopeSensitive} {
		if !slicesContains(strings.Fields(got), want) {
			t.Errorf("device auth requested scope %q, missing %q — a CLI login with this grant "+
				"cannot perform any operation (w7/037)", got, want)
		}
	}
	// Identity scopes must survive: the CLI needs openid for the subject and
	// offline_access for the refresh token it stores.
	for _, want := range []string{"openid", "offline_access"} {
		if !slicesContains(strings.Fields(got), want) {
			t.Errorf("device auth requested scope %q, missing %q", got, want)
		}
	}
}

func slicesContains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}
