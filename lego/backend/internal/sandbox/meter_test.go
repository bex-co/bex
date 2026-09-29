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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeMeterStore struct {
	keys         []store.SandboxTenantKey
	observations []store.SandboxMeterObservation
	missingWS    string
	missingSeen  []string
	labels       map[string]string // "<workspace>/<sandbox>" -> display name
}

func (f *fakeMeterStore) RecordResourceDisplayNames(_ context.Context, ws string, records []store.ResourceDisplayName) error {
	if f.labels == nil {
		f.labels = map[string]string{}
	}
	for _, r := range records {
		f.labels[ws+"/"+r.ID] = r.Name
	}
	return nil
}

func (f *fakeMeterStore) ListSandboxTenantKeys(context.Context) ([]store.SandboxTenantKey, error) {
	return f.keys, nil
}
func (f *fakeMeterStore) ObserveSandboxMeter(_ context.Context, obs store.SandboxMeterObservation) error {
	f.observations = append(f.observations, obs)
	return nil
}
func (f *fakeMeterStore) TerminateMissingSandboxMeters(_ context.Context, ws string, seen []string, _ time.Time) error {
	f.missingWS, f.missingSeen = ws, append([]string(nil), seen...)
	return nil
}

func TestComputeWeightMilliUsesAgentCoreCPUAndMemoryRatio(t *testing.T) {
	for _, tc := range []struct {
		cpu, memory string
		want        int64
	}{
		{cpu: "500m", memory: "512Mi", want: 553},
		{cpu: "1", memory: "1Gi", want: 1106},
	} {
		got, err := computeWeightMilli(tc.cpu, tc.memory)
		if err != nil || got != tc.want {
			t.Errorf("weight(%s,%s) = %d, %v; want %d", tc.cpu, tc.memory, got, err, tc.want)
		}
	}
	if _, err := computeWeightMilli("bad", "512Mi"); err == nil {
		t.Error("invalid CPU accepted")
	}
}

func TestMeterPollUsesTenantScopeAndAuthoritativePhase(t *testing.T) {
	var gotKey string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get(tenantKeyHeader)
		_, _ = fmt.Fprint(w, `[{"id":"os-1","metadata":{`+
			`"bex.co/workspace":"tea-a","bex.co/plan":"starter",`+
			`"bex.co/compute-weight-milli":"553","app.bex.co/regime":"sandbox"},`+
			`"status":{"state":"Running"}}]`)
	}))
	t.Cleanup(upstream.Close)
	at := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	fake := &fakeMeterStore{keys: []store.SandboxTenantKey{{WorkspaceID: "tea-a", APIKey: "secret-key"}}}
	m := &Meter{Client: NewClient(upstream.URL), Store: fake, Now: func() time.Time { return at }}
	m.poll(context.Background())
	if gotKey != "secret-key" {
		t.Fatalf("tenant key = %q", gotKey)
	}
	if len(fake.observations) != 1 {
		t.Fatalf("observations = %+v", fake.observations)
	}
	obs := fake.observations[0]
	if obs.Phase != "running" || obs.WeightMilli != 553 || obs.Tier != "starter" || !obs.ObservedAt.Equal(at) {
		t.Fatalf("observation = %+v", obs)
	}
	if fake.missingWS != "tea-a" || len(fake.missingSeen) != 1 || fake.missingSeen[0] != "os-1" {
		t.Fatalf("missing reconciliation = %q %v", fake.missingWS, fake.missingSeen)
	}
}

func TestMeterObservationExcludesSuspendedPhaseAndLegacyUnknownShape(t *testing.T) {
	base := osSandbox{ID: "os-1", Metadata: map[string]string{
		metadataWorkspace: "tea-a", metadataPlan: "starter", metadataRegime: metadataSandboxRegime,
		metadataComputeWeight: "553",
	}}
	base.Status.State = "Paused"
	obs, ok := meterObservation(base, time.Now())
	if !ok || obs.Phase != "suspended" {
		t.Fatalf("paused observation = %+v ok=%v", obs, ok)
	}
	delete(base.Metadata, metadataComputeWeight)
	if _, ok := meterObservation(base, time.Now()); ok {
		t.Error("legacy sandbox with unknown shape must not be retroactively charged")
	}
}

// w1/112: a sandbox outside any agent session had no charge label, so every
// such row read `starter sandbox (deleted)` and could not be told apart.
func TestSandboxChargeLabel(t *testing.T) {
	for _, tc := range []struct{ plan, image, want string }{
		{"starter", "docker.io/library/alpine:3@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b", "starter · alpine:3"},
		{"starter", "ghcr.io/bex-co/bex-agent-sandbox:v2", "starter · ghcr.io/bex-co/bex-agent-sandbox:v2"},
		{"pro", "docker.io/someone/tool:1", "pro · someone/tool:1"},
		{"", "alpine:3", "alpine:3"},
		{"starter", "", ""},
	} {
		if got := sandboxChargeLabel(tc.plan, tc.image); got != tc.want {
			t.Errorf("sandboxChargeLabel(%q, %q) = %q, want %q", tc.plan, tc.image, got, tc.want)
		}
	}
	if sandboxChargeLabel("starter", "alpine:3") == sandboxChargeLabel("starter", "alpine:3.20") {
		t.Error("two images share a label")
	}
}

func TestMeterLabelRetainsPlanAndImageAtCreate(t *testing.T) {
	fake := &fakeMeterStore{}
	m := &Meter{Store: fake, Now: time.Now}
	raw := osSandbox{ID: "os-1", Metadata: map[string]string{
		metadataWorkspace: "tea-a", metadataPlan: "starter", metadataRegime: metadataSandboxRegime,
		metadataComputeWeight: "553",
	}}
	raw.Status.State = "Running"
	raw.Image.URI = "docker.io/library/alpine:3@sha256:28bd5fe8"
	m.Label(context.Background(), raw)
	if got := fake.labels["tea-a/os-1"]; got != "starter · alpine:3" {
		t.Fatalf("retained label = %q (all: %v)", got, fake.labels)
	}

	// No image in hand: nothing is written rather than a label that misleads.
	fake.labels = nil
	raw.Image.URI = ""
	m.Label(context.Background(), raw)
	if len(fake.labels) != 0 {
		t.Fatalf("labelled a sandbox with no image: %v", fake.labels)
	}
}

// The create path itself retains the label, keyed by the public id the meter's
// usage rows carry, before any agent session could name it (w1/112).
func TestCreateRetainsTheChargeLabel(t *testing.T) {
	svc := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"os-1","status":{"state":"Creating"}}`))
	})
	fake := &fakeMeterStore{}
	svc.Meter = &Meter{Store: fake, Now: time.Now}
	sb, err := svc.Create(callerCtx(), CreateRequest{Template: "node", Plan: PlanStandard})
	if err != nil {
		t.Fatal(err)
	}
	if got := fake.labels["tea-a/"+sb.ID]; got != "standard · node:20" {
		t.Fatalf("retained label for %s = %q (all: %v)", sb.ID, got, fake.labels)
	}
}
