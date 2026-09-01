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

// recovery.go is the managed-Postgres data-protection surface: recovery info
// (the PITR window + backup list), recover-to-a-new-instance (Render's recover —
// always a NEW Database, never in place), and on-demand exports. Backups are the
// operator's doing (the Barman Cloud WAL-archiver plugin plus plugin-method
// ScheduledBackups write to object storage); this surface reads their status and
// drives recovery/exports, all keyed off the Database's Status.BackupsEnabled
// signal so a no-backup plan degrades gracefully rather than erroring.
package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// CNPG types the recovery surface reads/writes via unstructured (bex-api, like
// the operator, does not vendor CNPG's Go API).
var (
	cnpgClusterGVK = schema.GroupVersionKind{Group: "postgresql.cnpg.io", Version: "v1", Kind: "Cluster"}
	cnpgBackupGVK  = schema.GroupVersionKind{Group: "postgresql.cnpg.io", Version: "v1", Kind: "Backup"}
)

const (
	// labelCNPGCluster is CNPG's intrinsic label on every Backup it manages
	// (scheduled and on-demand) — the reliable Backup→Database link, so
	// recovery info can list the automatic base backups too, not just exports.
	labelCNPGCluster = "cnpg.io/cluster"
	// ExportURLTTL is the maximum life of a freshly minted download URL. It is
	// deliberately much shorter than the seven-day artifact-retention window.
	ExportURLTTL = 15 * time.Minute
)

// RecoveryInfoView mirrors Render's postgres recovery info: whether recovery is
// available and the point-in-time window, plus the visible backup list.
type RecoveryInfoView struct {
	// Enabled is false for a no-backup plan (Free) — recovery isn't an error
	// there, just unavailable, so a client can render a disabled state.
	Enabled bool `json:"enabled"`
	// EarliestRecoveryTime / LatestRecoveryTime bound the restorable window
	// (RFC3339). Earliest is CNPG's firstRecoverabilityPoint; latest tracks the
	// continuous WAL stream (≈ now).
	EarliestRecoveryTime string `json:"earliestRecoveryTime,omitempty"`
	LatestRecoveryTime   string `json:"latestRecoveryTime,omitempty"`
	// Backups is the visible backup history (base backups in object storage).
	Backups []BackupView `json:"backups"`
}

// BackupView is one physical base backup in object storage.
type BackupView struct {
	ID        string `json:"id"`
	Status    string `json:"status"` // pending|running|completed|failed
	CreatedAt string `json:"createdAt,omitempty"`
}

// ExportView is Render's logical-export object plus honest lifecycle fields.
// Render requires id/createdAt and returns url when the artifact is available;
// the additional fields are a safe superset shared by GraphQL and MCP.
type ExportView struct {
	ID            string `json:"id"`
	CreatedAt     string `json:"createdAt"`
	Status        string `json:"status"`
	URL           string `json:"url,omitempty"`
	URLExpiresAt  string `json:"urlExpiresAt,omitempty"`
	ExpiresAt     string `json:"expiresAt,omitempty"`
	Filename      string `json:"filename,omitempty"`
	FailureReason string `json:"failureReason,omitempty"`
}

// ExportURLSigner creates a short-lived URL for one available export. The
// service invokes it only after can_view_sensitive authorization succeeds.
type ExportURLSigner interface {
	Presign(context.Context, *appv1alpha1.Database, appv1alpha1.DatabaseExportStatus, time.Duration) (string, error)
}

// RecoverRequest is the recover body: the new instance's name (required) and the
// point in time to restore to (empty => the latest available point).
type RecoverRequest struct {
	Name       string `json:"name"`
	TargetTime string `json:"targetTime,omitempty"`
	Plan       string `json:"plan,omitempty"`
	Version    string `json:"version,omitempty"`
}

// RecoveryInfo returns the recovery window + backup list for a managed Postgres.
// A no-backup plan returns {enabled:false} rather than an error.
func (s *Service) RecoveryInfo(ctx context.Context, name string) (RecoveryInfoView, error) {
	d, err := s.fetchDatabase(ctx, core.RelCanView, name)
	if err != nil {
		return RecoveryInfoView{}, err
	}
	info := RecoveryInfoView{Enabled: d.Status.BackupsEnabled, Backups: []BackupView{}}
	if !info.Enabled {
		return info, nil
	}
	windowStart, err := s.recoveryWindowStart(ctx, d.Namespace, name)
	if err != nil {
		return RecoveryInfoView{}, err
	}
	if windowStart != "" {
		info.EarliestRecoveryTime = windowStart
		// Latest recoverable point ≈ now, but ONLY once the window is actually
		// open. Before the first recoverability point there is nothing to restore
		// to, so we report no latest rather than the wall clock — the one field
		// that used to touch no Kubernetes read and so survived every failure to
		// contradict the empty backup list beside it.
		info.LatestRecoveryTime = s.Now().UTC().Format(time.RFC3339)
	}
	backups, err := s.listBackups(ctx, d.Namespace, name)
	if err != nil {
		return RecoveryInfoView{}, fmt.Errorf("list backups for %q: %w", name, err)
	}
	info.Backups = backups
	return info, nil
}

// recoveryWindowStart reads the PITR window's opening point — CNPG's
// firstRecoverabilityPoint — off the Cluster status. It is the ONE spelling of
// this read, shared by reporting (RecoveryInfo) and provisioning (Recover), so
// the two can never disagree about what a failed read means (w6/m117). "" means
// the window is not yet established: the Cluster is absent (NotFound — a
// brand-new database) or does not report a point yet (a missing or empty field,
// deliberately not distinguished — both mean "nothing restorable yet"). A read
// that FAILS is an error — RBAC, an unavailable CRD, a transient fault — and is
// never presented as either an empty or an established window.
func (s *Service) recoveryWindowStart(ctx context.Context, namespace, name string) (string, error) {
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	switch err := s.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, cluster); {
	case err == nil:
		point, _, _ := unstructured.NestedString(cluster.Object, "status", "firstRecoverabilityPoint")
		return point, nil
	case apierrors.IsNotFound(err):
		return "", nil
	default:
		return "", fmt.Errorf("read recovery window for %q: %w", name, err)
	}
}

// listBackups lists the CNPG Backup objects for a database, mapping each to a
// BackupView. A failed List is returned as an ERROR, never a silent empty list:
// an empty result must mean "this database has no backups", not "I could not read
// them" (w6/m117). With CNPG installed and the read RBAC granted, a database that
// genuinely has none simply lists zero items.
func (s *Service) listBackups(ctx context.Context, namespace, name string) ([]BackupView, error) {
	sel := client.MatchingLabels{labelCNPGCluster: name}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(cnpgBackupGVK)
	if err := s.Client.List(ctx, list, client.InNamespace(namespace), sel); err != nil {
		return nil, err
	}
	out := make([]BackupView, 0, len(list.Items))
	for i := range list.Items {
		b := &list.Items[i]
		phase, _, _ := unstructured.NestedString(b.Object, "status", "phase")
		out = append(out, BackupView{
			ID:        b.GetName(),
			Status:    backupStatus(phase),
			CreatedAt: b.GetCreationTimestamp().UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

// backupStatus collapses CNPG's Backup phase onto a small stable enum.
func backupStatus(phase string) string {
	switch phase {
	case "completed":
		return "completed"
	case "failed":
		return "failed"
	case "running", "started":
		return "running"
	default:
		return "pending"
	}
}

// Recover provisions a NEW managed Postgres by restoring the named instance's
// object-store backups to a point in time (Render's recover) — the source is
// never touched. The new Database defaults to the source's plan/version.
func (s *Service) Recover(ctx context.Context, name string, req RecoverRequest) (PostgresView, error) {
	src, err := s.fetchDatabase(ctx, core.RelCanCreate, name)
	if err != nil {
		return PostgresView{}, err
	}
	if req.Name == "" {
		return PostgresView{}, fmt.Errorf("%w: name (the new instance) is required", core.ErrBadRequest)
	}
	if err := validateDatabaseName(req.Name); err != nil {
		return PostgresView{}, err
	}
	if req.Name == src.Spec.Name {
		return PostgresView{}, fmt.Errorf("%w: recover creates a NEW instance; name must differ from the source", core.ErrBadRequest)
	}
	tenantID := src.Labels[core.LabelTenant]
	if err := s.ensureDatabaseNameAvailable(ctx, tenantID, req.Name, ""); err != nil {
		return PostgresView{}, err
	}
	if _, err := core.ParseTime("targetTime", req.TargetTime); err != nil {
		return PostgresView{}, err
	}
	if !src.Status.BackupsEnabled {
		return PostgresView{}, fmt.Errorf("%w: %q has no backups to recover from", core.ErrBadRequest, name)
	}
	// BackupsEnabled proves backups are CONFIGURED, not that anything restorable
	// exists yet. Require the established window too (w6/m117 t003): without a
	// firstRecoverabilityPoint there is no point in time to restore to, and
	// proceeding provisioned a new billable database that could never bootstrap.
	// Refuse HERE — before the plan/billing gates and the Create — so nothing
	// billable is created against an unsubstantiated window; and a window we
	// could not read surfaces as the read's error, never as absent or present.
	windowStart, err := s.recoveryWindowStart(ctx, src.Namespace, src.Name)
	if err != nil {
		return PostgresView{}, err
	}
	if windowStart == "" {
		return PostgresView{}, fmt.Errorf("%w: %q has no restore point yet; recovery becomes available once its first backup completes", core.ErrBadRequest, name)
	}
	plan := req.Plan
	if plan == "" {
		plan = src.Spec.Plan
	}
	version := req.Version
	if version == "" {
		version = currentPostgresVersion(src)
	}
	// Recovery was written as its own create path and inherited none of the
	// shared create invariants (codex round-5 F5). A caller-supplied plan and
	// version reached the CR unvalidated, and neither billing gate ran, so a
	// recovery could provision paid capacity that CreatePostgres and SetPlan
	// would both have refused for the identical cost.
	//
	// Only CALLER-SUPPLIED values are validated, exactly as CreatePostgres
	// treats req.Version: an inherited plan/version comes from a Database that
	// already exists, and re-validating it would refuse to recover an instance
	// on a plan since retired from the catalog — or one whose version is empty
	// because it takes the operator's default. The billing gates use the
	// RESOLVED plan either way, because that is what the recovery actually costs.
	if req.Plan != "" {
		if _, ok := tiers.Postgres.ByID(req.Plan); !ok {
			return PostgresView{}, fmt.Errorf("%w: plan must be one of %s", core.ErrBadRequest, strings.Join(tiers.Postgres.IDs(), "|"))
		}
	}
	if req.Version != "" && !postgresVersionKnown(req.Version) {
		return PostgresView{}, fmt.Errorf("%w: version must be one of %s", core.ErrBadRequest, supportedPostgresVersionText())
	}
	if err := s.RequirePlanBilling(ctx, tenantID, plan); err != nil {
		return PostgresView{}, err
	}
	sourceBackupServerName := src.Status.BackupServerName
	if sourceBackupServerName == "" {
		sourceBackupServerName = src.Name
	}
	newDB := &appv1alpha1.Database{
		// A recovered database belongs to the same workspace as its source, so it
		// lands in the same namespace (ADR043 D8).
		ObjectMeta: metav1.ObjectMeta{Name: id.New(id.Postgres), Namespace: src.Namespace},
		Spec: appv1alpha1.DatabaseSpec{
			Name:         req.Name,
			DatabaseName: src.Spec.EffectiveDatabaseName(src.Name),
			DatabaseUser: src.Spec.EffectiveDatabaseUser(src.Name),
			Plan:         plan,
			Version:      version,
			Recovery: &appv1alpha1.DatabaseRecovery{
				SourceDatabase:         src.Name,
				SourceBackupServerName: sourceBackupServerName,
				TargetTime:             req.TargetTime,
			},
		},
	}
	if tenantID != "" {
		newDB.Labels = core.TenantLabels(tenantID)
	}
	resourcemeta.Touch(newDB, s.Now())
	if err := s.Client.Create(ctx, newDB); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return PostgresView{}, fmt.Errorf("%w: generated Postgres id collision; retry the request", core.ErrConflict)
		}
		return PostgresView{}, err
	}
	return pgView(newDB), nil
}

// ListExports lists logical pg_dump exports newest first. A dump is the entire
// database, so even listing it (Render's list response contains the download
// URL) is gated by can_view_sensitive rather than ordinary can_view.
func (s *Service) ListExports(ctx context.Context, name string) ([]ExportView, error) {
	d, err := s.fetchDatabase(ctx, core.RelCanViewSensitive, name)
	if err != nil {
		return nil, err
	}
	// codex round-9 #7: every available row below is presigned into a fresh
	// 15-minute bearer download URL — a bearer-capability mint. Reassert
	// can_view_sensitive uncached so a revocation inside PositiveTTL cannot
	// ride a cached positive to one last full-dump URL.
	if err := s.AuthorizeDatabaseFresh(ctx, core.RelCanViewSensitive, d); err != nil {
		return nil, err
	}
	statusByID := make(map[string]appv1alpha1.DatabaseExportStatus, len(d.Status.Exports))
	for _, status := range d.Status.Exports {
		statusByID[status.ID] = status
	}
	now := s.Now().UTC()
	out := make([]ExportView, 0, len(d.Spec.Exports))
	for i := len(d.Spec.Exports) - 1; i >= 0; i-- {
		request := d.Spec.Exports[i]
		status, ok := statusByID[request.ID]
		if !ok {
			status = appv1alpha1.DatabaseExportStatus{
				ID:        request.ID,
				Phase:     appv1alpha1.DatabaseExportCreated,
				CreatedAt: request.RequestedAt,
			}
		}
		view := ExportView{
			ID:            request.ID,
			CreatedAt:     status.CreatedAt,
			Status:        string(status.Phase),
			ExpiresAt:     status.ExpiresAt,
			Filename:      status.Filename,
			FailureReason: status.FailureReason,
		}
		if view.CreatedAt == "" {
			view.CreatedAt = request.RequestedAt
		}
		if status.Phase == appv1alpha1.DatabaseExportAvailable {
			if expiresAt, parseErr := time.Parse(time.RFC3339, status.ExpiresAt); parseErr == nil && !now.Before(expiresAt) {
				view.Status = string(appv1alpha1.DatabaseExportExpired)
			} else {
				ttl := ExportURLTTL
				if !expiresAt.IsZero() && expiresAt.Sub(now) < ttl {
					ttl = expiresAt.Sub(now)
				}
				if s.ExportSigner == nil {
					return nil, fmt.Errorf("presign export %s: download signer is not configured", status.ID)
				}
				view.URL, err = s.ExportSigner.Presign(ctx, d, status, ttl)
				if err != nil {
					return nil, fmt.Errorf("presign export %s: %w", status.ID, err)
				}
				view.URLExpiresAt = now.Add(ttl).Format(time.RFC3339)
			}
		}
		out = append(out, view)
	}
	return out, nil
}

// maxTerminalExportHistory bounds how many TERMINAL exports (failed/expired)
// stay in spec.exports once newer ones exist (round-11 #9): every non-terminal
// phase is always kept. Without the bound, spec.exports grows append-only, so
// repeated create-export requests bloat the Database CR (etcd object size) and
// make every reconcile and list traversal cost O(all history).
const maxTerminalExportHistory = 10

// pruneTerminalExports keeps every non-terminal request plus the most recent
// maxTerminalExportHistory terminal ones. Requests append in order, so the
// newest terminals are the last in the list; an entry whose status has not
// caught up yet is non-terminal by definition and always kept. Artifacts of
// pruned entries are unaffected: expiry is driven by the operator from the
// entries it already observed.
func pruneTerminalExports(spec []appv1alpha1.DatabaseExportRequest, statusByID map[string]appv1alpha1.DatabaseExportPhase) []appv1alpha1.DatabaseExportRequest {
	terminal := func(id string) bool {
		phase, observed := statusByID[id]
		return observed && (phase == appv1alpha1.DatabaseExportFailed || phase == appv1alpha1.DatabaseExportExpired)
	}
	var terminalIdx []int
	for i, request := range spec {
		if terminal(request.ID) {
			terminalIdx = append(terminalIdx, i)
		}
	}
	if len(terminalIdx) <= maxTerminalExportHistory {
		return spec
	}
	drop := make(map[int]bool, len(terminalIdx)-maxTerminalExportHistory)
	for _, i := range terminalIdx[:len(terminalIdx)-maxTerminalExportHistory] {
		drop[i] = true
	}
	kept := make([]appv1alpha1.DatabaseExportRequest, 0, len(spec)-len(drop))
	for i, request := range spec {
		if !drop[i] {
			kept = append(kept, request)
		}
	}
	return kept
}

// CreateExport appends logical-export intent to the Database CR. The operator
// owns the pg_dump/upload Job and writes the lifecycle back to status; bex-api
// never handles dump bytes. Render permits only one in-progress export per DB.
func (s *Service) CreateExport(ctx context.Context, name string) (ExportView, error) {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	d, err := s.fetchDatabase(ctx, core.RelCanOperate, name)
	if err != nil {
		return ExportView{}, err
	}
	if !d.Status.BackupsEnabled {
		return ExportView{}, fmt.Errorf("%w: %q has no backup store; exports are unavailable", core.ErrBadRequest, name)
	}
	if d.Spec.Suspended {
		return ExportView{}, fmt.Errorf("%w: %q is suspended; resume it before creating an export", core.ErrConflict, name)
	}
	statusByID := make(map[string]appv1alpha1.DatabaseExportPhase, len(d.Status.Exports))
	for _, status := range d.Status.Exports {
		statusByID[status.ID] = status.Phase
	}
	for _, request := range d.Spec.Exports {
		phase, observed := statusByID[request.ID]
		if !observed || phase == appv1alpha1.DatabaseExportCreated || phase == appv1alpha1.DatabaseExportRunning {
			return ExportView{}, fmt.Errorf("%w: an export is already in progress for %q", core.ErrConflict, name)
		}
	}

	now := s.Now().UTC()
	exportID := id.New(id.Export)
	requestedAt := now.Format(time.RFC3339)
	pruned := pruneTerminalExports(d.Spec.Exports, statusByID)
	d.Spec.Exports = append(pruned, appv1alpha1.DatabaseExportRequest{ID: exportID, RequestedAt: requestedAt})
	resourcemeta.Touch(d, now)
	if err := s.Client.Update(ctx, d); err != nil {
		return ExportView{}, err
	}
	s.RecordDatabaseEffect(ctx, d, core.DatabaseBackupStarted)
	return ExportView{ID: exportID, CreatedAt: requestedAt, Status: string(appv1alpha1.DatabaseExportCreated)}, nil
}
