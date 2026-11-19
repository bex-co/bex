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
	"cmp"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"maps"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	boundedhttp "github.com/bex-co/bex/lego/operator/internal/httpclient"
	"github.com/bex-co/bex/lego/types/netutil"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Stale read-replica detection (w4/m511, ADR009 § Stale read-replica detection).
//
// A CNPG replica Cluster whose WAL streaming stops keeps reporting
// phase="Cluster in healthy state" and Ready=True while serving stale reads,
// and CNPG's :9187/metrics pg_replication query fails on replica clusters
// because bootstrap.pg_basebackup leaves the application database name
// defaulted to "app". The operator therefore polls the replica instance
// manager's native HTTPS status endpoint (https://<pod-ip>:8000/pg/status),
// admitted by every tenant namespace's allow-operator-probe-ingress policy and
// verified against the Cluster's own serverCASecret.
const (
	// readReplicaStaleAfter is how long a ready read replica must stay
	// non-syncing before Database.status.readReplicaStatuses[].syncing flips
	// to false. Measured on dev-4: an HA primary failover dips
	// isWalReceiverActive for ~10.6 s and a network-policy recovery reconnects
	// in ~28 s, so 2 minutes filters transient hiccups while reporting a real
	// outage promptly.
	readReplicaStaleAfter = 2 * time.Minute
	// readReplicaSyncProbeInterval re-probes ready read replicas: a stopped
	// walreceiver changes nothing on the replica Cluster CR, so no watch event
	// would otherwise wake the reconciler.
	readReplicaSyncProbeInterval = 30 * time.Second
	// cnpgInstanceManagerStatusPort is CNPG's instance-manager HTTPS status
	// port (GET /pg/status).
	cnpgInstanceManagerStatusPort = "8000"
)

// ReadReplicaSyncSample is one replica instance's replication status from
// CNPG's instance manager (/pg/status).
type ReadReplicaSyncSample struct {
	IsPrimary           bool `json:"isPrimary"`
	IsWalReceiverActive bool `json:"isWalReceiverActive"`
	ReplayPaused        bool `json:"replayPaused"`
}

// Syncing reports whether the instance is a standby actively receiving and
// replaying WAL from its primary.
func (s ReadReplicaSyncSample) Syncing() bool {
	return !s.IsPrimary && s.IsWalReceiverActive && !s.ReplayPaused
}

// ReadReplicaSyncReader reads one replica instance's live replication status.
// Production queries CNPG's :8000/pg/status over TLS verified against the
// replica Cluster's serverCASecret; tests inject a deterministic reader.
type ReadReplicaSyncReader func(ctx context.Context, db *appv1alpha1.Database, replicaName string, replicaCluster *unstructured.Unstructured) (ReadReplicaSyncSample, error)

// readReplicaSyncState is the per-replica sync fields projected onto
// DatabaseReadReplicaStatus.
type readReplicaSyncState struct {
	syncing       *bool
	syncStoppedAt string
}

func (r *DatabaseReconciler) readReplicaStaleThreshold() time.Duration {
	if r.ReadReplicaStaleAfter > 0 {
		return r.ReadReplicaStaleAfter
	}
	return readReplicaStaleAfter
}

// ProbeReadReplicaSync queries the CNPG instance manager on the replica
// Cluster's designated primary pod for its live streaming state.
func (r *DatabaseReconciler) ProbeReadReplicaSync(ctx context.Context, db *appv1alpha1.Database, replicaName string, replicaCluster *unstructured.Unstructured) (ReadReplicaSyncSample, error) {
	podIP, err := replicaClusterPodIP(replicaCluster)
	if err != nil {
		return ReadReplicaSyncSample{}, err
	}
	caSecretName := replicaClusterCASecretName(db.Name, replicaCluster)
	var caSecret corev1.Secret
	if err := r.secretClient().Get(ctx, client.ObjectKey{Namespace: db.Namespace, Name: caSecretName}, &caSecret); err != nil {
		return ReadReplicaSyncSample{}, fmt.Errorf("read replica server CA secret %s: %w", caSecretName, err)
	}
	caPEM := caSecret.Data["ca.crt"]
	if len(caPEM) == 0 {
		return ReadReplicaSyncSample{}, fmt.Errorf("replica server CA secret %s has no ca.crt", caSecretName)
	}
	serverName := replicaClusterStatusServerName(db, replicaName, replicaCluster, r.DBDomain)
	statusURL := "https://" + net.JoinHostPort(podIP, cnpgInstanceManagerStatusPort) + "/pg/status"
	return probeReadReplicaStatusURL(ctx, statusURL, serverName, caPEM)
}

func replicaClusterPodIP(cluster *unstructured.Unstructured) (string, error) {
	states, found, _ := unstructured.NestedMap(cluster.Object, "status", "instancesReportedState")
	if !found || len(states) == 0 {
		return "", fmt.Errorf("replica cluster %s has no instancesReportedState", cluster.GetName())
	}
	checkInstance := func(name string) (string, bool) {
		entry, ok := states[name].(map[string]any)
		if !ok {
			return "", false
		}
		rawIP, _ := entry["ip"].(string)
		addr, err := netip.ParseAddr(rawIP)
		if err != nil || addr.IsUnspecified() || addr.IsMulticast() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() {
			return "", false
		}
		return addr.String(), true
	}
	if primary, _, _ := unstructured.NestedString(cluster.Object, "status", "currentPrimary"); primary != "" {
		if ip, ok := checkInstance(primary); ok {
			return ip, nil
		}
	}
	for _, name := range slices.Sorted(maps.Keys(states)) {
		if ip, ok := checkInstance(name); ok {
			return ip, nil
		}
	}
	return "", fmt.Errorf("replica cluster %s has no valid instance pod IP", cluster.GetName())
}

func replicaClusterCASecretName(dbName string, cluster *unstructured.Unstructured) string {
	statusCA, _, _ := unstructured.NestedString(cluster.Object, "status", "certificates", "serverCASecret")
	specCA, _, _ := unstructured.NestedString(cluster.Object, "spec", "certificates", "serverCASecret")
	return cmp.Or(statusCA, specCA, dbName+"-ca")
}

// replicaClusterStatusServerName returns a DNS SAN guaranteed on the replica's
// server certificate: for a public cert-manager certificate, the replica's
// public SNI host; for CNPG's generated certificate, the cluster's -rw Service.
func replicaClusterStatusServerName(db *appv1alpha1.Database, replicaName string, cluster *unstructured.Unstructured, dbDomain string) string {
	if tlsSecret, _, _ := unstructured.NestedString(cluster.Object, "spec", "certificates", "serverTLSSecret"); tlsSecret != "" && dbDomain != "" {
		return appv1alpha1.ReadReplicaServiceName(db.Name, replicaName) + "." + dbDomain
	}
	return fmt.Sprintf("%s-rw.%s.svc", cluster.GetName(), db.Namespace)
}

func probeReadReplicaStatusURL(ctx context.Context, statusURL, serverName string, caPEM []byte) (ReadReplicaSyncSample, error) {
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return ReadReplicaSyncSample{}, fmt.Errorf("parse replica server CA PEM")
	}
	probeCtx, cancel := boundedhttp.WithTimeout(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, statusURL, nil)
	if err != nil {
		return ReadReplicaSyncSample{}, err
	}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           netutil.SafeDialContextFunc(2*time.Second, netutil.UnsafeMetadataIP),
		TLSClientConfig:       &tls.Config{RootCAs: roots, ServerName: serverName, MinVersion: tls.VersionTLS12},
		DisableKeepAlives:     true,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   2 * time.Second,
		ResponseHeaderTimeout: 3 * time.Second,
	}
	defer transport.CloseIdleConnections()
	hc := &http.Client{Transport: transport, Timeout: boundedhttp.RequestTimeout}
	resp, err := hc.Do(req)
	if err != nil {
		return ReadReplicaSyncSample{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return ReadReplicaSyncSample{}, fmt.Errorf("replica status endpoint returned HTTP %d", resp.StatusCode)
	}
	var sample ReadReplicaSyncSample
	if err := boundedhttp.DecodeJSON(resp.Body, &sample); err != nil {
		return ReadReplicaSyncSample{}, fmt.Errorf("decode replica status: %w", err)
	}
	return sample, nil
}

func previousReplicaSyncState(previous *appv1alpha1.DatabaseReadReplicaStatus) readReplicaSyncState {
	if previous == nil {
		return readReplicaSyncState{}
	}
	var syncing *bool
	if previous.Syncing != nil {
		syncing = new(*previous.Syncing)
	}
	return readReplicaSyncState{syncing: syncing, syncStoppedAt: previous.SyncStoppedAt}
}

// evaluateReadReplicaSync computes one read replica's next (Syncing,
// SyncStoppedAt) status and requeue interval from its previous status, cluster
// readiness, and live /pg/status probe.
func evaluateReadReplicaSync(
	ctx context.Context,
	dbName, replicaName string,
	previous *appv1alpha1.DatabaseReadReplicaStatus,
	suspended, primarySettled, replicaReady bool,
	now time.Time,
	threshold time.Duration,
	probe func() (ReadReplicaSyncSample, error),
) (readReplicaSyncState, time.Duration) {
	if suspended {
		return readReplicaSyncState{}, 0
	}
	prev := previousReplicaSyncState(previous)
	if !primarySettled || !replicaReady {
		// Initial sync (prev.syncing == nil) stays unobserved. A replica that
		// was already stale stays stale rather than flapping back to healthy
		// during a transient primary or pod disruption. A healthy replica or
		// one inside hysteresis clears its pending countdown so primary
		// downtime, failover or a pod restart never counts as stale.
		if prev.syncing != nil && !*prev.syncing {
			return prev, 0
		}
		return readReplicaSyncState{syncing: prev.syncing}, 0
	}
	sample, err := probe()
	if err != nil {
		logf.FromContext(ctx).Info("read replica sync probe unavailable", "database", dbName, "replica", replicaName, "reason", err)
		return prev, readReplicaSyncProbeInterval
	}
	if sample.Syncing() {
		return readReplicaSyncState{syncing: new(true)}, readReplicaSyncProbeInterval
	}
	now = now.UTC().Truncate(time.Second)
	stoppedAt := now
	if prev.syncStoppedAt != "" {
		if parsed, parseErr := time.Parse(time.RFC3339, prev.syncStoppedAt); parseErr == nil && !parsed.After(now) {
			stoppedAt = parsed.UTC()
		}
	}
	elapsed := now.Sub(stoppedAt)
	stamp := stoppedAt.Format(time.RFC3339)
	if elapsed >= threshold {
		return readReplicaSyncState{syncing: new(false), syncStoppedAt: stamp}, readReplicaSyncProbeInterval
	}
	remaining := max(threshold-elapsed, time.Second)
	return readReplicaSyncState{syncing: new(true), syncStoppedAt: stamp}, min(readReplicaSyncProbeInterval, remaining)
}

func applyReadReplicaSyncStatus(db *appv1alpha1.Database, states map[string]readReplicaSyncState) {
	if len(states) == 0 {
		return
	}
	for i := range db.Status.ReadReplicaStatuses {
		state, ok := states[db.Status.ReadReplicaStatuses[i].Name]
		if !ok {
			continue
		}
		db.Status.ReadReplicaStatuses[i].Syncing = state.syncing
		db.Status.ReadReplicaStatuses[i].SyncStoppedAt = state.syncStoppedAt
	}
}
