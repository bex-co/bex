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
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// expiry.go is the free-plan lifecycle of ADR009 §Free-plan expiry (w8/m126):
// a free database expires 30 days after its free clock starts, is suspended at
// expiry, and is deleted after a 14-day grace unless it is upgraded. All of its
// state lives on the Database CR, so reads, the sweeper and plan changes share
// one source of truth.

const (
	// annotationFreeSince restarts the free clock when a paid database is
	// changed to free. Absent, the clock starts at the CR's creation.
	annotationFreeSince = "app.bex.co/free-since"
	// annotationFreeExpiry records the last stage the sweeper claimed, as
	// "<stage>@<expiresAt>", so a restarted clock starts its stages again.
	annotationFreeExpiry = "app.bex.co/free-expiry"
	// annotationFreeExpirySuspended marks a suspension the expiry made, so an
	// upgrade resumes it and never a suspension the user made.
	annotationFreeExpirySuspended = "app.bex.co/free-expiry-suspended"
)

const (
	FreeLifetime               = 30 * 24 * time.Hour
	FreeExpiryGrace            = 14 * 24 * time.Hour
	freeExpiryWarningLead      = 7 * 24 * time.Hour
	freeExpiryFinalWarningLead = 3 * 24 * time.Hour
)

// FreeExpiryRollout clamps the free clock's start, so a database created
// before expiry existed is not expired earlier than this date + 30 days.
var FreeExpiryRollout = time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)

// freeExpiryStage is a step of the lifecycle, in the order the sweeper takes
// them.
type freeExpiryStage int

const (
	stageNone freeExpiryStage = iota
	stageWarned
	stageExpired
	stageFinalWarned
	stageDeleted
)

// freeExpiryStageNames is indexed by stage; the names are also the stage
// argument FreeExpiryNotifier receives.
var freeExpiryStageNames = [...]string{
	stageWarned:      "warned",
	stageExpired:     "expired",
	stageFinalWarned: "final-warned",
	stageDeleted:     "deleted",
}

// reasonCodeFreeExpired is the statusReasonCode of a database the expiry
// suspended, and the code of a refused resume.
const reasonCodeFreeExpired = "POSTGRES_FREE_EXPIRED"

// freeExpiryAuditVerbs are the audit rows each stage writes. They are
// audit-only: Render publishes no webhook event for free-database expiry.
var freeExpiryAuditVerbs = map[freeExpiryStage]string{
	stageWarned:      "postgres.FreeExpiryWarned",
	stageExpired:     "postgres.FreeExpiryExpired",
	stageFinalWarned: "postgres.FreeExpiryFinalWarned",
	stageDeleted:     "postgres.FreeExpiryDeleted",
}

func isFreePlan(plan string) bool { return plan == "" || plan == "free" }

// FreeExpiresAt is the instant a free database expires; ok is false for a
// paid plan, which never expires.
func FreeExpiresAt(d *appv1alpha1.Database) (time.Time, bool) {
	if !isFreePlan(d.Spec.Plan) {
		return time.Time{}, false
	}
	start := d.CreationTimestamp.UTC()
	if since, err := time.Parse(time.RFC3339, d.Annotations[annotationFreeSince]); err == nil {
		start = since.UTC()
	}
	if start.Before(FreeExpiryRollout) {
		start = FreeExpiryRollout
	}
	return start.Add(FreeLifetime), true
}

// dueFreeExpiryStage is the furthest stage now has reached for expiresAt.
func dueFreeExpiryStage(expiresAt, now time.Time) freeExpiryStage {
	deleteAt := expiresAt.Add(FreeExpiryGrace)
	switch {
	case !now.Before(deleteAt):
		return stageDeleted
	case !now.Before(deleteAt.Add(-freeExpiryFinalWarningLead)):
		return stageFinalWarned
	case !now.Before(expiresAt):
		return stageExpired
	case !now.Before(expiresAt.Add(-freeExpiryWarningLead)):
		return stageWarned
	}
	return stageNone
}

// recordedFreeExpiryStage is the stage the sweeper last claimed for the
// database's current expiresAt. A stage recorded for another expiresAt (the
// clock restarted since) counts as none.
func recordedFreeExpiryStage(d *appv1alpha1.Database, expiresAt time.Time) freeExpiryStage {
	name, at, ok := strings.Cut(d.Annotations[annotationFreeExpiry], "@")
	if !ok || at != expiresAt.Format(time.RFC3339) {
		return stageNone
	}
	for stage := stageWarned; stage <= stageDeleted; stage++ {
		if freeExpiryStageNames[stage] == name {
			return stage
		}
	}
	return stageNone
}

// freeExpired reports a free database the sweeper has expired: it stays
// unavailable until it is upgraded.
func freeExpired(d *appv1alpha1.Database) bool {
	expiresAt, ok := FreeExpiresAt(d)
	return ok && recordedFreeExpiryStage(d, expiresAt) >= stageExpired
}

// expiredResumeError refuses a resume of an expired free database.
func expiredResumeError(d *appv1alpha1.Database) error {
	expiresAt, _ := FreeExpiresAt(d)
	return core.NewBadRequestError(reasonCodeFreeExpired, freeExpiredReason(expiresAt),
		map[string]any{"expiresAt": expiresAt.Format(time.RFC3339)})
}

func freeExpiredReason(expiresAt time.Time) string {
	return fmt.Sprintf("This free database expired on %s. Upgrade it to a paid plan before %s or it will be deleted.",
		expiresAt.Format(time.RFC3339), expiresAt.Add(FreeExpiryGrace).Format(time.RFC3339))
}

// SyncFreeClock applies a plan change to the expiry state, inside the same
// write as the plan. A change to a paid plan drops the clock and the stages,
// and resumes a suspension the expiry made; a change from paid to free starts
// a new 30-day clock at now.
func SyncFreeClock(fromPlan string, d *appv1alpha1.Database, now time.Time) {
	wasFree, isFree := isFreePlan(fromPlan), isFreePlan(d.Spec.Plan)
	if wasFree == isFree {
		return
	}
	if d.Annotations[annotationFreeExpirySuspended] != "" {
		d.Spec.Suspended = false
	}
	delete(d.Annotations, annotationFreeExpiry)
	delete(d.Annotations, annotationFreeExpirySuspended)
	delete(d.Annotations, annotationFreeSince)
	if isFree {
		metav1.SetMetaDataAnnotation(&d.ObjectMeta, annotationFreeSince, now.UTC().Format(time.RFC3339))
	}
}

// FreeExpiryNotifier mails a workspace's owners about one expiry stage.
// notifications.DatabaseExpiryNotifier satisfies it structurally.
type FreeExpiryNotifier interface {
	NotifyFreeDatabaseExpiry(ctx context.Context, workspaceID, databaseID, name, stage string, expiresAt, deleteAt time.Time) error
}

// FreeExpirySweeper drives every free database through the expiry stages.
// Each stage is claimed with an optimistic-concurrency Update of the CR before
// its effect runs, so concurrent bex-api replicas perform it once.
type FreeExpirySweeper struct {
	Service  *Service
	Notifier FreeExpiryNotifier
	// Interval between sweeps; non-positive means five minutes.
	Interval time.Duration
}

// Run sweeps until ctx is canceled. A nil sweeper or Service is a no-op.
func (w *FreeExpirySweeper) Run(ctx context.Context) {
	if w == nil || w.Service == nil || w.Service.Base == nil || w.Service.Client == nil {
		return
	}
	interval := w.Interval
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	core.Poll(ctx, "postgres free expiry", interval, w.SweepOnce)
}

// SweepOnce advances every free database that has reached a new stage.
func (w *FreeExpirySweeper) SweepOnce(ctx context.Context) error {
	var list appv1alpha1.DatabaseList
	if err := w.Service.Client.List(ctx, &list); err != nil {
		return fmt.Errorf("list databases: %w", err)
	}
	var errs []error
	for i := range list.Items {
		if ctx.Err() != nil {
			break
		}
		if err := w.advance(ctx, &list.Items[i]); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", list.Items[i].Name, err))
		}
	}
	return errors.Join(errs...)
}

// advance claims and performs the stage d has reached. Stages that came due
// together (a sweeper that was down for days) collapse into the furthest one,
// so a database is never mailed a warning about an expiry already past.
func (w *FreeExpirySweeper) advance(ctx context.Context, d *appv1alpha1.Database) error {
	if !d.DeletionTimestamp.IsZero() {
		return nil
	}
	expiresAt, ok := FreeExpiresAt(d)
	if !ok {
		return nil
	}
	due := dueFreeExpiryStage(expiresAt, w.Service.Now())
	if due <= recordedFreeExpiryStage(d, expiresAt) {
		return nil
	}
	if due == stageDeleted {
		return w.delete(ctx, d, expiresAt)
	}
	metav1.SetMetaDataAnnotation(&d.ObjectMeta, annotationFreeExpiry, freeExpiryStageNames[due]+"@"+expiresAt.Format(time.RFC3339))
	if due >= stageExpired && !d.Spec.Suspended {
		d.Spec.Suspended = true
		metav1.SetMetaDataAnnotation(&d.ObjectMeta, annotationFreeExpirySuspended, "true")
	}
	// Update, not a merge patch: the resourceVersion makes it the claim. A
	// conflict means another replica (or a user write) got there first, and
	// the next sweep re-reads the database.
	if err := w.Service.Client.Update(ctx, d); err != nil {
		if apierrors.IsConflict(err) || apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	w.record(ctx, d, due, expiresAt)
	return nil
}

// delete removes an expired database through the user delete's path: usage
// retirement first, then a CR delete preconditioned on the version this sweep
// read, so only one replica's delete lands.
func (w *FreeExpirySweeper) delete(ctx context.Context, d *appv1alpha1.Database, expiresAt time.Time) error {
	err := w.Service.deleteDatabase(ctx, d, client.Preconditions{UID: &d.UID, ResourceVersion: &d.ResourceVersion})
	if apierrors.IsConflict(err) || apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	w.record(ctx, d, stageDeleted, expiresAt)
	return nil
}

// record writes the stage's audit row and mails the owners. Mail is
// best-effort: the stage is already claimed, so a failed send is logged and
// never retried into a duplicate.
func (w *FreeExpirySweeper) record(ctx context.Context, d *appv1alpha1.Database, stage freeExpiryStage, expiresAt time.Time) {
	w.Service.RecordDatabaseSystemEffect(ctx, d, freeExpiryAuditVerbs[stage], "free-expiry-sweeper")
	log.Printf("postgres free expiry: %s %s (expires %s)", d.Name, freeExpiryStageNames[stage], expiresAt.Format(time.RFC3339))
	if w.Notifier == nil {
		return
	}
	if err := w.Notifier.NotifyFreeDatabaseExpiry(ctx, d.Labels[core.LabelTenant], d.Name, d.Spec.Name,
		freeExpiryStageNames[stage], expiresAt, expiresAt.Add(FreeExpiryGrace)); err != nil {
		log.Printf("postgres free expiry: notify %s %s: %v", d.Name, freeExpiryStageNames[stage], err)
	}
}
