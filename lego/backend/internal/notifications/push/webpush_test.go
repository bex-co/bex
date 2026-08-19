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

package push

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewWebPushAllUnsetIsNil(t *testing.T) {
	got, err := NewWebPush(WebPushConfig{})
	if err != nil || got != nil {
		t.Fatalf("NewWebPush(empty) = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestNewWebPushPartialConfigFailsClosed(t *testing.T) {
	pub, priv, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	for _, cfg := range []WebPushConfig{
		{PublicKey: pub, PrivateKey: priv},
		{PublicKey: pub, Subscriber: "mailto:webpush@bex.local"},
		{PrivateKey: priv, Subscriber: "mailto:webpush@bex.local"},
	} {
		if _, err := NewWebPush(cfg); err == nil {
			t.Fatalf("NewWebPush(%+v) succeeded, want startup error", cfg)
		}
	}
}

func TestNewWebPushMismatchedKeysFail(t *testing.T) {
	pub, _, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	_, priv, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewWebPush(WebPushConfig{
		PublicKey: pub, PrivateKey: priv, Subscriber: "mailto:webpush@bex.local",
	}); err == nil {
		t.Fatal("mismatched VAPID keys must fail closed")
	}
}

func TestWebPushLoopbackSendHeadersAndPrune(t *testing.T) {
	pub, priv, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	ua, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	p256dh := base64.RawURLEncoding.EncodeToString(ua.PublicKey().Bytes())
	auth := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 16))

	var saw410 bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Encoding") != "aes128gcm" {
			t.Errorf("Content-Encoding = %q", r.Header.Get("Content-Encoding"))
		}
		if authz := r.Header.Get("Authorization"); !strings.HasPrefix(authz, "vapid t=") || !strings.Contains(authz, ", k=") {
			t.Errorf("Authorization = %q", authz)
		}
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 || len(body) > MaxPayloadBytes+128 {
			t.Errorf("body length %d", len(body))
		}
		if r.URL.Path == "/gone" {
			saw410 = true
			w.WriteHeader(http.StatusGone)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	sender, err := NewWebPush(WebPushConfig{
		PublicKey: pub, PrivateKey: priv, Subscriber: "mailto:webpush@bex.local",
	}, WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	msg := WebPushMessage{
		Endpoint: server.URL + "/push", P256dh: p256dh, Auth: auth,
		Title: "Deploy failed", Body: "whoami failed.", Urgency: "important",
		Data: EnvelopeData{
			Schema: "bex.notification.v1", NotificationID: "evt-test",
			Event: "deploy_failed", Route: "/services/srv-test",
			Subject: "alice", WorkspaceID: "tea-a",
		},
	}
	if err := sender.Send(context.Background(), msg); err != nil {
		t.Fatalf("send created: %v", err)
	}
	msg.Endpoint = server.URL + "/gone"
	err = sender.Send(context.Background(), msg)
	var invalid *InvalidTokenError
	if !errors.As(err, &invalid) {
		t.Fatalf("410 error = %v, want InvalidTokenError", err)
	}
	if !saw410 {
		t.Fatal("expected the gone path to be hit")
	}
}

func TestWebPushRetryAndTransientDoNotLookLikeInvalidToken(t *testing.T) {
	pub, priv, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	ua, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	msg := WebPushMessage{
		P256dh: base64.RawURLEncoding.EncodeToString(ua.PublicKey().Bytes()),
		Auth:   base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{9}, 16)),
		Title:  "Deploy failed", Body: "whoami failed.",
		Data: EnvelopeData{
			Schema: "bex.notification.v1", NotificationID: "evt-test",
			Event: "deploy_failed", Route: "/services/srv-test",
		},
	}
	for _, code := range []int{http.StatusTooManyRequests, http.StatusBadGateway} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
		}))
		sender, err := NewWebPush(WebPushConfig{
			PublicKey: pub, PrivateKey: priv, Subscriber: "mailto:webpush@bex.local",
		}, WithHTTPClient(server.Client()))
		if err != nil {
			server.Close()
			t.Fatal(err)
		}
		msg.Endpoint = server.URL
		err = sender.Send(context.Background(), msg)
		server.Close()
		var invalid *InvalidTokenError
		if errors.As(err, &invalid) {
			t.Fatalf("status %d classified as invalid token", code)
		}
		if code == http.StatusTooManyRequests {
			var limited *RateLimitedError
			if !errors.As(err, &limited) {
				t.Fatalf("status 429 = %v, want RateLimitedError", err)
			}
		} else {
			var transient *TransientError
			if !errors.As(err, &transient) {
				t.Fatalf("status 502 = %v, want TransientError", err)
			}
		}
	}
}

func TestEncodeWebPushJSONRejectsAbsoluteRoute(t *testing.T) {
	_, err := encodeWebPushJSON(WebPushMessage{
		Title: "t", Body: "b",
		Data: EnvelopeData{Schema: "bex.notification.v1", NotificationID: "n", Event: "deploy_failed", Route: "https://evil.example/x"},
	})
	if err == nil {
		t.Fatal("absolute route must be rejected")
	}
}
