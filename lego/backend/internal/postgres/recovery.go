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
	"maps"
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
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

// CNPG/Barman types the recovery surface reads via unstructured (bex-api, like
// the operator, does not vendor either project's Go API).
var (
	cnpgBackupGVK             = schema.GroupVersionKind{Group: "postgresql.cnpg.io", Version: "v1", Kind: "Backup"}
	barmanCloudObjectStoreGVK = schema.GroupVersionKind{Group: "barmancloud.cnpg.io", Version: "v1", Kind: "ObjectStore"}
)

const (
	// labelCNPGCluster is CNPG's intrinsic label on every Backup it manages
	// (scheduled and on-demand) — the reliable Backup→Database link, so
	// recovery info can list the automatic base backups too, not just exports.
	labelCNPGCluster = "cnpg.io/cluster"
	// tenantBackupObjectStoreName is the shared namespaced ObjectStore projected
	// by the operator into every tenant namespace.
	tenantBackupObjectStoreName = "bex-tenant-postgres"
	// ExportURLTTL is the maximum life of a freshly minted download URL. It is
	// deliberately much shorter than the seven-day artifact-retention window.
	ExportURLTTL = 15 * time.Minute
	// recoveryMinimumAge is how far before now a recovery target must lie
	// (w7/m453). Render: "You can't restore to a time that's within ten minutes
	// of the current time" (render.com/docs/postgresql-backups, read
	// 2026-11-16).
	recoveryMinimumAge = 10 * time.Minute
)

// RecoveryInfoView mirrors Render's postgres recovery info: whether recovery is
// available and the point-in-time window, plus the visible backup list.
type RecoveryInfoView struct {
	// Enabled is false for a no-backup plan (Free) — recovery isn't an error
	// there, just unavailable, so a client can render a disabled state.
	Enabled bool `json:"enabled"`
	// EarliestRecoveryTime / LatestRecoveryTime bound the restorable window
	// (RFC3339), as reported by the Barman Cloud ObjectStore for this database's
	// exact backup server name. Where the workspace plan's recovery floor
	// (w7/m453, core.PITRFloor) cuts the window, the earliest bound is the
	// newest base backup completed at or before it, as on Render (w7/075).
	EarliestRecoveryTime string `json:"earliestRecoveryTime,omitempty"`
	LatestRecoveryTime   string `json:"latestRecoveryTime,omitempty"`
	// Backups is the visible backup history (base backups in object storage).
	Backups []BackupView `json:"backups"`
	// Retention is the workspace plan's recovery window, a Bex extension the
	// dashboard's recovery picker names (w7/m453). Absent where no plan applies
	// and for a database without backups. Render's REST route does not carry it.
	Retention *RecoveryRetention `json:"retention,omitempty"`
}

// RecoveryRetention is how far back a workspace's plan may restore a Postgres
// (core.PITRRetention), in the shape the log viewer's retention reads: the
// plan, its days, and the cheapest plan that restores further back with its
// days (empty and 0 on the longest window).
type RecoveryRetention struct {
	Plan        string `json:"plan"`
	Days        int    `json:"days"`
	UpgradePlan string `json:"upgradePlan,omitempty"`
	UpgradeDays int    `json:"upgradeDays,omitempty"`
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

type recoveryWindowBounds struct {
	earliest string
	latest   string
}

func (w recoveryWindowBounds) established() bool {
	return w.earliest != "" && w.latest != ""
}

// cutBy reports whether floor, the workspace plan's recovery floor (w7/m453),
// starts w later than the ObjectStore does; a zero floor never does.
func (w recoveryWindowBounds) cutBy(floor time.Time) bool {
	if floor.IsZero() || !w.established() {
		return false
	}
	earliest, err := time.Parse(time.RFC3339, w.earliest)
	return err != nil || earliest.Before(floor)
}

// flooredAt starts w where Render starts a plan's window (w7/075, captured
// 2026-11-17, docs/render-artifacts/postgres-recovery-window.md): at the newest
// base backup completed at or before the floor, the first of the backups every
// point since the floor restores from. So the start can lie up to one backup
// interval before the floor, and a database whose base backups stopped before
// the floor keeps a window from its last one (Render answers AVAILABLE there,
// not BACKUP_NOT_READY). completed are the database's base backup completion
// instants; the ObjectStore's last backup counts as one, and one before its
// first point does not. With none at or before the floor, the window starts at
// the floor. A window the floor does not cut (cutBy) is w itself.
func (w recoveryWindowBounds) flooredAt(floor time.Time, completed []time.Time) recoveryWindowBounds {
	if !w.cutBy(floor) {
		return w
	}
	earliest, _ := time.Parse(time.RFC3339, w.earliest)
	var start time.Time
	keep := func(at time.Time) {
		if !at.After(floor) && !at.Before(earliest) && at.After(start) {
			start = at
		}
	}
	for _, at := range completed {
		keep(at)
	}
	if latest, err := time.Parse(time.RFC3339, w.latest); err == nil {
		keep(latest)
	}
	if start.IsZero() {
		start = floor
	}
	return recoveryWindowBounds{earliest: start.UTC().Format(time.RFC3339), latest: w.latest}
}

// pitrWindow is the workspace plan's recovery window for one database
// (w7/m453): its floor at now, rounded up to the second the API reports, and
// the hint the dashboard names. Both are zero where no plan applies (no
// store, the default tenant), and the ObjectStore's window alone stands.
type pitrWindow struct {
	floor     time.Time
	retention *RecoveryRetention
}

// pitrWindowOf reads d's workspace plan and the recovery floor its last plan
// change kept, in one tenant read (core.WorkspacePITRPlanOf).
func (s *Service) pitrWindowOf(ctx context.Context, d *appv1alpha1.Database) (pitrWindow, error) {
	plan, visibleFrom, ok, err := s.WorkspacePITRPlanOf(ctx, d.Labels[core.LabelTenant])
	if err != nil || !ok {
		return pitrWindow{}, err
	}
	floor := core.PITRFloor(plan, visibleFrom, s.Now())
	if second := floor.Truncate(time.Second); second.Before(floor) {
		floor = second.Add(time.Second)
	}
	const day = 24 * time.Hour
	retention := &RecoveryRetention{Plan: plan, Days: int(core.PITRRetention(plan) / day)}
	if next, longer, ok := core.PITRRetentionUpgrade(plan); ok {
		retention.UpgradePlan, retention.UpgradeDays = next, int(longer/day)
	}
	return pitrWindow{floor: floor, retention: retention}, nil
}

func effectiveBackupServerName(d *appv1alpha1.Database) string {
	if d.Status.BackupServerName != "" {
		return d.Status.BackupServerName
	}
	return d.Name
}

// RecoveryInfo returns the recovery window + backup list for a managed Postgres.
// A no-backup plan returns {enabled:false} rather than an error.
func (s *Service) RecoveryInfo(ctx context.Context, name string) (RecoveryInfoView, error) {
	d, err := s.fetchDatabaseForRead(ctx, core.RelCanView, name)
	if err != nil {
		return RecoveryInfoView{}, err
	}
	info := RecoveryInfoView{Enabled: d.Status.BackupsEnabled, Backups: []BackupView{}}
	if !info.Enabled {
		return info, nil
	}
	var window recoveryWindowBounds
	var backups []BackupView
	var completed []time.Time
	var pitr pitrWindow
	group, readCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		var windowErr error
		window, windowErr = s.recoveryWindow(readCtx, d)
		return windowErr
	})
	group.Go(func() error {
		var backupErr error
		backups, completed, backupErr = s.listBackups(readCtx, d.Namespace, d.Name)
		return backupErr
	})
	group.Go(func() error {
		var planErr error
		pitr, planErr = s.pitrWindowOf(readCtx, d)
		return planErr
	})
	if err := group.Wait(); err != nil {
		return RecoveryInfoView{}, err
	}
	window = window.flooredAt(pitr.floor, completed)
	info.EarliestRecoveryTime = window.earliest
	info.LatestRecoveryTime = window.latest
	info.Backups = backups
	info.Retention = pitr.retention
	return info, nil
}

// recoveryWindow reads both authoritative bounds for the database's exact
// Barman server identity. Empty or partial status is not an established window;
// a failed source read remains an error rather than masquerading as empty.
func (s *Service) recoveryWindow(ctx context.Context, d *appv1alpha1.Database) (recoveryWindowBounds, error) {
	objectStore := &unstructured.Unstructured{}
	objectStore.SetGroupVersionKind(barmanCloudObjectStoreGVK)
	if err := s.Client.Get(ctx, client.ObjectKey{Namespace: d.Namespace, Name: tenantBackupObjectStoreName}, objectStore); err != nil {
		return recoveryWindowBounds{}, fmt.Errorf("read recovery window for %q: %w", d.Name, err)
	}
	serverName := effectiveBackupServerName(d)
	start, _, _ := unstructured.NestedString(objectStore.Object, "status", "serverRecoveryWindow", serverName, "firstRecoverabilityPoint")
	end, _, _ := unstructured.NestedString(objectStore.Object, "status", "serverRecoveryWindow", serverName, "lastSuccessfulBackupTime")
	window := recoveryWindowBounds{earliest: start, latest: end}
	if !window.established() {
		return recoveryWindowBounds{}, nil
	}
	return window, nil
}

// listBackups lists the CNPG Backup objects for a database, mapping each to a
// BackupView, plus the completion instants (status.stoppedAt) of the completed
// ones, which start a plan's window (flooredAt). A failed List is returned as an
// ERROR, never a silent empty list: an empty result must mean "this database has
// no backups", not "I could not read them" (w6/m117). With CNPG installed and the
// read RBAC granted, a database that genuinely has none simply lists zero items.
func (s *Service) listBackups(ctx context.Context, namespace, name string) ([]BackupView, []time.Time, error) {
	sel := client.MatchingLabels{labelCNPGCluster: name}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(cnpgBackupGVK)
	if err := s.Client.List(ctx, list, client.InNamespace(namespace), sel); err != nil {
		return nil, nil, fmt.Errorf("list backups for %q: %w", name, err)
	}
	out := make([]BackupView, 0, len(list.Items))
	var completed []time.Time
	for i := range list.Items {
		b := &list.Items[i]
		phase, _, _ := unstructured.NestedString(b.Object, "status", "phase")
		status := backupStatus(phase)
		out = append(out, BackupView{
			ID:        b.GetName(),
			Status:    status,
			CreatedAt: b.GetCreationTimestamp().UTC().Format(time.RFC3339),
		})
		if status != "completed" {
			continue
		}
		stoppedAt, _, _ := unstructured.NestedString(b.Object, "status", "stoppedAt")
		if at, err := time.Parse(time.RFC3339, stoppedAt); err == nil {
			completed = append(completed, at)
		}
	}
	return out, completed, nil
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
	if err := core.CheckDatastoreName(req.Name); err != nil {
		return PostgresView{}, err
	}
	if req.Name == src.Spec.Name {
		return PostgresView{}, fmt.Errorf("%w: recover creates a NEW instance; name must differ from the source", core.ErrBadRequest)
	}
	tenantID := src.Labels[core.LabelTenant]
	if err := s.ensureDatabaseNameAvailable(ctx, tenantID, req.Name, ""); err != nil {
		return PostgresView{}, err
	}
	// Resolved before the billing gates, as a create resolves its environment.
	environment, err := s.sourceEnvironment(ctx, src, tenantID)
	if err != nil {
		return PostgresView{}, err
	}
	target, err := core.ParseTime("targetTime", req.TargetTime)
	if err != nil {
		return PostgresView{}, err
	}
	if !src.Status.BackupsEnabled {
		return PostgresView{}, fmt.Errorf("%w: %q has no backups to recover from", core.ErrBadRequest, name)
	}
	// BackupsEnabled proves configuration, not a restorable point. Refuse an
	// unestablished window before billing or Create so a dead recovery cannot
	// provision paid capacity; source-read failures remain explicit errors.
	window, err := s.recoveryWindow(ctx, src)
	if err != nil {
		return PostgresView{}, err
	}
	// The workspace plan's floor bounds the window as recovery info reports it
	// (w7/m453, w7/075), so a target the picker never offered is refused here
	// too. The base backups are read only when the floor cuts the window.
	pitr, err := s.pitrWindowOf(ctx, src)
	if err != nil {
		return PostgresView{}, err
	}
	if window.cutBy(pitr.floor) {
		_, completed, err := s.listBackups(ctx, src.Namespace, src.Name)
		if err != nil {
			return PostgresView{}, err
		}
		window = window.flooredAt(pitr.floor, completed)
	}
	if !window.established() {
		return PostgresView{}, fmt.Errorf("%w: %q has no restore point yet; recovery becomes available once its first backup completes", core.ErrBadRequest, name)
	}
	// A point-in-time target outside [earliest restore point, now - ten
	// minutes] is refused: before it, it can never bootstrap or lies outside
	// the plan's window; after it, Render refuses it too. The restore database
	// it provisions is paid capacity (ADR009; w21/029). The upper bound is now,
	// not the last base backup: PITR replays archived WAL past it.
	if !target.IsZero() {
		earliest, parseErr := time.Parse(time.RFC3339, window.earliest)
		if parseErr == nil && target.Before(earliest) {
			return PostgresView{}, core.NewBadRequestError("RECOVERY_TARGET_BEFORE_WINDOW",
				"targetTime is before the earliest restore point ("+window.earliest+")",
				map[string]any{"earliestRecoveryTime": window.earliest})
		}
		now := s.Now()
		if target.After(now) {
			return PostgresView{}, core.NewBadRequestError("RECOVERY_TARGET_IN_FUTURE",
				"targetTime is in the future", nil)
		}
		if latest := now.Add(-recoveryMinimumAge); target.After(latest) {
			latestText := latest.UTC().Truncate(time.Second).Format(time.RFC3339)
			return PostgresView{}, core.NewBadRequestError("RECOVERY_TARGET_TOO_RECENT",
				"targetTime is within ten minutes of now; choose a time no later than "+latestText,
				map[string]any{"latestTargetTime": latestText})
		}
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
		req.Plan = tiers.Postgres.CanonicalID(req.Plan)
		if _, ok := tiers.Postgres.ByID(req.Plan); !ok {
			return PostgresView{}, fmt.Errorf("%w: plan must be one of %s", core.ErrBadRequest, strings.Join(tiers.Postgres.IDs(), "|"))
		}
	}
	if req.Version != "" && !postgresVersionKnown(req.Version) {
		return PostgresView{}, fmt.Errorf("%w: version must be one of %s", core.ErrBadRequest, supportedPostgresVersionText())
	}
	sourceBackupServerName := effectiveBackupServerName(src)
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
			// The restore must fit: the archive holds as much data as the
			// source's disk grew to, and a setting its data needs to load
			// travels with it (w4/360).
			StorageGB:  DatabaseStorageHighWater(src),
			Parameters: tenantParameters(src.Spec.Parameters),
			// A login retired after the restore target comes back from the
			// backup with its old password; carrying the source's retirements
			// puts them in the restored Cluster's managed roles, which CNPG
			// applies once the restored primary starts (w5/m384, round 24).
			// Not the owner: it is the restored instance's default credential,
			// since the source's added users are not carried.
			DeletedUsers: slices.DeleteFunc(slices.Clone(src.Spec.DeletedUsers), func(u string) bool {
				return u == src.Spec.EffectiveDatabaseUser(src.Name)
			}),
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
	inheritBoundary(src, newDB, environment)
	// The recovery is a new resource in the source's workspace, born inside
	// its inbound-IP rules as every create is (w9/m388).
	workspaceLayer, err := s.WorkspaceIPLayer(ctx, tenantID)
	if err != nil {
		return PostgresView{}, err
	}
	newDB.Spec.WorkspaceIPAllowList = workspaceLayer
	// It reads back its source's region, as it lives in the same placement
	// (w9/105).
	resourcemeta.StampRegion(newDB, src.Annotations[resourcemeta.AnnotRegion])
	// The shared create invariants, before the billing gate as on create, so a
	// plan too small for the source's data is refused here instead of failing
	// mid-restore (w4/360). An inherited plan since retired from the catalog
	// is not re-validated (see above): its storage already fit it.
	if _, known := tiers.Postgres.ByID(tiers.Postgres.CanonicalID(plan)); known {
		if err := CheckDatabaseAdmission(nil, newDB.Spec); err != nil {
			return PostgresView{}, err
		}
	}
	if err := s.RequirePlanBilling(ctx, tenantID, plan); err != nil {
		return PostgresView{}, err
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

// tenantParameters is the source's parameter overrides a recovery carries:
// every one but the platform-managed names, which the platform sets for the
// restore itself (recovery_target*, WAL archival, …) and admission refuses, so
// a legacy source holding one cannot wedge the restore (w4/360).
func tenantParameters(params map[string]string) map[string]string {
	out := maps.Clone(params)
	for _, name := range operatorManagedParameterNames(params) {
		delete(out, name)
	}
	return out
}

// sourceEnvironment resolves the environment a recovery joins: the source's,
// through the same resolver a create uses, so a recovery into a protected
// environment needs an Admin, as any create there does (w3/m281). A source in
// no environment yields none; one that cannot be resolved refuses the recovery
// (w4/356).
func (s *Service) sourceEnvironment(ctx context.Context, src *appv1alpha1.Database, tenantID string) (core.EnvironmentAssignment, error) {
	environmentID := src.Labels[core.LabelEnvironment]
	if environmentID == "" {
		return core.EnvironmentAssignment{}, nil
	}
	return core.ResolveEnvironmentForCreate(ctx, s.Environments, id.EnvironmentPublicID(environmentID), tenantID)
}

// inheritBoundary keeps a recovery inside its source's boundary: it is the
// source's data, so it must not land outside the protected, isolated or
// allow-listed environment that guards it (w4/356). The new instance joins the
// source's environment (and so its project and inbound-IP layer) or, failing
// one, its project, and always copies the source's own external access and IP
// allow-list. Render does the same: "the recovery instance always copies the
// IP address allow list from the original instance" (render.com/docs/
// postgresql-backups, 2026-10-19).
func inheritBoundary(src, dst *appv1alpha1.Database, environment core.EnvironmentAssignment) {
	dst.Spec.Public = src.Spec.Public
	dst.Spec.IPAllowList = slices.Clone(src.Spec.IPAllowList)
	if environment.ID != "" {
		dst.Spec.EnvironmentIPAllowList = core.ApplyGrouping(dst, environment)
		return
	}
	if projectID := src.Labels[core.LabelProject]; projectID != "" {
		if dst.Labels == nil {
			dst.Labels = map[string]string{}
		}
		dst.Labels[core.LabelProject] = projectID
	}
}

// ListExports lists logical pg_dump exports newest first. A dump is the entire
// database, so even listing it (Render's list response contains the download
// URL) is gated by can_view_sensitive rather than ordinary can_view.
//
// A request that returned at least one download URL is a download of a logical
// backup, Render's DownloadDatabaseBackupEvent, and records one row once every
// URL was minted (w21/105): the URL points at the object store, so this list is
// the one moment bex-api sees the download. A list that minted none (no export
// is available yet, or each has expired) records none, and a refused one is the
// denial its authorization already recorded.
func (s *Service) ListExports(ctx context.Context, name string) ([]ExportView, error) {
	d, err := s.fetchDatabaseForRead(ctx, core.RelCanViewSensitive, name)
	if err != nil {
		return nil, err
	}
	withURLs := !exportURLsWithheld(ctx)
	if withURLs {
		// codex round-9 #7: every available row below is presigned into a fresh
		// 15-minute bearer download URL — a bearer-capability mint. Reassert
		// can_view_sensitive uncached so a revocation inside PositiveTTL cannot
		// ride a cached positive to one last full-dump URL.
		if err := s.AuthorizeDatabaseFresh(ctx, core.RelCanViewSensitive, d); err != nil {
			return nil, err
		}
	}
	statusByID := make(map[string]appv1alpha1.DatabaseExportStatus, len(d.Status.Exports))
	for _, status := range d.Status.Exports {
		statusByID[status.ID] = status
	}
	now := s.Now().UTC()
	out := make([]ExportView, 0, len(d.Spec.Exports))
	minted := false
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
			} else if withURLs {
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
				minted = true
			}
		}
		out = append(out, view)
	}
	if minted {
		s.RecordSecretsViewed(ctx, core.AuditVerbDownloadDatabaseBackup, d.Labels[core.LabelTenant], core.DatabaseTarget(d.Name), d.Spec.Name)
	}
	return out, nil
}

// exportURLsKey marks a context whose ListExports must mint no download URL.
type exportURLsKey struct{}

// withoutExportURLs asks ListExports for the exports' status alone: no URL is
// minted, so no download is recorded. GraphQL's databaseExports uses it for a
// query that does not select url, the dashboard's 25-second status poll, which
// would otherwise write a DownloadDatabaseBackupEvent for every poll of an open
// Recovery tab (the env var list's Selects(p, "value") is the precedent, w4/m321).
func withoutExportURLs(ctx context.Context) context.Context {
	return context.WithValue(ctx, exportURLsKey{}, true)
}

func exportURLsWithheld(ctx context.Context) bool {
	withheld, _ := ctx.Value(exportURLsKey{}).(bool)
	return withheld
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
		// A Free database has no backups by plan (Render: "Free Render
		// Postgres databases don't support any form of backups"), which is the
		// user's to act on and reads like the other Free limits. A paid one
		// without a store is the platform's fault and keeps saying so (w8/277).
		if !core.PaidPlan(d.Spec.Plan) {
			return ExportView{}, core.NewBadRequestError("POSTGRES_EXPORT_PLAN_UNSUPPORTED",
				"logical exports require a paid Postgres plan; Free databases have no backups",
				map[string]any{"plan": "free"})
		}
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
