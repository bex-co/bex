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
// operator's doing (CNPG barmanObjectStore + ScheduledBackup → object storage);
// this surface reads their status and drives recovery/exports, all keyed off the
// Database's Status.BackupsEnabled signal so a no-backup plan degrades gracefully
// rather than erroring.
package postgres

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
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
	// Best-effort recovery window from the CNPG Cluster status (absent until the
	// first backup lands — then just report "now" as the latest recoverable point).
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	if err := s.Client.Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: name}, cluster); err == nil {
		if p, ok, _ := unstructured.NestedString(cluster.Object, "status", "firstRecoverabilityPoint"); ok {
			info.EarliestRecoveryTime = p
		}
	}
	info.LatestRecoveryTime = s.Now().UTC().Format(time.RFC3339)
	info.Backups = s.listBackups(ctx, name)
	return info, nil
}

// listBackups lists the CNPG Backup objects for a database, mapping each to a
// BackupView. Best-effort: an unavailable CNPG CRD (e.g. envtest) yields an
// empty list.
func (s *Service) listBackups(ctx context.Context, name string) []BackupView {
	sel := client.MatchingLabels{labelCNPGCluster: name}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(cnpgBackupGVK)
	if err := s.Client.List(ctx, list, client.InNamespace(s.Namespace), sel); err != nil {
		return []BackupView{}
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
	return out
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
	if req.Name == src.DisplayName() {
		return PostgresView{}, fmt.Errorf("%w: recover creates a NEW instance; name must differ from the source", core.ErrBadRequest)
	}
	tenantID := src.Labels[core.LabelTenant]
	if err := s.ensureDatabaseNameAvailable(ctx, tenantID, req.Name, ""); err != nil {
		return PostgresView{}, err
	}
	if req.TargetTime != "" {
		if _, err := time.Parse(time.RFC3339, req.TargetTime); err != nil {
			return PostgresView{}, fmt.Errorf("%w: targetTime must be an RFC3339 timestamp", core.ErrBadRequest)
		}
	}
	if !src.Status.BackupsEnabled {
		return PostgresView{}, fmt.Errorf("%w: %q has no backups to recover from", core.ErrBadRequest, name)
	}
	plan := req.Plan
	if plan == "" {
		plan = src.Spec.Plan
	}
	version := req.Version
	if version == "" {
		version = currentPostgresVersion(src)
	}
	sourceBackupServerName := src.Status.BackupServerName
	if sourceBackupServerName == "" {
		sourceBackupServerName = src.Name
	}
	newDB := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: id.New(id.Postgres), Namespace: s.Namespace},
		Spec: appv1alpha1.DatabaseSpec{
			Name:    req.Name,
			Plan:    plan,
			Version: version,
			Recovery: &appv1alpha1.DatabaseRecovery{
				SourceDatabase:         src.Name,
				SourceBackupServerName: sourceBackupServerName,
				TargetTime:             req.TargetTime,
			},
		},
	}
	if tenantID != "" {
		newDB.Labels = map[string]string{core.LabelTenant: tenantID, core.LabelWorkspace: tenantID}
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
	d.Spec.Exports = append(d.Spec.Exports, appv1alpha1.DatabaseExportRequest{ID: exportID, RequestedAt: requestedAt})
	resourcemeta.Touch(d, now)
	if err := s.Client.Update(ctx, d); err != nil {
		return ExportView{}, err
	}
	s.RecordDatabaseEffect(ctx, d, core.DatabaseBackupStarted)
	return ExportView{ID: exportID, CreatedAt: requestedAt, Status: string(appv1alpha1.DatabaseExportCreated)}, nil
}
