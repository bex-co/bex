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
	"time"

	"github.com/jackc/pgx/v5"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ServiceEventFactType is the closed vocabulary persisted outside deploy and
// audit rows. Keep this in the store package: the control-plane reconciler and
// signed Git webhook produce facts without importing the presentation feature.
type ServiceEventFactType string

const (
	EventFactImagePullFailed  ServiceEventFactType = "image_pull_failed"
	EventFactServiceSuspended ServiceEventFactType = "service_suspended"
	EventFactServiceResumed   ServiceEventFactType = "service_resumed"
	// Free-tier idle auto-sleep (w6/m47). Deliberately NOT the suspended/resumed
	// pair: those stay exclusively user-driven, so a webhook or push subscriber
	// watching for an unexpected suspension is not woken by every routine sleep
	// cycle of a free service.
	EventFactServiceHibernated  ServiceEventFactType = "service_hibernated"
	EventFactServiceWoken       ServiceEventFactType = "service_woken"
	EventFactServerFailed       ServiceEventFactType = "server_failed"
	EventFactServerAvailable    ServiceEventFactType = "server_available"
	EventFactBranchChanged      ServiceEventFactType = "branch_changed"
	EventFactBranchDeleted      ServiceEventFactType = "branch_deleted"
	EventFactCommitIgnored      ServiceEventFactType = "commit_ignored"
	EventFactAutoscalingStarted ServiceEventFactType = "autoscaling_started"
	EventFactAutoscalingEnded   ServiceEventFactType = "autoscaling_ended"
	// Deploy-lifecycle facts (w7/m66): the build, pre-deploy, and one-off-job
	// beats Render shows as distinct timeline entries. The *_ended kinds carry a
	// closed Status (succeeded|failed|canceled); the operator observes them
	// through the same control-plane reconciler path image_pull_failed rides.
	EventFactBuildStarted     ServiceEventFactType = "build_started"
	EventFactBuildEnded       ServiceEventFactType = "build_ended"
	EventFactPreDeployStarted ServiceEventFactType = "pre_deploy_started"
	EventFactPreDeployEnded   ServiceEventFactType = "pre_deploy_ended"
	EventFactJobRunEnded      ServiceEventFactType = "job_run_ended"
	EventFactCronRunStarted   ServiceEventFactType = "cron_job_run_started"
	EventFactCronRunEnded     ServiceEventFactType = "cron_job_run_ended"
)

var serviceEventFactTypes = map[ServiceEventFactType]bool{
	EventFactImagePullFailed:    true,
	EventFactServiceSuspended:   true,
	EventFactServiceResumed:     true,
	EventFactServiceHibernated:  true,
	EventFactServiceWoken:       true,
	EventFactServerFailed:       true,
	EventFactServerAvailable:    true,
	EventFactBranchChanged:      true,
	EventFactBranchDeleted:      true,
	EventFactCommitIgnored:      true,
	EventFactAutoscalingStarted: true,
	EventFactAutoscalingEnded:   true,
	EventFactBuildStarted:       true,
	EventFactBuildEnded:         true,
	EventFactPreDeployStarted:   true,
	EventFactPreDeployEnded:     true,
	EventFactJobRunEnded:        true,
	EventFactCronRunStarted:     true,
	EventFactCronRunEnded:       true,
}

// Closed lifecycle-step outcomes for a *_ended fact's Status column — the same
// structural discipline as reason_code: a step outcome is never an arbitrary
// string. Mirror service_event_facts' status CHECK (migration 0057).
const (
	EventStatusSucceeded = "succeeded"
	EventStatusFailed    = "failed"
	EventStatusCanceled  = "canceled"
)

var serviceEventStatuses = map[string]bool{
	"":                   true,
	EventStatusSucceeded: true,
	EventStatusFailed:    true,
	EventStatusCanceled:  true,
}

const (
	EventReasonImagePullBackoff = "image_pull_backoff"
	EventReasonReadinessFailed  = "readiness_failed"
	EventReasonRootDirectory    = "root_directory"
	EventReasonBuildFilter      = "build_filter"
	EventReasonSkipPhrase       = "skip_phrase"
)

var serviceEventReasonCodes = map[string]bool{
	"":                          true,
	EventReasonImagePullBackoff: true,
	EventReasonReadinessFailed:  true,
	EventReasonRootDirectory:    true,
	EventReasonBuildFilter:      true,
	EventReasonSkipPhrase:       true,
}

// ServiceEventFact is a closed, non-secret event record. SourceKey is a stable
// producer identity used for idempotency; it is never exposed directly.
type ServiceEventFact struct {
	SourceKey  string
	AppID      string
	Type       ServiceEventFactType
	At         time.Time
	DeployID   string
	Image      string
	ReasonCode string
	InstanceID string
	FromCount  *int32
	ToCount    *int32
	BranchFrom string
	BranchTo   string
	CommitID   string
	CommitURL  string
	// Status is the terminal outcome of a lifecycle-step fact (build_ended,
	// pre_deploy_ended, job_run_ended): one of EventStatus* or "" for the
	// started/observed kinds that have no outcome. Closed set (w7/m66).
	Status string
}

// EventFactWriter is the narrow producer seam used by apps and webhook code.
type EventFactWriter interface {
	InsertServiceEventFact(ctx context.Context, fact ServiceEventFact) (bool, error)
}

func validateServiceEventFact(f ServiceEventFact) error {
	if f.SourceKey == "" || f.AppID == "" || !serviceEventFactTypes[f.Type] {
		return fmt.Errorf("invalid service event fact identity/type")
	}
	if !serviceEventReasonCodes[f.ReasonCode] {
		return fmt.Errorf("invalid service event reason code %q", f.ReasonCode)
	}
	if !serviceEventStatuses[f.Status] {
		return fmt.Errorf("invalid service event status %q", f.Status)
	}
	terminal := f.Type == EventFactBuildEnded || f.Type == EventFactPreDeployEnded ||
		f.Type == EventFactJobRunEnded || f.Type == EventFactCronRunEnded
	if terminal && f.Status == "" {
		return fmt.Errorf("terminal service event fact %q requires status", f.Type)
	}
	if !terminal && f.Status != "" {
		return fmt.Errorf("nonterminal service event fact %q cannot carry status", f.Type)
	}
	return nil
}

// InsertServiceEventFact appends a fact exactly once. A producer retry with the
// same SourceKey is a successful no-op.
func (s *PGStore) InsertServiceEventFact(ctx context.Context, fact ServiceEventFact) (bool, error) {
	if err := validateServiceEventFact(fact); err != nil {
		return false, err
	}
	if fact.At.IsZero() {
		fact.At = time.Now().UTC()
	}
	tag, err := s.Pool.Exec(ctx, insertServiceEventFactSQL,
		fact.SourceKey, fact.AppID, fact.Type, fact.At, fact.DeployID, fact.Image,
		fact.ReasonCode, fact.InstanceID, fact.FromCount, fact.ToCount,
		fact.BranchFrom, fact.BranchTo, fact.CommitID, fact.CommitURL, fact.Status)
	if err != nil {
		return false, classify("service event fact", err)
	}
	return tag.RowsAffected() > 0, nil
}

// InsertServiceEventFacts appends a producer's bounded observation set in one
// database round trip. Each fact keeps InsertServiceEventFact's independent
// source-key idempotency; validation completes before anything is queued.
func (s *PGStore) InsertServiceEventFacts(ctx context.Context, facts []ServiceEventFact) error {
	batch := &pgx.Batch{}
	for i := range facts {
		if err := validateServiceEventFact(facts[i]); err != nil {
			return err
		}
		if facts[i].At.IsZero() {
			facts[i].At = time.Now().UTC()
		}
		fact := facts[i]
		batch.Queue(insertServiceEventFactSQL,
			fact.SourceKey, fact.AppID, fact.Type, fact.At, fact.DeployID, fact.Image,
			fact.ReasonCode, fact.InstanceID, fact.FromCount, fact.ToCount,
			fact.BranchFrom, fact.BranchTo, fact.CommitID, fact.CommitURL, fact.Status)
	}
	if batch.Len() == 0 {
		return nil
	}
	results := s.Pool.SendBatch(ctx, batch)
	for range facts {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return classify("service event fact", err)
		}
	}
	if err := results.Close(); err != nil {
		return classify("service event fact", err)
	}
	return nil
}

const insertServiceEventFactSQL = `
INSERT INTO service_event_facts (
    source_key, app_id, fact_type, at, deploy_id, image, reason_code,
    instance_id, from_count, to_count, branch_from, branch_to, commit_id, commit_url, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
ON CONFLICT (source_key) DO NOTHING`

// ObservedServiceState is the small level-triggered snapshot the control-plane
// reconciler derives from typed App status. Availability is empty outside a
// running service, healthy, or unhealthy.
type ObservedServiceState struct {
	AppID        string
	At           time.Time
	ServicePhase string
	// Suspended is the App's spec.suspended at observation time — the ONLY
	// signal that separates a user-driven suspension from free-tier idle
	// auto-hibernation, since both observe the same Hibernated phase (w6/m47).
	Suspended    bool
	Availability string
	// AvailabilityObserved distinguishes "do not advance this dimension" from
	// the observed empty state used while a service is hibernated.
	AvailabilityObserved bool
	ReasonCode           string
	InstanceID           string
	// ReadyTransitionAt is the Ready condition's LastTransitionTime backing
	// this availability conclusion (w6/m41) — the operator-side timestamp the
	// reconciler's stale-conclusion guard orders an unhealthy edge against the
	// last recorded healthy checkpoint, and the value recorded as that
	// checkpoint's healthy_transition_at when the conclusion is healthy. Zero
	// when availability was not derived from a Ready condition (e.g.
	// hibernation) or the condition carried no timestamp; the guard treats
	// zero as "cannot order" and fails open toward recording.
	ReadyTransitionAt time.Time
}

// RecordObservedServiceState atomically advances a service checkpoint and
// appends any phase/availability edges it crossed. The first observation is a
// baseline; replaying the same observation is a no-op.
func (s *PGStore) RecordObservedServiceState(ctx context.Context, obs ObservedServiceState) ([]ServiceEventFact, error) {
	if obs.AppID == "" {
		return nil, fmt.Errorf("observed service state requires app id")
	}
	if obs.Availability != "" && obs.Availability != "healthy" && obs.Availability != "unhealthy" {
		return nil, fmt.Errorf("invalid availability %q", obs.Availability)
	}
	if obs.ReasonCode != "" && obs.ReasonCode != EventReasonReadinessFailed {
		return nil, fmt.Errorf("invalid observed reason code %q", obs.ReasonCode)
	}
	if obs.At.IsZero() {
		obs.At = time.Now().UTC()
	}
	// healthyTransition is recorded as the checkpoint's healthy_transition_at —
	// the reference rejectStaleUnhealthy orders future unhealthy edges against.
	// It is set only by an observed healthy conclusion; nullTime maps a missing
	// condition timestamp to SQL NULL, and anything else leaves the column
	// alone (NULL included), so a checkpoint whose transition time is unknown
	// fails open toward recording.
	var healthyTransition *time.Time
	if obs.AvailabilityObserved && obs.Availability == "healthy" {
		healthyTransition = nullTime(obs.ReadyTransitionAt.UTC())
	}

	var inserted []ServiceEventFact
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var previousPhase, previousAvailability string
		var previousSuspended bool
		err := tx.QueryRow(ctx,
			`SELECT service_phase, availability, suspended FROM service_event_checkpoints WHERE app_id = $1 FOR UPDATE`,
			obs.AppID).Scan(&previousPhase, &previousAvailability, &previousSuspended)
		if errors.Is(err, pgx.ErrNoRows) {
			tag, insertErr := tx.Exec(ctx,
				`INSERT INTO service_event_checkpoints (app_id, service_phase, availability, suspended, updated_at, healthy_transition_at)
				 VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (app_id) DO NOTHING`,
				obs.AppID, obs.ServicePhase, obs.Availability, obs.Suspended, obs.At, healthyTransition)
			if insertErr != nil {
				return insertErr
			}
			if tag.RowsAffected() > 0 {
				return nil
			}
			err = tx.QueryRow(ctx,
				`SELECT service_phase, availability, suspended FROM service_event_checkpoints WHERE app_id = $1 FOR UPDATE`,
				obs.AppID).Scan(&previousPhase, &previousAvailability, &previousSuspended)
		}
		if err != nil {
			return err
		}

		availability := previousAvailability
		if obs.AvailabilityObserved {
			availability = obs.Availability
		}
		obs.Availability = availability
		facts := observedStateFacts(obs, previousPhase, previousAvailability, previousSuspended)
		for _, fact := range facts {
			if _, err := tx.Exec(ctx, insertServiceEventFactSQL,
				fact.SourceKey, fact.AppID, fact.Type, fact.At, fact.DeployID, fact.Image,
				fact.ReasonCode, fact.InstanceID, fact.FromCount, fact.ToCount,
				fact.BranchFrom, fact.BranchTo, fact.CommitID, fact.CommitURL, fact.Status); err != nil {
				return err
			}
			inserted = append(inserted, fact)
		}
		checkpointPhase := checkpointServicePhase(previousPhase, obs.ServicePhase)
		checkpointSuspended := checkpointServiceSuspended(previousPhase, obs.ServicePhase, previousSuspended, obs.Suspended)
		// IS DISTINCT FROM makes the steady state a no-op instead of a row
		// write: the reconciler records an observation for EVERY app on every
		// resync (30s) and every Kick (after each successful API write), and
		// almost none of them changed. Without the guard, updated_at alone
		// still moved, so each pass cost one real write + WAL + index churn per
		// app. Nothing reads updated_at — it is write-only bookkeeping — so
		// letting it stop advancing on a no-change pass loses nothing.
		// Same shape as SetDeployPreDeployStatus's guard in store.go.
		//
		// healthy_transition_at moves only inside this change-guarded write, so
		// a healthy conclusion re-observed with an older or missing timestamp
		// never erases a newer recorded one (COALESCE keeps the stored value
		// when this pass has none to offer).
		// suspended joins the guard tuple, not just the SET list: a user who
		// suspends an already auto-hibernated App changes only this flag (the
		// phase is Hibernated either way), and losing that write would make the
		// later wake edge report service_woken for a real user Resume.
		_, err = tx.Exec(ctx,
			`UPDATE service_event_checkpoints
			 SET service_phase = $2, availability = $3, suspended = $4, updated_at = $5,
			     healthy_transition_at = COALESCE($6, healthy_transition_at)
			 WHERE app_id = $1
			   AND (service_phase, availability, suspended) IS DISTINCT FROM ($2, $3, $4)`,
			obs.AppID, checkpointPhase, availability, checkpointSuspended, obs.At, healthyTransition)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("record observed service state: %w", err)
	}
	return inserted, nil
}

// LastHealthyTransitionAt returns the Ready=True transition time recorded with
// the service's CURRENT healthy checkpoint — the reference the reconciler's
// stale-conclusion guard (w6/m41, rejectStaleUnhealthy) orders an unhealthy
// edge against. Zero when there is no healthy checkpoint right now or its
// transition time is unknown (pre-migration row, timestamp-less condition):
// the guard cannot order and must fail open toward recording real outages,
// never toward silence.
func (s *PGStore) LastHealthyTransitionAt(ctx context.Context, appID string) (time.Time, error) {
	var at *time.Time
	err := s.Pool.QueryRow(ctx,
		`SELECT healthy_transition_at FROM service_event_checkpoints
		 WHERE app_id = $1 AND availability = 'healthy'`,
		appID).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("last healthy transition: %w", err)
	}
	if at == nil {
		return time.Time{}, nil
	}
	return at.UTC(), nil
}

// checkpointServicePhase keeps a suspended edge pending through the transient
// startup phases that can appear between a resume request and Running. Without
// this, a polling reconciler that samples Hibernated -> Deploying -> Running
// forgets the Hibernated baseline and loses the real service_resumed edge.
func checkpointServicePhase(previous, observed string) string {
	if previous == string(appv1alpha1.PhaseHibernated) && observed != string(appv1alpha1.PhaseHibernated) && observed != string(appv1alpha1.PhaseRunning) {
		return previous
	}
	return observed
}

// checkpointServiceSuspended pins the recorded suspend reason to whatever phase
// checkpointServicePhase kept. When a Hibernated baseline is held through the
// transient startup phases, the reason it hibernated must be held with it —
// otherwise the eventual Running observation (spec.suspended already back to
// false) reads every user Resume as an idle wake.
func checkpointServiceSuspended(previousPhase, observedPhase string, previousSuspended, observedSuspended bool) bool {
	if checkpointServicePhase(previousPhase, observedPhase) != observedPhase {
		return previousSuspended
	}
	return observedSuspended
}

func observedStateFacts(obs ObservedServiceState, previousPhase, previousAvailability string, previousSuspended bool) []ServiceEventFact {
	makeFact := func(suffix string, typ ServiceEventFactType) ServiceEventFact {
		return ServiceEventFact{
			SourceKey:  fmt.Sprintf("observed:%s:%s:%d", obs.AppID, suffix, obs.At.UnixNano()),
			AppID:      obs.AppID,
			Type:       typ,
			At:         obs.At,
			ReasonCode: obs.ReasonCode,
			InstanceID: obs.InstanceID,
		}
	}
	var facts []ServiceEventFact
	if obs.ServicePhase != previousPhase {
		switch obs.ServicePhase {
		case string(appv1alpha1.PhaseHibernated):
			// Both mechanisms land on Hibernated; spec.suspended is what tells
			// them apart. Keep the source-key suffixes distinct so an App that
			// sleeps and is later suspended at the same instant cannot collide.
			if obs.Suspended {
				facts = append(facts, makeFact("suspended", EventFactServiceSuspended))
			} else {
				facts = append(facts, makeFact("hibernated", EventFactServiceHibernated))
			}
		case string(appv1alpha1.PhaseRunning):
			if previousPhase == string(appv1alpha1.PhaseHibernated) {
				// Symmetric with the sleep side: how it went down decides how
				// coming back up reads. spec.suspended is false in BOTH cases by
				// the time Running is observed, so only the checkpoint knows.
				if previousSuspended {
					facts = append(facts, makeFact("resumed", EventFactServiceResumed))
				} else {
					facts = append(facts, makeFact("woken", EventFactServiceWoken))
				}
			}
		}
	}
	if obs.Availability != previousAvailability {
		switch {
		case previousAvailability == "healthy" && obs.Availability == "unhealthy":
			facts = append(facts, makeFact("failed", EventFactServerFailed))
		case previousAvailability == "unhealthy" && obs.Availability == "healthy":
			facts = append(facts, makeFact("available", EventFactServerAvailable))
		}
	}
	return facts
}
