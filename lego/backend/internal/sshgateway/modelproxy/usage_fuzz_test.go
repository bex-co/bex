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
	"encoding/json"
	"testing"
)

// modelProxyFuzzSeeds are the request bodies and metered responses the
// package's own tests exercise — a pinned-model Messages request, a Responses
// event, an Anthropic message_start/message_delta pair — plus the shapes the
// parser must reject without panicking: an empty or whitespace-only body, a
// non-object, a duplicate "model", a client "provider", trailing data, a
// null usage, and an unterminated SSE line. Seeds live as source so a reviewer
// sees exactly what the untrusted surface is probed with; the on-disk
// testdata/fuzz cache is never committed.
func modelProxyFuzzSeeds(f *testing.F) {
	f.Helper()
	const pinned = "vendor/pinned-1"
	for _, body := range []string{
		`{"model":"vendor/pinned-1","messages":[]}`,
		`{"messages":[],"model":"vendor/pinned-1"}`,
		`{"model":"vendor/pinned-1","model":"vendor/pinned-1"}`,
		`{"model":"vendor/pinned-1","provider":{"allow_fallbacks":true}}`,
		`{"model":"vendor/pinned-1","models":["other"]}`,
		`{"model":"vendor/pinned-1","plugins":[]}`,
		`{"model":"vendor/pinned-1"} trailing`,
		`{"model":"other"}`,
		`{}`,
		``,
		" \t\r\n",
		`[]`,
		`null`,
		`{"model":"vendor/pinned-1",`,
	} {
		f.Add(body, pinned)
	}
}

// FuzzPinnedModelRequest drives the platform route's request gate and the data
// policy merge that follows it. Both read a tenant's raw request body: the gate
// by token-walking JSON, the merge by slicing off the object's closing brace on
// the strength of the gate's verdict. Whatever bytes a client sends, the gate
// must decide without panicking, and a body it accepts must survive the merge
// as one JSON object that still pins the model and carries the platform data
// policy.
func FuzzPinnedModelRequest(f *testing.F) {
	modelProxyFuzzSeeds(f)
	f.Fuzz(func(t *testing.T, body, pinned string) {
		raw := []byte(body)
		if !pinnedModelRequest(raw, pinned) {
			return
		}
		merged := withPlatformDataPolicy(raw)
		var object map[string]any
		if err := json.Unmarshal(merged, &object); err != nil {
			t.Fatalf("pinnedModelRequest(%q) accepted, but withPlatformDataPolicy produced %q: %v", body, merged, err)
		}
		if object["model"] != pinned {
			t.Fatalf("merged %q lost the pinned model: model = %v, want %q", body, object["model"], pinned)
		}
		if _, ok := object["provider"]; !ok {
			t.Fatalf("merged %q carries no provider data policy: %q", body, merged)
		}
	})
}

// FuzzUsageMeter drives the tee that reads a relayed platform-route response
// for billing. Its bytes come from the model vendor and its chunk boundaries
// from the network, so an SSE line or JSON body can split anywhere; Write and
// result must never panic, and a meter that reports usage must report only
// non-negative counts.
func FuzzUsageMeter(f *testing.F) {
	for _, seed := range []struct {
		contentType string
		step        int
		body        string
	}{
		{"application/json", 4096, `{"object":"message","usage":{"input_tokens":10,"output_tokens":2,"cost":0.01}}`},
		{"application/json", 3, `{"object":"response","usage":{"input_tokens":10,"output_tokens":2,"input_tokens_details":{"cached_tokens":4}}}`},
		{"application/json", 1, `{"usage":null}`},
		{"application/json", 7, `{`},
		{"text/event-stream", 5, "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":10}}}\n\n"},
		{"text/event-stream", 1, "data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":2}}\n"},
		{"text/event-stream", 2, "data:{\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":3,\"output_tokens\":1}}}\n"},
		{"text/event-stream", 3, "data: {\"type\":\"message_start\""},
		{"text/event-stream", 4, "\n\n\n"},
		{"text/event-stream; charset=utf-8", 2, "data: {\"usage\":null}\n"},
		{"", 1, ""},
	} {
		if seed.step < 1 {
			seed.step = 1
		}
		f.Add(seed.contentType, seed.step, seed.body)
	}
	f.Fuzz(func(t *testing.T, contentType string, step int, body string) {
		meter := newUsageMeter(contentType)
		if step < 1 {
			step = 1
		}
		for len(body) > 0 {
			chunk := min(step, len(body))
			if _, err := meter.Write([]byte(body[:chunk])); err != nil {
				t.Fatalf("usageMeter.Write returned an error: %v", err)
			}
			body = body[chunk:]
		}
		tokens, cost, ok, _ := meter.result()
		if !ok {
			return
		}
		for i, token := range tokens {
			if token < 0 {
				t.Fatalf("usageMeter reported a negative count %d at %d for %q", token, i, contentType)
			}
		}
		if cost != nil && *cost < 0 {
			t.Fatalf("usageMeter reported a negative cost %v", *cost)
		}
	})
}
