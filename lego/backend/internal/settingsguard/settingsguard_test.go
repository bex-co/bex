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

package settingsguard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeWorkspaces struct {
	names map[string][]string
	err   error
}

func (f fakeWorkspaces) GoogleLoginWorkspaces(_ context.Context, subject string) ([]string, error) {
	return f.names[subject], f.err
}

type fakeIdentity struct {
	email        string
	googleLinked bool
}

type fakeIdentities struct {
	ids map[string]fakeIdentity
	err error
}

func (f fakeIdentities) SettingsFacts(_ context.Context, subject string) (string, bool, bool, error) {
	if f.err != nil {
		return "", false, false, f.err
	}
	id, ok := f.ids[subject]
	return id.email, id.googleLinked, ok, nil
}

const (
	enforcedMember = "11111111-1111-4111-8111-111111111111"
	freeMember     = "22222222-2222-4222-8222-222222222222"
	token          = "guard-token"
	settingsPost   = "https://auth.bex.co/self-service/settings?flow=f-1"
	githubCallback = "https://auth.bex.co/self-service/methods/oidc/callback/github?code=c&state=s"
)

func newHandler() *Handler {
	return &Handler{
		Token: token,
		Workspaces: fakeWorkspaces{names: map[string][]string{
			enforcedMember: {"Acme", "Acme Staging"},
		}},
		Identities: fakeIdentities{ids: map[string]fakeIdentity{
			enforcedMember: {email: "person@acme.example", googleLinked: true},
			freeMember:     {email: "free@acme.example", googleLinked: true},
		}},
	}
}

func hook(t *testing.T, h http.Handler, method string, body any, bearer string) (int, kratosMessages) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, Path+"?method="+method, bytes.NewReader(raw))
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out kratosMessages
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

// Render: a member of a workspace that enforces Google login "can't change
// their Render account's associated email address" and "can't disconnect"
// Google; a member of none changes both freely.
func TestTheGuardRefusesOnlyWhatRenderRefuses(t *testing.T) {
	for name, tc := range map[string]struct {
		method string
		body   request
		want   int // 0 = allowed; else the refusal's message id
	}{
		"email change in an enforcing workspace":          {method: "profile", body: request{IdentityID: enforcedMember, Email: "personal@gmail.com", RequestURL: settingsPost}, want: MessageEmailChangeRefused},
		"email change differing only in case":             {method: "profile", body: request{IdentityID: enforcedMember, Email: "Person@Acme.example", RequestURL: settingsPost}},
		"a name change keeps the email":                   {method: "profile", body: request{IdentityID: enforcedMember, Email: "person@acme.example", RequestURL: settingsPost}},
		"an unlink in an enforcing workspace":             {method: "oidc", body: request{IdentityID: enforcedMember, Email: "person@acme.example", RequestURL: settingsPost}, want: MessageUnlinkRefused},
		"a link in an enforcing workspace":                {method: "oidc", body: request{IdentityID: enforcedMember, Email: "person@acme.example", RequestURL: githubCallback}},
		"email change in no enforcing workspace":          {method: "profile", body: request{IdentityID: freeMember, Email: "else@acme.example", RequestURL: settingsPost}},
		"an unlink in no enforcing workspace":             {method: "oidc", body: request{IdentityID: freeMember, Email: "free@acme.example", RequestURL: settingsPost}},
		"an identity Kratos does not know is not blocked": {method: "profile", body: request{IdentityID: "unknown", Email: "x@acme.example", RequestURL: settingsPost}},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHandler()
			if tc.body.IdentityID == "unknown" {
				h.Workspaces = fakeWorkspaces{names: map[string][]string{"unknown": {"Acme"}}}
			}
			status, out := hook(t, h, tc.method, tc.body, token)
			if tc.want == 0 {
				if status != http.StatusOK || len(out.Messages) != 0 {
					t.Fatalf("hook = %d %+v, want 200 with no message", status, out)
				}
				return
			}
			if status != http.StatusForbidden || len(out.Messages) != 1 || len(out.Messages[0].Messages) != 1 {
				t.Fatalf("hook = %d %+v, want 403 with one message", status, out)
			}
			msg := out.Messages[0].Messages[0]
			if msg.ID != tc.want || msg.Type != "error" || !bytes.Contains([]byte(msg.Text), []byte("Acme, Acme Staging")) {
				t.Fatalf("message = %+v, want id %d naming the workspaces", msg, tc.want)
			}
			if names, _ := msg.Context["workspaces"].([]any); len(names) != 2 {
				t.Fatalf("context = %v, want the two workspace names", msg.Context)
			}
		})
	}
}

// The email refusal sits on the email field; the unlink one on the form.
func TestTheRefusalPointsAtWhatItRefuses(t *testing.T) {
	h := newHandler()
	_, email := hook(t, h, "profile", request{IdentityID: enforcedMember, Email: "other@acme.example", RequestURL: settingsPost}, token)
	_, unlink := hook(t, h, "oidc", request{IdentityID: enforcedMember, RequestURL: settingsPost}, token)
	if email.Messages[0].InstancePtr != "#/traits/email" || unlink.Messages[0].InstancePtr != "#/" {
		t.Fatalf("pointers = %q, %q", email.Messages[0].InstancePtr, unlink.Messages[0].InstancePtr)
	}
}

// Only Kratos holds the bearer, and a hook the guard cannot answer refuses:
// Kratos fails its flow on any non-2xx without messages.
func TestTheGuardFailsClosed(t *testing.T) {
	body := request{IdentityID: enforcedMember, Email: "other@acme.example", RequestURL: settingsPost}
	if status, _ := hook(t, newHandler(), "profile", body, ""); status != http.StatusUnauthorized {
		t.Fatalf("no bearer = %d, want 401", status)
	}
	if status, _ := hook(t, newHandler(), "profile", body, "wrong"); status != http.StatusUnauthorized {
		t.Fatalf("wrong bearer = %d, want 401", status)
	}
	for name, h := range map[string]*Handler{
		"store down":  {Token: token, Workspaces: fakeWorkspaces{err: errors.New("db down")}, Identities: newHandler().Identities},
		"kratos down": {Token: token, Workspaces: newHandler().Workspaces, Identities: fakeIdentities{err: errors.New("kratos down")}},
	} {
		if status, _ := hook(t, h, "profile", body, token); status != http.StatusServiceUnavailable {
			t.Fatalf("%s = %d, want 503", name, status)
		}
	}
	for name, tc := range map[string]struct {
		method string
		body   any
	}{
		"unknown method":   {"password", body},
		"no identity":      {"profile", request{Email: "x"}},
		"not json":         {"profile", "nope"},
		"no method at all": {"", body},
	} {
		if status, _ := hook(t, newHandler(), tc.method, tc.body, token); status != http.StatusBadRequest {
			t.Fatalf("%s = %d, want 400", name, status)
		}
	}
}

func TestRegisterMountsOnlyAWiredGuard(t *testing.T) {
	for name, h := range map[string]*Handler{
		"nil":           nil,
		"no token":      {Workspaces: newHandler().Workspaces, Identities: newHandler().Identities},
		"no identities": {Token: token, Workspaces: newHandler().Workspaces},
	} {
		mux := http.NewServeMux()
		Register(mux, h)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, Path, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: code = %d, want 404 (not mounted)", name, rec.Code)
		}
	}
	mux := http.NewServeMux()
	Register(mux, newHandler())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, Path+"?method=profile", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wired guard without a bearer = %d, want 401", rec.Code)
	}
}
