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

package notifications

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestDeviceSubscriptionRESTAndGraphQLStaySecretFree(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "alice", Method: "oauth2"})
	newService := func() *Service {
		return newTestService(newFakeStore(), fakeWorkspace{"alice": "tea-a"}, nil, nil)
	}

	restSvc := newService()
	mux := http.NewServeMux()
	restSvc.RegisterREST(mux)
	req := httptest.NewRequest(http.MethodPost, "/v1/notification-device-subscriptions", strings.NewReader(`{
		"deviceId":"phone", "sessionId":"session-rest", "provider":"expo", "platform":"ios",
		"token":"ExponentPushToken[rest-secret]"
	}`)).WithContext(ctx)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("REST register = %d: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "rest-secret") || strings.Contains(rec.Body.String(), `"token"`) {
		t.Fatalf("REST response leaked token: %s", rec.Body)
	}

	gqlSvc := newService()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: gqlSvc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: gqlSvc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	result := graphql.Do(graphql.Params{
		Schema: schema, Context: ctx,
		RequestString: `mutation {
			registerNotificationDeviceSubscription(
				deviceId:"phone", sessionId:"session-graphql", provider:"expo", platform:"android",
				token:"ExponentPushToken[graphql-secret]"
			) { deviceId provider platform preferenceRef createdAt updatedAt lastRegisteredAt }
		}`,
	})
	if len(result.Errors) > 0 {
		t.Fatalf("GraphQL register errors: %v", result.Errors)
	}
	raw, _ := json.Marshal(result.Data)
	if strings.Contains(string(raw), "graphql-secret") || strings.Contains(string(raw), `"token"`) {
		t.Fatalf("GraphQL response leaked token: %s", raw)
	}

	// The output type has no token field at all, not merely a redacted value.
	probe := graphql.Do(graphql.Params{
		Schema: schema, Context: ctx,
		RequestString: `{ notificationDeviceSubscriptions { deviceId token } }`,
	})
	if len(probe.Errors) == 0 {
		t.Fatal("GraphQL unexpectedly exposed a token field")
	}
}

func TestWebPushSubscriptionRESTAndGraphQLStaySecretFree(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "alice", Method: "oauth2"})
	ua, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	p256dh := base64.RawURLEncoding.EncodeToString(ua.PublicKey().Bytes())
	auth := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{5}, 16))
	endpoint := "https://fcm.googleapis.com/fcm/send/webpush-secret"

	newService := func() *Service {
		return newTestService(newFakeStore(), fakeWorkspace{"alice": "tea-a"}, nil, nil)
	}
	restSvc := newService()
	mux := http.NewServeMux()
	restSvc.RegisterREST(mux)
	body := `{"browserId":"wp-one","endpoint":"` + endpoint + `","p256dh":"` + p256dh + `","auth":"` + auth + `"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/notification-webpush-subscriptions", strings.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("REST register = %d: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "webpush-secret") || strings.Contains(rec.Body.String(), `"endpoint"`) || strings.Contains(rec.Body.String(), `"p256dh"`) {
		t.Fatalf("REST response leaked capability: %s", rec.Body)
	}

	gqlSvc := newService()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: gqlSvc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: gqlSvc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	result := graphql.Do(graphql.Params{
		Schema: schema, Context: ctx,
		RequestString: `mutation {
			registerNotificationWebPushSubscription(
				browserId:"wp-one", endpoint:"` + endpoint + `", p256dh:"` + p256dh + `", auth:"` + auth + `"
			) { browserId createdAt updatedAt lastRegisteredAt }
		}`,
	})
	if len(result.Errors) > 0 {
		t.Fatalf("GraphQL register errors: %v", result.Errors)
	}
	raw, _ := json.Marshal(result.Data)
	if strings.Contains(string(raw), "webpush-secret") || strings.Contains(string(raw), `"endpoint"`) {
		t.Fatalf("GraphQL response leaked capability: %s", raw)
	}
	probe := graphql.Do(graphql.Params{
		Schema: schema, Context: ctx,
		RequestString: `{ notificationWebPushSubscriptions { browserId endpoint p256dh auth } }`,
	})
	if len(probe.Errors) == 0 {
		t.Fatal("GraphQL unexpectedly exposed webpush secrets")
	}
}
