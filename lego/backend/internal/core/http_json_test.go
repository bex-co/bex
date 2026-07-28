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

package core

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type jsonDecoderFixture struct {
	Name   string `json:"name"`
	Nested struct {
		Enabled bool `json:"enabled"`
	} `json:"nested"`
}

func decodeFixture(t *testing.T, strict bool, body string) (jsonDecoderFixture, error) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if strict {
		req = req.WithContext(WithStrictJSONDecoding(req.Context()))
	}
	var got jsonDecoderFixture
	err = DecodeJSON(req, &got)
	return got, err
}

func TestDecodeJSONStrict(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := decodeFixture(t, true, `{"name":"web","nested":{"enabled":true}}`)
		if err != nil || got.Name != "web" || !got.Nested.Enabled {
			t.Fatalf("got %#v, err %v", got, err)
		}
	})

	tests := []struct {
		name       string
		body       string
		contains   string
		notContain string
	}{
		{name: "top-level unknown", body: `{"name":"web","mystery":"secret-value"}`, contains: `unknown field "mystery"`, notContain: "secret-value"},
		{name: "nested unknown", body: `{"name":"web","nested":{"enabled":true,"mystery":"secret-value"}}`, contains: `unknown field "mystery"`, notContain: "secret-value"},
		{name: "trailing object", body: `{"name":"web"} {"name":"other"}`, contains: "exactly one JSON value", notContain: "other"},
		{name: "trailing scalar", body: `{"name":"web"} 42`, contains: "exactly one JSON value", notContain: "42"},
		{name: "wrong type", body: `{"name":{"private":"secret-value"}}`, contains: `field "name"`, notContain: "secret-value"},
		{name: "malformed", body: `{"name":"secret-value"`, contains: "invalid JSON request body", notContain: "secret-value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeFixture(t, true, tt.body)
			if err == nil || !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("error %q does not contain %q", err, tt.contains)
			}
			if strings.Contains(err.Error(), tt.notContain) {
				t.Fatalf("error leaked input %q: %v", tt.notContain, err)
			}
		})
	}
}

func TestDecodeJSONPreservesLenientBehaviorOutsideRenderGate(t *testing.T) {
	got, err := decodeFixture(t, false, `{"name":"web","unknown":true} {"ignored":true}`)
	if err != nil || got.Name != "web" {
		t.Fatalf("got %#v, err %v", got, err)
	}
}

func TestDecodeJSONStrictEmptyBodyPreservesEOF(t *testing.T) {
	_, err := decodeFixture(t, true, "")
	if err != io.EOF {
		t.Fatalf("got %v, want io.EOF", err)
	}
}

func TestUnmarshalJSONUsesContextStrictness(t *testing.T) {
	var got jsonDecoderFixture
	if err := UnmarshalJSON(context.Background(), []byte(`{"name":"web","unknown":true}`), &got); err != nil {
		t.Fatalf("lenient nested decode: %v", err)
	}
	ctx := WithStrictJSONDecoding(context.Background())
	if err := UnmarshalJSON(ctx, []byte(`{"name":"web","unknown":true}`), &got); err == nil {
		t.Fatal("strict nested decode accepted unknown field")
	}
}

func TestDecodeJSONStrictChecksAllowListEntries(t *testing.T) {
	type request struct {
		IPAllowList []IPAllowListEntry `json:"ipAllowList"`
	}
	body := `{"ipAllowList":[{"cidrBlock":"10.0.0.0/8","description":"private","mystery":"secret-value"}]}`
	req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var lenient request
	if err := DecodeJSON(req, &lenient); err != nil {
		t.Fatalf("lenient decode: %v", err)
	}
	req, _ = http.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req = req.WithContext(WithStrictJSONDecoding(req.Context()))
	var strict request
	err = DecodeJSON(req, &strict)
	if err == nil || !strings.Contains(err.Error(), `unknown field "mystery"`) || strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("strict nested-entry error = %v", err)
	}

	// The retired bare-string entry is rejected in strict mode too.
	req, _ = http.NewRequest(http.MethodPost, "/", strings.NewReader(`{"ipAllowList":["10.0.0.0/8"]}`))
	req = req.WithContext(WithStrictJSONDecoding(req.Context()))
	if err := DecodeJSON(req, &strict); err == nil {
		t.Fatal("strict decode accepted a bare allowlist string")
	}
}
