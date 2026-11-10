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
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func ptr[T any](v T) *T { return &v }

// TestResolveTimeout is Render's timeoutSeconds truth table, the same for
// every workspace (CLI v2.36 commit c280b0b6; the pinned CreateSandbox
// description: 0 is no timeout, positive values at least 60, at most 30 days).
func TestResolveTimeout(t *testing.T) {
	for _, tc := range []struct {
		name      string
		requested *int
		want      int
		invalid   bool
	}{
		{name: "omitted", want: 86400},
		{name: "zero is no timeout", requested: ptr(0), want: 0},
		{name: "minimum", requested: ptr(60), want: 60},
		{name: "1h", requested: ptr(3600), want: 3600},
		{name: "past the old Hobby cap", requested: ptr(86401), want: 86401},
		{name: "30 days", requested: ptr(2592000), want: 2592000},
		{name: "over 30 days", requested: ptr(2592001), invalid: true},
		{name: "under the minimum", requested: ptr(59), invalid: true},
		{name: "one second", requested: ptr(1), invalid: true},
		{name: "negative", requested: ptr(-1), invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveTimeout(tc.requested)
			if tc.invalid {
				var coded *core.CodedError
				if !errors.As(err, &coded) || coded.Code != "SANDBOX_TIMEOUT_INVALID" || coded.Params["minimumNonZero"] != 60 || coded.Params["max"] != 2592000 {
					t.Fatalf("err = %#v, want SANDBOX_TIMEOUT_INVALID naming minimumNonZero 60 and max 2592000", err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("resolveTimeout = %d, %v; want %d", got, err, tc.want)
			}
		})
	}
}

func TestTimeoutMetadataRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want int
	}{
		{timeoutNone, 0},
		{"3600", 3600},
		{"2592000", 2592000},
		{"0", 86400}, // pre-w5/m99 legacy: never immortal
		{"", 86400},
		{"junk", 86400},
	} {
		if got := timeoutFromMetadata(tc.raw); got != tc.want {
			t.Errorf("timeoutFromMetadata(%q) = %d, want %d", tc.raw, got, tc.want)
		}
	}
	for _, timeout := range []int{0, 30, 86400, 2592000} {
		if got := timeoutFromMetadata(timeoutMetadata(timeout)); got != timeout {
			t.Errorf("round trip %d = %d", timeout, got)
		}
	}
}

func TestOpenSandboxTimeout(t *testing.T) {
	for in, want := range map[int]int{0: 0, 1: 60, 59: 60, 60: 60, 86400: 86400, 2592000: 2592000} {
		if got := openSandboxTimeout(in); got != want {
			t.Errorf("openSandboxTimeout(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestLifetimeExpired(t *testing.T) {
	created := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	box := func(state, timeout string, created *time.Time, stamps map[string]time.Time) osSandbox {
		raw := osSandbox{ID: "os-1", Created: created, Metadata: map[string]string{metadataTimeout: timeout}}
		for k, v := range stamps {
			raw.Metadata[k] = stampMetadataTime(v)
		}
		raw.Status.State = state
		return raw
	}
	resumed := created.Add(3 * day)
	suspended := created.Add(2 * time.Hour)
	for _, tc := range []struct {
		name    string
		raw     osSandbox
		at      time.Time
		expired bool
	}{
		{"86400s expires at 24h", box("Running", "86400", &created, nil), created.Add(day), true},
		{"3600s not early", box("Running", "3600", &created, nil), created.Add(30 * time.Minute), false},
		{"sub-minute at its exact deadline", box("Running", "30", &created, nil), created.Add(30 * time.Second), true},
		{"no timeout never", box("Running", timeoutNone, &created, nil), created.Add(365 * day), false},
		{"missing createdAt conservative", box("Running", "86400", nil, nil), created.Add(2 * day), false},
		// A resume restarts the timeout, as on Render.
		{"resumed: timeout from the resume", box("Running", "86400", &created, map[string]time.Time{metadataResumedAt: resumed}), resumed.Add(day - time.Second), false},
		{"resumed: expires at resume + timeout", box("Running", "86400", &created, map[string]time.Time{metadataResumedAt: resumed}), resumed.Add(day), true},
		{"resuming counts from the resume", box("Resuming", "3600", &created, map[string]time.Time{metadataResumedAt: resumed, metadataSuspendedAt: suspended}), resumed.Add(30 * time.Minute), false},
		{"resumed without createdAt", box("Running", "3600", nil, map[string]time.Time{metadataResumedAt: resumed}), resumed.Add(time.Hour), true},
		// While suspended the timeout does not run; 30 days resumable.
		{"suspended past its timeout is kept", box("Paused", "86400", &created, map[string]time.Time{metadataSuspendedAt: suspended}), created.Add(20 * day), false},
		{"suspending is kept too", box("Pausing", "3600", &created, map[string]time.Time{metadataSuspendedAt: suspended}), created.Add(2 * day), false},
		{"suspended 30 days is reaped", box("Paused", "86400", &created, map[string]time.Time{metadataSuspendedAt: suspended}), suspended.Add(suspendedRetention), true},
		{"no-timeout suspended 30 days is reaped", box("Paused", timeoutNone, &created, map[string]time.Time{metadataSuspendedAt: suspended}), suspended.Add(suspendedRetention), true},
		{"no-timeout suspended 29 days is kept", box("Paused", timeoutNone, &created, map[string]time.Time{metadataSuspendedAt: suspended}), suspended.Add(29 * day), false},
		// A resume that failed after its stamp leaves the sandbox suspended:
		// the window still runs from the suspension.
		{"failed resume keeps the suspended window", box("Paused", "3600", &created, map[string]time.Time{metadataSuspendedAt: suspended, metadataResumedAt: resumed}), suspended.Add(suspendedRetention), true},
		// An unstamped pause (before t010, or an agent-session sandbox) keeps
		// the creation rule; a suspension stamp no longer applies once running.
		{"unstamped paused keeps the creation rule", box("Paused", "86400", &created, nil), created.Add(day), true},
		{"running ignores an old suspension stamp", box("Running", "86400", &created, map[string]time.Time{metadataSuspendedAt: suspended}), created.Add(day), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := lifetimeExpired(tc.raw, tc.at); got != tc.expired {
				t.Fatalf("lifetimeExpired = %v, want %v", got, tc.expired)
			}
		})
	}
	bad := box("Running", "86400", &created, nil)
	bad.Metadata[metadataResumedAt] = "yesterday"
	if !lifetimeExpired(bad, created.Add(day)) {
		t.Fatal("a malformed resume stamp must not extend the lifetime")
	}
}

// TestCreateTimeout drives Service.Create end to end against a stub
// OpenSandbox: what it is asked for, and what the caller is told.
func TestCreateTimeout(t *testing.T) {
	for _, tc := range []struct {
		name      string
		requested *int
		upstream  int // 0 = field omitted, no expiry
		reported  int
	}{
		{name: "zero", requested: ptr(0), upstream: 0, reported: 0},
		{name: "30 days", requested: ptr(2592000), upstream: 2592000, reported: 2592000},
		{name: "minimum", requested: ptr(60), upstream: 60, reported: 60},
		{name: "omitted", upstream: 86400, reported: 86400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got *int
			svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Timeout  *int              `json:"timeout"`
					Metadata map[string]string `json:"metadata"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode: %v", err)
				}
				got = body.Timeout
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id": "os-1", "metadata": body.Metadata,
					"status": map[string]string{"state": "Running"},
				})
			})
			box, err := svc.Create(callerCtx(), CreateRequest{Template: "node", TimeoutSeconds: tc.requested})
			if err != nil {
				t.Fatal(err)
			}
			upstream := 0
			if got != nil {
				upstream = *got
			}
			if upstream != tc.upstream || box.TimeoutSeconds != tc.reported {
				t.Fatalf("upstream timeout %d, reported %d; want %d, %d", upstream, box.TimeoutSeconds, tc.upstream, tc.reported)
			}
		})
	}
}

func TestCreateRefusesAnInvalidTimeoutBeforeServer(t *testing.T) {
	svc := stubServer(t, func(http.ResponseWriter, *http.Request) { t.Error("must not reach server") })
	for _, timeout := range []int{30, 2592001} {
		_, err := svc.Create(callerCtx(), CreateRequest{Template: "node", TimeoutSeconds: ptr(timeout)})
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != "SANDBOX_TIMEOUT_INVALID" {
			t.Fatalf("timeout %d: err = %v", timeout, err)
		}
	}
}

func TestSandboxFromOpenSandboxTimeouts(t *testing.T) {
	for raw, want := range map[string]int{"0": 86400, timeoutNone: 0, "604800": 604800} {
		box := sandboxFromOpenSandbox(osSandbox{
			ID:       "os-legacy",
			Metadata: map[string]string{metadataTimeout: raw, metadataWorkspace: "tea-a", metadataPlan: "starter"},
		}, "tea-a")
		if box.TimeoutSeconds != want {
			t.Errorf("metadata %q → %d, want %d", raw, box.TimeoutSeconds, want)
		}
	}
}
