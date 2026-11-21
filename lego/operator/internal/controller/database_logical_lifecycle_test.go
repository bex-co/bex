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
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Logical replication through failover and recovery (w34/m543).

func pgParameters(spec map[string]any) map[string]any {
	return spec["postgresql"].(map[string]any)["parameters"].(map[string]any)
}

// CNPG's logical slot synchronization and the two settings its webhook
// requires beside it are projected only for a publisher with a standby whose
// cluster runs 17 or later (DoD 1); every other database, an HA one still
// upgrading from 16 included, projects none of it, so it is unchanged.
func TestLogicalSlotSyncOnlyForHAPublishersOn17(t *testing.T) {
	publisher := &appv1alpha1.DatabaseLogicalReplication{PublisherRole: "cdc_reader", Schemas: []string{"public"}}
	for _, tc := range []struct {
		name             string
		ha               bool
		lr               *appv1alpha1.DatabaseLogicalReplication
		running, version string
		creating         bool
		want             bool
	}{
		{name: "HA publisher on 17", ha: true, lr: publisher, running: "17", want: true},
		{name: "HA publisher on 18", ha: true, lr: publisher, running: "18", version: "18", want: true},
		{name: "HA publisher created on 18", ha: true, lr: publisher, version: "18", creating: true, want: true},
		{name: "HA publisher on 16", ha: true, lr: publisher, running: "16", version: "16"},
		{name: "HA publisher upgrading from 16 to 17", ha: true, lr: publisher, running: "16", version: "17"},
		{name: "publisher without a standby", lr: publisher, running: "18"},
		{name: "HA subscriber only", ha: true, lr: &appv1alpha1.DatabaseLogicalReplication{SubscriberRole: "cdc_reader"}, running: "18"},
		{name: "HA, not enabled", ha: true, running: "18"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := publisherParams(tc.lr)
			p.highAvailability, p.runningVersion, p.version, p.creating = tc.ha, tc.running, tc.version, tc.creating
			// A legacy declared value of either setting, in any spelling,
			// gives way while the projection owns it.
			p.parameters = map[string]string{"Hot_Standby_Feedback": "off", "sync_replication_slots": "off", "work_mem": "8MB"}
			spec := cnpgClusterSpec(p)
			params := pgParameters(spec)
			slots, projected := spec["replicationSlots"]
			if !tc.want {
				if projected {
					t.Fatalf("replicationSlots = %v, want none", slots)
				}
				if params["Hot_Standby_Feedback"] != "off" || params["sync_replication_slots"] != "off" {
					t.Fatalf("a database the rule leaves alone lost its declared settings: %v", params)
				}
				return
			}
			want := map[string]any{"highAvailability": map[string]any{"synchronizeLogicalDecoding": true}}
			if !reflect.DeepEqual(slots, want) {
				t.Fatalf("replicationSlots = %v, want %v", slots, want)
			}
			if params[paramHotStandbyFeedback] != "on" || params[paramSyncReplicationSlots] != "on" {
				t.Fatalf("parameters = %v, want hot_standby_feedback and sync_replication_slots on", params)
			}
			if _, legacy := params["Hot_Standby_Feedback"]; legacy || params["work_mem"] != "8MB" {
				t.Fatalf("parameters = %v, want the legacy spelling dropped and the tenant's work_mem kept", params)
			}
		})
	}
}

// Turning the synchronization on merges into CNPG's defaulted stanza, and
// turning it off removes what bex projected, so CNPG's own defaults return
// (its webhook re-defaults the stanza) and the settings go.
func TestLogicalSlotSyncWithdraws(t *testing.T) {
	p := publisherParams(&appv1alpha1.DatabaseLogicalReplication{PublisherRole: "cdc_reader", Schemas: []string{"public"}})
	p.highAvailability, p.runningVersion = true, "18"
	cluster := &unstructured.Unstructured{Object: map[string]any{}}
	if err := projectUnstructuredSpec(cluster, cnpgClusterSpec(publisherParams(nil))); err != nil {
		t.Fatal(err)
	}
	// CNPG's webhook defaults the stanza on the Cluster bex created.
	if err := unstructured.SetNestedMap(cluster.Object, map[string]any{
		"highAvailability":    map[string]any{"enabled": true, "slotPrefix": "_cnpg_"},
		"synchronizeReplicas": map[string]any{"enabled": true},
		"updateInterval":      int64(30),
	}, "spec", "replicationSlots"); err != nil {
		t.Fatal(err)
	}
	if err := projectUnstructuredSpec(cluster, cnpgClusterSpec(p)); err != nil {
		t.Fatal(err)
	}
	ha, _, _ := unstructured.NestedMap(cluster.Object, "spec", "replicationSlots", "highAvailability")
	if !reflect.DeepEqual(ha, map[string]any{"enabled": true, "slotPrefix": "_cnpg_", "synchronizeLogicalDecoding": true}) {
		t.Fatalf("highAvailability = %v, want CNPG's defaults plus synchronizeLogicalDecoding", ha)
	}
	p.logicalReplication = nil
	if err := projectUnstructuredSpec(cluster, cnpgClusterSpec(p)); err != nil {
		t.Fatal(err)
	}
	if slots, found, _ := unstructured.NestedMap(cluster.Object, "spec", "replicationSlots"); found {
		t.Fatalf("replicationSlots after disabling = %v, want it withdrawn for CNPG to default", slots)
	}
	params, _, _ := unstructured.NestedMap(cluster.Object, "spec", "postgresql", "parameters")
	for _, name := range []string{paramHotStandbyFeedback, paramSyncReplicationSlots} {
		if _, ok := params[name]; ok {
			t.Errorf("%s still projected after disabling: %v", name, params)
		}
	}
}

// A recovered copy runs no logical replication worker, so the subscriptions
// its source's backup restored do not start (DoD 5): from its Cluster's
// creation, and for as long as the live Cluster carries the guard, until the
// copy is enabled as a subscriber. A copy recovered before the rule, whose
// live Cluster lacks it, is never restarted for it; a database that was not
// recovered never gets it.
func TestRecoveredCopyHoldsItsLogicalWorkers(t *testing.T) {
	recovered := &appv1alpha1.DatabaseRecovery{SourceDatabase: "dpg-source"}
	live := func(value string) *unstructured.Unstructured {
		params := map[string]any{"work_mem": "8MB"}
		if value != "" {
			params[paramMaxLogicalReplicationWorkers] = value
		}
		return &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"postgresql": map[string]any{"parameters": params}}}}
	}
	for _, tc := range []struct {
		name     string
		recovery *appv1alpha1.DatabaseRecovery
		lr       *appv1alpha1.DatabaseLogicalReplication
		live     *unstructured.Unstructured
		creating bool
		want     bool
	}{
		{name: "a copy being created", recovery: recovered, live: live(""), creating: true, want: true},
		{name: "a copy whose Cluster holds them", recovery: recovered, live: live("0"), want: true},
		{name: "a copy recovered before the rule", recovery: recovered, live: live("")},
		{name: "a copy enabled as a subscriber", recovery: recovered, lr: &appv1alpha1.DatabaseLogicalReplication{SubscriberRole: "orders_user"}, live: live("0")},
		{name: "a copy enabled with a subscription bex creates", recovery: recovered, lr: &appv1alpha1.DatabaseLogicalReplication{Subscriptions: []appv1alpha1.DatabaseLogicalSubscription{{Name: "s"}}}, live: live("0")},
		{name: "a copy enabled only as a publisher", recovery: recovered, lr: &appv1alpha1.DatabaseLogicalReplication{PublisherRole: "orders_user"}, live: live("0"), want: true},
		{name: "a database created empty", live: live(""), creating: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &appv1alpha1.Database{Spec: appv1alpha1.DatabaseSpec{Recovery: tc.recovery, LogicalReplication: tc.lr}}
			got := holdsRecoveredSubscriptions(db, tc.live, tc.creating)
			if got != tc.want {
				t.Fatalf("holdsRecoveredSubscriptions = %v, want %v", got, tc.want)
			}
			p := publisherParams(tc.lr)
			p.holdLogicalWorkers = got
			p.parameters = map[string]string{"Max_Logical_Replication_Workers": "8"}
			params := pgParameters(cnpgClusterSpec(p))
			value, projected := params[paramMaxLogicalReplicationWorkers]
			if _, legacy := params["Max_Logical_Replication_Workers"]; got != (projected && value == "0") || got == legacy {
				t.Fatalf("parameters = %v; guard %v", params, got)
			}
		})
	}
}
