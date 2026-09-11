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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"github.com/jackc/pgx/v5"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const productInventoryInterval = 5 * time.Minute

const productInventoryCollectedSQL = `SELECT EXISTS(SELECT 1 FROM product_inventory_batches
    WHERE source=$1 AND complete AND bucket>=date_bin('5 minutes',$2::timestamptz,'2000-01-01'::timestamptz))`

// ProductInventoryCollector runs off the reconciliation/request paths. A source
// is published atomically only after its entire authoritative list succeeds.
type ProductInventoryCollector struct {
	Store    *PGStore
	Client   client.Client
	Identity string
}

func (c *ProductInventoryCollector) Run(ctx context.Context) {
	core.Poll(ctx, "product inventory", productInventoryInterval, c.Collect)
}

func (c *ProductInventoryCollector) Collect(ctx context.Context) error {
	var errs []error
	for _, source := range []string{"services", "postgres", "keyvalue", "domains"} {
		if err := c.collectSource(ctx, source); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", source, err))
		}
	}
	return errors.Join(errs...)
}

func (c *ProductInventoryCollector) collectSource(ctx context.Context, source string) error {
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	at := time.Now().UTC()
	var collected bool
	if err := c.Store.Pool.QueryRow(bounded, productInventoryCollectedSQL, source, at).Scan(&collected); err != nil {
		return err
	}
	if collected {
		return nil
	}
	var items []productInventoryResource
	var listErr error
	if source != "domains" {
		// Reserve time to persist the failure marker or DB-backed inventory.
		listCtx, listCancel := context.WithTimeout(bounded, 7*time.Second)
		items, listErr = c.listResources(listCtx, source)
		listCancel()
	}
	at = time.Now().UTC()
	if listErr != nil && source != "services" {
		_, err := c.Store.Pool.Exec(bounded, `INSERT INTO product_inventory_batches(source,bucket,observed_at,complete)
            VALUES ($1,date_bin('5 minutes',$2::timestamptz,'2000-01-01'::timestamptz),$2,false)
            ON CONFLICT(source,bucket) DO NOTHING`, source, at)
		return errors.Join(listErr, err)
	}
	// Postgres owns App existence even when Kubernetes cannot report readiness.
	if listErr != nil {
		items = nil
	}
	return errors.Join(listErr, c.Store.recordProductInventory(bounded, source, at, items))
}

// Only opaque IDs, closed state/type names and timestamps cross into analytics.
type productInventoryResource struct {
	ID        string    `json:"resource_id"`
	Workspace string    `json:"workspace_id"`
	Kind      string    `json:"resource_type"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
}

func (c *ProductInventoryCollector) listResources(ctx context.Context, source string) ([]productInventoryResource, error) {
	var newList func() client.ObjectList
	switch source {
	case "services":
		newList = func() client.ObjectList { return &appv1alpha1.AppList{} }
	case "postgres":
		newList = func() client.ObjectList { return &appv1alpha1.DatabaseList{} }
	case "keyvalue":
		newList = func() client.ObjectList { return &appv1alpha1.KeyValueList{} }
	default:
		return nil, fmt.Errorf("unknown inventory source %q", source)
	}
	identity := c.Identity
	if identity == "" {
		identity = DefaultControlPlaneIdentity
	}
	// Retain only analytics fields, not customer specs, while deduplicating
	// migration twins. The canonical workspace namespace wins.
	type candidate struct {
		resource  productInventoryResource
		canonical bool
	}
	resources := make(map[string]candidate)
	token := ""
	listed := 0
	for {
		// A fresh list also clears metadata omitted by the final page.
		list := newList()
		if err := c.Client.List(ctx, list, client.Limit(500), client.Continue(token)); err != nil {
			return nil, err
		}
		entries, err := meta.ExtractList(list)
		if err != nil {
			return nil, err
		}
		listed += len(entries)
		if listed > 100000 {
			return nil, fmt.Errorf("inventory list exceeds safety bound")
		}
		for _, entry := range entries {
			obj := entry.(client.Object)
			if obj.GetLabels()[LabelTenant] == "" {
				continue
			}
			key := obj.GetName()
			if source == "services" {
				if !ownedBy(obj.GetLabels(), identity) {
					continue
				}
				key = obj.GetLabels()[LabelAppID]
				if key == "" {
					continue
				}
			} else if owner := obj.GetLabels()[ControlPlaneLabel]; owner != "" && owner != identity {
				continue
			}
			canonical := core.DatastoreInOwnWorkspaceNamespace(obj)
			previous, exists := resources[key]
			if !exists || (canonical && !previous.canonical) {
				resources[key] = candidate{resource: projectProductInventoryResource(key, obj), canonical: canonical}
			}
		}
		token = list.GetContinue()
		if token == "" {
			break
		}
	}
	items := make([]productInventoryResource, 0, len(resources))
	for _, item := range resources {
		items = append(items, item.resource)
	}
	return items, nil
}

func projectProductInventoryResource(key string, obj client.Object) productInventoryResource {
	item := productInventoryResource{ID: key, Workspace: obj.GetLabels()[LabelTenant], CreatedAt: obj.GetCreationTimestamp().Time}
	var phase string
	var suspended bool
	var conditions []metav1.Condition
	switch v := obj.(type) {
	case *appv1alpha1.App:
		item.Kind = v.Spec.Type
		if item.Kind == "" {
			item.Kind = appv1alpha1.TypeWebService
		}
		phase, suspended, conditions = string(v.Status.Phase), v.Spec.Suspended, v.Status.Conditions
	case *appv1alpha1.Database:
		item.Kind = "postgres"
		phase, suspended, conditions = string(v.Status.Phase), v.Spec.Suspended, v.Status.Conditions
	case *appv1alpha1.KeyValue:
		item.Kind = "keyvalue"
		phase, suspended, conditions = string(v.Status.Phase), v.Spec.Suspended, v.Status.Conditions
	}
	item.State = productInventoryState(obj, phase, suspended, conditions)
	return item
}

func productInventoryState(obj client.Object, phase string, suspended bool, conditions []metav1.Condition) string {
	if obj.GetDeletionTimestamp() != nil {
		return "deleting"
	}
	if suspended || phase == string(appv1alpha1.PhaseHibernated) {
		return "suspended"
	}
	ready := meta.FindStatusCondition(conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.ObservedGeneration != obj.GetGeneration() {
		return "unknown"
	}
	switch phase {
	case string(appv1alpha1.PhaseCanceled):
		return "canceled"
	case string(appv1alpha1.PhaseFailed):
		return "failed"
	case string(appv1alpha1.PhasePending), string(appv1alpha1.PhaseBuilding), string(appv1alpha1.PhaseDeploying), string(appv1alpha1.DBPhaseProvisioning), string(appv1alpha1.DBPhaseUpgrading):
		return "provisioning"
	case string(appv1alpha1.PhaseRunning), string(appv1alpha1.DBPhaseReady):
		if ready.Status == metav1.ConditionTrue {
			return "ready"
		}
	}
	return "unknown"
}

func (s *PGStore) recordProductInventory(ctx context.Context, source string, at time.Time, items []productInventoryResource) error {
	if items == nil {
		items = []productInventoryResource{}
	}
	payload, err := json.Marshal(items)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		// Serialize replicas without waiting behind an unhealthy collector.
		var locked bool
		if err := tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(hashtext('product_inventory:'||$1))", source).Scan(&locked); err != nil {
			return err
		}
		if !locked {
			return nil
		}
		var newer bool
		if err := tx.QueryRow(ctx, productInventoryCollectedSQL, source, at).Scan(&newer); err != nil {
			return err
		}
		if newer {
			return nil
		}
		if err := populateProductInventorySample(ctx, tx, source, at, payload); err != nil {
			return err
		}
		bucket := at.Truncate(productInventoryInterval)
		if _, err := tx.Exec(ctx, `INSERT INTO product_inventory_batches(source,bucket,observed_at,complete)
            VALUES($1,$2,$3,true) ON CONFLICT(source,bucket) DO UPDATE SET observed_at=EXCLUDED.observed_at,complete=true`, source, bucket, at); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "DELETE FROM product_inventory_counts WHERE source=$1 AND bucket=$2", source, bucket); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO product_inventory_counts(source,bucket,workspace_id,resource_type,state,resources)
            SELECT $1,$2,workspace_id,resource_type,state,count(*) FROM product_sample GROUP BY workspace_id,resource_type,state`, source, bucket); err != nil {
			return err
		}
		if source == "domains" {
			return nil
		}
		if _, err := tx.Exec(ctx, `INSERT INTO product_inventory_lifecycle
            (resource_id,workspace_id,source,resource_type,created_at,first_seen_at,last_seen_at,first_ready_at,state)
            SELECT resource_id,workspace_id,$1,resource_type,created_at,$2,$2,CASE WHEN state='ready' THEN $2::timestamptz END,state FROM product_sample
            ON CONFLICT(resource_id) DO UPDATE SET last_seen_at=EXCLUDED.last_seen_at,state=EXCLUDED.state,
                removed_at=NULL,first_ready_at=COALESCE(product_inventory_lifecycle.first_ready_at,EXCLUDED.first_ready_at)`, source, at); err != nil {
			return err
		}
		// Absence proves removal only after a COMPLETE authoritative list.
		if _, err := tx.Exec(ctx, `UPDATE product_inventory_lifecycle SET removed_at=$2
            WHERE source=$1 AND removed_at IS NULL AND last_seen_at<$2
              AND NOT EXISTS(SELECT 1 FROM product_sample p WHERE p.resource_id=product_inventory_lifecycle.resource_id)`, source, at); err != nil {
			return err
		}
		if source != "services" {
			// Batch datastore hosting off the reconciliation path; one source
			// budget covers every resource, even when analytics is unhealthy.
			if _, err := tx.Exec(ctx, `INSERT INTO product_hosting_daily
				(workspace_id,resource_id,resource_type,day,observed_at,live,was_live)
				SELECT workspace_id,resource_id,resource_type,($1::timestamptz AT TIME ZONE 'UTC')::date,
					$1,state='ready',state='ready' FROM product_sample WHERE true
				`+productHostingConflictSQL, at); err != nil {
				return err
			}
			// Datastore absence is a sampled removal, not an API delete timestamp.
			_, err := tx.Exec(ctx, `INSERT INTO product_activity_events(source_key,workspace_id,resource_id,resource_type,event_type,at,actor_type)
                SELECT 'deleted:'||resource_id,workspace_id,resource_id,resource_type,'deleted',removed_at,'system'
                FROM product_inventory_lifecycle WHERE source=$1 AND removed_at=$2
                ON CONFLICT DO NOTHING`, source, at)
			return err
		}
		return nil
	})
}

func populateProductInventorySample(ctx context.Context, tx pgx.Tx, source string, at time.Time, payload []byte) error {
	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE product_sample(
            resource_id text PRIMARY KEY,workspace_id text,resource_type text,state text,created_at timestamptz) ON COMMIT DROP`); err != nil {
		return err
	}
	input := `SELECT * FROM jsonb_to_recordset($1::jsonb) AS x(resource_id text,workspace_id text,resource_type text,state text,created_at timestamptz)`
	var err error
	switch source {
	case "services":
		_, err = tx.Exec(ctx, `INSERT INTO product_sample
                SELECT a.id,a.tenant_id,COALESCE(NULLIF(a.type,''),'web_service'),
                    CASE WHEN x.state='deleting' THEN 'deleting' WHEN a.suspended THEN 'suspended' ELSE COALESCE(x.state,'unknown') END,a.created_at
                FROM apps a LEFT JOIN (`+input+`) x ON x.resource_id=a.id AND x.workspace_id=a.tenant_id`, payload)
	case "postgres", "keyvalue":
		_, err = tx.Exec(ctx, `INSERT INTO product_sample SELECT x.* FROM (`+input+`) x
                JOIN tenants t ON t.id=x.workspace_id WHERE x.resource_type=$2`, payload, source)
	case "domains":
		_, err = tx.Exec(ctx, `INSERT INTO product_sample
                SELECT d.id,a.tenant_id,COALESCE(NULLIF(a.type,''),'web_service'),
                    CASE WHEN d.claim_state<>'verified' THEN 'unverified'
                         WHEN o.observed_at IS NULL OR o.observed_at<$1::timestamptz-interval '30 minutes' THEN 'verified_tls_unknown'
                         WHEN o.tls_ready THEN 'verified_tls_ready' ELSE 'verified_tls_pending' END,d.created_at
                FROM domains d JOIN apps a ON a.id=d.app_id
                LEFT JOIN product_domain_observations o ON o.domain_id=d.id
                WHERE COALESCE(d.redirect_for_name,'')=''`, at)
	default:
		return fmt.Errorf("unknown inventory source %q", source)
	}
	return err
}
