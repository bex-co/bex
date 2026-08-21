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
// It is the datastore sibling of internal/postgres: a managed key-value store
// carries a stable, immutable red- resource id (metadata.name) and a separate
// required mutable display name (spec.name), the same identity split Postgres
// shipped in w9/m3.
package keyvalue

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Service exposes managed key-value stores as Render's "key-value" shape.
type Service struct {
	*core.Base
	Protection core.EnvironmentProtectionStore
	Owners     resourcemeta.OwnerResolver
	Metadata   resourcemeta.Config
	// Environments is the shared create-time assignment resolver used by all
	// three resource kinds.
	Environments core.EnvironmentResolver
	// KeyValueLogs is the production query seam for typed red- resources. The
	// compatibility adapters (REST/GraphQL/MCP) call QueryKeyValueLogs, which
	// delegates here; the logs feature wires it via api/server.go. nil with no
	// PodLogs source => ErrLogsUnavailable.
	KeyValueLogs KeyValueLogQuerySource
	// PodLogs backs the direct-pod fallback when KeyValueLogs is nil (no durable
	// store). nil with no KeyValueLogs source => ErrLogsUnavailable.
	PodLogs core.PodLogSource
}

// KeyValueView is the Render-shaped "key-value" object. maxmemoryPolicy /
// persistenceMode (Render's eviction + persistence settings) are now backed by
// KeyValue CR fields (w5/011); the object stays a safe superset a Render client
// can read (docs/ADR018-render-parity.md § Key Value).
type KeyValueView struct {
	ID        string `json:"id"` // the immutable red- id (metadata.name); stable across rename
	Name      string `json:"name"`
	Plan      string `json:"plan"`
	Version   string `json:"version,omitempty"`
	Status    string `json:"status"`    // Render keyValueStatus enum
	Suspended string `json:"suspended"` // Render string enum (like services/postgres)
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`

	// MaxmemoryPolicy / PersistenceMode mirror Render's Key Value settings, read
	// from the CR spec (empty until set — the operator applies its default then).
	MaxmemoryPolicy string `json:"maxmemoryPolicy,omitempty"`
	PersistenceMode string `json:"persistenceMode,omitempty"`

	// IPAllowList is the allowlist gating the EXTERNAL endpoint (Render's
	// ipAllowList, {cidrBlock, description} entries — descriptions persist on
	// the CR since w4/m24). Empty => the external route is open to all source IPs.
	IPAllowList []core.IPAllowListEntry `json:"ipAllowList,omitempty"`

	// bex-native extras (Render clients ignore unknown keys).
	ExternalHost string `json:"externalHost,omitempty"`
	Public       bool   `json:"public"`

	// OwnerID is Render's workspace-scoping field (w6/m4/t002), read from the
	// KeyValue CR's core.LabelTenant label — mirroring apps.AppView.OwnerID and
	// postgres.PostgresView.OwnerID. Populated for any KeyValue created via
	// CreateKeyValue with the store on; a hand-applied CR without the label
	// still reads as unowned.
	OwnerID string `json:"ownerId,omitempty"`

	// ProjectID is the owning Project's id (w1/m31 extension), read from the
	// KeyValue CR's core.LabelProject label. Empty means unassigned. Set via
	// SetProjectID; the projects feature is the only writer.
	ProjectID string `json:"projectId,omitempty"`

	// EnvironmentID is the owning Environment's id (w6/m20 extension), read
	// from the KeyValue CR's core.LabelEnvironment label. Empty means
	// unassigned. Set via SetEnvironmentID; the environments feature is the
	// only writer.
	EnvironmentID string `json:"environmentId,omitempty"`

	// Region / DashboardURL mirror the Render fields on services; populated by
	// the Service.view wrapper (not kvView) so they flow through GraphQL/MCP.
	Region       string `json:"region,omitempty"`
	DashboardURL string `json:"dashboardUrl,omitempty"`
}

// KeyValueConnectionInfo mirrors Render's keyValueConnectionInfo schema: the
// internal string, an optional external (TLS) string, and a ready-to-run CLI
// command. The password is embedded inside the connection strings (Valkey's
// redis://default:<password>@host form), never a standalone field — matching
// Render. The explicit "default" user, not the empty-username redis://:<password>@
// shorthand: verified live that valkey-cli 8.1.8's URI parser fails AUTH
// against the empty-username form on a --requirepass server.
type KeyValueConnectionInfo struct {
	InternalConnectionString string `json:"internalConnectionString"`
	ExternalConnectionString string `json:"externalConnectionString,omitempty"`
	CLICommand               string `json:"cliCommand"`
}

// CreateKeyValueRequest is the POST /v1/key-value body (bex subset of Render's).
type CreateKeyValueRequest struct {
	// OwnerID is the workspace to create IN — Render's `ownerId` (w6/m14). Empty
	// means the caller's default workspace; a workspace the caller is not a
	// member of is core.ErrForbidden, never a create in the wrong one. Bound to
	// the context by the verb, before its authorization check.
	OwnerID       string `json:"ownerId,omitempty"`
	EnvironmentID string `json:"environmentId,omitempty"`
	Name          string `json:"name"`
	Plan          string `json:"plan,omitempty"`
	Version       string `json:"version,omitempty"`
	// Region is accepted from Render clients as a placement hint. bex currently
	// exposes one configured region per control plane, so the server-owned
	// BEX_REGION remains authoritative instead of persisting this request field.
	Region    string `json:"region,omitempty"`
	StorageGB int32  `json:"storageGB,omitempty"`
	Public    bool   `json:"public,omitempty"`
	// IPAllowList optionally seeds the external-endpoint allowlist at create —
	// Render's {cidrBlock, description} entries.
	IPAllowList []core.IPAllowListEntry `json:"ipAllowList,omitempty"`
	// MaxmemoryPolicy / PersistenceMode are Render's eviction + persistence
	// settings. Empty => the CRD default (allkeys-lru / journal-snapshot).
	MaxmemoryPolicy string `json:"maxmemoryPolicy,omitempty"`
	PersistenceMode string `json:"persistenceMode,omitempty"`
	// DryRun, when true, resolves and returns the spec preview without any k8s
	// write — zero side effects (w2/m29). Validation still runs.
	DryRun bool `json:"dryRun,omitempty"`
}

// validMaxmemoryPolicies / validPersistenceModes mirror the KeyValue CRD's enum
// markers (types/v1alpha1/keyvalue_types.go). The CRD would reject a bad value
// at admission, but validating here first turns it into a clean 400 that names
// the offending value — the same courtesy CreateKeyValue extends to `plan`.
var validMaxmemoryPolicies = []string{
	"noeviction", "allkeys-lru", "allkeys-lfu", "volatile-lru",
	"volatile-lfu", "allkeys-random", "volatile-random", "volatile-ttl",
}

var validPersistenceModes = []string{"journal-snapshot", "snapshot", "off"}

// Render's wire format for these two enums is underscore-separated
// (maxmemoryPolicy: "allkeys_lru"; persistenceMode: "journal_snapshot" —
// verified against the render-oss/cli generated client's MaxmemoryPolicy/
// PersistenceMode constants), while the KeyValue CRD's markers (and the
// Valkey CLI flags the operator ultimately passes) are hyphenated. renderToCRD
// normalizes an incoming Render-shaped value before validation/storage;
// crdToRender converts a stored value back for a Render client to read. Both
// are no-ops on values that already use the other separator, so accepting a
// bare hyphenated value (bex's pre-w9 behavior) keeps working too.
func renderToCRD(s string) string { return strings.ReplaceAll(s, "_", "-") }
func crdToRender(s string) string { return strings.ReplaceAll(s, "-", "_") }

// maxmemoryPolicyKnown converts a Render-shaped (underscore) maxmemoryPolicy to
// the CRD's hyphenated form and reports whether it is a valid policy — the one
// check shared by CreateKeyValue and KeyValuePatch.validate so create and update
// can never accept different values.
func maxmemoryPolicyKnown(render string) bool {
	return slices.Contains(validMaxmemoryPolicies, renderToCRD(render))
}

// validateKeyValueName enforces the user-facing display-name shape (the CRD's
// spec.name markers). Shared by create and rename so the two paths can never
// accept different names — the same courtesy validateDatabaseName extends.
func validateKeyValueName(name string) error {
	if !appv1alpha1.ValidKeyValueName(name) {
		return fmt.Errorf("%w: name must use lowercase letters, digits, and hyphens, be at most 30 characters, and not start or end with a hyphen", core.ErrBadRequest)
	}
	return nil
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
	status := kvStatus(kv.Status.Phase)
	if !kv.DeletionTimestamp.IsZero() {
		status = "deleting"
	}
	return KeyValueView{
		ID:              kv.Name,
		Name:            kv.Spec.Name,
		Plan:            kv.Spec.Plan,
		Version:         kv.Spec.EffectiveVersion(),
		Status:          status,
		Suspended:       core.SuspendedEnum(kv.Spec.Suspended),
		CreatedAt:       created,
		UpdatedAt:       resourcemeta.UpdatedAt(kv),
		IPAllowList:     core.AllowListFromSpec(kv.Spec.IPAllowList),
		MaxmemoryPolicy: crdToRender(kv.Spec.MaxmemoryPolicy),
		PersistenceMode: crdToRender(kv.Spec.PersistenceMode),
		ExternalHost:    kv.Status.ExternalHost,
		Public:          kv.Spec.Public,
		OwnerID:         kv.Labels[core.LabelTenant],
		ProjectID:       kv.Labels[core.LabelProject],
		EnvironmentID:   kv.Labels[core.LabelEnvironment],
	}
}

// view wraps kvView and stamps the platform region and dashboard URL, matching
// the apps.Service.view pattern so GraphQL and MCP return the enriched shape.
func (s *Service) view(kv *appv1alpha1.KeyValue) KeyValueView {
	v := kvView(kv)
	v.Region = s.Metadata.PlatformRegion()
	v.DashboardURL = s.Metadata.DashboardURL(resourcemeta.KeyValueDashboardRoute, v.ID)
	return v
}

// fetchKeyValue resolves a KeyValue by name through the shared core.Base seam
// (w6/m17's core.Base.AuthorizeKeyValue: authorize + fetch in one call, against
// the KeyValue's OWN workspace — the same rule apps.AuthorizeApp applies) — kept
// as a thin wrapper so this package's many call sites don't all need to spell
// core.Base.AuthorizeKeyValue.
func (s *Service) fetchKeyValue(ctx context.Context, relation, name string) (*appv1alpha1.KeyValue, error) {
	return s.AuthorizeKeyValue(ctx, relation, name)
}

// loadSecret resolves a KeyValue and its operator-generated credentials Secret
// (username/password/host/port/uri, and externalUri when public) — the path the
// connection-info verb reads. Returns core.ErrNotFound when the KeyValue or its
// Secret isn't provisioned yet.
func (s *Service) loadSecret(ctx context.Context, relation, name string) (*appv1alpha1.KeyValue, *corev1.Secret, error) {
	kv, err := s.fetchKeyValue(ctx, relation, name)
	if err != nil {
		return nil, nil, err
	}
	secretName := kv.Status.CredentialSecretName
	if secretName == "" {
		secretName = kv.Status.SecretName
	}
	if secretName == "" {
		secretName = kv.Name // the operator names the Secret after the KeyValue
	}
	var sec corev1.Secret
	// The credential Secret is written by the reconciler beside its KeyValue, so
	// it lives in the CR's own namespace (ADR043 D8).
	if err := s.Client.Get(ctx, client.ObjectKey{Namespace: kv.Namespace, Name: secretName}, &sec); err != nil {
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
// ownerID == "" resolves to the caller's default workspace when the
// control-plane workspace resolver is enabled. A non-empty ownerID names the workspace to
// list (core.WithWorkspace), authorized+membership-checked by the same
// resolveWorkspace mechanism every other verb uses (w6/m17 — previously an
// OpenFGA-only check with no IsMember) and then filters by core.LabelTenant;
// never silently returns unscoped data for a scoped request.
func (s *Service) ListKeyValues(ctx context.Context, ownerID string) ([]KeyValueView, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	tenantID := ownerID
	if tenantID == "" && s.Workspace != nil {
		var ok bool
		tenantID, ok = s.Tenant(ctx)
		if !ok {
			return []KeyValueView{}, nil
		}
	}
	// Label-scoped and cluster-wide, mirroring postgres.ListPostgres — see
	// core.DatastoreListOptions for why the namespace can no longer carry the
	// tenant boundary here.
	opts := s.DatastoreListOptions(tenantID)
	var list appv1alpha1.KeyValueList
	if err := s.Client.List(ctx, &list, opts...); err != nil {
		return nil, err
	}
	out := make([]KeyValueView, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, s.view(&list.Items[i]))
	}
	return out, nil
}

// ensureKeyValueNameAvailable enforces Render's workspace-scoped display-name
// uniqueness without coupling identity to that name (mirrors postgres's
// ensureDatabaseNameAvailable). excludeID is the stable metadata.name of an
// object being renamed. Unlabelled dev objects form their own scope when
// the control-plane tenant resolver is disabled.
func (s *Service) ensureKeyValueNameAvailable(ctx context.Context, tenantID, name, excludeID string) error {
	var list appv1alpha1.KeyValueList
	if err := s.Client.List(ctx, &list, s.DatastoreListOptions(tenantID)...); err != nil {
		return fmt.Errorf("checking key-value name: %w", err)
	}
	for i := range list.Items {
		kv := &list.Items[i]
		if kv.Name == excludeID || kv.Labels[core.LabelTenant] != tenantID {
			continue
		}
		if kv.Spec.Name == name {
			return fmt.Errorf("%w: a key-value store named %q already exists in this workspace", core.ErrConflict, name)
		}
	}
	return nil
}

// GetKeyValue returns one managed key-value store, or core.ErrNotFound.
func (s *Service) GetKeyValue(ctx context.Context, name string) (KeyValueView, error) {
	kv, err := s.fetchKeyValue(ctx, core.RelCanView, name)
	if err != nil {
		return KeyValueView{}, err
	}
	return s.view(kv), nil
}

// CreateKeyValue provisions a managed key-value store (a KeyValue CR the operator
// projects to a single-instance Valkey StatefulSet + Service + Secret).
func (s *Service) CreateKeyValue(ctx context.Context, req CreateKeyValueRequest) (KeyValueView, error) {
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return KeyValueView{}, err
	}
	if err := validateKeyValueName(req.Name); err != nil {
		return KeyValueView{}, err
	}
	if req.Plan != "" {
		if _, ok := tiers.Valkey.ByID(req.Plan); !ok {
			return KeyValueView{}, fmt.Errorf("%w: unknown plan %q (valid: %v)", core.ErrBadRequest, req.Plan, tiers.Valkey.IDs())
		}
	}
	if err := core.ValidateAllowList(req.IPAllowList); err != nil {
		return KeyValueView{}, err
	}
	maxmemoryPolicy := renderToCRD(req.MaxmemoryPolicy)
	if req.MaxmemoryPolicy != "" && !maxmemoryPolicyKnown(req.MaxmemoryPolicy) {
		return KeyValueView{}, fmt.Errorf("%w: unknown maxmemoryPolicy %q (valid: %v)", core.ErrBadRequest, req.MaxmemoryPolicy, validMaxmemoryPolicies)
	}
	persistenceMode := renderToCRD(req.PersistenceMode)
	if persistenceMode != "" && !slices.Contains(validPersistenceModes, persistenceMode) {
		return KeyValueView{}, fmt.Errorf("%w: unknown persistenceMode %q (valid: %v)", core.ErrBadRequest, req.PersistenceMode, validPersistenceModes)
	}
	tenantID, tenantOK := s.Tenant(ctx)
	if err := s.ensureKeyValueNameAvailable(ctx, tenantID, req.Name, ""); err != nil {
		return KeyValueView{}, err
	}
	environment, err := core.ResolveEnvironmentForCreate(ctx, s.Environments, req.EnvironmentID, tenantID)
	if err != nil {
		return KeyValueView{}, err
	}
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: id.New(id.KeyValue), Namespace: s.TenantNamespace(tenantID)},
		Spec: appv1alpha1.KeyValueSpec{
			Name:            req.Name,
			Plan:            req.Plan,
			Version:         req.Version,
			StorageGB:       req.StorageGB,
			Public:          req.Public,
			IPAllowList:     core.AllowListToSpec(req.IPAllowList),
			MaxmemoryPolicy: maxmemoryPolicy,
			PersistenceMode: persistenceMode,
		},
	}
	// Stamp both the tenant label (ownerId scoping — kvView/ListKeyValues read
	// this) and the workspace label (same-workspace NetworkPolicy selectors,
	// docs/ADR022-tenant-isolation.md — this is also what lets a tenant's own App
	// reach its own KeyValue instance), mirroring postgres.CreatePostgres.
	// Skip when the store is off (no resolver).
	if tenantOK {
		kv.Labels = core.TenantLabels(tenantID)
	}
	// Newborn members inherit the environment's inbound-IP layer (w4/m28).
	kv.Spec.EnvironmentIPAllowList = core.ApplyGrouping(kv, environment)
	// Dry-run: return the resolved spec preview without any k8s write (w2/m29).
	if req.DryRun {
		return s.view(kv), nil
	}
	if err := s.RequirePlanBilling(ctx, tenantID, req.Plan); err != nil {
		return KeyValueView{}, err
	}
	resourcemeta.Touch(kv, s.Now())
	if err := s.Client.Create(ctx, kv); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return KeyValueView{}, fmt.Errorf("%w: generated key-value id collision; retry the request", core.ErrConflict)
		}
		// Plan cap, enforced by the namespace ResourceQuota — see the identical
		// mapping in postgres.CreatePostgres.
		if mapped, ok := core.QuotaCapError(err, store.KeyValuesQuotaCountKey, "key-value store"); ok {
			return KeyValueView{}, mapped
		}
		return KeyValueView{}, err
	}
	return s.view(kv), nil
}

// DeleteKeyValue removes a managed key-value store (cascades the StatefulSet,
// PVC, Secret and any external route via owner refs).
func (s *Service) DeleteKeyValue(ctx context.Context, name string) error {
	kv, err := s.fetchKeyValue(ctx, core.RelCanCreate, name)
	if err != nil {
		return err
	}
	if err := s.requireUnprotected(ctx, kv, "delete"); err != nil {
		return err
	}
	// codex round-9 #7: deletion is irreversible — reassert can_create uncached
	// immediately before the CR delete, so a revocation inside PositiveTTL
	// cannot ride a cached positive to destroy the store and its data.
	if err := s.AuthorizeKeyValueFresh(ctx, core.RelCanCreate, kv); err != nil {
		return err
	}
	return s.Client.Delete(ctx, kv)
}

// Suspend scales a store to zero (data preserved on the PVC) — Render's KV
// suspend. Resume brings it back with the same password and endpoint.
func (s *Service) Suspend(ctx context.Context, name string) (KeyValueView, error) {
	return s.setSuspended(ctx, name, true)
}

// Resume restores a suspended store.
func (s *Service) Resume(ctx context.Context, name string) (KeyValueView, error) {
	return s.setSuspended(ctx, name, false)
}

// setSuspended flips spec.suspended and lets the operator converge (scale to /
// from zero). The spec is the single writer of intent — the same discipline the
// App lifecycle verbs follow.
func (s *Service) setSuspended(ctx context.Context, name string, suspended bool) (KeyValueView, error) {
	kv, err := s.fetchKeyValue(ctx, core.RelCanOperate, name)
	if err != nil {
		return KeyValueView{}, err
	}
	if suspended {
		if err := s.requireUnprotected(ctx, kv, "suspend"); err != nil {
			return KeyValueView{}, err
		}
	} else if err := s.RequireBillingMutation(ctx, kv.Labels[core.LabelTenant]); err != nil {
		return KeyValueView{}, err
	}
	if kv.Spec.Suspended != suspended {
		kv.Spec.Suspended = suspended
		resourcemeta.Touch(kv, s.Now())
		if err := s.Client.Update(ctx, kv); err != nil {
			return KeyValueView{}, err
		}
	}
	return s.view(kv), nil
}

// SetProjectID assigns (or, with an empty projectID, clears) this KeyValue's
// project (w1/m31 extension) — the internal/projects feature's write path,
// mirroring postgres.Service.SetProjectID. Authorized the same as the other
// tenant-mutating verbs on a named KeyValue (RelCanCreate, matching DeleteKeyValue).
func (s *Service) SetProjectID(ctx context.Context, name, projectID string) error {
	kv, err := s.fetchKeyValue(ctx, core.RelCanCreate, name)
	if err != nil {
		return err
	}
	_, err = s.patchKeyValueObj(ctx, kv, func(kv *appv1alpha1.KeyValue) {
		if projectID == "" {
			delete(kv.Labels, core.LabelProject)
			return
		}
		if kv.Labels == nil {
			kv.Labels = map[string]string{}
		}
		kv.Labels[core.LabelProject] = projectID
	})
	return err
}

// SetEnvironmentIPAllowList projects (or, with nil, clears) the environment
// inbound-IP layer onto this KeyValue (w4/m28) — the internal/environments
// fan-out's write path, mirroring postgres.Service.SetEnvironmentIPAllowList.
// The store's OWN IPAllowList is never touched.
func (s *Service) SetEnvironmentIPAllowList(ctx context.Context, name string, cidrs []string) error {
	kv, err := s.fetchKeyValue(ctx, core.RelCanCreate, name)
	if err != nil {
		return err
	}
	if slices.Equal(kv.Spec.EnvironmentIPAllowList, cidrs) {
		return nil // unchanged layer: no write, no resourceVersion churn
	}
	_, err = s.patchKeyValueObj(ctx, kv, func(kv *appv1alpha1.KeyValue) {
		kv.Spec.EnvironmentIPAllowList = cidrs
	})
	return err
}

// SetEnvironmentID assigns (or, with an empty environmentID, clears) this
// KeyValue's environment (w6/m20 extension) — the internal/environments
// feature's write path, mirroring postgres.Service.SetEnvironmentID and
// SetProjectID above. Authorized the same as the other tenant-mutating verbs
// on a named KeyValue (RelCanCreate, matching DeleteKeyValue).
func (s *Service) SetEnvironmentID(ctx context.Context, name, environmentID string) error {
	kv, err := s.fetchKeyValue(ctx, core.RelCanCreate, name)
	if err != nil {
		return err
	}
	_, err = s.patchKeyValueObj(ctx, kv, func(kv *appv1alpha1.KeyValue) {
		if environmentID == "" {
			delete(kv.Labels, core.LabelEnvironment)
			return
		}
		if kv.Labels == nil {
			kv.Labels = map[string]string{}
		}
		kv.Labels[core.LabelEnvironment] = environmentID
	})
	return err
}

// GetIPAllowList returns the allowlist gating the external endpoint (empty
// => open to all source IPs). The internal path is never gated.
func (s *Service) GetIPAllowList(ctx context.Context, name string) ([]core.IPAllowListEntry, error) {
	kv, err := s.fetchKeyValue(ctx, core.RelCanView, name)
	if err != nil {
		return nil, err
	}
	return core.AllowListFromSpec(kv.Spec.IPAllowList), nil
}

// SetIPAllowList replaces the external-endpoint allowlist — full replace, so
// entries written without descriptions clear any stored ones. Every entry's
// CIDR must be valid (a bad one is a 400 before any write); an empty list opens
// the endpoint to all source IPs. The operator maps the CIDRs (never the
// descriptions) to a Traefik ipAllowList middleware on the SNI route — the
// same gate managed Postgres uses.
func (s *Service) SetIPAllowList(ctx context.Context, name string, entries []core.IPAllowListEntry) (KeyValueView, error) {
	kv, err := s.fetchKeyValue(ctx, core.RelCanOperate, name)
	if err != nil {
		return KeyValueView{}, err
	}
	if err := core.ValidateAllowList(entries); err != nil {
		return KeyValueView{}, err
	}
	kv.Spec.IPAllowList = core.AllowListToSpec(entries)
	resourcemeta.Touch(kv, s.Now())
	if err := s.Client.Update(ctx, kv); err != nil {
		return KeyValueView{}, err
	}
	return s.view(kv), nil
}

// SetPlan changes the managed key-value store's instance type (spec.plan).
// Unknown plans are rejected before any write (the caller maps core.ErrBadRequest
// to 400/a GraphQL error, listing the valid plans). A plan change resizes the
// Valkey StatefulSet's pod resources on the next operator reconcile.
func (s *Service) SetPlan(ctx context.Context, name, plan string) (KeyValueView, error) {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	kv, err := s.fetchKeyValue(ctx, core.RelCanOperate, name)
	if err != nil {
		return KeyValueView{}, err
	}
	if _, ok := tiers.Valkey.ByID(plan); !ok {
		return KeyValueView{}, fmt.Errorf("%w: plan must be one of %s", core.ErrBadRequest, strings.Join(tiers.Valkey.IDs(), "|"))
	}
	if err := s.RequirePlanBilling(ctx, kv.Labels[core.LabelTenant], plan); err != nil {
		return KeyValueView{}, err
	}
	from := kv.Spec.Plan
	view, err := s.patchKeyValueObj(ctx, kv, func(kv *appv1alpha1.KeyValue) {
		kv.Spec.Plan = plan
	})
	if err != nil {
		return KeyValueView{}, err
	}
	// Recorded even when from == plan, matching apps' SetPlan precedent: the
	// verb names the call the caller made and the equal pair shows nothing
	// changed — never the Update* verb of a call they didn't make (w10/m5).
	s.RecordKeyValuePlanChanged(ctx, kv, from, plan)
	return view, nil
}

// PreviewSetPlan returns what SetPlan would produce without writing — the same
// validation and in-memory spec update — zero side effects (w2/m29 dry-run).
// Requires can_view on the named key-value store (no audit event, no write).
func (s *Service) PreviewSetPlan(ctx context.Context, name, plan string) (KeyValueView, error) {
	kv, err := s.fetchKeyValue(ctx, core.RelCanView, name)
	if err != nil {
		return KeyValueView{}, err
	}
	if _, ok := tiers.Valkey.ByID(plan); !ok {
		return KeyValueView{}, fmt.Errorf("%w: plan must be one of %s", core.ErrBadRequest, strings.Join(tiers.Valkey.IDs(), "|"))
	}
	preview := kv.DeepCopy()
	preview.Spec.Plan = plan
	return s.view(preview), nil
}

// patchKeyValueObj applies mutate to an already-fetched KeyValue and merge-
// patches it — for callers (SetPlan) that must validate input BEFORE the
// write but AFTER authorizing+fetching, reusing the KeyValue fetchKeyValue
// already fetched rather than fetching (and authorizing, and auditing) again.
func (s *Service) patchKeyValueObj(ctx context.Context, kv *appv1alpha1.KeyValue, mutate func(kv *appv1alpha1.KeyValue)) (KeyValueView, error) {
	patch := client.MergeFrom(kv.DeepCopy())
	mutate(kv)
	resourcemeta.Touch(kv, s.Now())
	if err := s.Client.Patch(ctx, kv, patch); err != nil {
		return KeyValueView{}, err
	}
	return s.view(kv), nil
}

// KeyValuePatch is the mutable-field set for PATCH /v1/key-value/{id} — Render's
// "only the fields you pass are changed" semantics (nil = leave unchanged,
// mirroring the pointer fields on the CLI's generated KeyValuePATCHInput). name
// is the rename field this milestone (w9/m6) adds; plan keeps the pre-existing
// plan-change path (the GraphQL updateKeyValuePlan mutation; the MCP half
// folded into update_key_value at w1/m74
// verbs still call SetPlan directly). Extend this set as more KeyValue fields
// become updatable, exactly as PostgresPatch grew.
type KeyValuePatch struct {
	Name *string
	Plan *string
	// MaxmemoryPolicy is Render's eviction policy (w7/m45): nil = unchanged.
	// Render-shaped (underscore) or hyphenated values both accepted, like create.
	MaxmemoryPolicy *string
	// IPAllowList is the external-endpoint allowlist (w7/m45): nil = unchanged;
	// a non-nil empty slice CLEARS it (what `keyvalues update --clear-ip-allow-list`
	// sends). Mirrors PostgresPatch.IPAllowList; the same field the dedicated
	// PUT .../ip-allow-list route writes, so both entry points converge.
	IPAllowList *[]core.IPAllowListEntry
}

// validate checks every field present in the patch before any write; shared by
// UpdateKeyValue and PreviewUpdateKeyValue so the two paths can never accept
// different inputs (mirrors PostgresPatch.validate).
func (patch KeyValuePatch) validate() error {
	if patch.Name != nil {
		if err := validateKeyValueName(*patch.Name); err != nil {
			return err
		}
	}
	if patch.Plan != nil {
		if _, ok := tiers.Valkey.ByID(*patch.Plan); !ok {
			return fmt.Errorf("%w: plan must be one of %s", core.ErrBadRequest, strings.Join(tiers.Valkey.IDs(), "|"))
		}
	}
	if patch.MaxmemoryPolicy != nil && !maxmemoryPolicyKnown(*patch.MaxmemoryPolicy) {
		return fmt.Errorf("%w: unknown maxmemoryPolicy %q (valid: %v)", core.ErrBadRequest, *patch.MaxmemoryPolicy, validMaxmemoryPolicies)
	}
	if patch.IPAllowList != nil {
		if err := core.ValidateAllowList(*patch.IPAllowList); err != nil {
			return err
		}
	}
	return nil
}

func (patch KeyValuePatch) apply(kv *appv1alpha1.KeyValue) {
	if patch.Name != nil {
		kv.Spec.Name = *patch.Name
	}
	if patch.Plan != nil {
		kv.Spec.Plan = *patch.Plan
	}
	if patch.MaxmemoryPolicy != nil {
		kv.Spec.MaxmemoryPolicy = renderToCRD(*patch.MaxmemoryPolicy)
	}
	if patch.IPAllowList != nil {
		kv.Spec.IPAllowList = core.AllowListToSpec(*patch.IPAllowList)
	}
}

// UpdateKeyValue applies a partial update (Render's PATCH /key-value/{id}
// semantics — only fields set in patch change). SetPlan above stays the
// plan-only entry point GraphQL/MCP use; this is the general handler REST's
// PATCH route needs so `keyvalues update --name` (which sends no plan) stops
// 400ing, and the rename lands on the immutable red- id.
func (s *Service) UpdateKeyValue(ctx context.Context, name string, patch KeyValuePatch) (KeyValueView, error) {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	kv, err := s.fetchKeyValue(ctx, core.RelCanOperate, name)
	if err != nil {
		return KeyValueView{}, err
	}
	if err := patch.validate(); err != nil {
		return KeyValueView{}, err
	}
	if patch.Plan != nil {
		if err := s.RequirePlanBilling(ctx, kv.Labels[core.LabelTenant], *patch.Plan); err != nil {
			return KeyValueView{}, err
		}
	}
	if patch.Name != nil {
		if err := s.ensureKeyValueNameAvailable(ctx, kv.Labels[core.LabelTenant], *patch.Name, kv.Name); err != nil {
			return KeyValueView{}, err
		}
	}
	fromPlan := kv.Spec.Plan
	view, err := s.patchKeyValueObj(ctx, kv, patch.apply)
	if err != nil {
		return KeyValueView{}, err
	}
	if patch.Plan != nil && fromPlan != kv.Spec.Plan {
		s.RecordKeyValueEffect(ctx, kv, core.KeyValuePlanChanged)
	} else {
		s.RecordKeyValueEffect(ctx, kv, core.KeyValueUpdated)
	}
	return view, nil
}

// PreviewUpdateKeyValue is UpdateKeyValue's dry-run twin (w2/m29 pattern): same
// validation, zero side effects. Requires can_view (no audit event, no write).
func (s *Service) PreviewUpdateKeyValue(ctx context.Context, name string, patch KeyValuePatch) (KeyValueView, error) {
	kv, err := s.fetchKeyValue(ctx, core.RelCanView, name)
	if err != nil {
		return KeyValueView{}, err
	}
	if err := patch.validate(); err != nil {
		return KeyValueView{}, err
	}
	if patch.Name != nil {
		if err := s.ensureKeyValueNameAvailable(ctx, kv.Labels[core.LabelTenant], *patch.Name, kv.Name); err != nil {
			return KeyValueView{}, err
		}
	}
	preview := kv.DeepCopy()
	patch.apply(preview)
	return s.view(preview), nil
}

// KeyValueConnectionInfo assembles the internal + external connection strings
// from the operator-generated Secret (the only place the connection strings, and
// so the password, are surfaced — to an authenticated caller).
func (s *Service) KeyValueConnectionInfo(ctx context.Context, name string) (KeyValueConnectionInfo, error) {
	kv, sec, err := s.loadSecret(ctx, core.RelCanViewSensitive, name)
	if err != nil {
		return KeyValueConnectionInfo{}, err
	}
	// codex round-8 #8: the connection URIs embed the password — re-assert
	// can_view_sensitive uncached before assembling them, so a revocation inside
	// PositiveTTL cannot surface one last credential.
	if err := s.AuthorizeKeyValueFresh(ctx, core.RelCanViewSensitive, kv); err != nil {
		return KeyValueConnectionInfo{}, err
	}
	internal := string(sec.Data["uri"])         // legacy connection Secret
	external := string(sec.Data["externalUri"]) // legacy connection Secret (public only)
	if kv.Status.CredentialSecretName != "" {
		password := sec.Data["password"]
		if kv.Status.Phase != appv1alpha1.KVPhaseReady || kv.Status.CredentialRevision == "" ||
			kv.Status.CredentialRevision != appv1alpha1.KeyValueCredentialRevision(password) {
			return KeyValueConnectionInfo{}, fmt.Errorf("%w: key value credentials are still converging", core.ErrConflict)
		}
		port := kv.Status.Port
		if port == 0 {
			port = 6379
		}
		if errs := validation.IsDNS1123Subdomain(kv.Status.Host); len(errs) != 0 {
			return KeyValueConnectionInfo{}, errors.New("key value internal host is missing or invalid")
		}
		internal = keyValueURI("redis", string(password), kv.Status.Host, port)
		if kv.Status.ExternalHost != "" {
			external = keyValueURI("rediss", string(password), kv.Status.ExternalHost, port)
		}
	}

	// Render's cliCommand connects over the reachable endpoint — the external
	// (TLS) one when public, otherwise the internal one. redis-cli reads the URI
	// (rediss:// auto-negotiates TLS), but does not derive a TLS server name from
	// it, so the public SNI router requires the explicit operator-owned host.
	conn := internal
	cliCommand := fmt.Sprintf("redis-cli -u %s", conn)
	if external != "" {
		conn = external
		externalHost := kv.Status.ExternalHost
		if errs := validation.IsDNS1123Subdomain(externalHost); len(errs) != 0 {
			// Keep the credential-bearing URI out of this error. A public Secret
			// without matching operator status is an internal contract failure.
			return KeyValueConnectionInfo{}, errors.New("key value external host is missing or invalid")
		}
		cliCommand = fmt.Sprintf("redis-cli --sni %s -u %s", externalHost, conn)
	}
	return KeyValueConnectionInfo{
		InternalConnectionString: internal,
		ExternalConnectionString: external,
		CLICommand:               cliCommand,
	}, nil
}

func keyValueURI(scheme, password, host string, port int32) string {
	return (&url.URL{
		Scheme: scheme,
		User:   url.UserPassword("default", password),
		Host:   net.JoinHostPort(host, strconv.Itoa(int(port))),
	}).String()
}
