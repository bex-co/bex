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

package controller

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func testReplicaTLSBundle(t *testing.T, dnsName string) (caPEM []byte, serverCert tls.Certificate) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-replica-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	caPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})

	srvKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	srvTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: dnsName},
		DNSNames:     []string{dnsName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	srvDER, err := x509.CreateCertificate(rand.Reader, srvTmpl, caCert, &srvKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	srvCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srvDER})
	srvKeyDER, err := x509.MarshalECPrivateKey(srvKey)
	if err != nil {
		t.Fatal(err)
	}
	srvKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: srvKeyDER})
	cert, err := tls.X509KeyPair(srvCertPEM, srvKeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return caPEM, cert
}

func TestProbeReadReplicaStatusURL(t *testing.T) {
	const dnsName = "dpg-sync-r-deadbeef01-rw.tenant.svc"
	caPEM, cert := testReplicaTLSBundle(t, dnsName)
	otherCA, _ := testReplicaTLSBundle(t, dnsName)

	body := `{"isPrimary":false,"isWalReceiverActive":true,"replayPaused":false}`
	statusCode := http.StatusOK
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pg/status" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	ctx := context.Background()
	sample, err := probeReadReplicaStatusURL(ctx, srv.URL+"/pg/status", dnsName, caPEM)
	if err != nil {
		t.Fatalf("healthy probe: %v", err)
	}
	if !sample.Syncing() {
		t.Fatalf("healthy sample = %+v, want Syncing()=true", sample)
	}

	body = `{"isPrimary":false,"isWalReceiverActive":false,"replayPaused":false}`
	sample, err = probeReadReplicaStatusURL(ctx, srv.URL+"/pg/status", dnsName, caPEM)
	if err != nil || sample.Syncing() {
		t.Fatalf("stopped walreceiver = %+v err=%v, want Syncing()=false", sample, err)
	}

	body = `{"isPrimary":false,"isWalReceiverActive":true,"replayPaused":true}`
	sample, err = probeReadReplicaStatusURL(ctx, srv.URL+"/pg/status", dnsName, caPEM)
	if err != nil || sample.Syncing() {
		t.Fatalf("paused replay = %+v err=%v, want Syncing()=false", sample, err)
	}

	if _, err := probeReadReplicaStatusURL(ctx, srv.URL+"/pg/status", dnsName, otherCA); err == nil {
		t.Fatal("expected TLS verification failure for untrusted CA")
	}
	if _, err := probeReadReplicaStatusURL(ctx, srv.URL+"/pg/status", "wrong.tenant.svc", caPEM); err == nil {
		t.Fatal("expected TLS verification failure for mismatched serverName")
	}

	statusCode = http.StatusServiceUnavailable
	if _, err := probeReadReplicaStatusURL(ctx, srv.URL+"/pg/status", dnsName, caPEM); err == nil {
		t.Fatal("expected error for HTTP 503")
	}
}

func TestReplicaClusterPodIPAndServerName(t *testing.T) {
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "dpg-1-r-0123456789", "namespace": "tea-ws"},
		"status": map[string]any{
			"currentPrimary": "dpg-1-r-0123456789-1",
			"instancesReportedState": map[string]any{
				"dpg-1-r-0123456789-1": map[string]any{"ip": "10.244.1.12"},
			},
		},
	}}
	ip, err := replicaClusterPodIP(cluster)
	if err != nil || ip != "10.244.1.12" {
		t.Fatalf("pod IP = %q err=%v, want 10.244.1.12", ip, err)
	}

	// Refuse cloud metadata / link-local IPs.
	_ = unstructured.SetNestedField(cluster.Object, "169.254.169.254", "status", "instancesReportedState", "dpg-1-r-0123456789-1", "ip")
	if _, err := replicaClusterPodIP(cluster); err == nil {
		t.Fatal("expected error for link-local metadata IP")
	}

	db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "dpg-1", Namespace: "tea-ws"}}
	if got := replicaClusterStatusServerName(db, "reader", cluster, "db.bex.co"); got != "dpg-1-r-0123456789-rw.tea-ws.svc" {
		t.Fatalf("private serverName = %q", got)
	}
	_ = unstructured.SetNestedField(cluster.Object, "dpg-1-public-tls", "spec", "certificates", "serverTLSSecret")
	if got := replicaClusterStatusServerName(db, "reader", cluster, "db.bex.co"); got != "dpg-1-ro-reader.db.bex.co" {
		t.Fatalf("public serverName = %q", got)
	}
}

func assertSyncState(t *testing.T, label string, got readReplicaSyncState, gotReq time.Duration, wantSyncing *bool, wantStopped string, wantReq time.Duration) {
	t.Helper()
	syncMatch := (got.syncing == nil && wantSyncing == nil) || (got.syncing != nil && wantSyncing != nil && *got.syncing == *wantSyncing)
	if !syncMatch || got.syncStoppedAt != wantStopped || (wantReq >= 0 && gotReq != wantReq) {
		t.Fatalf("%s = {syncing:%v stoppedAt:%q} req=%v, want {syncing:%v stoppedAt:%q} req=%v",
			label, got.syncing, got.syncStoppedAt, gotReq, wantSyncing, wantStopped, wantReq)
	}
}

func TestEvaluateReadReplicaSyncTransitions(t *testing.T) {
	ctx := context.Background()
	t0 := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	syncing := func() (ReadReplicaSyncSample, error) {
		return ReadReplicaSyncSample{IsWalReceiverActive: true}, nil
	}
	stopped := func() (ReadReplicaSyncSample, error) {
		return ReadReplicaSyncSample{IsWalReceiverActive: false}, nil
	}
	probeErr := func() (ReadReplicaSyncSample, error) {
		return ReadReplicaSyncSample{}, errors.New("connection refused")
	}
	toPrev := func(st readReplicaSyncState) *appv1alpha1.DatabaseReadReplicaStatus {
		return &appv1alpha1.DatabaseReadReplicaStatus{Name: "a", Syncing: st.syncing, SyncStoppedAt: st.syncStoppedAt}
	}

	// 1. Initial sync (replica not yet ready): unobserved, never stale.
	st, req := evaluateReadReplicaSync(ctx, "dpg-1", "a", nil, false, true, false, t0, readReplicaStaleAfter, stopped)
	assertSyncState(t, "initial sync", st, req, nil, "", 0)

	// 2. Ready and syncing.
	st, req = evaluateReadReplicaSync(ctx, "dpg-1", "a", nil, false, true, true, t0, readReplicaStaleAfter, syncing)
	assertSyncState(t, "healthy", st, req, new(true), "", readReplicaSyncProbeInterval)

	// 3. First non-syncing sample starts hysteresis: Syncing stays true, SyncStoppedAt stamped.
	t1 := t0.Add(30 * time.Second)
	st, req = evaluateReadReplicaSync(ctx, "dpg-1", "a", toPrev(st), false, true, true, t1, readReplicaStaleAfter, stopped)
	assertSyncState(t, "first non-syncing", st, req, new(true), t1.Format(time.RFC3339), readReplicaSyncProbeInterval)

	// 4. Short hiccup / failover recovers before threshold: clears SyncStoppedAt, never stale.
	t2 := t1.Add(15 * time.Second)
	st, req = evaluateReadReplicaSync(ctx, "dpg-1", "a", toPrev(st), false, true, true, t2, readReplicaStaleAfter, syncing)
	assertSyncState(t, "hiccup recovery", st, req, new(true), "", readReplicaSyncProbeInterval)

	// 5. Real outage crosses 2m threshold: stays true at 119s, flips false at 120s.
	t3 := t2.Add(30 * time.Second)
	st, _ = evaluateReadReplicaSync(ctx, "dpg-1", "a", toPrev(st), false, true, true, t3, readReplicaStaleAfter, stopped)

	st, req = evaluateReadReplicaSync(ctx, "dpg-1", "a", toPrev(st), false, true, true, t3.Add(119*time.Second), readReplicaStaleAfter, stopped)
	assertSyncState(t, "at 119s", st, req, new(true), t3.Format(time.RFC3339), time.Second)

	st, req = evaluateReadReplicaSync(ctx, "dpg-1", "a", toPrev(st), false, true, true, t3.Add(2*time.Minute), readReplicaStaleAfter, stopped)
	assertSyncState(t, "at 120s", st, req, new(false), t3.Format(time.RFC3339), readReplicaSyncProbeInterval)

	// 6. Transient probe error while stale keeps stale state rather than flapping.
	st, req = evaluateReadReplicaSync(ctx, "dpg-1", "a", toPrev(st), false, true, true, t3.Add(3*time.Minute), readReplicaStaleAfter, probeErr)
	assertSyncState(t, "probe error while stale", st, req, new(false), t3.Format(time.RFC3339), readReplicaSyncProbeInterval)

	// 7. Recovery clears stale and SyncStoppedAt.
	st, req = evaluateReadReplicaSync(ctx, "dpg-1", "a", toPrev(st), false, true, true, t3.Add(4*time.Minute), readReplicaStaleAfter, syncing)
	assertSyncState(t, "recovery", st, req, new(true), "", readReplicaSyncProbeInterval)

	// 8. Suspension clears both fields.
	st, req = evaluateReadReplicaSync(ctx, "dpg-1", "a", toPrev(st), true, false, false, t3.Add(5*time.Minute), readReplicaStaleAfter, stopped)
	assertSyncState(t, "suspended", st, req, nil, "", 0)

	// 9. Primary unsettled during hysteresis clears pending SyncStoppedAt; primary
	// unsettled while already stale preserves stale so recovery does not flap.
	t4 := t3.Add(10 * time.Minute)
	st, _ = evaluateReadReplicaSync(ctx, "dpg-1", "a", &appv1alpha1.DatabaseReadReplicaStatus{Name: "a", Syncing: new(true), SyncStoppedAt: t4.Format(time.RFC3339)}, false, false, true, t4.Add(time.Minute), readReplicaStaleAfter, stopped)
	assertSyncState(t, "unsettled during hysteresis", st, 0, new(true), "", 0)

	st, _ = evaluateReadReplicaSync(ctx, "dpg-1", "a", &appv1alpha1.DatabaseReadReplicaStatus{Name: "a", Syncing: new(false), SyncStoppedAt: t4.Format(time.RFC3339)}, false, false, true, t4.Add(3*time.Minute), readReplicaStaleAfter, stopped)
	assertSyncState(t, "unsettled while stale", st, 0, new(false), t4.Format(time.RFC3339), 0)

	// 10. Replay paused counts as not syncing even when walreceiver is active.
	paused := func() (ReadReplicaSyncSample, error) {
		return ReadReplicaSyncSample{IsWalReceiverActive: true, ReplayPaused: true}, nil
	}
	st, _ = evaluateReadReplicaSync(ctx, "dpg-1", "a", &appv1alpha1.DatabaseReadReplicaStatus{Name: "a", Syncing: new(true), SyncStoppedAt: t4.Format(time.RFC3339)}, false, true, true, t4.Add(2*time.Minute), readReplicaStaleAfter, paused)
	assertSyncState(t, "replay paused at 120s", st, 0, new(false), t4.Format(time.RFC3339), 0)
}
