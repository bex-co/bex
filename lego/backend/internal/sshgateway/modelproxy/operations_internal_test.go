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

package modelproxy

import (
	"net/http"
	"testing"
)

// Muse Code (ADR108 D6) reaches Meta only through the Responses API; every
// other operation on that origin, and any query, stays refused.
func TestAllowedProviderOperationMetaIsResponsesOnly(t *testing.T) {
	const host = "api.meta.ai"
	if !allowedProviderOperation(host, http.MethodPost, "/v1/responses", "", "application/json") {
		t.Fatal("Muse's inference call was refused")
	}
	for _, tc := range []struct{ method, path, query, contentType string }{
		{http.MethodGet, "/v1/responses", "", "application/json"},
		{http.MethodPost, "/v1/responses", "", "text/plain"},
		{http.MethodPost, "/v1/responses", "stream=true", "application/json"},
		{http.MethodPost, "/v1/chat/completions", "", "application/json"},
		{http.MethodPost, "/v1/files", "", "application/json"},
		{http.MethodGet, "/v1/muse-code/models", "", "application/json"},
		{http.MethodPost, "/v1/responses/compact", "", "application/json"},
	} {
		if allowedProviderOperation(host, tc.method, tc.path, tc.query, tc.contentType) {
			t.Fatalf("%s %s?%s (%s) admitted for Meta", tc.method, tc.path, tc.query, tc.contentType)
		}
	}
}

// DeepSeek Harness (ADR114) speaks Chat Completions to OpenRouter; the
// operation is OpenRouter's alone and takes no query.
func TestAllowedProviderOperationOpenRouterChatCompletions(t *testing.T) {
	if !allowedProviderOperation("openrouter.ai", http.MethodPost, "/v1/chat/completions", "", "application/json") {
		t.Fatal("DeepSeek Harness's inference call was refused")
	}
	for _, tc := range []struct{ host, method, path, query string }{
		{"openrouter.ai", http.MethodPost, "/v1/chat/completions", "stream=true"},
		{"openrouter.ai", http.MethodGet, "/v1/chat/completions", ""},
		{"openrouter.ai", http.MethodPost, "/v1/completions", ""},
		{"openrouter.ai", http.MethodPost, "/v1/keys", ""},
		{"api.anthropic.com", http.MethodPost, "/v1/chat/completions", ""},
	} {
		if allowedProviderOperation(tc.host, tc.method, tc.path, tc.query, "application/json") {
			t.Fatalf("%s %s%s?%s admitted", tc.method, tc.host, tc.path, tc.query)
		}
	}
}
