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
	// labelExport narrows to the on-demand exports this API created (the
	// ScheduledBackup-produced automatic backups don't carry it).
	labelExport = "app.bex.co/export"
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

// BackupView is one base backup / export in object storage.
type BackupView struct {
	ID        string `json:"id"`
	Status    string `json:"status"` // pending|running|completed|failed
	CreatedAt string `json:"createdAt,omitempty"`
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
	info.Backups = s.listBackups(ctx, name, false)
	return info, nil
}

// listBackups lists the CNPG Backup objects for a database, mapping each to a
// BackupView. onlyExports narrows to the on-demand export-labeled backups.
// Best-effort: an unavailable CNPG CRD (e.g. envtest) yields an empty list.
func (s *Service) listBackups(ctx context.Context, name string, onlyExports bool) []BackupView {
	sel := client.MatchingLabels{labelCNPGCluster: name}
	if onlyExports {
		sel[labelExport] = "true"
	}
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
	if req.Name == name {
		return PostgresView{}, fmt.Errorf("%w: recover creates a NEW instance; name must differ from the source", core.ErrBadRequest)
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
		version = src.Spec.Version
	}
	newDB := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: req.Name, Namespace: s.Namespace},
		Spec: appv1alpha1.DatabaseSpec{
			Plan:    plan,
			Version: version,
			Recovery: &appv1alpha1.DatabaseRecovery{
				SourceDatabase: name,
				TargetTime:     req.TargetTime,
			},
		},
	}
	if tenantID, ok := s.Tenant(ctx); ok {
		newDB.Labels = map[string]string{core.LabelWorkspace: tenantID}
	}
	if err := s.Client.Create(ctx, newDB); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return PostgresView{}, fmt.Errorf("%w: a database named %q already exists", core.ErrBadRequest, req.Name)
		}
		return PostgresView{}, err
	}
	return pgView(newDB), nil
}

// ListExports lists the on-demand exports (base-backup snapshots to object
// storage) taken for a database, newest first not guaranteed by the API.
func (s *Service) ListExports(ctx context.Context, name string) ([]BackupView, error) {
	if _, err := s.fetchDatabase(ctx, core.RelCanView, name); err != nil {
		return nil, err
	}
	return s.listBackups(ctx, name, true), nil
}

// CreateExport triggers an on-demand export: a CNPG on-demand Backup of the
// cluster to object storage — a discrete, restorable snapshot. (bex's export is
// a physical base-backup snapshot, not Render's logical pg_dump: a documented
// divergence — see docs/ADR009-postgresql-management.md.) Requires backups enabled.
func (s *Service) CreateExport(ctx context.Context, name string) (BackupView, error) {
	d, err := s.fetchDatabase(ctx, core.RelCanCreate, name)
	if err != nil {
		return BackupView{}, err
	}
	if !d.Status.BackupsEnabled {
		return BackupView{}, fmt.Errorf("%w: %q has no backup store; exports are unavailable", core.ErrBadRequest, name)
	}
	backupName := id.New(id.Export)
	backup := &unstructured.Unstructured{}
	backup.SetGroupVersionKind(cnpgBackupGVK)
	backup.SetName(backupName)
	backup.SetNamespace(s.Namespace)
	// cnpg.io/cluster is what CNPG stamps on every Backup for the cluster; set it
	// so this on-demand export is found by the same query as the scheduled ones,
	// and labelExport so it can be told apart from them.
	backup.SetLabels(map[string]string{labelCNPGCluster: name, labelExport: "true"})
	// GC the export with its Database (it's not owned by the CNPG Cluster).
	backup.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: appv1alpha1.GroupVersion.String(),
		Kind:       "Database",
		Name:       d.Name,
		UID:        d.UID,
	}})
	backup.Object["spec"] = map[string]any{
		"cluster": map[string]any{"name": name},
		"method":  "barmanObjectStore",
	}
	if err := s.Client.Create(ctx, backup); err != nil {
		return BackupView{}, err
	}
	return BackupView{ID: backupName, Status: "pending"}, nil
}
