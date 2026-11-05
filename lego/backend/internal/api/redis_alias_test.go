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
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// redis_alias_test.go — Render's deprecated /v1/redis family (w3/m274) is the
// Key Value API under its pre-rename spelling. These tests pin what makes it
// an alias rather than a second implementation: the same store, the same
// refusals, the same request-gate concessions, and only the two documented
// field renames (the list envelope key, the connection-info CLI command).

// redisAliasServer is the composed server (Render request gate included)
// over one ready Key Value kv1 with its connection Secret.
func redisAliasServer(t *testing.T) http.Handler {
	t.Helper()
	h, _ := serverWith(t,
		&core.Base{
			Client: fakeClient(
				conformKeyValue("kv1"),
				conformDatastoreSecret("kv1", map[string]string{"uri": "redis://default:p@kv1.default:6379"}),
			),
			Namespace: "default",
			Clock:     func() time.Time { return conformEpoch },
		},
		Deps{APIKeys: newFakeKeyStore()},
	)
	return h
}

func getJSON[T any](t *testing.T, h http.Handler, path string) T {
	t.Helper()
	w := do(t, h, http.MethodGet, path, testToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200: %s", path, w.Code, w.Body.String())
	}
	var out T
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("GET %s: decode %s: %v", path, w.Body.String(), err)
	}
	return out
}

func TestLegacyRedisRoutesAnswerTheKeyValueInRendersRedisShapes(t *testing.T) {
	h := redisAliasServer(t)

	// retrieve: redisDetail and keyValueDetail are the same schema, so the
	// body is byte-identical.
	kv := do(t, h, http.MethodGet, "/v1/key-value/kv1", testToken, "")
	redis := do(t, h, http.MethodGet, "/v1/redis/kv1", testToken, "")
	if redis.Code != http.StatusOK || redis.Body.String() != kv.Body.String() {
		t.Fatalf("GET /v1/redis/kv1 = %d %s\nwant the Key Value body %s", redis.Code, redis.Body.String(), kv.Body.String())
	}

	// list: the same item, keyed "redis" instead of "keyValue".
	kvList := getJSON[[]map[string]json.RawMessage](t, h, "/v1/key-value")
	redisList := getJSON[[]map[string]json.RawMessage](t, h, "/v1/redis")
	if len(redisList) != 1 || len(kvList) != 1 {
		t.Fatalf("lists = %d redis / %d key-value items, want 1 each", len(redisList), len(kvList))
	}
	if keys := slices.Sorted(maps.Keys(redisList[0])); !slices.Equal(keys, []string{"cursor", "redis"}) {
		t.Fatalf("redis list item keys = %v, want [cursor redis]", keys)
	}
	if string(redisList[0]["redis"]) != string(kvList[0]["keyValue"]) || string(redisList[0]["cursor"]) != string(kvList[0]["cursor"]) {
		t.Fatalf("redis list item %v differs from the Key Value item %v", redisList[0], kvList[0])
	}

	// connection-info: cliCommand is spelled redisCLICommand, nothing else moves.
	kvInfo := getJSON[map[string]string](t, h, "/v1/key-value/kv1/connection-info")
	redisInfo := getJSON[map[string]string](t, h, "/v1/redis/kv1/connection-info")
	want := map[string]string{
		"internalConnectionString": kvInfo["internalConnectionString"],
		"externalConnectionString": kvInfo["externalConnectionString"],
		"redisCLICommand":          kvInfo["cliCommand"],
	}
	if kvInfo["cliCommand"] == "" || !maps.Equal(redisInfo, want) {
		t.Fatalf("redis connection-info = %v, want %v", redisInfo, want)
	}
}

// redisAliasTwins pairs each by-id /redis request with its /key-value twin.
func redisAliasTwins(id string) [][2]string {
	var out [][2]string
	for _, route := range []struct{ method, suffix string }{
		{http.MethodGet, ""},
		{http.MethodPatch, ""},
		{http.MethodDelete, ""},
		{http.MethodGet, "/connection-info"},
		{http.MethodPost, "/suspend"},
		{http.MethodPost, "/resume"},
	} {
		out = append(out, [2]string{route.method, id + route.suffix})
	}
	return out
}

// TestLegacyRedisRoutesRefuseExactlyLikeKeyValue: an unknown id and a
// non-member caller get the Key Value route's status and body, byte for byte,
// on every by-id /redis operation — one verb, one authz gate.
func TestLegacyRedisRoutesRefuseExactlyLikeKeyValue(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		h := redisAliasServer(t)
		for _, twin := range redisAliasTwins("red-c185th5c2rvvnhbfiltg") {
			method, rest := twin[0], twin[1]
			body := ""
			if method == http.MethodPatch {
				body = `{"name":"cache"}`
			}
			kv := do(t, h, method, "/v1/key-value/"+rest, testToken, body)
			redis := do(t, h, method, "/v1/redis/"+rest, testToken, body)
			if redis.Code != http.StatusNotFound || redis.Body.String() != kv.Body.String() {
				t.Errorf("%s /v1/redis/%s = %d %s, want the Key Value 404 %d %s",
					method, rest, redis.Code, redis.Body.String(), kv.Code, kv.Body.String())
			}
		}
	})

	t.Run("non-member", func(t *testing.T) {
		mux := restMatrixFixture()
		serve := func(method, path string) *httptest.ResponseRecorder {
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "attacker", Method: "session"})
			body := ""
			if method == http.MethodPatch {
				body = `{"name":"cache"}`
			}
			req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
			req.Header.Set("content-type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			return rec
		}
		for _, twin := range redisAliasTwins(crossWorkspaceTarget) {
			method, rest := twin[0], twin[1]
			kv := serve(method, "/v1/key-value/"+rest)
			redis := serve(method, "/v1/redis/"+rest)
			if redis.Code < 400 || redis.Code >= 500 || redis.Code != kv.Code || redis.Body.String() != kv.Body.String() {
				t.Errorf("%s /v1/redis/%s as a non-member = %d %s, want the Key Value refusal %d %s",
					method, rest, redis.Code, redis.Body.String(), kv.Code, kv.Body.String())
			}
		}
		// The create names a workspace the caller is not a member of.
		kv := serve(http.MethodPost, "/v1/key-value")
		redis := serve(http.MethodPost, "/v1/redis")
		if redis.Code < 400 || redis.Code != kv.Code || redis.Body.String() != kv.Body.String() {
			t.Errorf("POST /v1/redis as a non-member = %d %s, want the Key Value refusal %d %s",
				redis.Code, redis.Body.String(), kv.Code, kv.Body.String())
		}
	})
}

// TestLegacyRedisRequestGateInheritsKeyValueConcessions: the strict Render
// request gate validates /redis against Render's redis* operations, so each
// concession bex makes on the key-value operations (query extensions, relaxed
// required fields, the public body field, hyphenated enums) must reach the
// alias too, or a request the Key Value route accepts 400s here.
func TestLegacyRedisRequestGateInheritsKeyValueConcessions(t *testing.T) {
	h := redisAliasServer(t)
	for _, tc := range []struct {
		name, method, path, body string
		want                     int
	}{
		// dryRun query, public body field, hyphenated eviction policy, and
		// no ownerId (relaxed requiredness): previewed, not created.
		{"create dry run", http.MethodPost, "/v1/redis?dryRun=true",
			`{"name":"cache","plan":"starter","public":true,"maxmemoryPolicy":"allkeys-lru"}`, http.StatusOK},
		{"suspended filter", http.MethodGet, "/v1/redis?suspended=not_suspended", "", http.StatusOK},
		{"repeated region filter", http.MethodGet, "/v1/redis?region=oregon&region=frankfurt", "", http.StatusOK},
		{"update confirm", http.MethodPatch, "/v1/redis/kv1?dryRun=true&confirm=x", `{"persistenceMode":"journal-snapshot"}`, http.StatusOK},
		// The gate still refuses what Render's contract does not allow.
		{"unknown query", http.MethodGet, "/v1/redis?bogus=1", "", http.StatusBadRequest},
		{"unknown region", http.MethodGet, "/v1/redis?region=oregon&region=mars", "", http.StatusBadRequest},
		{"unknown body field", http.MethodPost, "/v1/redis", `{"name":"cache","plan":"starter","bogus":1}`, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := do(t, h, tc.method, tc.path, testToken, tc.body)
			if w.Code != tc.want {
				t.Fatalf("%s %s = %d, want %d: %s", tc.method, tc.path, w.Code, tc.want, w.Body.String())
			}
			// The Key Value spelling answers the same request the same way.
			twin := strings.Replace(tc.path, "/v1/redis", "/v1/key-value", 1)
			if kv := do(t, h, tc.method, twin, testToken, tc.body); kv.Code != w.Code {
				t.Fatalf("%s %s = %d but its Key Value twin %s = %d: %s", tc.method, tc.path, w.Code, twin, kv.Code, kv.Body.String())
			}
		})
	}
	if list := getJSON[[]map[string]json.RawMessage](t, h, "/v1/redis"); len(list) != 1 {
		t.Fatalf("dry-run creates must not create a store: list has %d items, want only kv1", len(list))
	}
}
