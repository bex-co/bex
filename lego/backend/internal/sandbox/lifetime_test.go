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
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func ptr[T any](v T) *T { return &v }

// TestResolveTimeout is Render's timeoutSeconds truth table (pinned CLI v2.35
// `--timeout` help + CreateSandbox invalid_timeout_seconds; w2/m263 D3=A).
func TestResolveTimeout(t *testing.T) {
	for _, tc := range []struct {
		name      string
		requested *int
		plan      string
		want      int
		invalid   bool
	}{
		{name: "omitted paid", plan: "pro", want: 86400},
		{name: "omitted hobby", plan: "hobby", want: 86400},
		{name: "omitted unknown plan", plan: "", want: 86400},
		{name: "paid zero is no timeout", requested: ptr(0), plan: "pro", want: 0},
		{name: "scale zero is no timeout", requested: ptr(0), plan: "scale", want: 0},
		{name: "store-off zero is no timeout", requested: ptr(0), plan: "", want: 0},
		{name: "paid 7 days", requested: ptr(604800), plan: "pro", want: 604800},
		{name: "paid max", requested: ptr(2592000), plan: "enterprise", want: 2592000},
		{name: "paid over max", requested: ptr(2592001), plan: "pro", invalid: true},
		{name: "paid short", requested: ptr(30), plan: "pro", want: 30},
		{name: "paid negative", requested: ptr(-1), plan: "pro", invalid: true},
		{name: "hobby zero is the 24h cap", requested: ptr(0), plan: "hobby", want: 86400},
		{name: "hobby 1h", requested: ptr(3600), plan: "hobby", want: 3600},
		{name: "hobby max", requested: ptr(86400), plan: "hobby", want: 86400},
		{name: "hobby over 24h", requested: ptr(86401), plan: "hobby", invalid: true},
		{name: "hobby 30 days", requested: ptr(2592000), plan: "hobby", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveTimeout(tc.requested, tc.plan)
			if tc.invalid {
				var coded *core.CodedError
				if !errors.As(err, &coded) || coded.Code != "SANDBOX_TIMEOUT_INVALID" {
					t.Fatalf("err = %v, want SANDBOX_TIMEOUT_INVALID", err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("resolveTimeout = %d, %v; want %d", got, err, tc.want)
			}
		})
	}
}

func TestHobbyRefusalNamesThePlan(t *testing.T) {
	_, err := resolveTimeout(ptr(86401), "hobby")
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Params["plan"] != "hobby" || coded.Params["max"] != 86400 {
		t.Fatalf("hobby refusal = %#v", err)
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

// TestCreateTimeoutByPlan drives Service.Create end to end against a stub
// OpenSandbox: what it is asked for, and what the caller is told.
func TestCreateTimeoutByPlan(t *testing.T) {
	for _, tc := range []struct {
		name      string
		plan      string
		requested *int
		upstream  int // 0 = field omitted, no expiry
		reported  int
	}{
		{name: "paid zero", plan: "pro", requested: ptr(0), upstream: 0, reported: 0},
		{name: "paid 30 days", plan: "pro", requested: ptr(2592000), upstream: 2592000, reported: 2592000},
		{name: "paid sub-minute", plan: "pro", requested: ptr(30), upstream: 60, reported: 30},
		{name: "hobby zero", plan: "hobby", requested: ptr(0), upstream: 86400, reported: 86400},
		{name: "omitted", plan: "pro", upstream: 86400, reported: 86400},
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
			svc.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return tc.plan, nil })
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

func TestCreateRefusesOverPlanMaximumBeforeServer(t *testing.T) {
	svc := stubServer(t, func(http.ResponseWriter, *http.Request) { t.Error("must not reach server") })
	svc.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return "hobby", nil })
	_, err := svc.Create(callerCtx(), CreateRequest{Template: "node", TimeoutSeconds: ptr(86401)})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "SANDBOX_TIMEOUT_INVALID" {
		t.Fatalf("err = %v", err)
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
