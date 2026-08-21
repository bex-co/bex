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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Managed-datastore readiness, so a stuck one is observable (w7/036).
//
// A Database that never provisions is the platform's quietest failure mode: the
// CR sits in Provisioning forever, no pod is ever created, nothing logs an error
// after the first reconcile, and the tenant sees a resource that simply never
// arrives. Two production defects hid there in one day — an operator RBAC gap and
// a barman-plugin delegation gap — and neither was noticed by anything; both were
// found only because a rehearsal happened to create a Database by hand.
//
// Nothing else can see it either: CNPG's own metrics come from instance pods, so
// a Cluster that never got a pod exports nothing at all, and absence is exactly
// the state worth alerting on. Hence a gauge sourced from the CR's own status.
const (
	datastoreObserveFor = 5 * time.Second

	datastoreKindDatabase = "database"
	datastoreKindKeyValue = "keyvalue"
)

var (
	datastoreReadyDesc = prometheus.NewDesc(
		"bex_datastore_ready",
		"Managed datastore readiness: 1 when the CR reports its ready phase, 0 otherwise.",
		[]string{"kind", "namespace", "name", "phase"}, nil,
	)
	datastoreAgeDesc = prometheus.NewDesc(
		"bex_datastore_age_seconds",
		"Seconds since the managed datastore CR was created. Paired with bex_datastore_ready to alert on one that never provisions.",
		[]string{"kind", "namespace", "name"}, nil,
	)
	datastoreObserveErrorsDesc = prometheus.NewDesc(
		"bex_datastore_observe_errors_total",
		"Scrapes whose datastore listing failed. Non-zero means bex_datastore_ready is incomplete and must not be read as healthy.",
		nil, nil,
	)
)

type datastoreLister interface {
	List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
}

type datastoreCollector struct {
	list   datastoreLister
	errors prometheus.Counter
}

// NewDatastoreCollector reports readiness for every Database and KeyValue.
// Collection is scrape-time only, so it cannot block a reconcile.
func NewDatastoreCollector(list datastoreLister) prometheus.Collector {
	return &datastoreCollector{
		list: list,
		errors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "bex_datastore_observe_errors_total",
		}),
	}
}

func (c *datastoreCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- datastoreReadyDesc
	ch <- datastoreAgeDesc
	ch <- datastoreObserveErrorsDesc
}

func (c *datastoreCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), datastoreObserveFor)
	defer cancel()

	failed := false
	var databases appv1alpha1.DatabaseList
	if err := c.list.List(ctx, &databases); err != nil {
		failed = true
	} else {
		for i := range databases.Items {
			db := &databases.Items[i]
			c.emit(ch, datastoreKindDatabase, db.Namespace, db.Name,
				string(db.Status.Phase), db.Status.Phase == appv1alpha1.DBPhaseReady,
				db.CreationTimestamp.Time)
		}
	}

	var keyvalues appv1alpha1.KeyValueList
	if err := c.list.List(ctx, &keyvalues); err != nil {
		failed = true
	} else {
		for i := range keyvalues.Items {
			kv := &keyvalues.Items[i]
			c.emit(ch, datastoreKindKeyValue, kv.Namespace, kv.Name,
				string(kv.Status.Phase), kv.Status.Phase == appv1alpha1.KVPhaseReady,
				kv.CreationTimestamp.Time)
		}
	}

	if failed {
		c.errors.Inc()
	}
	ch <- c.errors
}

func (c *datastoreCollector) emit(
	ch chan<- prometheus.Metric,
	kind, namespace, name, phase string,
	ready bool,
	created time.Time,
) {
	value := float64(0)
	if ready {
		value = 1
	}
	// The phase rides as a label so an alert can say WHY, not merely "not ready".
	ch <- prometheus.MustNewConstMetric(datastoreReadyDesc, prometheus.GaugeValue, value,
		kind, namespace, name, phase)
	age := float64(0)
	if !created.IsZero() {
		age = time.Since(created).Seconds()
	}
	ch <- prometheus.MustNewConstMetric(datastoreAgeDesc, prometheus.GaugeValue, age,
		kind, namespace, name)
}

// RegisterDatastoreMetrics wires the datastore collector into the controller
// metrics registry.
func RegisterDatastoreMetrics(list datastoreLister) {
	ctrlmetrics.Registry.MustRegister(NewDatastoreCollector(list))
}
