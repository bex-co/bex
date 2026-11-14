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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeStorageUsage struct {
	bytes int64
	err   error
	asked []string
}

func (f *fakeStorageUsage) SandboxStorageBytes(_ context.Context, ws string) (int64, error) {
	f.asked = append(f.asked, ws)
	return f.bytes, f.err
}

func (f *fakeStorageUsage) ReserveAgentRepoEnvironmentBuildImage(_ context.Context, _, _ string, size, limit int64) error {
	if f.bytes+size > limit {
		return &store.SandboxStorageLimitError{UsedBytes: f.bytes}
	}
	f.bytes += size
	return nil
}

// TestStorageCapRefusesTheSuspendThatWouldCommitAnotherImage: a workspace at its
// cap cannot suspend (explicitly or on idle) and gets a named 429; the refused
// sandbox keeps running, below the cap suspends as before, a sandbox of another
// workspace is judged on its own bytes, and 0 disables the cap.
func TestStorageCapRefusesTheSuspendThatWouldCommitAnotherImage(t *testing.T) {
	const limit = int64(10_000_000_000)
	f := newIdleFixture(t)
	usage := &fakeStorageUsage{bytes: limit}
	f.svc.Storage, f.svc.MaxStorageBytes = usage, limit
	f.svc.Metrics = NewMetrics(prometheus.NewRegistry())
	f.os.add("os-1", "Running", f.now.Add(-time.Hour), "900", nil)

	err := f.svc.Suspend(callerCtx(), "os-1")
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != CodeSandboxStorageLimit || !errors.Is(err, core.ErrTooManyRequests) {
		t.Fatalf("suspend at the cap = %v, want 429 %s", err, CodeSandboxStorageLimit)
	}
	if !strings.Contains(err.Error(), "10.0 GB") || coded.Params["limitBytes"] != limit {
		t.Errorf("error %q params %v: it must say what the limit is", err, coded.Params)
	}
	if f.os.state("os-1") != "Running" || len(f.os.lifecycleCalls()) != 0 {
		t.Errorf("a refused suspend must not reach the runtime: state %s calls %v", f.os.state("os-1"), f.os.lifecycleCalls())
	}
	if got := testutil.ToFloat64(f.svc.Metrics.refusals.WithLabelValues("storage", "suspend")); got != 1 {
		t.Errorf("storage refusals = %v, want 1", got)
	}

	// The idle sweep is refused the same way, and quietly: it is a steady state.
	if n := f.sweeper.Sweep(context.Background()); n != 0 || len(f.os.lifecycleCalls()) != 0 {
		t.Fatalf("idle sweep at the cap suspended %d (calls %v)", n, f.os.lifecycleCalls())
	}
	if f.activity.claimed("os-1") {
		t.Error("a refused idle suspend must release its claim")
	}

	// Below the cap it suspends, and a read failure never refuses.
	usage.bytes = limit - 1
	if err := f.svc.Suspend(callerCtx(), "os-1"); err != nil {
		t.Fatalf("suspend below the cap: %v", err)
	}
	f.os.add("os-2", "Running", f.now.Add(-time.Hour), "900", nil)
	usage.bytes, usage.err = limit, errors.New("database down")
	if err := f.svc.Suspend(callerCtx(), "os-2"); err != nil {
		t.Fatalf("a failed usage read must not refuse a suspend: %v", err)
	}

	// 0 disables the cap.
	f.os.add("os-3", "Running", f.now.Add(-time.Hour), "900", nil)
	usage.err = nil
	f.svc.MaxStorageBytes = 0
	if err := f.svc.Suspend(callerCtx(), "os-3"); err != nil {
		t.Fatalf("suspend with the cap off: %v", err)
	}
}

// An agent-session sandbox is neither idle-suspended nor judged by the cap, and
// the storage meter leaves it out.
func TestAgentSessionSandboxesStayOutOfTheStorageCapAndMeter(t *testing.T) {
	agent := osSandbox{ID: "os-agent", Metadata: map[string]string{
		metadataWorkspace: "tea-a", metadataRegime: metadataSandboxRegime, metadataComputeWeight: "553",
		metadataAgentSession: "ags-1",
	}}
	agent.Status.State = "Paused"
	obs, ok := meterObservation(agent, time.Now())
	if !ok || obs.StorageRef != "" {
		t.Fatalf("agent-session observation = %+v ok=%v, want no storage ref", obs, ok)
	}
	ordinary := agent
	ordinary.Metadata = map[string]string{metadataWorkspace: "tea-a", metadataRegime: metadataSandboxRegime, metadataComputeWeight: "553"}
	if obs, _ := meterObservation(ordinary, time.Now()); obs.StorageRef != "os-agent" {
		t.Errorf("ordinary sandbox storage ref = %q, want its substrate id", obs.StorageRef)
	}

	usage := &fakeStorageUsage{bytes: 1 << 40}
	s := &Service{Storage: usage, MaxStorageBytes: 1}
	agent.Status.State = "Running"
	if err := s.beforeSuspend(context.Background(), "key", agent); err != nil || len(usage.asked) != 0 {
		t.Errorf("an agent-session suspend was judged by the cap: err %v asked %v", err, usage.asked)
	}
}

type fakeReaper struct {
	failFor  map[string]bool
	deleted  []string
	attempts []string
}

func (f *fakeReaper) DeleteImages(_ context.Context, ws, ref string) error {
	f.attempts = append(f.attempts, ref)
	if f.failFor[ref] {
		return errors.New("registry down")
	}
	f.deleted = append(f.deleted, ws+"/"+ref)
	return nil
}

func TestMeterReapsTheImagesOfGoneSandboxesAndRetriesFailures(t *testing.T) {
	ended := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	fake := &fakeMeterStore{reapable: []store.SandboxStorageObject{
		{WorkspaceID: "tea-a", ObjectID: "sbx-1", Kind: store.StorageKindRootfs, Ref: "os-1", StartedAt: ended},
		{WorkspaceID: "tea-a", ObjectID: "sbx-2", Kind: store.StorageKindRootfs, Ref: "os-2", StartedAt: ended},
	}}
	metrics := NewStorageMetrics(prometheus.NewRegistry())
	reaper := &fakeReaper{failFor: map[string]bool{"os-1": true}}
	m := &Meter{Store: fake, Reaper: reaper, StorageMetrics: metrics}
	m.settleStorage(context.Background())
	if len(reaper.deleted) != 1 || reaper.deleted[0] != "tea-a/os-2" || len(fake.reaped) != 1 || fake.reaped[0] != "tea-a/sbx-2" {
		t.Errorf("deleted %v marked %v: only the image that was deleted may be marked reaped", reaper.deleted, fake.reaped)
	}
	if got := testutil.ToFloat64(metrics.reapFailures); got != 1 {
		t.Errorf("reap failures = %v, want 1", got)
	}
	// Three failures in a row end the round, so a hung registry cannot hold the poll.
	fake.reapable = nil
	for _, id := range []string{"1", "2", "3", "4"} {
		fake.reapable = append(fake.reapable, store.SandboxStorageObject{WorkspaceID: "tea-a", ObjectID: "sbx-" + id, Ref: "os-" + id})
	}
	failing := &fakeReaper{failFor: map[string]bool{"os-1": true, "os-2": true, "os-3": true, "os-4": true}}
	(&Meter{Store: fake, Reaper: failing, StorageMetrics: NewStorageMetrics(prometheus.NewRegistry())}).reapImages(context.Background())
	if len(failing.attempts) != storageRegistryFailures {
		t.Errorf("delete attempts = %v, want %d before the round gives up", failing.attempts, storageRegistryFailures)
	}
	// With no reaper nothing is touched.
	fake.reaped = nil
	(&Meter{Store: fake}).settleStorage(context.Background())
	if len(fake.reaped) != 0 {
		t.Errorf("no reaper configured but %v were marked reaped", fake.reaped)
	}
}

// fakeZotWithRepo serves one repository's tags and records the tags deleted,
// with Zot's answers: 202 for a tag delete (as measured on Zot v2.1.22), 404 for
// anything else.
func fakeZotWithRepo(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var deleted []string
	const repo = "/v2/snapshots/tea-a-sandbox/os-1-sandbox"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == repo+"/tags/list":
			_, _ = fmt.Fprint(w, `{"tags":["snap-gen1","snap-gen2","keep-me"]}`)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, repo+"/manifests/"):
			deleted = append(deleted, strings.TrimPrefix(r.URL.Path, repo+"/manifests/"))
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &deleted
}

func TestRegistryReaperDeletesOnlyTheControllersSnapshotTagsByTag(t *testing.T) {
	srv, deleted := fakeZotWithRepo(t)
	reaper := &RegistryReaper{RegistryConfig{BaseURL: srv.URL}}
	if err := reaper.DeleteImages(context.Background(), "tea-a", "os-1"); err != nil {
		t.Fatal(err)
	}
	// By tag, not by digest: a digest delete would take every tag on the manifest.
	if want := []string{"snap-gen1", "snap-gen2"}; strings.Join(*deleted, ",") != strings.Join(want, ",") {
		t.Errorf("deleted %v, want %v (the tag that is not the controller's must stay)", *deleted, want)
	}
	// A repository the registry does not have is success: the image is already gone.
	if err := reaper.DeleteImages(context.Background(), "tea-a", "os-9"); err != nil {
		t.Errorf("an unknown repository must be success, got %v", err)
	}
	// An unsafe sandbox id never reaches the registry.
	if err := reaper.DeleteImages(context.Background(), "tea-a", "../os-1"); err == nil {
		t.Error("a path in the sandbox id must be refused")
	}
}

func TestRegistryReaperReportsAFailedDeleteSoItIsRetried(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/tags/list") {
			_, _ = fmt.Fprint(w, `{"tags":["snap-gen1"]}`)
			return
		}
		w.WriteHeader(http.StatusForbidden) // the credential has no delete action
	}))
	t.Cleanup(srv.Close)
	reaper := &RegistryReaper{RegistryConfig{BaseURL: srv.URL}}
	if err := reaper.DeleteImages(context.Background(), "tea-a", "os-1"); err == nil {
		t.Error("a refused delete must be an error, not a silent success")
	}
}

// The REST pause route is the one surface with an explicit suspend, so it is
// where a caller meets the cap: a 429 with the named code and the limit in its
// parameters, and nothing sent to the runtime.
func TestPauseRouteAnswers429AtTheStorageCap(t *testing.T) {
	const limit = int64(5_000_000_000)
	f := newIdleFixture(t)
	f.svc.Storage, f.svc.MaxStorageBytes = &fakeStorageUsage{bytes: limit + 1}, limit
	f.os.add("os-1", "Running", f.now.Add(-time.Hour), "900", nil)
	mux := http.NewServeMux()
	f.svc.RegisterREST(mux)

	req := httptest.NewRequest(http.MethodPost, "/v1/sandboxes/os-1/pause", nil).WithContext(callerCtx())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("pause at the cap = %d %s, want 429", rec.Code, rec.Body.String())
	}
	for _, want := range []string{CodeSandboxStorageLimit, `"limitBytes":5000000000`, `"usedBytes":5000000001`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("body %s lacks %s", rec.Body.String(), want)
		}
	}
	if len(f.os.lifecycleCalls()) != 0 {
		t.Errorf("a refused pause reached the runtime: %v", f.os.lifecycleCalls())
	}
}
