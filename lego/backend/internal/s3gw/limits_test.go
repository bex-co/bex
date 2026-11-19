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

package s3gw

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// replica is a second bex-s3 pod: its own limiters and in-flight accounting over the same
// database and Wasabi.
func (p *pgRig) replica() *pgRig {
	svc := objectstorage.NewService(p.PG, p.stores())
	g := New(Config{Service: svc, Keys: p.keys, Base: p.gw.cfg.Base, Now: func() time.Time { return p.now }})
	q := *p
	q.gw = g
	q.Svc = svc
	return &q
}

func (p *pgRig) stores() map[objectstorage.Region]objectstorage.Store {
	out := map[objectstorage.Region]objectstorage.Store{}
	for _, r := range objectstorage.Regions {
		out[r] = objectstorage.NewStore(objectstorage.Config{
			Endpoint: p.Fakes[r].URL, Region: r.WasabiRegion(), Bucket: r.Bucket(), AccessKey: "a", SecretKey: "s", MaxAttempts: 1,
		})
	}
	return out
}

// observeUsage records the sweep's observation of a region: bytes and objects in total, at t.
func (p *pgRig) observeUsage(region objectstorage.Region, bucketID string, bytes, objects int64, at time.Time) {
	p.t.Helper()
	err := p.PG.RecordObjectStorageObservation(context.Background(), p.ws, string(region),
		[]store.ObjectStorageUsage{{BucketID: bucketID, WorkspaceID: p.ws, Region: string(region), Bytes: bytes, Objects: objects, ObservedAt: at}}, at)
	if err != nil {
		p.t.Fatal(err)
	}
}

func (p *pgRig) put(k testKey, target string, n int) (*httptest.ResponseRecorder, errorDoc) {
	return p.do(p.signed(k, "PUT", target, bytes.NewReader(payload(n)), func(r *http.Request) { r.ContentLength = int64(n) }))
}

func TestACompletedPutCountsAgainstTheCapBeforeTheNextSweepOnAnyReplica(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("bkt", objectstorage.Oregon)
	b, _ := p.Svc.GetBucket(context.Background(), p.ws, "bkt")
	// The last sweep saw the workspace 1,000 bytes under its cap.
	p.observeUsage(objectstorage.Oregon, b.ID, objectstorage.MaxWorkspaceBytes-1000, 10, time.Now().Add(-time.Minute))
	other := p.replica()
	if rec, doc := p.put(k, "/bkt/a", 600); rec.Code != 200 {
		t.Fatalf("a put under the cap = %d %q", rec.Code, doc.Code)
	}
	// Replica B has never seen that put in memory; the counter is in the database.
	rec, doc := other.put(k, "/bkt/b", 600)
	if rec.Code != 403 || doc.Code != "QuotaExceeded" || !strings.Contains(doc.Message, "bytes") {
		t.Fatalf("a put that the other replica's completed write pushes over = %d %q %q, want 403 QuotaExceeded naming the cap", rec.Code, doc.Code, doc.Message)
	}
	if _, ok := p.stored(objectstorage.Oregon, objectstorage.Prefix(p.ws, b.ID), "b"); ok {
		t.Error("a refused put stored its object")
	}
	// A put that fits still goes through, and so does one after the sweep's next observation.
	if rec, doc := other.put(k, "/bkt/c", 300); rec.Code != 200 {
		t.Errorf("a put that fits = %d %q", rec.Code, doc.Code)
	}
	p.observeUsage(objectstorage.Oregon, b.ID, objectstorage.MaxWorkspaceBytes-1000+900, 12, time.Now().Add(2*time.Second))
	if rec, _ := other.put(k, "/bkt/d", 600); rec.Code != 403 {
		t.Errorf("after an observation that includes both writes the cap still holds: %d", rec.Code)
	}
	// Each observation clears the counter: 600 bytes written, a new observation at the
	// same total, and another 600 fit again (they would not if the counter survived).
	p.observeUsage(objectstorage.Oregon, b.ID, objectstorage.MaxWorkspaceBytes-1000, 10, time.Now().Add(4*time.Second))
	if rec, doc := other.put(k, "/bkt/x", 600); rec.Code != 200 {
		t.Fatalf("a put after an observation = %d %q %q", rec.Code, doc.Code, doc.Message)
	}
	p.observeUsage(objectstorage.Oregon, b.ID, objectstorage.MaxWorkspaceBytes-1000, 10, time.Now().Add(6*time.Second))
	if rec, doc := p.put(k, "/bkt/y", 600); rec.Code != 200 {
		t.Errorf("a put after the counter was cleared by an observation = %d %q, want it to fit", rec.Code, doc.Code)
	}
	p.observeUsage(objectstorage.Oregon, b.ID, 1000, 3, time.Now().Add(8*time.Second)) // the workspace shrank
	if rec, doc := other.put(k, "/bkt/e", 600); rec.Code != 200 {
		t.Errorf("after the sweep freed space and cleared the counter = %d %q", rec.Code, doc.Code)
	}
	// The object cap counts the same way.
	p.observeUsage(objectstorage.Oregon, b.ID, 1000, objectstorage.MaxWorkspaceObjects, time.Now().Add(10*time.Second))
	if rec, doc := p.put(k, "/bkt/f", 1); rec.Code != 403 || !strings.Contains(doc.Message, "objects") {
		t.Errorf("a put at the object cap = %d %q %q, want 403 QuotaExceeded naming objects", rec.Code, doc.Code, doc.Message)
	}
}

func TestPutsInFlightOnOneReplicaCountAgainstTheCapOfTheNext(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("bkt", objectstorage.Oregon)
	b, _ := p.Svc.GetBucket(context.Background(), p.ws, "bkt")
	p.observeUsage(objectstorage.Oregon, b.ID, objectstorage.MaxWorkspaceBytes-2000, 1, time.Now().Add(-time.Minute))
	// A slow upload of 1,500 declared bytes holds its length while its body trickles in.
	pr, pw := io.Pipe()
	done := make(chan int, 1)
	go func() {
		req := p.signed(k, "PUT", "/bkt/slow", pr, func(r *http.Request) { r.ContentLength = 1500 })
		req.ContentLength = 1500
		rec, _ := p.do(req)
		done <- rec.Code
	}()
	pw.Write([]byte("first bytes"))
	waitFor(t, func() bool { in, _ := p.Svc.InflightForTest(p.ws, objectstorage.Oregon); return in == 1500 })
	if rec, doc := p.put(k, "/bkt/second", 600); rec.Code != 403 || doc.Code != "QuotaExceeded" {
		t.Errorf("a put that the in-flight upload's declared length pushes over = %d %q, want 403 QuotaExceeded", rec.Code, doc.Code)
	}
	pw.Write(payload(1500 - len("first bytes")))
	pw.Close()
	if code := <-done; code != 200 {
		t.Errorf("the slow upload finished with %d", code)
	}
	if in, _ := p.Svc.InflightForTest(p.ws, objectstorage.Oregon); in != 0 {
		t.Errorf("%d bytes still held after the upload ended", in)
	}
	// An upload that fails frees its hold too.
	failing := p.signed(k, "PUT", "/bkt/broken", bytes.NewReader(payload(10)), func(r *http.Request) { r.ContentLength = 1500 })
	failing.ContentLength = 1500
	p.do(failing)
	if in, _ := p.Svc.InflightForTest(p.ws, objectstorage.Oregon); in != 0 {
		t.Errorf("a failed upload left %d bytes held", in)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 400; i++ {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the condition never held")
}

func TestDownloadsArePacedPerWorkspaceAndTenParallelRangesAreNeverRefused(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("bkt", objectstorage.Oregon)
	const size = 8 << 20
	p.put(k, "/bkt/big", size)
	// 4 MiB/s with a one-second burst: ten parallel 512 KiB ranges (5 MiB) must all succeed,
	// and the pacing, not a refusal, is what makes them wait.
	p.Svc.SetPacing(4<<20, 0)
	var wg sync.WaitGroup
	codes := make([]int, 10)
	start := time.Now()
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			from := i * (512 << 10)
			rec, _ := p.do(p.signed(k, "GET", "/bkt/big", nil, func(r *http.Request) {
				r.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", from, from+512<<10-1))
			}))
			codes[i] = rec.Code
			if !bytes.Equal(rec.Body.Bytes(), payload(size)[from:from+512<<10]) {
				codes[i] = -1
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)
	for i, c := range codes {
		if c != 206 {
			t.Errorf("range %d = %d, want 206 with the right bytes (never SlowDown or 429)", i, c)
		}
	}
	// 5 MiB at 4 MiB/s with a 4 MiB burst: the last bytes wait for at least a quarter second.
	if elapsed < 150*time.Millisecond {
		t.Errorf("ten ranges took %v: the pacing did not slow them", elapsed)
	}
	// Another workspace's budget is its own.
	if lim := p.Svc.PaceDownload(context.Background(), "tea-someone-else", io.Discard); lim == nil {
		t.Fatal("no pacer")
	}
	// Uploads are paced too.
	p.Svc.SetPacing(0, 1<<20)
	start = time.Now()
	if rec, doc := p.put(k, "/bkt/up", 2<<20); rec.Code != 200 {
		t.Fatalf("a paced upload = %d %q", rec.Code, doc.Code)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Errorf("a 2 MiB upload at 1 MiB/s took %v", time.Since(start))
	}
}

func TestRequestRatesAreCappedPerWorkspaceAndPerKeyWithSlowDownNever429(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionRead, 0)
	other := p.newKey(objectstorage.KeyPermissionRead, 0)
	refused, ok := 0, 0
	for i := 0; i < objectstorage.S3RequestBurstPerKey+50; i++ {
		rec, doc := p.do(p.signed(k, "GET", "/", nil))
		switch {
		case rec.Code == 200:
			ok++
		case rec.Code == 503 && doc.Code == "SlowDown":
			refused++
		default:
			t.Fatalf("request %d = %d %q, want 200 or 503 SlowDown", i, rec.Code, doc.Code)
		}
	}
	if ok < objectstorage.S3RequestBurstPerKey || refused < 40 {
		t.Errorf("%d served and %d refused of %d requests in a burst: the per-key budget of %d a second (burst %d) did not hold", ok, refused, ok+refused, objectstorage.S3RequestsPerSecondPerKey, objectstorage.S3RequestBurstPerKey)
	}
	// Another key of the workspace has its own budget, until the workspace's runs out.
	if rec, _ := p.do(p.signed(other, "GET", "/", nil)); rec.Code != 200 {
		t.Errorf("a second key's first request = %d, want it served", rec.Code)
	}
	served := 0
	for i := 0; i < 2*objectstorage.S3RequestBurstPerWorkspace; i++ {
		fresh := p.newKey(objectstorage.KeyPermissionRead, 0)
		if rec, _ := p.do(p.signed(fresh, "GET", "/", nil)); rec.Code == 200 {
			served++
		}
	}
	if served > objectstorage.S3RequestBurstPerWorkspace {
		t.Errorf("%d requests of one workspace were served across keys, more than its burst of %d", served, objectstorage.S3RequestBurstPerWorkspace)
	}
}

func TestTheMonthlyEgressBudgetRefusesReadsPastTheLargerOfStoredBytesAndFiveGiB(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("bkt", objectstorage.Oregon)
	b, _ := p.Svc.GetBucket(context.Background(), p.ws, "bkt")
	p.put(k, "/bkt/o", 100)
	add := func(n int64) {
		if err := p.PG.AddObjectStorageUsage(context.Background(), []store.ObjectStorageUsageDelta{
			{WorkspaceID: p.ws, Kind: store.UsageKindObjectStorageEgressBytes, Window: time.Now(), Quantity: n},
		}); err != nil {
			t.Fatal(err)
		}
	}
	get := func() (int, string) {
		rec, doc := p.do(p.signed(k, "GET", "/bkt/o", nil))
		return rec.Code, doc.Code
	}
	// A new Service has no cached reading of the meter.
	fresh := func() *pgRig { return p.replica() }
	add(objectstorage.S3EgressBudgetFloor - 1)
	if c, _ := get(); c != 200 {
		t.Fatalf("a read just under the 5 GiB floor = %d", c)
	}
	add(1)
	if c, code := fresh().do2(k, "GET", "/bkt/o"); c != 403 || code != "QuotaExceeded" {
		t.Errorf("a read past the floor = %d %q, want 403 QuotaExceeded", c, code)
	}
	// A workspace that stores more than 5 GiB may read that much: the budget is the larger.
	p.observeUsage(objectstorage.Oregon, b.ID, 3*objectstorage.S3EgressBudgetFloor, 5, time.Now())
	if c, code := fresh().do2(k, "GET", "/bkt/o"); c != 200 {
		t.Errorf("a read by a workspace that stores 15 GiB = %d %q, want it within its budget", c, code)
	}
	add(3 * objectstorage.S3EgressBudgetFloor)
	if c, code := fresh().do2(k, "GET", "/bkt/o"); c != 403 || code != "QuotaExceeded" {
		t.Errorf("a read past the stored bytes = %d %q, want 403", c, code)
	}
	// Writes and heads are not reads of bytes.
	if rec, _ := fresh().do(fresh().signed(k, "HEAD", "/bkt/o", nil)); rec.Code != 200 {
		t.Errorf("a HEAD past the budget = %d, want it served", rec.Code)
	}
}

func (p *pgRig) do2(k testKey, method, target string) (int, string) {
	rec, doc := p.do(p.signed(k, method, target, nil))
	return rec.Code, doc.Code
}
