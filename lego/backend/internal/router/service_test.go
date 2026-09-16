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
package router

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/graphql-go/graphql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type checker struct{ allowed bool }

func (c *checker) Check(context.Context, string, string, string) (bool, error) { return c.allowed, nil }

type workspaceResolver struct{ member bool }

func (w workspaceResolver) Tenant(context.Context, core.Identity) (string, bool) {
	if !w.member {
		return "", false
	}
	return BetaWorkspace, true
}
func (w workspaceResolver) IsMember(_ context.Context, _ core.Identity, tenant string) (bool, error) {
	return w.member && tenant == BetaWorkspace, nil
}

type members struct{}

func (members) GetTenantMember(context.Context, string, string) (store.TenantMember, error) {
	return store.TenantMember{Role: "admin"}, nil
}

func testService(url string) (*Service, context.Context, *checker) {
	authz := &checker{allowed: true}
	service := &Service{Base: &core.Base{Authz: authz, Workspace: workspaceResolver{member: true}}, URL: url, Secret: strings.Repeat("s", 32), Members: members{}}
	ctx := core.WithWorkspace(core.WithIdentity(context.Background(), core.Identity{Subject: "test-identity", Method: "session", Email: "test@example.com", EmailVerified: true}), BetaWorkspace)
	return service, ctx, authz
}

func TestBridgeAssertsWorkspaceWithFreshSignedTokens(t *testing.T) {
	seen := map[string]bool{}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		token := strings.Split(r.Header.Get("X-Bex-Identity-Assertion"), ".")
		if len(token) != 3 {
			t.Error("missing assertion")
			w.WriteHeader(401)
			return
		}
		mac := hmac.New(sha256.New, []byte(strings.Repeat("s", 32)))
		mac.Write([]byte(token[0] + "." + token[1]))
		if base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) != token[2] {
			t.Error("bad signature")
		}
		var claims map[string]any
		body, _ := base64.RawURLEncoding.DecodeString(token[1])
		json.Unmarshal(body, &claims)
		if claims["workspace"] != "workspace:"+BetaWorkspace || claims["sub"] != "test-identity" || claims["workspace_role"] != "admin" || claims["aud"] != "backend-v2" || claims["iss"] != "bexbe" {
			t.Errorf("wrong assertion claims: %v", claims)
		}
		if claims["exp"].(float64)-claims["iat"].(float64) != 60 || time.Now().Unix() > int64(claims["exp"].(float64)) {
			t.Error("bad TTL")
		}
		nonce := claims["jti"].(string)
		if seen[nonce] {
			t.Error("reused assertion")
		}
		seen[nonce] = true
		var request struct {
			Query     string
			Variables map[string]any
		}
		json.NewDecoder(r.Body).Decode(&request)
		if strings.Contains(request.Query, "RouterWorkspace") {
			w.Write([]byte(`{"data":{"myWorkspaces":[{"id":"ws-mapped"}]}}`))
			return
		}
		if request.Variables["workspaceId"] != "ws-mapped" {
			t.Error("not using mapped workspace")
		}
		w.Write([]byte(`{"data":{"workspaceQuotaStatus":{"observedAt":"2026-09-12T00:00:00Z","windows":[{"kind":"MONTHLY","utilizationBps":481,"resetsAt":"2026-10-01T00:00:00Z"}]},"accessKeys":{"edges":[{"node":{"id":"key-one","name":"Example","accessKey":"test-only-key","createdAt":"2026-01-01T00:00:00Z"}}]}}}`))
	}))
	defer server.Close()
	s, ctx, _ := testService(server.URL)
	result, err := s.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 || result.Keys[0].ID != "key-one" || result.Quota.Windows[0].UtilizationBps != 481 {
		t.Fatalf("unexpected overview: %#v", result)
	}
}

func TestDeniedRequestsNeverReachUpstream(t *testing.T) {
	activeCase := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unauthorized upstream request: %s", activeCase)
	}))
	defer server.Close()
	for _, test := range []string{"foreign-workspace", "non-member", "denied-role", "unverified-email", "machine", "missing-config"} {
		t.Run(test, func(t *testing.T) {
			activeCase = test
			s, ctx, c := testService(server.URL)
			switch test {
			case "foreign-workspace":
				ctx = core.WithWorkspace(ctx, "tea-other")
			case "non-member":
				s.Workspace = workspaceResolver{}
			case "denied-role":
				c.allowed = false
			case "unverified-email":
				ctx = core.WithIdentity(ctx, core.Identity{Subject: "test-identity", Method: "session"})
			case "machine":
				ctx = core.WithIdentity(ctx, core.Identity{Subject: "test-identity", Method: "oauth2", EmailVerified: true})
			case "missing-config":
				s.Secret = ""
			}
			if _, err := s.Overview(ctx); err == nil {
				t.Error("expected denial")
			}
		})
	}
}

func TestUpstreamErrorsAndRedirectsAreNotExposed(t *testing.T) {
	for _, status := range []int{200, 302, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "https://example.com")
				w.WriteHeader(status)
				w.Write([]byte(`{"errors":[{"message":"private upstream details"}]}`))
			}))
			defer server.Close()
			s, ctx, _ := testService(server.URL)
			if _, err := s.Overview(ctx); !errors.Is(err, ErrUnavailable) {
				t.Errorf("expected sanitized unavailable, got %v", err)
			}
		})
	}
}

func TestForeignKeyIsRejectedBeforeMutation(t *testing.T) {
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct{ Query string }
		json.NewDecoder(r.Body).Decode(&request)
		if strings.Contains(request.Query, "mutation") {
			writes++
		}
		if strings.Contains(request.Query, "RouterWorkspace") {
			w.Write([]byte(`{"data":{"myWorkspaces":[{"id":"ws-mapped"}]}}`))
		} else {
			w.Write([]byte(`{"data":{"accessKeys":{"edges":[]}}}`))
		}
	}))
	defer server.Close()
	s, ctx, _ := testService(server.URL)
	if _, err := s.Delete(ctx, "foreign-key"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("got %v", err)
	}
	if _, err := s.Update(ctx, "foreign-key", "Name", Options{}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("got %v", err)
	}
	if writes != 0 {
		t.Fatal("sent foreign key mutation")
	}
}

func TestQuotaFailurePreservesKeysAndKeyMutations(t *testing.T) {
	writes := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Query     string
			Variables map[string]any
		}
		json.NewDecoder(r.Body).Decode(&request)
		switch {
		case strings.Contains(request.Query, "RouterWorkspace"):
			w.Write([]byte(`{"data":{"myWorkspaces":[{"id":"ws-mapped"}]}}`))
		case strings.Contains(request.Query, "RouterQuota"):
			w.Write([]byte(`{"errors":[{"message":"quota collector offline"}]}`))
		case strings.Contains(request.Query, "RouterKeys"):
			w.Write([]byte(`{"data":{"accessKeys":{"edges":[{"node":{"id":"key-one","name":"Example","accessKey":"test-only-key","createdAt":"2026-01-01T00:00:00Z"}}]}}}`))
		case strings.Contains(request.Query, "RouterCreate"):
			if request.Variables["workspaceId"] != "ws-mapped" || request.Variables["name"] != "New key" {
				t.Error("wrong create arguments")
			}
			writes = append(writes, "create")
			w.Write([]byte(`{"data":{"createAccessKey":true}}`))
		case strings.Contains(request.Query, "RouterUpdate"):
			if request.Variables["accessKeyId"] != "key-one" || request.Variables["allowOrigin"] != "https://example.com" {
				t.Error("wrong update arguments")
			}
			writes = append(writes, "update")
			w.Write([]byte(`{"data":{"updateAccessKey":true}}`))
		case strings.Contains(request.Query, "RouterDelete"):
			if request.Variables["accessKey"] != "test-only-key" {
				t.Error("delete must use owned key's value")
			}
			writes = append(writes, "delete")
			w.Write([]byte(`{"data":{"deleteAccessKey":true}}`))
		}
	}))
	defer server.Close()
	s, ctx, _ := testService(server.URL)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: s.GraphQLQuery()}), Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: s.GraphQLMutation()})})
	if err != nil {
		t.Fatal(err)
	}
	result := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `query { routerOverview(ownerId:"` + BetaWorkspace + `") { keys { id name } quota { observedAt } } }`})
	if len(result.Errors) > 0 {
		t.Fatal(result.Errors)
	}
	overview := result.Data.(map[string]any)["routerOverview"].(map[string]any)
	if overview["quota"] != nil || len(overview["keys"].([]any)) != 1 {
		t.Fatalf("unexpected partial result: %v", overview)
	}
	if ok, err := s.Create(ctx, "New key"); err != nil || !ok {
		t.Fatalf("create: %v", err)
	}
	if ok, err := s.Update(ctx, "key-one", "Renamed", Options{AllowOrigin: "https://example.com"}); err != nil || !ok {
		t.Fatalf("update: %v", err)
	}
	if ok, err := s.Delete(ctx, "key-one"); err != nil || !ok {
		t.Fatalf("delete: %v", err)
	}
	if strings.Join(writes, ",") != "create,update,delete" {
		t.Fatalf("writes: %v", writes)
	}
	result = graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `query { routerOverview(ownerId:"tea-other") { keys { id } } }`})
	if len(result.Errors) == 0 {
		t.Fatal("GraphQL ignored ownerId")
	}
}
