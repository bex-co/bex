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
	"errors"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// prometheusPath is the scrape config whose bex-s3gw job keeps only the families its keep-list
// names; a family outside it is exported by the pods and stored by nothing.
const prometheusPath = "../../../../deploy/gitops/base/prometheus.yaml"

func newTestMetrics(t *testing.T, g *rig) (*Metrics, *atomic.Int64) {
	t.Helper()
	var inFlight atomic.Int64
	m := NewMetrics(prometheus.NewRegistry(), func() (int, int) { return int(inFlight.Load()), 64 })
	g.gw.cfg.Metrics = m
	return m, &inFlight
}

func TestEveryS3MetricIsKeptByTheScrapeJob(t *testing.T) {
	raw, err := os.ReadFile(prometheusPath)
	if err != nil {
		t.Fatal(err)
	}
	job := regexp.MustCompile(`(?s)- job_name: bex-s3gw\n(.*?)\n\s*(?:#[^\n]*\n\s*)*- job_name:`).FindSubmatch(raw)
	if job == nil {
		t.Fatalf("there is no bex-s3gw scrape job in %s", prometheusPath)
	}
	found := regexp.MustCompile(`(?m)^\s*regex:\s*(bex_s3gw_\S+)\s*$`).FindSubmatch(job[1])
	if found == nil {
		t.Fatalf("the bex-s3gw job has no metric keep-list regex starting bex_s3gw_: %s", job[1])
	}
	keep := regexp.MustCompile(`^(?:` + string(found[1]) + `)$`) // Prometheus anchors relabel regexes

	reg := prometheus.NewRegistry()
	m := NewMetrics(reg, func() (int, int) { return 1, 64 })
	// Every labeled family has a series once it is used.
	m.requests.WithLabelValues("GetObject", "OK", "200").Inc()
	m.egressRefusals.Inc()
	m.listingBytes.Add(1)
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	if len(families) != 7 {
		t.Fatalf("NewMetrics registered %d families, want 7 (an alert or panel may read a new one: update the keep-list and this count)", len(families))
	}
	for _, f := range families {
		if !keep.MatchString(f.GetName()) {
			t.Errorf("%s is not matched by the bex-s3gw scrape job's keep-list %q: Prometheus would drop it", f.GetName(), found[1])
		}
	}
}

func TestRequestsAreCountedByOperationCodeAndStatusAndBytesByDirection(t *testing.T) {
	g := newRig(t)
	m, _ := newTestMetrics(t, g)
	k := g.newKey(objectstorage.KeyPermissionRead, 0)
	spyOn(t, "PutObject", func(rq *request) *Error {
		n, _ := io.Copy(io.Discard, rq.r.Body)
		_, _ = rq.w.Write([]byte("0123456789"))
		_ = n
		return nil
	})
	spyOn(t, "ListBuckets", func(rq *request) *Error { return nil })
	g.do(g.signed(k, "GET", "/", nil))
	g.do(g.signed(g.newKey(objectstorage.KeyPermissionReadWrite, 0), "PUT", "/bkt/o", strings.NewReader("abcdef"), func(r *http.Request) { r.ContentLength = 6 }))
	g.do(g.signed(k, "PUT", "/bkt/o", strings.NewReader("x"), func(r *http.Request) { r.ContentLength = 1 })) // a read key: denied
	if got := testutil.ToFloat64(m.requests.WithLabelValues("ListBuckets", "OK", "200")); got != 1 {
		t.Errorf("ListBuckets OK 200 = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.requests.WithLabelValues("PutObject", "OK", "200")); got != 1 {
		t.Errorf("PutObject OK 200 = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.requests.WithLabelValues("PutObject", "AccessDenied", "403")); got != 1 {
		t.Errorf("a refused PutObject = %v, want one AccessDenied 403 (the denial is an operation's, not an authentication failure)", got)
	}
	if got := testutil.ToFloat64(m.bytes.WithLabelValues("in")); got != 6 {
		t.Errorf("bytes in = %v, want the 6 the one accepted put sent", got)
	}
	if got := testutil.ToFloat64(m.bytes.WithLabelValues("out")); got < 10 {
		t.Errorf("bytes out = %v, want at least the 10 the handler wrote", got)
	}
	for _, reason := range []string{"unknown_key", "bad_signature", "secret_store"} {
		if got := testutil.ToFloat64(m.authFailures.WithLabelValues(reason)); got != 0 {
			t.Errorf("auth failures %q = %v after authenticated requests, want 0", reason, got)
		}
	}
}

func TestAuthenticationFailuresAreCountedByReasonAndOpenBaoIsItsOwn(t *testing.T) {
	g := newRig(t)
	m, _ := newTestMetrics(t, g)
	spyOn(t, "ListBuckets", func(*request) *Error { return nil })
	k := g.newKey(objectstorage.KeyPermissionRead, 0)
	wrong := testKey{id: k.id, secret: k.secret + "x"}
	unknown := testKey{id: "s3k-" + strings.Repeat("a", 20), secret: "s"}
	g.do(g.signed(wrong, "GET", "/", nil))
	g.do(g.signed(unknown, "GET", "/", nil))
	g.do(g.signed(unknown, "GET", "/", nil))
	// An OpenBao failure on a key this replica has not served: a 503, and the platform's.
	fresh := g.newKey(objectstorage.KeyPermissionRead, 0)
	g.kv.GetErr = errors.New("openbao is down")
	rec, doc := g.do(g.signed(fresh, "GET", "/", nil))
	if rec.Code != 503 || doc.Code != "ServiceUnavailable" {
		t.Fatalf("a key whose secret cannot be read = %d %q, want 503 ServiceUnavailable", rec.Code, doc.Code)
	}
	for reason, want := range map[string]float64{"bad_signature": 1, "unknown_key": 2, "secret_store": 1, "clock_skew": 0} {
		if got := testutil.ToFloat64(m.authFailures.WithLabelValues(reason)); got != want {
			t.Errorf("auth failures %q = %v, want %v", reason, got, want)
		}
	}
	if got := testutil.ToFloat64(m.requests.WithLabelValues("unauthenticated", "InvalidAccessKeyId", "403")); got != 2 {
		t.Errorf("unauthenticated InvalidAccessKeyId 403 = %v, want 2", got)
	}
}

func TestAnEgressBudgetRefusalAndTheBytesOfAListingAreCounted(t *testing.T) {
	g := newRig(t)
	m, inFlight := newTestMetrics(t, g)
	k := g.newKey(objectstorage.KeyPermissionRead, 0)
	spyOn(t, "GetObject", func(*request) *Error {
		return newError(http.StatusForbidden, "QuotaExceeded", "egress")
	})
	spyOn(t, "ListObjectsV2", func(rq *request) *Error {
		_, _ = rq.w.Write([]byte(strings.Repeat("k", 700)))
		return nil
	})
	spyOn(t, "PutObject", func(*request) *Error { return newError(http.StatusForbidden, "QuotaExceeded", "bytes") })
	g.do(g.signed(k, "GET", "/b/o", nil))
	g.do(g.signed(k, "GET", "/b?list-type=2", nil))
	g.do(g.signed(g.newKey(objectstorage.KeyPermissionReadWrite, 0), "PUT", "/b/o", strings.NewReader("x"), func(r *http.Request) { r.ContentLength = 1 }))
	if got := testutil.ToFloat64(m.egressRefusals); got != 1 {
		t.Errorf("egress refusals = %v, want 1: a put refused for bytes is not an egress refusal", got)
	}
	if got := testutil.ToFloat64(m.listingBytes); got != 700 {
		t.Errorf("listing bytes = %v, want the 700 the listing answered", got)
	}
	// The stream gauges read the replica's load when they are scraped.
	inFlight.Store(7)
	reg := prometheus.NewRegistry()
	NewMetrics(reg, func() (int, int) { return int(inFlight.Load()), 64 })
	families, _ := reg.Gather()
	got := map[string]float64{}
	for _, f := range families {
		got[f.GetName()] = f.GetMetric()[0].GetGauge().GetValue()
	}
	if got["bex_s3gw_streams_in_flight"] != 7 || got["bex_s3gw_stream_limit"] != 64 {
		t.Errorf("the stream gauges read %v, want 7 in flight of 64", got)
	}
}
