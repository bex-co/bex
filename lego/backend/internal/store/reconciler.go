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
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Labels stamped on every projected App CR. LabelAppID ties the CR back to
// its apps row (the reconciler's join key); LabelTenant carries the owning
// tenant's id (tea-<id>) — core.Base.GetApp's single source of truth for
// gating a fetched App against the caller's tenant, so every feature that
// fetches by name (apps/logs/metrics/secrets) inherits the same check; the
// managed-by label scopes list/delete so hand-applied Apps are never touched.
//
// ManagedByValue is PERSISTED DATA — it lives on every projected App CR in
// live clusters. The service was renamed bex-backend, but changing this value
// would blind the projection to existing CRs (List filters on it) and make it
// re-Create them into name conflicts. Flip it only with a relabel migration.
// LabelTenant is re-stamped on every resync (stampLabels), so a value change
// (it once carried the tenant name) migrates existing CRs without a relabel.
const (
	LabelManagedBy = "app.kubernetes.io/managed-by"
	ManagedByValue = "bex-controlplane"
	LabelAppID     = "bex.co/app-id"
	// LabelTenant aliases core.LabelTenant — one label, one constant, so the
	// stamp (here) and the gate (core.Base.GetApp) can never drift apart.
	LabelTenant = core.LabelTenant
	// LabelWorkspace aliases core.LabelWorkspace so the stamp (here) and the
	// operator's propagation to pod templates share one canonical value.
	LabelWorkspace      = core.LabelWorkspace
	defaultResyncPeriod = 30 * time.Second
)

// Reconciler projects the source of truth into the cluster: each apps row
// (+ its domains) becomes an App CR; rows deleted from Postgres get their CR
// deleted; the CR's observed status (phase, url) is written back to the row.
// It is level-triggered — a full resync every Resync plus a Kick after API
// writes — so etcd stays a rebuildable projection of Postgres.
type Reconciler struct {
	Client    client.Client
	Store     Store
	Namespace string        // namespace the App CRs are projected into
	Resync    time.Duration // full-resync interval

	kick chan struct{}
}

func NewReconciler(cl client.Client, store Store, namespace string) *Reconciler {
	return &Reconciler{
		Client:    cl,
		Store:     store,
		Namespace: namespace,
		Resync:    defaultResyncPeriod,
		kick:      make(chan struct{}, 1),
	}
}

// Kick schedules an immediate reconcile (non-blocking, coalescing). The API
// calls it after every successful write so a POST is projected within
// milliseconds instead of a resync period.
func (r *Reconciler) Kick() {
	select {
	case r.kick <- struct{}{}:
	default:
	}
}

// Run reconciles until ctx is done.
func (r *Reconciler) Run(ctx context.Context) {
	ticker := time.NewTicker(r.Resync)
	defer ticker.Stop()
	for {
		if err := r.ReconcileOnce(ctx); err != nil && ctx.Err() == nil {
			log.Printf("controlplane: reconcile: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-r.kick:
		}
	}
}

// ReconcileOnce drives one full pass: desired rows vs. existing managed CRs →
// create / update / delete, then copies each CR's status back to its row.
// Per-app failures are collected, not fatal — one bad row can't block the rest.
func (r *Reconciler) ReconcileOnce(ctx context.Context) error {
	desired, err := r.Store.ListDesiredApps(ctx)
	if err != nil {
		return fmt.Errorf("list desired apps: %w", err)
	}
	var existing appv1alpha1.AppList
	if err := r.Client.List(ctx, &existing,
		client.InNamespace(r.Namespace),
		client.MatchingLabels{LabelManagedBy: ManagedByValue}); err != nil {
		return fmt.Errorf("list App CRs: %w", err)
	}
	byID := make(map[string]*appv1alpha1.App, len(existing.Items))
	for i := range existing.Items {
		if id := existing.Items[i].Labels[LabelAppID]; id != "" {
			byID[id] = &existing.Items[i]
		}
	}

	var errs []error
	seen := make(map[string]bool, len(desired))
	for _, d := range desired {
		seen[d.ID] = true
		cur, ok := byID[d.ID]
		if !ok {
			if err := r.Client.Create(ctx, projectApp(d, r.Namespace)); err != nil {
				errs = append(errs, fmt.Errorf("create App %s/%s: %w", d.TenantName, d.Name, err))
			}
			continue
		}
		// Project desired spec only; observed status (phase/url) is read from
		// the CR by bex-api, never copied back into Postgres. Labels are
		// re-stamped so a LabelTenant value change migrates without a relabel.
		specChanged := applyOwnedSpec(&cur.Spec, projectSpec(d))
		labelsChanged := stampLabels(cur, d)
		if specChanged || labelsChanged {
			if err := r.Client.Update(ctx, cur); err != nil {
				errs = append(errs, fmt.Errorf("update App %s: %w", cur.Name, err))
				continue
			}
		}
	}
	// Rows deleted from Postgres → delete their projected CR.
	for id, cur := range byID {
		if seen[id] {
			continue
		}
		if err := r.Client.Delete(ctx, cur); err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("delete App %s: %w", cur.Name, err))
		}
	}
	return errors.Join(errs...)
}

// CRName is the projected CR's name, "<tenant>-<app>". Both parts are
// API-validated DNS labels of ≤30 chars, so the result always fits the
// 63-char object-name limit.
func CRName(tenant, app string) string { return tenant + "-" + app }

// stampLabels ensures cur carries the projection's three labels (managed-by,
// app-id, tenant-id), returning whether any changed. Called on both Create and
// Update so a LabelTenant value change (name → id) converges on the next resync.
func stampLabels(cur *appv1alpha1.App, d DesiredApp) bool {
	if cur.Labels == nil {
		cur.Labels = map[string]string{}
	}
	changed := false
	set := func(k, v string) {
		if cur.Labels[k] != v {
			cur.Labels[k] = v
			changed = true
		}
	}
	set(LabelManagedBy, ManagedByValue)
	set(LabelAppID, d.ID)
	set(LabelTenant, d.TenantID)    // the tenant id (tea-<id>), what List/Get filter on
	set(LabelWorkspace, d.TenantID) // workspace identity for NetworkPolicy selectors (t002)
	return changed
}

func projectApp(d DesiredApp, namespace string) *appv1alpha1.App {
	a := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CRName(d.TenantName, d.Name),
			Namespace: namespace,
		},
		Spec: projectSpec(d),
	}
	stampLabels(a, d)
	return a
}

// projectSpec maps a row to the App.spec fields the control plane owns.
// Expose is always set so the operator serves the app at the platform
// hostname (<name>.<BEX_BASE_DOMAIN>) even before a custom domain exists.
func projectSpec(d DesiredApp) appv1alpha1.AppSpec {
	s := appv1alpha1.AppSpec{
		Repo:           d.Repo,
		Image:          d.Image,
		Branch:         d.Branch,
		Replicas:       d.Replicas,
		Port:           d.Port,
		Tier:           d.Tier,
		IdleTTLSeconds: d.IdleTTLSeconds,
		Suspended:      d.Suspended,
		Expose:         true,
	}
	if len(d.Hosts) > 0 {
		s.Host = d.Hosts[0]
		s.Hosts = slices.Clone(d.Hosts[1:])
	}
	return s
}

// applyOwnedSpec copies the control-plane-owned fields of want onto dst and
// reports whether anything changed. Fields it doesn't own (Builder,
// HealthCheckPath, AutoDeploy, RestartedAt, …) are left untouched so
// bex-api's lifecycle verbs and server-side defaulting survive a resync.
func applyOwnedSpec(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) bool {
	changed := false
	set := func(cur *string, want string) {
		if *cur != want {
			*cur = want
			changed = true
		}
	}
	set(&dst.Repo, want.Repo)
	set(&dst.Image, want.Image)
	set(&dst.Branch, want.Branch)
	set(&dst.Tier, want.Tier)
	set(&dst.Host, want.Host)
	if dst.Replicas != want.Replicas {
		dst.Replicas, changed = want.Replicas, true
	}
	if dst.Port != want.Port {
		dst.Port, changed = want.Port, true
	}
	if dst.IdleTTLSeconds != want.IdleTTLSeconds {
		dst.IdleTTLSeconds, changed = want.IdleTTLSeconds, true
	}
	if dst.Suspended != want.Suspended {
		dst.Suspended, changed = want.Suspended, true
	}
	if dst.Expose != want.Expose {
		dst.Expose, changed = want.Expose, true
	}
	if !slices.Equal(dst.Hosts, want.Hosts) {
		dst.Hosts, changed = slices.Clone(want.Hosts), true
	}
	return changed
}
