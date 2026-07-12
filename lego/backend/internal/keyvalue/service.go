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

// Package keyvalue is the managed key-value (Valkey / Redis-compatible) feature
// over the KeyValue CR, mirroring Render's /v1/key-value API. One Service the
// REST + GraphQL + MCP adapters share; the connection-info verb is the one place
// the connection strings (which embed the password) are surfaced — to an
// authenticated caller — read from the operator-generated Secret at request time.
//
// It is the datastore sibling of internal/postgres: the KeyValue's user-chosen
// name is its id (name-as-id), the same deliberate deviation managed databases
// take (docs/ADR020-identifiers.md § Known deviations) — a named CR with no separate
// opaque key. Keeping both datastore surfaces name-keyed is what makes them
// uniform for clients.
package keyvalue

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Service exposes managed key-value stores as Render's "key-value" shape.
type Service struct {
	*core.Base
	// Selections is the shared MCP per-session workspace selection
	// (w6/m2/t005): list_key_value_instances falls back to the caller's
	// selected workspace when its ownerId argument is omitted. Read-only
	// (key-value never selects a workspace). Nil => no fallback.
	Selections core.WorkspaceSelectionReader
}

// KeyValueView is the Render-shaped "key-value" object. Fields bex cannot back
// yet (Render's maxmemoryPolicy / persistenceMode / ipAllowList) are omitted
// rather than faked — the object stays a safe superset a Render client can read
// (docs/ADR018-render-parity.md § Key Value records the omissions).
type KeyValueView struct {
	ID        string `json:"id"` // the KeyValue name (name-as-id, postgres sibling)
	Name      string `json:"name"`
	Plan      string `json:"plan"`
	Version   string `json:"version,omitempty"`
	Status    string `json:"status"`    // Render keyValueStatus enum
	Suspended string `json:"suspended"` // Render string enum (like services/postgres)
	CreatedAt string `json:"createdAt,omitempty"`

	// bex-native extras (Render clients ignore unknown keys).
	ExternalHost string `json:"externalHost,omitempty"`
	Public       bool   `json:"public"`

	// OwnerID is Render's workspace-scoping field (w6/m4/t002), read from the
	// KeyValue CR's core.LabelTenant label — mirroring apps.AppView.OwnerID and
	// postgres.PostgresView.OwnerID. Populated for any KeyValue created via
	// CreateKeyValue with the store on; a hand-applied CR without the label
	// still reads as unowned.
	OwnerID string `json:"ownerId,omitempty"`
}

// KeyValueConnectionInfo mirrors Render's keyValueConnectionInfo schema: the
// internal string, an optional external (TLS) string, and a ready-to-run CLI
// command. The password is embedded inside the connection strings (Valkey's
// redis://:<password>@host form), never a standalone field — matching Render.
type KeyValueConnectionInfo struct {
	InternalConnectionString string `json:"internalConnectionString"`
	ExternalConnectionString string `json:"externalConnectionString,omitempty"`
	CLICommand               string `json:"cliCommand"`
}

// CreateKeyValueRequest is the POST /v1/key-value body (bex subset of Render's).
type CreateKeyValueRequest struct {
	Name      string `json:"name"`
	Plan      string `json:"plan,omitempty"`
	Version   string `json:"version,omitempty"`
	StorageGB int32  `json:"storageGB,omitempty"`
	Public    bool   `json:"public,omitempty"`
}

// kvStatus maps bex's KeyValue phase onto a Render-shaped keyValueStatus string.
func kvStatus(p appv1alpha1.KeyValuePhase) string {
	switch p {
	case appv1alpha1.KVPhaseReady:
		return "available"
	case appv1alpha1.KVPhaseFailed:
		return "unavailable"
	default:
		return "creating"
	}
}

func kvView(kv *appv1alpha1.KeyValue) KeyValueView {
	created := ""
	if !kv.CreationTimestamp.IsZero() {
		created = kv.CreationTimestamp.UTC().Format(time.RFC3339)
	}
	return KeyValueView{
		ID:           kv.Name,
		Name:         kv.Name,
		Plan:         kv.Spec.Plan,
		Version:      kv.Spec.Version,
		Status:       kvStatus(kv.Status.Phase),
		Suspended:    core.SuspendedEnum(kv.Spec.Suspended),
		CreatedAt:    created,
		ExternalHost: kv.Status.ExternalHost,
		Public:       kv.Spec.Public,
		OwnerID:      kv.Labels[core.LabelTenant],
	}
}

func (s *Service) fetchKeyValue(ctx context.Context, name string) (*appv1alpha1.KeyValue, error) {
	var kv appv1alpha1.KeyValue
	err := s.Client.Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: name}, &kv)
	if apierrors.IsNotFound(err) {
		return nil, core.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &kv, nil
}

// loadSecret resolves a KeyValue and its operator-generated credentials Secret
// (username/password/host/port/uri, and externalUri when public) — the path the
// connection-info verb reads. Returns core.ErrNotFound when the KeyValue or its
// Secret isn't provisioned yet.
func (s *Service) loadSecret(ctx context.Context, name string) (*appv1alpha1.KeyValue, *corev1.Secret, error) {
	kv, err := s.fetchKeyValue(ctx, name)
	if err != nil {
		return nil, nil, err
	}
	secretName := kv.Status.SecretName
	if secretName == "" {
		secretName = kv.Name // the operator names the Secret after the KeyValue
	}
	var sec corev1.Secret
	if err := s.Client.Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: secretName}, &sec); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil, core.ErrNotFound // not provisioned yet
		}
		return nil, nil, err
	}
	return kv, &sec, nil
}

// ListKeyValues returns every managed key-value store in the namespace,
// optionally narrowed to a single owning workspace — Render's `ownerId`
// list-filter contract (w6/m4/t002), mirroring postgres.Service.ListPostgres.
// ownerID == "" lists unscoped. A non-empty ownerID authorizes can_view on
// that exact workspace (an inaccessible ownerId is ErrForbidden) and then
// filters by core.LabelTenant; never silently returns unscoped data for a
// scoped request.
func (s *Service) ListKeyValues(ctx context.Context, ownerID string) ([]KeyValueView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	opts := []client.ListOption{client.InNamespace(s.Namespace)}
	if ownerID != "" {
		if err := s.AuthorizeOn(ctx, core.RelCanView, core.WorkspaceObject(ownerID)); err != nil {
			return nil, err
		}
		opts = append(opts, client.MatchingLabels{core.LabelTenant: ownerID})
	}
	var list appv1alpha1.KeyValueList
	if err := s.Client.List(ctx, &list, opts...); err != nil {
		return nil, err
	}
	out := make([]KeyValueView, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, kvView(&list.Items[i]))
	}
	return out, nil
}

// GetKeyValue returns one managed key-value store, or core.ErrNotFound.
func (s *Service) GetKeyValue(ctx context.Context, name string) (KeyValueView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return KeyValueView{}, err
	}
	kv, err := s.fetchKeyValue(ctx, name)
	if err != nil {
		return KeyValueView{}, err
	}
	return kvView(kv), nil
}

// CreateKeyValue provisions a managed key-value store (a KeyValue CR the operator
// projects to a single-instance Valkey StatefulSet + Service + Secret).
func (s *Service) CreateKeyValue(ctx context.Context, req CreateKeyValueRequest) (KeyValueView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return KeyValueView{}, err
	}
	if req.Name == "" {
		return KeyValueView{}, fmt.Errorf("%w: name is required", core.ErrBadRequest)
	}
	if req.Plan != "" {
		if _, ok := tiers.Valkey.ByID(req.Plan); !ok {
			return KeyValueView{}, fmt.Errorf("%w: unknown plan %q (valid: %v)", core.ErrBadRequest, req.Plan, tiers.Valkey.IDs())
		}
	}
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: req.Name, Namespace: s.Namespace},
		Spec: appv1alpha1.KeyValueSpec{
			Plan:      req.Plan,
			Version:   req.Version,
			StorageGB: req.StorageGB,
			Public:    req.Public,
		},
	}
	// Stamp both the tenant label (ownerId scoping — kvView/ListKeyValues read
	// this) and the workspace label (same-workspace NetworkPolicy selectors,
	// docs/ADR022-tenant-isolation.md — this is also what lets a tenant's own App
	// reach its own KeyValue instance), mirroring postgres.CreatePostgres.
	// Skip when the store is off (no resolver).
	if tenantID, ok := s.Tenant(ctx); ok {
		kv.Labels = map[string]string{core.LabelTenant: tenantID, core.LabelWorkspace: tenantID}
	}
	if err := s.Client.Create(ctx, kv); err != nil {
		return KeyValueView{}, err
	}
	return kvView(kv), nil
}

// DeleteKeyValue removes a managed key-value store (cascades the StatefulSet,
// PVC, Secret and any external route via owner refs).
func (s *Service) DeleteKeyValue(ctx context.Context, name string) error {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return err
	}
	kv, err := s.fetchKeyValue(ctx, name)
	if err != nil {
		return err
	}
	return s.Client.Delete(ctx, kv)
}

// Suspend scales a store to zero (data preserved on the PVC) — Render's KV
// suspend. Resume brings it back with the same password and endpoint.
func (s *Service) Suspend(ctx context.Context, name string) (KeyValueView, error) {
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return KeyValueView{}, err
	}
	return s.setSuspended(ctx, name, true)
}

// Resume restores a suspended store.
func (s *Service) Resume(ctx context.Context, name string) (KeyValueView, error) {
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return KeyValueView{}, err
	}
	return s.setSuspended(ctx, name, false)
}

// setSuspended flips spec.suspended and lets the operator converge (scale to /
// from zero). The spec is the single writer of intent — the same discipline the
// App lifecycle verbs follow.
func (s *Service) setSuspended(ctx context.Context, name string, suspended bool) (KeyValueView, error) {
	kv, err := s.fetchKeyValue(ctx, name)
	if err != nil {
		return KeyValueView{}, err
	}
	if kv.Spec.Suspended != suspended {
		kv.Spec.Suspended = suspended
		if err := s.Client.Update(ctx, kv); err != nil {
			return KeyValueView{}, err
		}
	}
	return kvView(kv), nil
}

// KeyValueConnectionInfo assembles the internal + external connection strings
// from the operator-generated Secret (the only place the connection strings, and
// so the password, are surfaced — to an authenticated caller).
func (s *Service) KeyValueConnectionInfo(ctx context.Context, name string) (KeyValueConnectionInfo, error) {
	if err := s.Authorize(ctx, core.RelCanViewSensitive); err != nil {
		return KeyValueConnectionInfo{}, err
	}
	_, sec, err := s.loadSecret(ctx, name)
	if err != nil {
		return KeyValueConnectionInfo{}, err
	}
	internal := string(sec.Data["uri"])         // redis://:<password>@<host>:6379
	external := string(sec.Data["externalUri"]) // rediss://:<password>@<host>:6379 (public only)

	// Render's cliCommand connects over the reachable endpoint — the external
	// (TLS) one when public, otherwise the internal one. redis-cli reads the URI
	// (rediss:// auto-negotiates TLS).
	conn := internal
	if external != "" {
		conn = external
	}
	return KeyValueConnectionInfo{
		InternalConnectionString: internal,
		ExternalConnectionString: external,
		CLICommand:               fmt.Sprintf("redis-cli -u %s", conn),
	}, nil
}
