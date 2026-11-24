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

package main_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Since v2.38.0 the pinned client prints a 429's message after "too many
// requests: " (render-oss/cli pkg/client/client.go ErrorFromResponse), where
// v2.37.0 printed "too many requests" alone. These are the 429 bodies bex-api
// writes for a command the CLI runs (w22/m604 t005), each with the line the
// user reads. None may show the code: the client prints no 429 code, and a
// message never names one.
func TestPinnedClientPrintsBexTooManyRequestsMessages(t *testing.T) {
	const group = "sbg-cph1rs3idesc73a2b2mg"
	for _, tc := range []struct {
		name string
		args []string
		// body is what bex-api writes: core.WriteErr for a coded refusal, the
		// limiter (internal/api/ratelimit.go) and the auth gate's admission
		// (internal/api/authadmission.go) for the rest.
		body string
		code string
		line string
	}{{
		name: "image imports in progress (SANDBOX_BUILD_LIMIT)",
		args: []string{"sandbox-groups", "images", "import", "python:3.13", "--name", "python", "--group", group},
		body: `{"id":"too_many_requests","code":"SANDBOX_BUILD_LIMIT","error":"this sandbox group already has 2 images importing, its limit; wait for one to finish, then import again","message":"this sandbox group already has 2 images importing, its limit; wait for one to finish, then import again","params":{"limit":2}}`,
		code: "SANDBOX_BUILD_LIMIT",
		line: "too many requests: this sandbox group already has 2 images importing, its limit; wait for one to finish, then import again",
	}, {
		name: "the group's concurrency limit (SANDBOX_GROUP_CONCURRENCY_LIMIT)",
		args: []string{"sandboxes", "create", "--group", group},
		body: `{"id":"too_many_requests","code":"SANDBOX_GROUP_CONCURRENCY_LIMIT","error":"sandbox group \"agents\" is at its concurrency limit of 2 active sandboxes (2 active); suspend or terminate one, raise the limit with PATCH /v1/sandbox-groups/` + group + `, or use another group","message":"sandbox group \"agents\" is at its concurrency limit of 2 active sandboxes (2 active); suspend or terminate one, raise the limit with PATCH /v1/sandbox-groups/` + group + `, or use another group","params":{"active":2,"concurrencyLimit":2,"sandboxGroupId":"` + group + `"}}`,
		code: "SANDBOX_GROUP_CONCURRENCY_LIMIT",
		line: `too many requests: sandbox group "agents" is at its concurrency limit of 2 active sandboxes (2 active); suspend or terminate one, raise the limit with PATCH /v1/sandbox-groups/` + group + `, or use another group`,
	}, {
		name: "a sandbox operation's per-user rate (w22/m443)",
		args: []string{"sandboxes", "create", "--group", group},
		body: `{"id":"rate_limited","message":"rate limit exceeded; retry in 3 seconds"}`,
		code: "rate_limited",
		line: "too many requests: rate limit exceeded; retry in 3 seconds",
	}, {
		name: "Render's per-endpoint rate or the general limit",
		args: []string{"services", "-o", "text"},
		body: `{"id":"rate_limited","message":"rate limit exceeded; retry in 1 second"}`,
		code: "rate_limited",
		line: "too many requests: rate limit exceeded; retry in 1 second",
	}, {
		name: "authentication shed under load",
		args: []string{"services", "-o", "text"},
		body: `{"id":"auth_overloaded","message":"authentication upstream overloaded; retry shortly"}`,
		code: "auth_overloaded",
		line: "too many requests: authentication upstream overloaded; retry shortly",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(api.Close)

			_, stderr, err := runBexAgainst(t, api.URL, tc.args...)
			if err == nil {
				t.Fatalf("bex %v succeeded on a 429", tc.args)
			}
			if !strings.Contains(stderr, tc.line) {
				t.Errorf("bex %v printed:\n%s\nwant the line:\n%s", tc.args, stderr, tc.line)
			}
			if strings.Contains(stderr, tc.code) {
				t.Errorf("bex %v printed the code %s:\n%s", tc.args, tc.code, stderr)
			}
		})
	}
}
