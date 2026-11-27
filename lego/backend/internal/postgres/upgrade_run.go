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

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A major-version upgrade's attempt record (w3/m676): Render's dashboard reads
// the database's lastUpgradeRun { acknowledged runId startedAt state toVersion }
// and dismisses a failed run's result with acknowledgeDatabaseUpgrade(id,
// runId). bex keeps the record on the Database itself, in two annotations only
// bex-api writes:
//
//   - annotationUpgradeRun is stamped by the write that accepts an upgrade
//     (patchDatabaseObj, so REST's PATCH, GraphQL and MCP all record one): the
//     run's id, its source and target majors and when it was accepted.
//   - annotationUpgradeRunAcknowledged names the run a member acknowledged.
//
// The run's state is never stored. It is read from what the operator reports
// about the upgrade itself (status.currentVersion, the Ready condition and
// spec.version, which the operator returns to the source when pg_upgrade
// fails), so it cannot drift from the database, and a deleted database takes
// its record with it.
const (
	annotationUpgradeRun             = "app.bex.co/upgrade-run"
	annotationUpgradeRunAcknowledged = "app.bex.co/upgrade-run-acknowledged"
)

// The states of an upgrade run: Render's enum, whose values its dashboard
// compares against (FAILED, IN_PROGRESS, SUCCEEDED).
const (
	UpgradeRunInProgress = "IN_PROGRESS"
	UpgradeRunSucceeded  = "SUCCEEDED"
	UpgradeRunFailed     = "FAILED"
)

// The codes acknowledgeDatabaseUpgrade answers with.
const (
	PostgresUpgradeRunNotFoundCode   = "POSTGRES_UPGRADE_RUN_NOT_FOUND"
	PostgresUpgradeRunInProgressCode = "POSTGRES_UPGRADE_RUN_IN_PROGRESS"
)

// upgradeRunRecord is annotationUpgradeRun's JSON.
type upgradeRunRecord struct {
	RunID       string `json:"runId"`
	FromVersion string `json:"fromVersion"`
	ToVersion   string `json:"toVersion"`
	StartedAt   string `json:"startedAt"`
}

// UpgradeRunView is Render's lastUpgradeRun, plus the source major (a bex
// extension the page does not need but a client can show).
type UpgradeRunView struct {
	RunID        string
	FromVersion  string
	ToVersion    string
	StartedAt    string
	State        string
	Acknowledged bool
}

// Finished reports whether the run reached a result (succeeded or failed).
func (r UpgradeRunView) Finished() bool { return r.State != UpgradeRunInProgress }

// stampUpgradeRun records a new run on d when the write in hand moves
// spec.version: before is the Database as it was read, so its observed major
// is the run's source. It leaves an unchanged version alone, so any other
// update keeps the record it has.
func stampUpgradeRun(before, d *appv1alpha1.Database, now time.Time) {
	if d.Spec.Version == before.Spec.Version || d.Spec.Version == "" {
		return
	}
	record, err := json.Marshal(upgradeRunRecord{
		RunID:       id.New(id.PostgresUpgradeRun),
		FromVersion: currentPostgresVersion(before),
		ToVersion:   d.Spec.Version,
		StartedAt:   now.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return
	}
	if d.Annotations == nil {
		d.Annotations = map[string]string{}
	}
	d.Annotations[annotationUpgradeRun] = string(record)
}

// upgradeRunOf is d's last upgrade run, or nil when bex-api never accepted
// one (or its record is unreadable).
func upgradeRunOf(d *appv1alpha1.Database) *UpgradeRunView {
	raw := d.Annotations[annotationUpgradeRun]
	if raw == "" {
		return nil
	}
	var record upgradeRunRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil || record.RunID == "" {
		return nil
	}
	if _, err := strconv.Atoi(record.ToVersion); err != nil {
		return nil
	}
	return &UpgradeRunView{
		RunID:        record.RunID,
		FromVersion:  record.FromVersion,
		ToVersion:    record.ToVersion,
		StartedAt:    record.StartedAt,
		State:        upgradeRunState(d, record.ToVersion),
		Acknowledged: d.Annotations[annotationUpgradeRunAcknowledged] == record.RunID,
	}
}

// upgradeRunState reads a run's state from the operator's report:
//
//   - the database observed on the target (or a later) major: succeeded. The
//     operator adopts the new major only once its cluster is healthy.
//   - a Ready condition naming a failed major upgrade for the current spec:
//     failed, before the operator has returned spec.version to the source.
//   - spec.version still the target: in progress, accepted and not finished
//     (CNPG may not have started it yet).
//   - otherwise the operator returned spec.version to the source, which it
//     does only when pg_upgrade failed: failed.
func upgradeRunState(d *appv1alpha1.Database, to string) string {
	toMajor, _ := strconv.Atoi(to)
	if current, err := strconv.Atoi(d.Status.CurrentVersion); err == nil && current >= toMajor {
		return UpgradeRunSucceeded
	}
	ready := meta.FindStatusCondition(d.Status.Conditions, appv1alpha1.ConditionReady)
	if ready != nil && ready.Status == metav1.ConditionFalse &&
		ready.Reason == appv1alpha1.ReasonMajorVersionUpgradeFailed && ready.ObservedGeneration == d.Generation {
		return UpgradeRunFailed
	}
	if d.Spec.Version == to {
		return UpgradeRunInProgress
	}
	return UpgradeRunFailed
}

// AcknowledgeUpgrade is Render's acknowledgeDatabaseUpgrade: a member who may
// upgrade the database dismisses its finished run's result. Only the last run
// can be acknowledged, and only once it has finished; acknowledging it again
// changes nothing.
func (s *Service) AcknowledgeUpgrade(ctx context.Context, name, runID string) (PostgresView, error) {
	d, err := s.fetchLiveDatabase(ctx, core.RelCanCreate, name)
	if err != nil {
		return PostgresView{}, err
	}
	run := upgradeRunOf(d)
	if run == nil || run.RunID != runID {
		return PostgresView{}, core.NewNotFoundError(PostgresUpgradeRunNotFoundCode,
			fmt.Sprintf("upgrade run %q is not this database's last major-version upgrade", runID),
			map[string]any{"runId": runID})
	}
	if !run.Finished() {
		return PostgresView{}, core.NewConflictError(PostgresUpgradeRunInProgressCode,
			"the major-version upgrade is still running; acknowledge it once it has finished",
			map[string]any{"runId": runID, "targetVersion": run.ToVersion})
	}
	if run.Acknowledged {
		return s.view(d), nil
	}
	patch := client.MergeFrom(d.DeepCopy())
	if d.Annotations == nil {
		d.Annotations = map[string]string{}
	}
	d.Annotations[annotationUpgradeRunAcknowledged] = runID
	if err := s.PatchObject(ctx, d, patch, false); err != nil {
		return PostgresView{}, err
	}
	return s.view(d), nil
}

// LatestPostgresVersion is the newest major bex runs, and so the one target
// an in-place upgrade offers in the dashboard, as Render's always targets its
// latest supported major (Render's versionUpgradeAvailable).
func LatestPostgresVersion() string {
	return supportedPostgresVersions[len(supportedPostgresVersions)-1]
}

// DatabaseExtensionView is one extension installed in the application
// database, Render's databaseExtensions { name blocksUpgradeDatabase }.
type DatabaseExtensionView struct {
	Name                  string
	BlocksUpgradeDatabase bool
}

// upgradeBlockingExtensions are the extensions a bex operand image of some
// major carries that the target major's does not (deploy/pg-operand/matrix.json;
// TestUpgradeBlockingExtensionsMatchTheOperands holds them together).
// pg_upgrade refuses a cluster that has one installed, because its library is
// missing from the new binaries, and only after the downtime has begun.
func upgradeBlockingExtensions(target int) []string {
	var blocked []string
	if target >= 17 {
		// PostgreSQL 17 removed both from contrib.
		blocked = append(blocked, "adminpack", "old_snapshot")
	}
	if target >= 16 {
		// pg_similarity does not build against PostgreSQL 16 or later.
		blocked = append(blocked, "pg_similarity")
	}
	slices.Sort(blocked)
	return blocked
}

// installedExtensionsSQL lists the application database's extensions.
const installedExtensionsSQL = "SELECT extname FROM pg_catalog.pg_extension ORDER BY extname"

// installedExtensions reads the extensions installed in d's application
// database over its connection credential, read-only. The caller has
// authorized reading d.
func (s *Service) installedExtensions(ctx context.Context, d *appv1alpha1.Database) ([]string, error) {
	if err := requireQueryable(d); err != nil {
		return nil, err
	}
	uri, err := s.databaseURI(ctx, d)
	if err != nil {
		return nil, err
	}
	result, err := s.sqlExecutor()(ctx, uri, installedExtensionsSQL, consoleQueryLimits, true)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) > 0 {
			if name := strVal(row[0]); name != "" {
				names = append(names, name)
			}
		}
	}
	return names, nil
}

// DatabaseExtensions is Render's databaseExtensions: each extension installed
// in the database, and whether it blocks an in-place upgrade to the latest
// major, the one the dashboard offers.
func (s *Service) DatabaseExtensions(ctx context.Context, name string) ([]DatabaseExtensionView, error) {
	d, err := s.fetchLiveDatabase(ctx, core.RelCanView, name)
	if err != nil {
		return nil, err
	}
	names, err := s.installedExtensions(ctx, d)
	if err != nil {
		if errors.Is(err, core.ErrUnavailable) {
			return nil, err
		}
		// The cause is logged, not answered: it can name the instance's
		// internal host.
		log.Printf("postgres extensions %s: %v", d.Name, err)
		return nil, core.NewUnavailableError(PostgresExtensionsUnreadableCode,
			"could not read the database's extensions; try again once the database is available", nil)
	}
	target, _ := strconv.Atoi(LatestPostgresVersion())
	blocked := upgradeBlockingExtensions(target)
	out := make([]DatabaseExtensionView, len(names))
	for i, n := range names {
		out[i] = DatabaseExtensionView{Name: n, BlocksUpgradeDatabase: slices.Contains(blocked, n)}
	}
	return out, nil
}

// PostgresExtensionsUnreadableCode answers databaseExtensions when the
// database cannot be reached.
const PostgresExtensionsUnreadableCode = "POSTGRES_EXTENSIONS_UNREADABLE"

// PostgresUpgradeExtensionsBlockedCode refuses an upgrade pg_upgrade would
// refuse for an installed extension.
const PostgresUpgradeExtensionsBlockedCode = "POSTGRES_UPGRADE_EXTENSIONS_BLOCKED"

// checkUpgradeExtensions refuses an upgrade to target while an extension the
// target's image lacks is installed, naming them, so the refusal comes before
// any downtime rather than as a failed pg_upgrade. A database whose
// extensions cannot be read is not refused: pg_upgrade's own check still
// stops that upgrade, and leaves the database on its version.
func (s *Service) checkUpgradeExtensions(ctx context.Context, d *appv1alpha1.Database, target string) error {
	targetMajor, err := strconv.Atoi(target)
	if err != nil {
		return nil
	}
	blocking := upgradeBlockingExtensions(targetMajor)
	if len(blocking) == 0 {
		return nil
	}
	names, err := s.installedExtensions(ctx, d)
	if err != nil {
		log.Printf("postgres upgrade %s: reading the installed extensions: %v", d.Name, err)
		return nil
	}
	var blocked []string
	for _, n := range names {
		if slices.Contains(blocking, n) {
			blocked = append(blocked, n)
		}
	}
	if len(blocked) == 0 {
		return nil
	}
	return core.NewConflictError(PostgresUpgradeExtensionsBlockedCode, fmt.Sprintf(
		"PostgreSQL %s does not include %s, so pg_upgrade cannot upgrade a database that has it installed: drop it (DROP EXTENSION), then upgrade again",
		target, strings.Join(blocked, ", ")),
		map[string]any{"targetVersion": target, "extensions": blocked})
}
