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
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// RetiredUsageResource is a deleted resource the usage meters still owe hours
// for, through the hour it was deleted in (w4/m173; ADR023). Apps are retired
// by a trigger on their row's deletion (migration 0151); datastores, which
// have no row, by their delete verbs through UsageRetirementRecorder.
type RetiredUsageResource struct {
	ResourceKind string
	ResourceID   string
	WorkspaceID  string
	// Name is an App's service name (its pod identity derives from it) or a
	// datastore's CR name; Namespace, Display and Public are datastores' only.
	Name      string
	Namespace string
	Display   string
	Public    bool
	Tier      string
	CreatedAt time.Time
	RetiredAt time.Time
	// Egress is a deleted App's last resolved egress identity, nil when its
	// live egress meter never resolved one (w6/m333).
	Egress *EgressIdentity
}

// EgressIdentity is what the egress meter needs to read a service's bandwidth
// without its App CR: the Traefik router names its Ingresses carried, which
// key the HTTP counter, and whether it has direct (non-static-site) egress,
// whose counters are keyed by its id (w6/m333; ADR023).
type EgressIdentity struct {
	Routers []string
	Direct  bool
}

// RecordAppEgressIdentity keeps the egress identity the live meter resolved
// for an App, so its final hours can be metered after it is deleted. An
// unchanged identity writes nothing.
func (s *PGStore) RecordAppEgressIdentity(ctx context.Context, appID string, identity EgressIdentity) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO usage_egress_identities (service_id, routers, direct) VALUES ($1, $2, $3)
		 ON CONFLICT (service_id) DO UPDATE SET routers = EXCLUDED.routers, direct = EXCLUDED.direct, recorded_at = CURRENT_TIMESTAMP
		 WHERE usage_egress_identities.routers IS DISTINCT FROM EXCLUDED.routers
		    OR usage_egress_identities.direct IS DISTINCT FROM EXCLUDED.direct`,
		// Known with no routers stays distinct from never resolved (no row).
		appID, nonNil(identity.Routers), identity.Direct)
	return err
}

// UsageRetirementRecorder records a datastore's retirement before its CR is
// deleted.
type UsageRetirementRecorder interface {
	RecordRetiredUsageResource(ctx context.Context, r RetiredUsageResource) error
}

// RetiredDatastoreFor is what metering needs from a Database or KeyValue CR
// about to be deleted; ok is false for anything else, or a CR no workspace
// owns (which the meters never listed).
func RetiredDatastoreFor(obj client.Object) (r RetiredUsageResource, ok bool) {
	switch o := obj.(type) {
	case *appv1alpha1.Database:
		r = RetiredUsageResource{ResourceKind: ResourceKindPostgres, Display: o.Spec.Name, Public: o.Spec.Public, Tier: o.Spec.Plan}
	case *appv1alpha1.KeyValue:
		r = RetiredUsageResource{ResourceKind: ResourceKindKeyValue, Display: o.Spec.Name, Public: o.Spec.Public, Tier: o.Spec.Plan}
	default:
		return RetiredUsageResource{}, false
	}
	r.WorkspaceID = obj.GetLabels()[core.LabelTenant]
	r.ResourceID, r.Name, r.Namespace = obj.GetName(), obj.GetName(), obj.GetNamespace()
	r.CreatedAt = obj.GetCreationTimestamp().UTC()
	return r, r.WorkspaceID != ""
}

// RecordRetiredUsageResource stores r, retired now. Retiring the same resource
// again (a delete retried after one that failed) replaces the record.
func (s *PGStore) RecordRetiredUsageResource(ctx context.Context, r RetiredUsageResource) error {
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO usage_retired_resources (resource_kind, resource_id, workspace_id, name, namespace, display_name, public, tier, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (resource_kind, resource_id) DO UPDATE SET
		   workspace_id = EXCLUDED.workspace_id, name = EXCLUDED.name, namespace = EXCLUDED.namespace,
		   display_name = EXCLUDED.display_name, public = EXCLUDED.public, tier = EXCLUDED.tier,
		   created_at = EXCLUDED.created_at, retired_at = EXCLUDED.retired_at`,
		NormalizeResourceKind(r.ResourceKind), r.ResourceID, r.WorkspaceID, r.Name, r.Namespace, r.Display, r.Public, r.Tier, r.CreatedAt.UTC())
	return err
}

// ListRetiredUsageResources returns the resources retired at or after since
// whose workspace still exists.
func (s *PGStore) ListRetiredUsageResources(ctx context.Context, since time.Time) ([]RetiredUsageResource, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT r.resource_kind, r.resource_id, r.workspace_id, r.name, r.namespace, r.display_name, r.public, r.tier, r.created_at, r.retired_at,
		        i.routers, i.direct
		 FROM usage_retired_resources AS r
		 LEFT JOIN usage_egress_identities AS i ON r.resource_kind = 'service' AND i.service_id = r.resource_id
		 WHERE r.retired_at >= $1 AND EXISTS (SELECT 1 FROM tenants AS t WHERE t.id = r.workspace_id)
		 ORDER BY r.retired_at, r.resource_kind, r.resource_id`, since.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RetiredUsageResource
	for rows.Next() {
		var r RetiredUsageResource
		var routers []string
		var direct *bool
		if err := rows.Scan(&r.ResourceKind, &r.ResourceID, &r.WorkspaceID, &r.Name, &r.Namespace, &r.Display, &r.Public, &r.Tier, &r.CreatedAt, &r.RetiredAt,
			&routers, &direct); err != nil {
			return nil, err
		}
		if direct != nil {
			r.Egress = &EgressIdentity{Routers: routers, Direct: *direct}
		}
		r.CreatedAt, r.RetiredAt = r.CreatedAt.UTC(), r.RetiredAt.UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

// PruneRetiredUsageResources drops records retired before before — their
// hours are past the meters' catch-up window — and records whose workspace no
// longer exists, which has no usage left to write.
//
// An egress identity goes with the last record that needs it: once its App is
// neither live nor retired, nothing will meter it again.
func (s *PGStore) PruneRetiredUsageResources(ctx context.Context, before time.Time) (int64, error) {
	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM usage_retired_resources AS r
		 WHERE r.retired_at < $1 OR NOT EXISTS (SELECT 1 FROM tenants AS t WHERE t.id = r.workspace_id)`,
		before.UTC())
	if err != nil {
		return 0, err
	}
	if _, err := s.Pool.Exec(ctx,
		`DELETE FROM usage_egress_identities AS i
		 WHERE NOT EXISTS (SELECT 1 FROM apps AS a WHERE a.id = i.service_id)
		   AND NOT EXISTS (SELECT 1 FROM usage_retired_resources AS r WHERE r.resource_kind = 'service' AND r.resource_id = i.service_id)`); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
