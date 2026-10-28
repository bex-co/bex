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

package agentsession

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeEnvironments struct {
	row store.AgentEnvironment
	err error
}

func (f fakeEnvironments) GetAgentEnvironment(context.Context, string) (store.AgentEnvironment, error) {
	return f.row, f.err
}

func grantRequest() ModelMintRequest {
	return ModelMintRequest{SessionID: "ags-one", Workspace: "tea-a", PodName: "sbx-1-0", PodUID: "uid-1"}
}

func TestEgressProxyURLRoundTripsTheSessionClaim(t *testing.T) {
	raw, err := EgressProxyURL("http://bex-ssh-gateway.bex-system.svc.cluster.local:8085", "tea-a-sandbox", "ags-one")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host != "bex-ssh-gateway.bex-system.svc.cluster.local:8085" || parsed.Path != "" {
		t.Fatalf("proxy URL = %q (%v)", raw, err)
	}
	// Exactly what an HTTP client sends for proxy userinfo.
	request := &http.Request{Header: http.Header{}}
	password, _ := parsed.User.Password()
	request.SetBasicAuth(parsed.User.Username(), password)
	namespace, sessionID, err := ParseEgressProxyAuthorization(request.Header.Get("Authorization"))
	if err != nil || namespace != "tea-a-sandbox" || sessionID != "ags-one" {
		t.Fatalf("parsed claim = %q %q %v", namespace, sessionID, err)
	}
	for _, bad := range []string{"", "Bearer x", "Basic !!!", "Basic " + "bm9jb2xvbg", "Basic Og"} {
		if _, _, err := ParseEgressProxyAuthorization(bad); err == nil {
			t.Errorf("ParseEgressProxyAuthorization(%q) accepted", bad)
		}
	}
	for _, base := range []string{"", "https://gateway:8085", "gateway:8085"} {
		if _, err := EgressProxyURL(base, "tea-a-sandbox", "ags-one"); err == nil {
			t.Errorf("EgressProxyURL(%q) accepted", base)
		}
	}
}

func TestEgressGrantRequiresALiveSandboxInAFullTierWorkspace(t *testing.T) {
	full := store.AgentEnvironment{WorkspaceID: "tea-a", NetworkTier: "full"}
	cases := map[string]struct {
		sessions SessionStore
		envs     EgressTierStore
		req      ModelMintRequest
		want     error
	}{
		"full tier":           {fakeSessions{session: activeSession()}, fakeEnvironments{row: full}, grantRequest(), nil},
		"registries tier":     {fakeSessions{session: activeSession()}, fakeEnvironments{row: store.AgentEnvironment{NetworkTier: "registries"}}, grantRequest(), ErrForbidden},
		"custom tier":         {fakeSessions{session: activeSession()}, fakeEnvironments{row: store.AgentEnvironment{NetworkTier: "custom"}}, grantRequest(), ErrForbidden},
		"no environment row":  {fakeSessions{session: activeSession()}, fakeEnvironments{err: store.ErrNotFound}, grantRequest(), ErrForbidden},
		"unknown session":     {fakeSessions{err: store.ErrNotFound}, fakeEnvironments{row: full}, grantRequest(), ErrForbidden},
		"stale sandbox pod":   {fakeSessions{session: activeSession()}, fakeEnvironments{row: full}, ModelMintRequest{SessionID: "ags-one", Workspace: "tea-a", PodName: "sbx-0-0", PodUID: "uid-1"}, ErrForbidden},
		"cross workspace":     {fakeSessions{session: activeSession()}, fakeEnvironments{row: full}, ModelMintRequest{SessionID: "ags-one", Workspace: "tea-b", PodName: "sbx-1-0", PodUID: "uid-1"}, ErrForbidden},
		"incomplete identity": {fakeSessions{session: activeSession()}, fakeEnvironments{row: full}, ModelMintRequest{SessionID: "ags-one", Workspace: "tea-a"}, ErrInvalidRequest},
	}
	for name, tc := range cases {
		granter := &EgressGranter{Sessions: tc.sessions, Environments: tc.envs}
		got, err := granter.Grant(context.Background(), tc.req)
		if tc.want == nil {
			if err != nil || !got.Allowed || got.SessionID != "ags-one" {
				t.Errorf("%s: grant = %+v %v, want allowed", name, got, err)
			}
			continue
		}
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
	terminal := activeSession()
	terminal.Phase = "done"
	if _, err := (&EgressGranter{Sessions: fakeSessions{session: terminal}, Environments: fakeEnvironments{row: full}}).Grant(context.Background(), grantRequest()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("terminal session grant err = %v, want forbidden", err)
	}
	if _, err := (&EgressGranter{Sessions: fakeSessions{session: activeSession()}, Environments: fakeEnvironments{err: errors.New("db down")}}).Grant(context.Background(), grantRequest()); err == nil || errors.Is(err, ErrForbidden) {
		t.Fatalf("environment outage err = %v, want an unavailable error", err)
	}
}

func TestEgressGrantClientAndHandlerShareTheSignedEnvelope(t *testing.T) {
	secret := []byte("shared-gateway-secret")
	handler := &EgressGrantHandler{Secret: secret, Granter: &EgressGranter{
		Sessions:     fakeSessions{session: activeSession()},
		Environments: fakeEnvironments{row: store.AgentEnvironment{NetworkTier: "full"}},
	}}
	server := httptest.NewServer(handler)
	defer server.Close()
	client := &EgressGrantClient{URL: server.URL, Secret: secret}
	got, err := client.Grant(context.Background(), grantRequest())
	if err != nil || !got.Allowed {
		t.Fatalf("signed grant = %+v %v", got, err)
	}
	if _, err := (&EgressGrantClient{URL: server.URL, Secret: []byte("wrong")}).Grant(context.Background(), grantRequest()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("wrong secret err = %v, want forbidden", err)
	}
	handler.Granter.Environments = fakeEnvironments{row: store.AgentEnvironment{NetworkTier: "registries"}}
	if _, err := client.Grant(context.Background(), grantRequest()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("registries grant err = %v, want forbidden", err)
	}
}
