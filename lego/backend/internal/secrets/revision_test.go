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

package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestEnvironmentRevisionRoundTrip(t *testing.T) {
	for _, version := range []uint64{0, 1, 42, math.MaxUint64} {
		token := encodeEnvRevision(version)
		if strings.Contains(token, strconv.FormatUint(version, 10)) && version > 9 {
			t.Fatalf("revision %d was exposed as a decimal token: %q", version, token)
		}
		got, err := decodeEnvRevision(token)
		if err != nil || got != version {
			t.Fatalf("decode(%q) = %d, %v; want %d", token, got, err, version)
		}
	}
	if got := encodeEnvRevision(1); got != "evr1_AAAAAAAAAAE" {
		t.Fatalf("version 1 token = %q", got)
	}
}

func TestEnvironmentRevisionRejectsNonCanonicalTokensWithoutEcho(t *testing.T) {
	for _, token := range []string{
		"",
		"evr2_AAAAAAAAAAE",
		"evr1_AAAAAAAAAAE=",
		"evr1_short",
		"evr1_!!!!!!!!!!!",
	} {
		if _, err := decodeEnvRevision(token); !errors.Is(err, errInvalidEnvRevision) {
			t.Errorf("decode(%q) error = %v", token, err)
		} else if token != "" && strings.Contains(err.Error(), token) {
			t.Errorf("decode error echoed token %q: %v", token, err)
		}
	}
}

type casBaoStub struct {
	t        *testing.T
	data     map[string]map[string]string
	versions map[string]uint64
	token    string
	logins   int
	rejectN  int
	lastCAS  uint64
}

func newCASBaoStub(t *testing.T) *casBaoStub {
	return &casBaoStub{
		t:        t,
		data:     map[string]map[string]string{},
		versions: map[string]uint64{},
	}
}

func (b *casBaoStub) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/auth/kubernetes/login", func(w http.ResponseWriter, _ *http.Request) {
		b.logins++
		b.token = "cas-token-" + strconv.Itoa(b.logins)
		core.WriteJSON(w, http.StatusOK, map[string]any{
			"auth": map[string]any{"client_token": b.token, "lease_duration": 3600},
		})
	})
	mux.HandleFunc("/v1/tenants/data/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != b.token {
			http.Error(w, "missing token", http.StatusForbidden)
			return
		}
		if b.rejectN > 0 {
			b.rejectN--
			// PutCAS must never surface this body.
			http.Error(w, "do-not-return-auth-body", http.StatusForbidden)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/v1/tenants/data/")
		switch r.Method {
		case http.MethodGet:
			data, ok := b.data[path]
			if !ok {
				http.Error(w, "do-not-return-not-found-body", http.StatusNotFound)
				return
			}
			core.WriteJSON(w, http.StatusOK, map[string]any{
				"data": map[string]any{
					"data":     cloneRevisionTestMap(data),
					"metadata": map[string]any{"version": b.versions[path]},
				},
			})
		case http.MethodPost:
			var in struct {
				Options struct {
					CAS uint64 `json:"cas"`
				} `json:"options"`
				Data map[string]string `json:"data"`
			}
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				b.t.Fatalf("decode CAS request: %v", err)
			}
			b.lastCAS = in.Options.CAS
			if in.Options.CAS != b.versions[path] {
				core.WriteJSON(w, http.StatusBadRequest, map[string]any{
					"errors": []string{"do-not-return-cas-body SUPER_SECRET_KEY super-secret-value 999"},
				})
				return
			}
			b.versions[path]++
			b.data[path] = cloneRevisionTestMap(in.Data)
			core.WriteJSON(w, http.StatusOK, map[string]any{
				"data": map[string]any{"version": b.versions[path]},
			})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	return mux
}

func cloneRevisionTestMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func TestOpenBaoStoreVersionedCAS(t *testing.T) {
	stub := newCASBaoStub(t)
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)
	store := newOpenBaoStoreForTest(t, srv.URL)
	ctx := context.Background()
	path := "services/web/env"

	empty, err := store.GetVersioned(ctx, path)
	if err != nil || empty.Version != 0 || len(empty.Data) != 0 {
		t.Fatalf("missing versioned read = %+v, %v", empty, err)
	}
	created, err := store.PutCAS(ctx, path, map[string]string{"A": "one"}, 0)
	if err != nil || created != 1 || stub.lastCAS != 0 {
		t.Fatalf("create CAS = version %d, err %v, request cas %d", created, err, stub.lastCAS)
	}
	snapshot, err := store.GetVersioned(ctx, path)
	if err != nil || snapshot.Version != 1 || snapshot.Data["A"] != "one" {
		t.Fatalf("versioned read = %+v, %v", snapshot, err)
	}

	updated, err := store.PutCAS(ctx, path, map[string]string{"A": "two", "B": "kept"}, snapshot.Version)
	if err != nil || updated != 2 || stub.lastCAS != 1 {
		t.Fatalf("update CAS = version %d, err %v, request cas %d", updated, err, stub.lastCAS)
	}
	if stub.data["default/"+path]["A"] != "two" || stub.data["default/"+path]["B"] != "kept" {
		t.Fatalf("CAS body was not stored exactly: %#v", stub.data["default/"+path])
	}
}

func TestOpenBaoStoreStaleCASIsSafeConflict(t *testing.T) {
	stub := newCASBaoStub(t)
	path := "default/services/web/env"
	stub.data[path] = map[string]string{"KEEP": "original"}
	stub.versions[path] = 2
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)
	store := newOpenBaoStoreForTest(t, srv.URL)

	_, err := store.PutCAS(context.Background(), "services/web/env", map[string]string{
		"SUPER_SECRET_KEY": "super-secret-value",
	}, 999)
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("stale CAS error = %v, want ErrConflict", err)
	}
	for _, forbidden := range []string{"SUPER_SECRET_KEY", "super-secret-value", "999", "do-not-return-cas-body"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("stale CAS error leaked %q: %v", forbidden, err)
		}
	}
	if stub.versions[path] != 2 || stub.data[path]["KEEP"] != "original" || len(stub.data[path]) != 1 {
		t.Fatalf("stale CAS mutated state: version=%d data=%#v", stub.versions[path], stub.data[path])
	}
}

func TestOpenBaoStoreCASReloginRetriesExactWrite(t *testing.T) {
	stub := newCASBaoStub(t)
	stub.rejectN = 1
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)
	store := newOpenBaoStoreForTest(t, srv.URL)

	version, err := store.PutCAS(context.Background(), "services/web/env", map[string]string{"A": "one"}, 0)
	if err != nil || version != 1 {
		t.Fatalf("CAS after re-login = %d, %v", version, err)
	}
	if stub.logins != 2 {
		t.Fatalf("CAS logins = %d, want 2", stub.logins)
	}
	if stub.lastCAS != 0 || stub.data["default/services/web/env"]["A"] != "one" {
		t.Fatalf("CAS retry changed request: cas=%d data=%#v", stub.lastCAS, stub.data)
	}
}

func TestOpenBaoStoreVersionedReadReloginRetries(t *testing.T) {
	stub := newCASBaoStub(t)
	stub.data["default/services/web/env"] = map[string]string{"A": "one"}
	stub.versions["default/services/web/env"] = 7
	stub.rejectN = 1
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)
	store := newOpenBaoStoreForTest(t, srv.URL)

	snapshot, err := store.GetVersioned(context.Background(), "services/web/env")
	if err != nil || snapshot.Version != 7 || snapshot.Data["A"] != "one" {
		t.Fatalf("versioned read after re-login = %+v, %v", snapshot, err)
	}
	if stub.logins != 2 {
		t.Fatalf("versioned read logins = %d, want 2", stub.logins)
	}
}

func TestOpenBaoStoreVersionedHTTPAndMalformedFailuresAreRedacted(t *testing.T) {
	const (
		tenantMarker   = "tenant-sensitive-marker"
		pathMarker     = "services/private-service/env"
		bodyMarker     = "upstream-sensitive-body"
		keyMarker      = "SUPER_SECRET_KEY"
		valueMarker    = "super-secret-value"
		expectedMarker = uint64(987654321)
	)
	operations := []struct {
		name string
		call func(context.Context, *openBaoStore) error
	}{
		{
			name: "read",
			call: func(ctx context.Context, store *openBaoStore) error {
				_, err := store.GetVersioned(ctx, pathMarker)
				return err
			},
		},
		{
			name: "write",
			call: func(ctx context.Context, store *openBaoStore) error {
				_, err := store.PutCAS(ctx, pathMarker, map[string]string{keyMarker: valueMarker}, expectedMarker)
				return err
			},
		},
	}
	failures := []struct {
		name       string
		status     int
		body       string
		wantStatus int
	}{
		{name: "server error", status: http.StatusInternalServerError, body: bodyMarker, wantStatus: http.StatusInternalServerError},
		{name: "malformed success response", status: http.StatusOK, body: "{" + bodyMarker},
	}

	for _, failure := range failures {
		for _, operation := range operations {
			t.Run(failure.name+"/"+operation.name, func(t *testing.T) {
				mux := http.NewServeMux()
				mux.HandleFunc("POST /v1/auth/kubernetes/login", func(w http.ResponseWriter, _ *http.Request) {
					core.WriteJSON(w, http.StatusOK, map[string]any{
						"auth": map[string]any{"client_token": "test-token", "lease_duration": 3600},
					})
				})
				mux.HandleFunc("/v1/tenants/data/", func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(failure.status)
					_, _ = w.Write([]byte(failure.body))
				})
				srv := httptest.NewServer(mux)
				t.Cleanup(srv.Close)
				store := newOpenBaoStoreForTest(t, srv.URL)

				err := operation.call(withTenant(context.Background(), tenantMarker), store)
				if err == nil {
					t.Fatal("versioned operation unexpectedly succeeded")
				}
				if failure.wantStatus != 0 {
					var statusErr *core.HTTPStatusError
					if !errors.As(err, &statusErr) || statusErr.Code != failure.wantStatus {
						t.Fatalf("status error = %#v, want HTTP %d", err, failure.wantStatus)
					}
				}
				assertVersionedErrorRedacted(t, err, srv.URL, tenantMarker, pathMarker, bodyMarker, keyMarker, valueMarker, strconv.FormatUint(expectedMarker, 10))
			})
		}
	}
}

type revisionRoundTripFunc func(*http.Request) (*http.Response, error)

func (f revisionRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestOpenBaoStoreVersionedTransportFailuresPreserveSentinelAndRedact(t *testing.T) {
	transportSentinel := errors.New("transport sentinel")
	const (
		addrMarker  = "https://openbao.internal/url-sensitive-marker"
		pathMarker  = "services/private-service/env"
		keyMarker   = "SUPER_SECRET_KEY"
		valueMarker = "super-secret-value"
	)
	store := &openBaoStore{
		addr: addrMarker, mount: baoMount, token: "cached-token", tokenExp: time.Now().Add(time.Hour),
		client: &http.Client{Transport: revisionRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, transportSentinel
		})},
	}
	operations := []struct {
		name string
		call func() error
	}{
		{name: "read", call: func() error {
			_, err := store.GetVersioned(context.Background(), pathMarker)
			return err
		}},
		{name: "write", call: func() error {
			_, err := store.PutCAS(context.Background(), pathMarker, map[string]string{keyMarker: valueMarker}, 42)
			return err
		}},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			err := operation.call()
			if !errors.Is(err, transportSentinel) {
				t.Fatalf("transport sentinel was not preserved: %v", err)
			}
			assertVersionedErrorRedacted(t, err, addrMarker, pathMarker, keyMarker, valueMarker, "42")
		})
	}
}

func assertVersionedErrorRedacted(t *testing.T, err error, forbidden ...string) {
	t.Helper()
	for _, material := range forbidden {
		if material != "" && strings.Contains(err.Error(), material) {
			t.Fatalf("versioned store error leaked %q: %v", material, err)
		}
	}
}
