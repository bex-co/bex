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

package store

import (
	"context"
	"fmt"
	"log"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Ready-condition reasons the Database and KeyValue controllers write. They are
// string literals on the operator side (no exported constant to import, and the
// backend must never import the operator), so they are re-declared here and
// held in agreement by the tests in datastore_event_facts_test.go.
const (
	datastoreReasonProvisioned = "Provisioned"
	// A major-version upgrade in flight is the one Ready=False the availability
	// dimension deliberately declines to observe: it is planned downtime whose
	// own postgres_upgrade_* facts describe it, and reporting it twice would
	// page an operator for a maintenance window they scheduled.
	datastoreReasonMajorVersionUpgrade       = "MajorVersionUpgrade"
	datastoreReasonMajorVersionUpgradeFailed = "MajorVersionUpgradeFailed"
	// Refusals of an intent, not evidence that the datastore stopped serving —
	// the datastore precedent for the App path's preRuntimeFailure.
	datastoreReasonStorageShrinkRejected      = "StorageShrinkRejected"
	datastoreReasonConnectionSecretRebuilding = "ConnectionSecretRebuilding"
)

// recordDatastoreObservations runs the App observation path over every managed
// Database and KeyValue in the cluster (w3/m82).
//
// Unlike Apps, datastore CRs are NOT projected by this reconciler — bex-api's
// postgres/keyvalue features create them directly — so they carry no
// managed-by label to select on. What they do carry is core.TenantLabels'
// tenant stamp, which is both the ownership proof and the workspace an
// observed fact is scoped to; a CR without it is hand-applied and has no
// workspace to attribute an event to, so it is skipped.
func (r *Reconciler) recordDatastoreObservations(ctx context.Context) error {
	var databases appv1alpha1.DatabaseList
	if err := r.Client.List(ctx, &databases); err != nil {
		return fmt.Errorf("list Database CRs: %w", err)
	}
	var keyValues appv1alpha1.KeyValueList
	if err := r.Client.List(ctx, &keyValues); err != nil {
		return fmt.Errorf("list KeyValue CRs: %w", err)
	}

	seen := make(map[string]bool, len(databases.Items)+len(keyValues.Items))
	for i := range databases.Items {
		obs, ok := observedDatabaseStateFor(&databases.Items[i])
		if !ok {
			continue
		}
		seen[obs.DatastoreID] = true
		r.recordDatastoreObservation(ctx, obs)
	}
	for i := range keyValues.Items {
		obs, ok := observedKeyValueStateFor(&keyValues.Items[i])
		if !ok {
			continue
		}
		seen[obs.DatastoreID] = true
		r.recordDatastoreObservation(ctx, obs)
	}
	// A deleted datastore must not leave its debounce armed: the same id can
	// never come back (dpg-/red- ids are minted per resource), so the entry is
	// pure leak otherwise.
	for id := range r.datastoreUnhealthyOnce {
		if !seen[id] {
			delete(r.datastoreUnhealthyOnce, id)
		}
	}
	return nil
}

// recordDatastoreObservation applies the two guards the App path applies, in
// the same order and for the same reasons: the stale-conclusion rejection runs
// first so a time-traveled phantom cannot consume the debounce's one free tick
// belonging to the real outage that may follow it.
func (r *Reconciler) recordDatastoreObservation(ctx context.Context, obs ObservedDatastoreState) {
	if r.datastoreUnhealthyOnce == nil {
		r.datastoreUnhealthyOnce = make(map[string]bool)
	}
	obs = debounceDatastoreUnhealthy(r.rejectStaleDatastoreUnhealthy(ctx, obs), r.datastoreUnhealthyOnce)
	if _, err := r.Store.RecordObservedDatastoreState(ctx, obs); err != nil {
		log.Printf("controlplane: record observed datastore state %s: %v", obs.DatastoreID, err)
	}
}

// suppressDatastoreAvailability marks an observation as availability-unseen:
// the checkpoint keeps its previous availability and no edge can fire. Shared
// by both datastore guards so they cannot drift about which fields to blank —
// the App path's suppressAvailability, for the datastore snapshot.
func suppressDatastoreAvailability(obs ObservedDatastoreState) ObservedDatastoreState {
	obs.AvailabilityObserved = false
	obs.Availability = ""
	obs.ReasonCode = ""
	obs.ReadyTransitionAt = time.Time{}
	return obs
}

// debounceDatastoreUnhealthy is debounceUnhealthy keyed on the datastore id:
// the first pass that sees unhealthy is remembered but recorded as
// availability-unseen, and only a second consecutive unhealthy pass can emit
// an unavailable edge. A real outage persists across resyncs and is merely
// delayed one tick; recovery stays immediate.
func debounceDatastoreUnhealthy(obs ObservedDatastoreState, unhealthyOnce map[string]bool) ObservedDatastoreState {
	unhealthy := obs.AvailabilityObserved && obs.Availability == "unhealthy"
	if unhealthy && !unhealthyOnce[obs.DatastoreID] {
		unhealthyOnce[obs.DatastoreID] = true
		return suppressDatastoreAvailability(obs)
	}
	unhealthyOnce[obs.DatastoreID] = unhealthy
	return obs
}

// rejectStaleDatastoreUnhealthy is rejectStaleUnhealthy for a datastore: an
// unhealthy conclusion whose Ready transition PREDATES the recorded healthy
// checkpoint is a time-traveled re-read of a cached client, not a new outage.
// Every unknown fails open toward recording — no checkpoint, an unknown
// transition time, or a lookup error all record normally.
func (r *Reconciler) rejectStaleDatastoreUnhealthy(ctx context.Context, obs ObservedDatastoreState) ObservedDatastoreState {
	if !obs.AvailabilityObserved || obs.Availability != "unhealthy" || obs.ReadyTransitionAt.IsZero() {
		return obs
	}
	healthyAt, err := r.Store.LastDatastoreHealthyTransitionAt(ctx, obs.DatastoreID)
	if err != nil {
		log.Printf("controlplane: healthy checkpoint lookup for %s failed (%v); recording observation anyway", obs.DatastoreID, err)
		return obs
	}
	if healthyAt.IsZero() || !obs.ReadyTransitionAt.Before(healthyAt) {
		return obs
	}
	r.Metrics.Rejection(rejectReasonStaleTransition, rejectSubjectDatastore)
	log.Printf("controlplane: refusing time-traveled unhealthy conclusion for %s: ready transition %s predates the recorded healthy checkpoint %s",
		obs.DatastoreID, obs.ReadyTransitionAt, healthyAt)
	return suppressDatastoreAvailability(obs)
}

// observedDatabaseStateFor derives one managed Postgres' availability snapshot.
// ok is false for a CR this control plane has no workspace to attribute — a
// hand-applied Database carries no tenant label and belongs to no workspace's
// event feed.
func observedDatabaseStateFor(db *appv1alpha1.Database) (ObservedDatastoreState, bool) {
	obs, ok := newObservedDatastoreState(DatastoreKindPostgres, db.Name, db.Labels[LabelTenant],
		string(db.Status.Phase), db.Spec.Suspended)
	if !ok {
		return ObservedDatastoreState{}, false
	}
	obs = applyDatastoreReadyCondition(obs, db.Status.Conditions, db.Status.Phase == appv1alpha1.DBPhaseReady)
	stampBackupObservation(&obs, db.Status.LastBackup)
	if db.Spec.Recovery != nil && db.Spec.Recovery.SourceBackupServerName != "" {
		obs.Recovering = true
	}
	obs.SpecVersion = db.Spec.Version
	obs.CurrentVersion = db.Status.CurrentVersion
	if ready := findReadyCondition(db.Status.Conditions); ready != nil &&
		ready.Status == metav1.ConditionFalse && ready.Reason == datastoreReasonMajorVersionUpgradeFailed {
		obs.UpgradeFailed = true
	}
	return obs, true
}

// observedKeyValueStateFor is observedDatabaseStateFor's Valkey twin. The one
// shape difference that matters is suspension: a suspended KeyValue reports
// Ready=TRUE with the Suspended reason (its zero-replica StatefulSet is what
// the spec asked for), while a suspended Database reports Ready=False. Keying
// suspension off spec.suspended and the reason — never off Ready=False alone —
// is what makes one rule cover both.
func observedKeyValueStateFor(kv *appv1alpha1.KeyValue) (ObservedDatastoreState, bool) {
	obs, ok := newObservedDatastoreState(DatastoreKindKeyValue, kv.Name, kv.Labels[LabelTenant],
		string(kv.Status.Phase), kv.Spec.Suspended)
	if !ok {
		return ObservedDatastoreState{}, false
	}
	return applyDatastoreReadyCondition(obs, kv.Status.Conditions, kv.Status.Phase == appv1alpha1.KVPhaseReady), true
}

func newObservedDatastoreState(kind, name, workspaceID, phase string, suspended bool) (ObservedDatastoreState, bool) {
	if name == "" || workspaceID == "" {
		return ObservedDatastoreState{}, false
	}
	return ObservedDatastoreState{
		DatastoreID: name,
		WorkspaceID: workspaceID,
		Kind:        kind,
		At:          time.Now().UTC(),
		Phase:       phase,
		Suspended:   suspended,
	}, true
}

// applyDatastoreReadyCondition maps the Ready condition onto the availability
// dimension. phaseReady is whether the CR's own phase agrees the datastore is
// serving — a Ready=True condition left over from before a phase regression
// must not be read as healthy.
func applyDatastoreReadyCondition(obs ObservedDatastoreState, conditions []metav1.Condition, phaseReady bool) ObservedDatastoreState {
	ready := findReadyCondition(conditions)
	// Suspension is intentional downtime, so it is observed as availability
	// EMPTY rather than unhealthy — the hibernated-App precedent. Checked
	// before the condition status because the two controllers disagree about
	// which status a suspended datastore reports.
	if obs.Suspended || (ready != nil && ready.Reason == appv1alpha1.ReasonSuspended) {
		obs.AvailabilityObserved = true
		return obs
	}
	if ready == nil {
		return obs
	}
	switch ready.Status {
	case metav1.ConditionTrue:
		if phaseReady && ready.Reason == datastoreReasonProvisioned {
			obs.Availability = "healthy"
			obs.AvailabilityObserved = true
			obs.ReadyTransitionAt = ready.LastTransitionTime.Time
		}
	case metav1.ConditionFalse:
		if datastoreAvailabilityUnobserved(ready.Reason) {
			break
		}
		obs.Availability = "unhealthy"
		obs.AvailabilityObserved = true
		obs.ReasonCode = EventReasonReadinessFailed
		obs.ReadyTransitionAt = ready.LastTransitionTime.Time
	}
	return obs
}

// datastoreAvailabilityUnobserved reports Ready=False reasons that say nothing
// about whether the datastore is serving.
//
// Provisioning is the load-bearing one, and it is deliberately NOT treated as
// an outage here even though it can mean the instance is gone: for managed
// Postgres, "being created" and "lost its only instance" are the same CR state
// (zero ready CNPG instances, Reason=Provisioning), so telling them apart is
// the checkpoint's job, not this function's — nextDatastoreAvailability latches
// unhealthy only once a healthy observation has armed the edge. Leaving
// Provisioning observable here is what lets a real outage be reported at all.
func datastoreAvailabilityUnobserved(reason string) bool {
	switch reason {
	case datastoreReasonMajorVersionUpgrade,
		datastoreReasonStorageShrinkRejected,
		datastoreReasonConnectionSecretRebuilding:
		return true
	}
	return false
}

func findReadyCondition(conditions []metav1.Condition) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == appv1alpha1.ConditionReady {
			return &conditions[i]
		}
	}
	return nil
}
