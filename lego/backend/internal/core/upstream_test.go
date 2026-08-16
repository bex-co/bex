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
	"errors"
	"strings"
	"testing"
)

// codex round-8 #10: the bounded upstream decode rejects an oversized body
// before the unmarshal, never truncates into it, and the shared default client
// carries the total-request bound the raw http.DefaultClient never had.
func TestDecodeLimitedJSONRejectsOversizedBeforeDecoding(t *testing.T) {
	var out struct {
		Status string `json:"status"`
	}
	if err := DecodeLimitedJSON(strings.NewReader(`{"status":"success"}`), 64, &out); err != nil || out.Status != "success" {
		t.Fatalf("within-limit body must decode, got err=%v status=%q", err, out.Status)
	}
	// One byte over: rejected as too large — even though the JSON itself would
	// parse if truncated or not.
	err := DecodeLimitedJSON(strings.NewReader(`{"status":"success"}`), 8, &out)
	if !errors.Is(err, ErrUpstreamResponseTooLarge) {
		t.Fatalf("oversized body => %v, want ErrUpstreamResponseTooLarge", err)
	}
	if out.Status != "success" {
		t.Fatal("out must be untouched by a rejected decode")
	}
	// Exactly at the limit is allowed — the bound is inclusive.
	if err := DecodeLimitedJSON(strings.NewReader(`{"a":1}`), 7, &out); err != nil {
		t.Fatalf("exactly-at-limit body must decode, got %v", err)
	}
	// Malformed but in-limit JSON surfaces the unmarshal error, not the size.
	if err := DecodeLimitedJSON(strings.NewReader("not json"), 64, &out); err == nil || errors.Is(err, ErrUpstreamResponseTooLarge) {
		t.Fatalf("malformed in-limit body => %v, want a decode error", err)
	}
}

// The production bound is the constant the observability sources share, and
// the default client is actually bounded — the DefaultClient zero-timeout hole
// this round closed.
func TestUpstreamClientAndBoundAreWired(t *testing.T) {
	if MaxUpstreamResponseBytes != 64<<20 {
		t.Fatalf("MaxUpstreamResponseBytes = %d, want 64 MiB", MaxUpstreamResponseBytes)
	}
	if UpstreamClient.Timeout != UpstreamTimeout || UpstreamTimeout <= 0 {
		t.Fatalf("UpstreamClient.Timeout = %v, want the positive UpstreamTimeout %v", UpstreamClient.Timeout, UpstreamTimeout)
	}
	if UpstreamClient.Transport == nil {
		t.Fatal("UpstreamClient must pool connections (shared OryTransport), not dial per request")
	}
}
