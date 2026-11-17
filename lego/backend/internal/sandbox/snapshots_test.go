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
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/routers"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w32/m285: Render's sandbox snapshots against a real control-plane Postgres,
// the fake OpenSandbox's captures and an in-process sandbox registry, with
// every snapshot response checked against Render's pinned schema.

type snapshotFixture struct {
	*imageFixture
}

// renderSandboxID pads bex's 20-character sandbox ids in a response to
// Render's 25 (the recorded sandbox id divergence, ADR018) so the rest of a
// snapshot is checked against the pinned schema.
var renderSandboxID = regexp.MustCompile(`"(sourceSandboxId)":"(sbx-[0-9a-v]{20})"`)

func newSnapshotFixture(t *testing.T) *snapshotFixture {
	t.Helper()
	f := newImageFixture(t)
	f.svc.Snapshots = &Snapshots{
		Store:  f.st,
		Reaper: &RegistryReaper{RegistryConfig: f.svc.Images.Registry},
		Limits: store.SandboxSnapshotLimits{
			PerRequesterPerMinute: DefaultSnapshotCapturesPerMinute,
			InFlight:              DefaultSnapshotCapturesInFlight,
			PerGroup:              DefaultSnapshotsPerGroup,
		},
	}
	contract := rendertest.Load(t).MuxMatcher(f.mux)
	groups := rendertest.Wrap(t, f.mux, func(r *http.Request) *routers.Route {
		if !strings.HasPrefix(r.URL.Path, "/v1/sandbox-groups") || strings.Contains(r.URL.Path, "/snapshots") {
			return nil
		}
		return contract(r)
	})
	f.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var route *routers.Route
		if strings.Contains(r.URL.Path, "/snapshots") {
			route = contract(r)
		}
		if route == nil {
			groups.ServeHTTP(w, r)
			return
		}
		rec := httptest.NewRecorder()
		groups.ServeHTTP(rec, r)
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
		body := renderSandboxID.ReplaceAll(rec.Body.Bytes(), []byte(`"$1":"${2}00000"`))
		if errs := rendertest.Unallowed(route.Operation.OperationID, rendertest.Validate(route, rec.Code, rec.Header(), body)); len(errs) > 0 {
			t.Errorf("Render response conformance: %s %s => %d (operationId=%s)\n  %s",
				r.Method, r.URL.Path, rec.Code, route.Operation.OperationID, strings.Join(errs, "\n  "))
		}
	})
	return &snapshotFixture{imageFixture: f}
}

// sandbox starts a running sandbox in the fixture's group with body's fields.
func (f *snapshotFixture) sandbox(t *testing.T, fields string) Sandbox {
	t.Helper()
	body := fmt.Sprintf(`{"ownerId":%q,"sandboxGroupId":%q}`, f.ws, f.group.ID)
	if fields != "" {
		body = fmt.Sprintf(`{"ownerId":%q,"sandboxGroupId":%q,%s}`, f.ws, f.group.ID, fields)
	}
	code, out := f.do(t, http.MethodPost, "/v1/sandboxes", body)
	var sb Sandbox
	if err := json.Unmarshal([]byte(out), &sb); code != http.StatusCreated || err != nil {
		t.Fatalf("create %s = %d %s", body, code, out)
	}
	return sb
}

func (f *snapshotFixture) capture(t *testing.T, sandboxID, body string) (int, string) {
	t.Helper()
	return f.do(t, http.MethodPost, "/v1/sandboxes/"+sandboxID+"/snapshots", body)
}

func (f *snapshotFixture) mustCapture(t *testing.T, sandboxID, body string) Snapshot {
	t.Helper()
	code, out := f.capture(t, sandboxID, body)
	var snap Snapshot
	if err := json.Unmarshal([]byte(out), &snap); code != http.StatusAccepted || err != nil {
		t.Fatalf("capture %s %s = %d %s", sandboxID, body, code, out)
	}
	return snap
}

func (f *snapshotFixture) get(t *testing.T, groupID, id string) (int, Snapshot, string) {
	t.Helper()
	code, out := f.do(t, http.MethodGet, "/v1/sandbox-groups/"+groupID+"/snapshots/"+id, "")
	var snap Snapshot
	if code == http.StatusOK {
		if err := json.Unmarshal([]byte(out), &snap); err != nil {
			t.Fatal(err)
		}
	}
	return code, snap, out
}

func (f *snapshotFixture) list(t *testing.T, query string) []Snapshot {
	t.Helper()
	sep := "?"
	if query != "" {
		sep = "&"
	}
	code, out := f.do(t, http.MethodGet, "/v1/sandbox-groups/"+f.group.ID+"/snapshots"+query+sep+"ownerId="+f.ws, "")
	var page []SnapshotEnvelope
	if err := json.Unmarshal([]byte(out), &page); code != http.StatusOK || err != nil {
		t.Fatalf("list %s = %d %s", query, code, out)
	}
	snaps := make([]Snapshot, 0, len(page))
	for _, e := range page {
		if e.Cursor != e.Snapshot.ID {
			t.Fatalf("cursor %s of %s", e.Cursor, e.Snapshot.ID)
		}
		snaps = append(snaps, e.Snapshot)
	}
	return snaps
}

// snapshotRow is what a test reads of a stored snapshot, deleted or expired
// or not.
type snapshotRow struct {
	RuntimeID       *string
	SourceSandboxID string
	ExpiresAt       *time.Time
	ReapedAt        *time.Time
}

func (f *snapshotFixture) row(t *testing.T, id string) snapshotRow {
	t.Helper()
	var row snapshotRow
	if err := f.st.Pool.QueryRow(context.Background(), `SELECT runtime_id, source_sandbox_id, expires_at, reaped_at FROM sandbox_snapshots WHERE id = $1`, id).
		Scan(&row.RuntimeID, &row.SourceSandboxID, &row.ExpiresAt, &row.ReapedAt); err != nil {
		t.Fatalf("snapshot row %s: %v", id, err)
	}
	return row
}

// commitRepo is the repository the substrate's commit pushes a sandbox's
// captures to.
func (f *snapshotFixture) commitRepo(t *testing.T, sandboxID string) string {
	t.Helper()
	return fmt.Sprintf("%s/snapshots/%s/%s-sandbox", hostOf(f.sink), store.SandboxNamespace(f.ws), f.os.idOf(sandboxID))
}

// commit plays the substrate's commit Job for a capture in flight: it pushes
// the sandbox's root filesystem image with the capture's tag and marks the
// capture Ready. It returns the image.
func (f *snapshotFixture) commit(t *testing.T, snap Snapshot) v1.Image {
	t.Helper()
	row := f.row(t, snap.ID)
	if row.RuntimeID == nil {
		t.Fatalf("snapshot %s has no capture", snap.ID)
	}
	img, _ := pushImage(t, f.commitRepo(t, snap.SourceSandboxID)+":"+commitTag(*row.RuntimeID), linuxAMD64, 512)
	f.os.settleCapture(*row.RuntimeID, osSnapshotReady)
	return img
}

func (f *snapshotFixture) sweep(t *testing.T) {
	t.Helper()
	if err := f.svc.sweepSnapshots(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
}

// available captures a snapshot of the sandbox and settles it.
func (f *snapshotFixture) available(t *testing.T, sandboxID, body string) (Snapshot, v1.Image) {
	t.Helper()
	snap := f.mustCapture(t, sandboxID, body)
	img := f.commit(t, snap)
	f.sweep(t)
	code, got, out := f.get(t, f.group.ID, snap.ID)
	if code != http.StatusOK || got.Status != store.SandboxSnapshotAvailable {
		t.Fatalf("settled snapshot = %d %s", code, out)
	}
	return got, img
}

func (f *snapshotFixture) restore(t *testing.T, fields string) (int, string) {
	t.Helper()
	return f.do(t, http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"sandboxGroupId":%q,%s}`, f.ws, f.group.ID, fields))
}

func headOK(t *testing.T, ref string) bool {
	t.Helper()
	r, err := name.ParseReference(ref, name.Insecure)
	if err != nil {
		t.Fatal(err)
	}
	_, err = remote.Head(r)
	return err == nil
}

// A filesystem snapshot of a running sandbox is captured without stopping it,
// settles available with its size, is metered as snapshot storage, and starts
// new sandboxes by name or id that run its image and inherit the source's
// settings but not its environment. Each restore restarts its lifetime, and
// the capture's own traces in the substrate and the registry are removed.
func TestCaptureAFilesystemSnapshotAndStartSandboxesFromIt(t *testing.T) {
	f := newSnapshotFixture(t)
	ctx := context.Background()
	source := f.sandbox(t, `"plan":"2c-4g","timeoutSeconds":7200,"idleTimeoutSeconds":600,"networkPolicy":{"type":"deny-all"},"env":{"SECRET":"s3cret"}`)

	snap := f.mustCapture(t, source.ID, `{"kind":"filesystem","name":"base","ttlSeconds":3600}`)
	if snap.Status != store.SandboxSnapshotCreating || snap.Kind != store.SandboxSnapshotFilesystem || snap.SandboxGroupID != f.group.ID ||
		snap.SourceSandboxID != source.ID || snap.Name == nil || *snap.Name != "base" || snap.Plan != "2c-4g" ||
		snap.TTLSeconds != 3600 || !snap.ExpiresAt.Equal(f.now.Add(time.Hour)) || snap.CapturedAt != nil || snap.SizeBytes != nil {
		t.Fatalf("capture = %+v", snap)
	}
	// Creating a snapshot does not terminate or suspend the sandbox.
	osID := f.os.idOf(source.ID)
	if calls := f.os.lifecycleCalls(); !slices.Contains(calls, "snapshot "+osID) || slices.ContainsFunc(calls, func(c string) bool {
		return strings.HasPrefix(c, "pause ") || strings.HasPrefix(c, "terminate ")
	}) || f.os.state(osID) != "Running" {
		t.Fatalf("lifecycle calls = %v, state %s; want one capture and the sandbox running", calls, f.os.state(osID))
	}
	// A sweep before the commit finishes leaves it creating.
	f.sweep(t)
	if _, got, _ := f.get(t, f.group.ID, snap.ID); got.Status != store.SandboxSnapshotCreating {
		t.Fatalf("snapshot before the commit = %s", got.Status)
	}

	img := f.commit(t, snap)
	captureID := *f.row(t, snap.ID).RuntimeID
	f.sweep(t)
	_, got, out := f.get(t, f.group.ID, snap.ID)
	size, _ := imageBytes(img)
	digest, _ := img.Digest()
	if got.Status != store.SandboxSnapshotAvailable || got.CapturedAt == nil || got.SizeBytes == nil || *got.SizeBytes != size || got.Error != nil {
		t.Fatalf("settled snapshot = %s, want available with size %d", out, size)
	}
	// Its image is a repository of its own; the capture's object and the tag
	// its commit left in the source sandbox's repository are gone.
	stored := fmt.Sprintf("%s/snapshots/%s/snapshots/%s", hostOf(f.sink), store.SandboxNamespace(f.ws), snap.ID)
	if !headOK(t, stored+":"+snapshotImageTag) {
		t.Fatal("the snapshot's image is not stored")
	}
	if headOK(t, f.commitRepo(t, source.ID)+":"+commitTag(captureID)) || len(f.os.captureIDs()) != 0 || f.row(t, snap.ID).RuntimeID != nil {
		t.Fatalf("the capture was not cleaned up: captures %v", f.os.captureIDs())
	}
	// It is metered as snapshot storage from its capture, at its size.
	var kind string
	var bytes int64
	if err := f.st.Pool.QueryRow(ctx, `SELECT kind, bytes FROM sandbox_storage_objects WHERE workspace_id = $1 AND object_id = $2 AND ended_at IS NULL`,
		f.ws, snap.ID).Scan(&kind, &bytes); err != nil || kind != store.StorageKindSnapshot || bytes != size {
		t.Fatalf("storage meter = %q %d, %v; want snapshot %d", kind, bytes, err, size)
	}

	// Restored by name, then by id, ten minutes later: the image is the
	// snapshot's, the settings are the source's, the environment is not.
	f.now = f.now.Add(10 * time.Minute)
	want := fmt.Sprintf("%s@%s", stored, digest)
	var restored []Sandbox
	for _, sel := range []string{`"snapshotName":"base"`, fmt.Sprintf(`"snapshotId":%q`, snap.ID)} {
		code, out := f.restore(t, sel)
		var sb Sandbox
		if err := json.Unmarshal([]byte(out), &sb); code != http.StatusCreated || err != nil {
			t.Fatalf("restore with %s = %d %s", sel, code, out)
		}
		box := f.os.box(f.os.idOf(sb.ID))
		if box.image != want || sb.Image != snap.ID {
			t.Fatalf("restore with %s started %q (reads %q), want %q reading %s", sel, box.image, sb.Image, want, snap.ID)
		}
		if sb.Plan != "2c-4g" || sb.TimeoutSeconds != 7200 || sb.IdleTimeoutSeconds != 600 || sb.NetworkPolicy == nil ||
			sb.NetworkPolicy.Default != NetworkPolicyDenyAll || sb.SandboxGroupID != f.group.ID {
			t.Fatalf("restore with %s = %+v, want the source's plan, timeouts and policy", sel, sb)
		}
		if _, leaked := box.env["SECRET"]; leaked {
			t.Fatalf("restore with %s carried the source's environment: %v", sel, box.env)
		}
		restored = append(restored, sb)
	}
	// Each restore restarts the snapshot's lifetime; the snapshot remains.
	if _, got, _ := f.get(t, f.group.ID, snap.ID); !got.ExpiresAt.Equal(f.now.Add(time.Hour)) {
		t.Fatalf("expiresAt after a restore = %s, want %s", got.ExpiresAt, f.now.Add(time.Hour))
	}
	// A restore's own fields win over the source's.
	code, out := f.restore(t, `"snapshotName":"base","plan":"1c-2g","timeoutSeconds":0,"idleTimeoutSeconds":0,"networkPolicy":{"type":"allow-all"}`)
	var sb Sandbox
	if err := json.Unmarshal([]byte(out), &sb); code != http.StatusCreated || err != nil {
		t.Fatalf("restore with overrides = %d %s", code, out)
	}
	if sb.Plan != "1c-2g" || sb.TimeoutSeconds != 0 || sb.IdleTimeoutSeconds != 0 || sb.NetworkPolicy.Default != NetworkPolicyAllowAll {
		t.Fatalf("restore with overrides = %+v", sb)
	}
	// A reused name restores its newest available snapshot.
	newer, newerImg := f.available(t, restored[0].ID, `{"kind":"filesystem","name":"base"}`)
	newerDigest, _ := newerImg.Digest()
	if code, out := f.restore(t, `"snapshotName":"base"`); code != http.StatusCreated {
		t.Fatalf("restore after a newer capture = %d %s", code, out)
	} else if err := json.Unmarshal([]byte(out), &sb); err != nil || sb.Image != newer.ID ||
		!strings.HasSuffix(f.os.box(f.os.idOf(sb.ID)).image, newer.ID+"@"+newerDigest.String()) {
		t.Fatalf("restore by a reused name = %s, want %s", out, newer.ID)
	}
}

// Render's default kind is runtime, which bex cannot capture yet: an omitted
// kind and `runtime` are refused by name and never downgraded, as are a bad
// name and lifetime, a sandbox that is not running, and a capture past a limit.
func TestCaptureRefusals(t *testing.T) {
	f := newSnapshotFixture(t)
	sb := f.sandbox(t, "")
	for _, tc := range []struct {
		name, body string
		status     int
		code       string
	}{
		{"no body", "", 400, CodeSnapshotRuntimeUnsupported},
		{"omitted kind", `{"name":"base"}`, 400, CodeSnapshotRuntimeUnsupported},
		{"runtime kind", `{"kind":"runtime"}`, 400, CodeSnapshotRuntimeUnsupported},
		{"unknown field", `{"kind":"filesystem","memory":true}`, 400, ""},
		{"name with the id prefix", `{"kind":"filesystem","name":"snp-base"}`, 400, CodeInvalidSnapshotName},
		{"empty name", `{"kind":"filesystem","name":""}`, 400, CodeInvalidSnapshotName},
		{"name over 64", `{"kind":"filesystem","name":"` + strings.Repeat("n", 65) + `"}`, 400, CodeInvalidSnapshotName},
		{"ttl and expiresAt", `{"kind":"filesystem","ttlSeconds":60,"expiresAt":"2030-01-01T00:00:00Z"}`, 400, CodeSandboxSnapshotInvalid},
		{"expiry in the past", `{"kind":"filesystem","expiresAt":"2020-01-01T00:00:00Z"}`, 400, CodeSandboxSnapshotInvalid},
		{"negative ttl", `{"kind":"filesystem","ttlSeconds":-1}`, 400, CodeSandboxSnapshotInvalid},
	} {
		code, body := f.capture(t, sb.ID, tc.body)
		if code != tc.status || errCode(body) != tc.code {
			t.Errorf("%s = %d %s, want %d %s", tc.name, code, body, tc.status, tc.code)
		}
	}
	if code, body := f.capture(t, sb.ID, `{"kind":"runtime"}`); !strings.Contains(body, "--kind filesystem") {
		t.Errorf("the runtime refusal = %d %s, want it to name --kind filesystem", code, body)
	}
	if code, body := f.capture(t, "sbx-"+strings.Repeat("z", 20), `{"kind":"filesystem"}`); code != http.StatusNotFound {
		t.Errorf("capture of an unknown sandbox = %d %s", code, body)
	}
	suspended := f.sandbox(t, "")
	f.suspend(t, suspended)
	if code, body := f.capture(t, suspended.ID, `{"kind":"filesystem"}`); code != http.StatusConflict || errCode(body) != CodeSandboxNotRunning {
		t.Errorf("capture of a suspended sandbox = %d %s, want 409 %s", code, body, CodeSandboxNotRunning)
	}
	if calls := f.os.lifecycleCalls(); slices.ContainsFunc(calls, func(c string) bool { return strings.HasPrefix(c, "snapshot ") }) {
		t.Fatalf("a refused capture reached the substrate: %v", calls)
	}
	if rows, err := f.st.SandboxSnapshots(context.Background(), f.ws, f.group.ID, "", nil, f.now); err != nil || len(rows) != 0 {
		t.Fatalf("refused captures left %d snapshots, %v", len(rows), err)
	}

	// One capture at a time per sandbox: each one freezes it.
	first := f.mustCapture(t, sb.ID, `{"kind":"filesystem"}`)
	if code, body := f.capture(t, sb.ID, `{"kind":"filesystem"}`); code != http.StatusTooManyRequests || errCode(body) != CodeSandboxSnapshotLimit || !strings.Contains(body, first.ID) {
		t.Errorf("a second capture while the first is in flight = %d %s", code, body)
	}
	other := f.sandbox(t, "")
	f.svc.Snapshots.Limits = store.SandboxSnapshotLimits{InFlight: 1}
	if code, body := f.capture(t, other.ID, `{"kind":"filesystem"}`); code != http.StatusTooManyRequests || errCode(body) != CodeSandboxSnapshotLimit || !strings.Contains(body, "being captured") {
		t.Errorf("a capture past the workspace's in-flight limit = %d %s", code, body)
	}
	f.svc.Snapshots.Limits = store.SandboxSnapshotLimits{PerRequesterPerMinute: 1}
	if code, body := f.capture(t, other.ID, `{"kind":"filesystem"}`); code != http.StatusTooManyRequests || errCode(body) != CodeSandboxSnapshotLimit || !strings.Contains(body, "last minute") {
		t.Errorf("a capture past the per-minute limit = %d %s", code, body)
	}
	f.now = f.now.Add(time.Minute)
	f.svc.Snapshots.Limits = store.SandboxSnapshotLimits{PerGroup: 1}
	if code, body := f.capture(t, other.ID, `{"kind":"filesystem"}`); code != http.StatusTooManyRequests || errCode(body) != CodeSandboxSnapshotLimit || !strings.Contains(body, f.group.ID) {
		t.Errorf("a capture past the group's snapshot limit = %d %s", code, body)
	}
	f.svc.Snapshots.Limits = store.SandboxSnapshotLimits{PerRequesterPerMinute: 1}
	f.mustCapture(t, other.ID, `{"kind":"filesystem"}`) // a minute later, the rate limit has room again

	// A substrate that refuses the capture leaves no snapshot creating.
	f.svc.Snapshots.Limits = store.SandboxSnapshotLimits{}
	third := f.sandbox(t, "")
	f.os.mu.Lock()
	f.os.captureStatus = http.StatusConflict
	f.os.mu.Unlock()
	if code, body := f.capture(t, third.ID, `{"kind":"filesystem","name":"raced"}`); code != http.StatusConflict || errCode(body) != CodeSandboxNotRunning {
		t.Errorf("a capture the substrate refused = %d %s", code, body)
	}
	rows, err := f.st.SandboxSnapshots(context.Background(), f.ws, f.group.ID, "raced", nil, f.now)
	if err != nil || len(rows) != 1 || rows[0].Status != store.SandboxSnapshotFailed {
		t.Fatalf("the refused capture's snapshot = %+v, %v; want one failed", rows, err)
	}
}

// Render's lifetimes: 30 days by default, ttlSeconds after the last use, 0
// for never (9999-12-31T23:59:59Z), and the deprecated expiresAt as the
// duration from the request. An expired snapshot is gone from every read and
// restore, deleting it succeeds, and its image is purged, while a sandbox
// restored from it is not affected.
func TestSnapshotLifetimesAndExpiry(t *testing.T) {
	f := newSnapshotFixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		body    string
		ttl     int64
		expires time.Time
	}{
		{`{"kind":"filesystem"}`, 30 * 24 * 3600, f.now.Add(30 * 24 * time.Hour)},
		{`{"kind":"filesystem","ttlSeconds":0}`, 0, snapshotNoExpiry},
		{fmt.Sprintf(`{"kind":"filesystem","expiresAt":%q}`, f.now.Add(2*time.Hour).Format(time.RFC3339)), 7200, f.now.Add(2 * time.Hour)},
	} {
		snap := f.mustCapture(t, f.sandbox(t, "").ID, tc.body)
		if snap.TTLSeconds != tc.ttl || !snap.ExpiresAt.Equal(tc.expires) {
			t.Errorf("%s = ttl %d expires %s, want %d %s", tc.body, snap.TTLSeconds, snap.ExpiresAt, tc.ttl, tc.expires)
		}
	}

	source := f.sandbox(t, "")
	snap, img := f.available(t, source.ID, `{"kind":"filesystem","name":"short","ttlSeconds":60}`)
	digest, _ := img.Digest()
	code, out := f.restore(t, `"snapshotName":"short"`)
	var restored Sandbox
	if err := json.Unmarshal([]byte(out), &restored); code != http.StatusCreated || err != nil {
		t.Fatalf("restore = %d %s", code, out)
	}

	f.now = f.now.Add(2 * time.Minute)
	if code, _, out := f.get(t, f.group.ID, snap.ID); code != http.StatusNotFound || errCode(out) != CodeSnapshotNotFound {
		t.Fatalf("get an expired snapshot = %d %s", code, out)
	}
	if slices.ContainsFunc(f.list(t, ""), func(s Snapshot) bool { return s.ID == snap.ID }) {
		t.Fatal("an expired snapshot is listed")
	}
	for _, sel := range []string{`"snapshotName":"short"`, fmt.Sprintf(`"snapshotId":%q`, snap.ID)} {
		if code, out := f.restore(t, sel); code != http.StatusNotFound || errCode(out) != CodeSnapshotNotFound {
			t.Errorf("restore %s from an expired snapshot = %d %s", sel, code, out)
		}
	}
	f.sweep(t)
	stored := fmt.Sprintf("%s/snapshots/%s/snapshots/%s:%s", hostOf(f.sink), store.SandboxNamespace(f.ws), snap.ID, snapshotImageTag)
	if headOK(t, stored) {
		t.Fatal("an expired snapshot's image is still in the registry")
	}
	var ended time.Time
	if err := f.st.Pool.QueryRow(ctx, `SELECT ended_at FROM sandbox_storage_objects WHERE workspace_id = $1 AND object_id = $2`, f.ws, snap.ID).Scan(&ended); err != nil ||
		!ended.Equal(f.row(t, snap.ID).ExpiresAt.Truncate(time.Second)) {
		t.Fatalf("storage meter ended at %s (%v), want the expiry %s", ended, err, f.row(t, snap.ID).ExpiresAt)
	}
	if code, out := f.do(t, http.MethodDelete, "/v1/sandbox-groups/"+f.group.ID+"/snapshots/"+snap.ID, ""); code != http.StatusNoContent {
		t.Fatalf("delete an expired snapshot = %d %s", code, out)
	}
	if box := f.os.box(f.os.idOf(restored.ID)); box.state != "Running" || !strings.HasSuffix(box.image, "@"+digest.String()) {
		t.Fatalf("the sandbox restored from it = %+v", box)
	}
}

// List is newest first with Render's filters and cursor; get and delete are
// scoped to the snapshot's group; delete is refused while the capture runs,
// succeeds again once done, and purges the image.
func TestSnapshotReadsAndDelete(t *testing.T) {
	f := newSnapshotFixture(t)
	a, _ := f.available(t, f.sandbox(t, "").ID, `{"kind":"filesystem","name":"a"}`)
	b, _ := f.available(t, f.sandbox(t, "").ID, `{"kind":"filesystem","name":"b"}`)
	creating := f.mustCapture(t, f.sandbox(t, "").ID, `{"kind":"filesystem","name":"a"}`)

	ids := func(snaps []Snapshot) []string {
		out := make([]string, 0, len(snaps))
		for _, s := range snaps {
			out = append(out, s.ID)
		}
		return out
	}
	if got, want := ids(f.list(t, "")), []string{creating.ID, b.ID, a.ID}; !slices.Equal(got, want) {
		t.Fatalf("list = %v, want newest first %v", got, want)
	}
	if got, want := ids(f.list(t, "?name=a")), []string{creating.ID, a.ID}; !slices.Equal(got, want) {
		t.Fatalf("list name=a = %v, want %v", got, want)
	}
	if got, want := ids(f.list(t, "?status=available")), []string{b.ID, a.ID}; !slices.Equal(got, want) {
		t.Fatalf("list status=available = %v, want %v", got, want)
	}
	if got, want := ids(f.list(t, "?limit=2")), []string{creating.ID, b.ID}; !slices.Equal(got, want) {
		t.Fatalf("first page = %v, want %v", got, want)
	}
	if got, want := ids(f.list(t, "?limit=2&cursor="+b.ID)), []string{a.ID}; !slices.Equal(got, want) {
		t.Fatalf("second page = %v, want %v", got, want)
	}

	other := f.createGroup(t, "other")
	for _, tc := range []struct {
		name, method, path string
		status             int
		code               string
	}{
		{"get from another group", http.MethodGet, "/v1/sandbox-groups/" + other.ID + "/snapshots/" + a.ID, 404, CodeSnapshotNotFound},
		{"delete from another group", http.MethodDelete, "/v1/sandbox-groups/" + other.ID + "/snapshots/" + a.ID, 404, CodeSnapshotNotFound},
		{"get malformed id", http.MethodGet, "/v1/sandbox-groups/" + f.group.ID + "/snapshots/snp-1", 400, CodeInvalidSnapshotID},
		{"get unknown", http.MethodGet, "/v1/sandbox-groups/" + f.group.ID + "/snapshots/snp-" + strings.Repeat("a", 20), 404, CodeSnapshotNotFound},
		{"list unknown status", http.MethodGet, "/v1/sandbox-groups/" + f.group.ID + "/snapshots?status=ready", 400, CodeInvalidSnapshotStatus},
		{"list unknown group", http.MethodGet, "/v1/sandbox-groups/sbg-" + strings.Repeat("a", 20) + "/snapshots", 404, CodeSandboxGroupNotFound},
		{"delete while creating", http.MethodDelete, "/v1/sandbox-groups/" + f.group.ID + "/snapshots/" + creating.ID, 409, CodeSnapshotCreating},
	} {
		if code, body := f.do(t, tc.method, tc.path, ""); code != tc.status || errCode(body) != tc.code {
			t.Errorf("%s = %d %s, want %d %s", tc.name, code, body, tc.status, tc.code)
		}
	}

	stored := fmt.Sprintf("%s/snapshots/%s/snapshots/%s:%s", hostOf(f.sink), store.SandboxNamespace(f.ws), a.ID, snapshotImageTag)
	for range 2 { // Render answers 204 to a repeated delete
		if code, body := f.do(t, http.MethodDelete, "/v1/sandbox-groups/"+f.group.ID+"/snapshots/"+a.ID, ""); code != http.StatusNoContent {
			t.Fatalf("delete = %d %s", code, body)
		}
	}
	if code, _, _ := f.get(t, f.group.ID, a.ID); code != http.StatusNotFound {
		t.Fatalf("a deleted snapshot reads %d", code)
	}
	if got := ids(f.list(t, "")); slices.Contains(got, a.ID) {
		t.Fatalf("a deleted snapshot is listed: %v", got)
	}
	if code, body := f.restore(t, fmt.Sprintf(`"snapshotId":%q`, a.ID)); code != http.StatusNotFound || errCode(body) != CodeSnapshotNotFound {
		t.Fatalf("restore from a deleted snapshot = %d %s", code, body)
	}
	if headOK(t, stored) || f.row(t, a.ID).ReapedAt == nil {
		t.Fatal("a deleted snapshot's image is still in the registry")
	}
	var open int
	if err := f.st.Pool.QueryRow(context.Background(), `SELECT count(*) FROM sandbox_storage_objects WHERE workspace_id = $1 AND object_id = $2 AND ended_at IS NULL`,
		f.ws, a.ID).Scan(&open); err != nil || open != 0 {
		t.Fatalf("a deleted snapshot is still metered (%d open, %v)", open, err)
	}
}

// A capture that fails, is lost or is too large settles failed and says why;
// a create from it, or from one still creating, is Render's 409, and the
// create's other refusals are named.
func TestFailedCapturesAndRestoreRefusals(t *testing.T) {
	f := newSnapshotFixture(t)
	failed := f.mustCapture(t, f.sandbox(t, "").ID, `{"kind":"filesystem","name":"broken"}`)
	f.os.settleCapture(*f.row(t, failed.ID).RuntimeID, osSnapshotFailed)
	lost := f.mustCapture(t, f.sandbox(t, "").ID, `{"kind":"filesystem"}`)
	f.os.mu.Lock()
	delete(f.os.captures, *f.row(t, lost.ID).RuntimeID)
	f.os.mu.Unlock()
	big := f.mustCapture(t, f.sandbox(t, "").ID, `{"kind":"filesystem"}`)
	f.svc.Snapshots.MaxBytes = 1024
	f.commit(t, big)
	f.sweep(t)
	for _, tc := range []struct {
		snap     Snapshot
		mentions string
	}{
		{failed, "could not be committed"},
		{lost, "lost"},
		{big, "more than"},
	} {
		_, got, out := f.get(t, f.group.ID, tc.snap.ID)
		if got.Status != store.SandboxSnapshotFailed || got.Error == nil || !strings.Contains(*got.Error, tc.mentions) || got.SizeBytes != nil {
			t.Errorf("snapshot %s = %s, want failed mentioning %q", tc.snap.ID, out, tc.mentions)
		}
	}
	if len(f.os.captureIDs()) != 0 {
		t.Fatalf("failed captures left %v in the substrate", f.os.captureIDs())
	}
	f.svc.Snapshots.MaxBytes = 0

	creating := f.mustCapture(t, f.sandbox(t, "").ID, `{"kind":"filesystem","name":"slow"}`)
	ready, _ := f.available(t, f.sandbox(t, "").ID, `{"kind":"filesystem","name":"ready"}`)
	other := f.createGroup(t, "other")
	build := "sbb-" + strings.Repeat("b", 20)
	for _, tc := range []struct {
		name, fields string
		status       int
		code         string
	}{
		{"failed snapshot", fmt.Sprintf(`"snapshotId":%q`, failed.ID), 409, CodeSnapshotNotAvailable},
		{"creating snapshot", fmt.Sprintf(`"snapshotId":%q`, creating.ID), 409, CodeSnapshotNotAvailable},
		{"name whose only snapshot failed", `"snapshotName":"broken"`, 409, CodeSnapshotNotAvailable},
		{"name whose only snapshot is creating", `"snapshotName":"slow"`, 409, CodeSnapshotNotAvailable},
		{"unknown name", `"snapshotName":"nope"`, 404, CodeSnapshotNotFound},
		{"unknown id", `"snapshotId":"snp-` + strings.Repeat("c", 20) + `"`, 404, CodeSnapshotNotFound},
		{"malformed id", `"snapshotId":"snp-1"`, 400, CodeInvalidSnapshotID},
		{"name with the id prefix", `"snapshotName":"snp-x"`, 400, CodeInvalidSnapshotName},
		{"both id and name", fmt.Sprintf(`"snapshotId":%q,"snapshotName":"ready"`, ready.ID), 400, CodeSandboxSnapshotInvalid},
		{"snapshot and image", fmt.Sprintf(`"snapshotId":%q,"buildId":%q`, ready.ID, build), 400, CodeSandboxSnapshotInvalid},
	} {
		if code, body := f.restore(t, tc.fields); code != tc.status || errCode(body) != tc.code {
			t.Errorf("%s = %d %s, want %d %s", tc.name, code, body, tc.status, tc.code)
		}
	}
	// A snapshot restores only into its own group.
	code, body := f.do(t, http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"sandboxGroupId":%q,"snapshotId":%q}`, f.ws, other.ID, ready.ID))
	if code != http.StatusNotFound || errCode(body) != CodeSnapshotNotFound {
		t.Fatalf("restore into another group = %d %s", code, body)
	}
}

// Deleting a group deletes its snapshots with their images and captures, and
// a deleted workspace's snapshots leave the registry with it.
func TestGroupAndWorkspaceDeletesTakeTheirSnapshots(t *testing.T) {
	f := newSnapshotFixture(t)
	ctx := context.Background()
	doomed := f.createGroup(t, "doomed")
	code, out := f.do(t, http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"sandboxGroupId":%q}`, f.ws, doomed.ID))
	var sb Sandbox
	if err := json.Unmarshal([]byte(out), &sb); code != http.StatusCreated || err != nil {
		t.Fatalf("create = %d %s", code, out)
	}
	snap := f.mustCapture(t, sb.ID, `{"kind":"filesystem"}`)
	f.commit(t, snap)
	f.sweep(t)
	stored := fmt.Sprintf("%s/snapshots/%s/snapshots/%s:%s", hostOf(f.sink), store.SandboxNamespace(f.ws), snap.ID, snapshotImageTag)
	if !headOK(t, stored) {
		t.Fatal("the snapshot's image is not stored")
	}
	if code, body := f.do(t, http.MethodDelete, "/v1/sandbox-groups/"+doomed.ID, ""); code != http.StatusNoContent {
		t.Fatalf("delete group = %d %s", code, body)
	}
	if headOK(t, stored) {
		t.Fatal("the deleted group's snapshot image is still in the registry")
	}
	if code, _, _ := f.get(t, doomed.ID, snap.ID); code != http.StatusNotFound {
		t.Fatalf("a deleted group's snapshot reads %d", code)
	}

	kept, _ := f.available(t, f.sandbox(t, "").ID, `{"kind":"filesystem","name":"kept"}`)
	keptRef := fmt.Sprintf("%s/snapshots/%s/snapshots/%s:%s", hostOf(f.sink), store.SandboxNamespace(f.ws), kept.ID, snapshotImageTag)
	purger := &WorkspacePurger{Snapshots: f.svc.Snapshots, Now: func() time.Time { return f.now }}
	if err := purger.purgeSnapshots(ctx, f.ws); err != nil {
		t.Fatal(err)
	}
	if headOK(t, keptRef) {
		t.Fatal("a deleted workspace's snapshot image is still in the registry")
	}
}

// REST, GraphQL and MCP answer the snapshot verbs the same way.
func TestSnapshotSurfacesAgree(t *testing.T) {
	f := newSnapshotFixture(t)
	gql := newGQLDriver(t, f.svc)
	mcpd := mcpDriver{t: t, client: mcpClient(t, f.svc)}
	sb := f.sandbox(t, "")

	// Every surface refuses the runtime kind, and an omitted one, by name.
	if out := gql.run("createSandboxSnapshot", `mutation($id: String!) { createSandboxSnapshot(sandboxId: $id) { id } }`, map[string]any{"id": sb.ID}); out.Code != CodeSnapshotRuntimeUnsupported {
		t.Errorf("GraphQL capture without a kind = %+v", out)
	}
	if out := mcpd.call("create_sandbox_snapshot", map[string]any{"sandboxId": sb.ID, "kind": "runtime"}); out.Code != CodeSnapshotRuntimeUnsupported {
		t.Errorf("MCP runtime capture = %+v", out)
	}
	if out := mcpd.call("create_sandbox_snapshot", map[string]any{"sandboxId": sb.ID, "kind": "filesystem", "name": "snp-x"}); out.Code != CodeInvalidSnapshotName {
		t.Errorf("MCP bad name = %+v", out)
	}

	viaGQL := gql.run("createSandboxSnapshot", `mutation($id: String!) { createSandboxSnapshot(sandboxId: $id, kind: "filesystem", name: "gql", ttlSeconds: 600) { id status kind name ttlSeconds expiresAt sourceSandboxId sandboxGroupId } }`,
		map[string]any{"id": sb.ID})
	if viaGQL.Code != "" {
		t.Fatalf("GraphQL capture = %+v", viaGQL)
	}
	gqlSnap, _ := viaGQL.Raw["result"].(map[string]any)
	gqlID, _ := gqlSnap["id"].(string)
	if gqlSnap["status"] != store.SandboxSnapshotCreating || gqlSnap["ttlSeconds"] != float64(600) || gqlSnap["sourceSandboxId"] != sb.ID {
		t.Fatalf("GraphQL capture = %v", gqlSnap)
	}
	viaMCP := mcpd.call("create_sandbox_snapshot", map[string]any{"sandboxId": f.sandbox(t, "").ID, "kind": "filesystem", "name": "mcp"})
	if viaMCP.Code != "" {
		t.Fatalf("MCP capture = %+v", viaMCP)
	}
	mcpSnap, _ := viaMCP.Raw["result"].(map[string]any)
	mcpID, _ := mcpSnap["id"].(string)

	// Each surface reads, lists and refuses to delete the other's capture alike.
	if out := gql.run("sandboxSnapshot", `query($g: String!, $id: String!) { sandboxSnapshot(sandboxGroupId: $g, id: $id) { id status } }`,
		map[string]any{"g": f.group.ID, "id": mcpID}); out.Code != "" {
		t.Errorf("GraphQL get = %+v", out)
	}
	if out := mcpd.call("get_sandbox_snapshot", map[string]any{"sandboxGroupId": f.group.ID, "id": gqlID}); out.Code != "" {
		t.Errorf("MCP get = %+v", out)
	}
	listed := mcpd.call("list_sandbox_snapshots", map[string]any{"sandboxGroupId": f.group.ID, "status": []string{"creating"}})
	inner, _ := listed.Raw["result"].(map[string]any)
	if items, _ := inner["sandboxSnapshots"].([]any); len(items) != 2 {
		t.Errorf("MCP list = %+v", listed)
	}
	gqlList := gql.run("sandboxSnapshots", `query($g: String!) { sandboxSnapshots(sandboxGroupId: $g, name: "gql") { id } }`, map[string]any{"g": f.group.ID})
	if items, _ := gqlList.Raw["result"].([]any); len(items) != 1 {
		t.Errorf("GraphQL list = %+v", gqlList)
	}
	if out := gql.run("deleteSandboxSnapshot", `mutation($g: String!, $id: String!) { deleteSandboxSnapshot(sandboxGroupId: $g, id: $id) }`,
		map[string]any{"g": f.group.ID, "id": mcpID}); out.Code != CodeSnapshotCreating {
		t.Errorf("GraphQL delete while creating = %+v", out)
	}
	if out := mcpd.call("delete_sandbox_snapshot", map[string]any{"sandboxGroupId": f.group.ID, "id": gqlID}); out.Code != CodeSnapshotCreating {
		t.Errorf("MCP delete while creating = %+v", out)
	}
	for _, id := range []string{gqlID, mcpID} {
		f.commit(t, Snapshot{ID: id, SourceSandboxID: f.row(t, id).SourceSandboxID})
	}
	f.sweep(t)
	if out := mcpd.call("delete_sandbox_snapshot", map[string]any{"sandboxGroupId": f.group.ID, "id": gqlID}); out.Code != "" {
		t.Errorf("MCP delete = %+v", out)
	}
	if out := gql.run("deleteSandboxSnapshot", `mutation($g: String!, $id: String!) { deleteSandboxSnapshot(sandboxGroupId: $g, id: $id) }`,
		map[string]any{"g": f.group.ID, "id": mcpID}); out.Code != "" {
		t.Errorf("GraphQL delete = %+v", out)
	}
	for _, id := range []string{gqlID, mcpID} {
		if out := gql.run("sandboxSnapshot", `query($g: String!, $id: String!) { sandboxSnapshot(sandboxGroupId: $g, id: $id) { id } }`,
			map[string]any{"g": f.group.ID, "id": id}); out.Code != CodeSnapshotNotFound {
			t.Errorf("GraphQL get of a deleted snapshot = %+v", out)
		}
	}
	// MCP's spawn_sandbox restores by name like REST's create.
	ready, _ := f.available(t, f.sandbox(t, "").ID, `{"kind":"filesystem","name":"spawned"}`)
	if out := mcpd.call("spawn_sandbox", map[string]any{"sandboxGroupId": f.group.ID, "snapshotName": "spawned"}); out.Code != "" {
		t.Errorf("MCP restore = %+v", out)
	} else if res, _ := out.Raw["result"].(map[string]any); res["image"] != ready.ID {
		t.Errorf("MCP restore = %v, want image %s", res, ready.ID)
	}
}
