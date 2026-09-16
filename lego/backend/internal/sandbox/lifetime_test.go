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

package sandbox

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestEffectiveTimeoutSeconds(t *testing.T) {
	for _, tc := range []struct {
		in, want int
	}{
		{0, maxSandboxTimeout},
		{-1, maxSandboxTimeout}, // callers validate before normalize; defensive
		{60, 60},
		{3600, 3600},
		{86400, 86400},
	} {
		if got := EffectiveTimeoutSeconds(tc.in); got != tc.want {
			t.Errorf("EffectiveTimeoutSeconds(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestLifetimeExpired(t *testing.T) {
	created := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	now := created.Add(24 * time.Hour)
	if !LifetimeExpired(&created, 0, now) {
		t.Fatal("legacy timeout 0 must expire at the 24h default bound")
	}
	if LifetimeExpired(&created, 3600, created.Add(30*time.Minute)) {
		t.Fatal("explicit 3600s must not expire early")
	}
	if LifetimeExpired(nil, 0, now) {
		t.Fatal("missing createdAt must not expire (conservative)")
	}
}

func TestCreateNormalizesZeroTimeout(t *testing.T) {
	var gotTimeout int
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			return
		}
		var body struct {
			Timeout  int               `json:"timeout"`
			Metadata map[string]string `json:"metadata"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		gotTimeout = body.Timeout
		meta := body.Metadata
		if meta == nil {
			meta = map[string]string{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":       "os-1",
			"metadata": meta,
			"status":   map[string]string{"state": "Running"},
			"created":  time.Now().UTC().Format(time.RFC3339Nano),
			"image":    map[string]string{"uri": "node:20"},
		})
	})
	box, err := svc.Create(callerCtx(), CreateRequest{Template: "node", TimeoutSeconds: 0})
	if err != nil {
		t.Fatal(err)
	}
	if gotTimeout != maxSandboxTimeout {
		t.Fatalf("OpenSandbox create timeout = %d, want %d", gotTimeout, maxSandboxTimeout)
	}
	if box.TimeoutSeconds != maxSandboxTimeout {
		t.Fatalf("read shape timeoutSeconds = %d, want %d", box.TimeoutSeconds, maxSandboxTimeout)
	}
}

func TestSandboxFromOpenSandboxLegacyZero(t *testing.T) {
	created := time.Now().UTC()
	box := sandboxFromOpenSandbox(osSandbox{
		ID:       "os-legacy",
		Metadata: map[string]string{metadataTimeout: "0", metadataWorkspace: "tea-a", metadataPlan: "starter"},
		Created:  &created,
		Status: struct {
			State string `json:"state"`
		}{State: "Running"},
	}, "tea-a")
	if box.TimeoutSeconds != maxSandboxTimeout {
		t.Fatalf("legacy metadata 0 → %d, want %d", box.TimeoutSeconds, maxSandboxTimeout)
	}
}
