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
	EventFactServiceHibernated ServiceEventFactType = "service_hibernated"
	EventFactServiceWoken      ServiceEventFactType = "service_woken"
	EventFactServerFailed      ServiceEventFactType = "server_failed"
	// EventFactServerHardwareFailure replaces server_failed when the instance
	// was lost with its node (Ready reason NodeLost, w5/m142) — Render's
	// server_hardware_failure. server_available closes it the same way.
	EventFactServerHardwareFailure ServiceEventFactType = "server_hardware_failure"
	EventFactServerAvailable       ServiceEventFactType = "server_available"
	EventFactBranchChanged         ServiceEventFactType = "branch_changed"
	EventFactBranchDeleted         ServiceEventFactType = "branch_deleted"
	EventFactCommitIgnored         ServiceEventFactType = "commit_ignored"
	EventFactAutoscalingStarted    ServiceEventFactType = "autoscaling_started"
	EventFactAutoscalingEnded      ServiceEventFactType = "autoscaling_ended"
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
	// EventFactPipelineMinutesExhausted is a deploy whose build the operator
	// refused because the workspace has used its plan's pipeline minutes and
	// cannot buy more (w9/m345): Render's pipeline_minutes_exhausted, one per
	// refused deploy, recorded by the reconciler from the same Build condition
	// that closes the deploy build_failed.
	EventFactPipelineMinutesExhausted ServiceEventFactType = "pipeline_minutes_exhausted"
)

var serviceEventFactTypes = map[ServiceEventFactType]bool{
	EventFactImagePullFailed:           true,
	EventFactServiceSuspended:          true,
	EventFactServiceResumed:            true,
	EventFactServiceHibernated:         true,
	EventFactServiceWoken:              true,
	EventFactServerFailed:              true,
	EventFactServerHardwareFailure:     true,
	EventFactServerAvailable:           true,
	EventFactBranchChanged:             true,
	EventFactBranchDeleted:             true,
	EventFactCommitIgnored:             true,
	EventFactAutoscalingStarted:        true,
	EventFactAutoscalingEnded:          true,
	EventFactBuildStarted:              true,
	EventFactBuildEnded:                true,
	EventFactPreDeployStarted:          true,
	EventFactPreDeployEnded:            true,
	EventFactJobRunEnded:               true,
	EventFactCronRunStarted:            true,
	EventFactCronRunEnded:              true,
	EventFactServiceDiskUsageHigh:      true,
	EventFactServiceDiskUsageRecovered: true,
	EventFactPipelineMinutesExhausted:  true,
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
	// EventReasonSuperseded is a deploy canceled because a newer release
	// replaced it (w4/089) — distinct from a user-initiated cancel, which
	// carries no reason code.
	EventReasonSuperseded = "superseded"
	// Why a cron run failed (w4/m114), one per field of the reason object on
	// Render's "Cron Job Run Ended" event.
	EventReasonNonZeroExit = "non_zero_exit"
	EventReasonOOMKilled   = "oom_killed"
	EventReasonEvicted     = "evicted"
	EventReasonTimedOut    = "timed_out"
	// A running service's process that ended on its own with status 0: the
	// earlyExit reason of Render's "Server Failed" event (w8/m158). A crash
	// and a memory kill of a running service use non_zero_exit and oom_killed.
	EventReasonEarlyExit = "early_exit"
)

var serviceEventReasonCodes = map[string]bool{
	"":                          true,
	EventReasonImagePullBackoff: true,
	EventReasonReadinessFailed:  true,
	EventReasonRootDirectory:    true,
	EventReasonBuildFilter:      true,
	EventReasonSkipPhrase:       true,
	EventReasonSuperseded:       true,
	EventReasonNonZeroExit:      true,
	EventReasonOOMKilled:        true,
	EventReasonEvicted:          true,
	EventReasonTimedOut:         true,
	EventReasonEarlyExit:        true,
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
	// ExitCode is a cron_job_run_ended detail (w4/m114): a failed run's
	// container exit status (with ReasonCode non_zero_exit or oom_killed).
	// RunID is the run's public crr- id on both cron_job_run_started (w4/259)
	// and cron_job_run_ended.
	ExitCode *int32
	RunID    string
	// DiskUsage is a service_disk_usage_high/_recovered fact's details
	// (w2/m255); written only through RecordDiskUsageCrossing.
	DiskUsage *DiskUsageDetails
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
	return validateDiskUsageDetails(f)
}

// InsertServiceEventFact appends a fact exactly once. A producer retry with the
// same SourceKey is a successful no-op.
func (s *PGStore) InsertServiceEventFact(ctx context.Context, fact ServiceEventFact) (bool, error) {
	if err := validateServiceEventFact(fact); err != nil {
		return false, err
	}
	if fact.DiskUsage != nil {
		return false, fmt.Errorf("disk usage facts are recorded with RecordDiskUsageCrossing: %w", ErrInvalid)
	}
	if fact.At.IsZero() {
		fact.At = time.Now().UTC()
	}
	tag, err := s.Pool.Exec(ctx, insertServiceEventFactSQL, fact.insertArgs()...)
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
	exits := make([]bool, len(facts))
	for i := range facts {
		if err := validateServiceEventFact(facts[i]); err != nil {
			return err
		}
		if facts[i].DiskUsage != nil {
			return fmt.Errorf("disk usage facts are recorded with RecordDiskUsageCrossing: %w", ErrInvalid)
		}
		if facts[i].At.IsZero() {
			facts[i].At = time.Now().UTC()
		}
		fact := facts[i]
		if fact.isInstanceExit() {
			// An exit read after its outage was already recorded is that
			// outage's cause, and the outage has its event (w8/m158 t009).
			args := append(fact.insertArgs(), fact.At.Add(-instanceExitOutageLag), fact.At.Add(instanceExitOutageFallbackLead))
			batch.Queue(insertInstanceExitFactSQL, args...)
			exits[i] = true
			continue
		}
		batch.Queue(insertServiceEventFactSQL, fact.insertArgs()...)
	}
	if batch.Len() == 0 {
		return nil
	}
	results := s.Pool.SendBatch(ctx, batch)
	var recorded []ServiceEventFact
	for i := range facts {
		tag, err := results.Exec()
		if err != nil {
			_ = results.Close()
			return classify("service event fact", err)
		}
		if exits[i] && tag.RowsAffected() > 0 {
			recorded = append(recorded, facts[i])
		}
	}
	if err := results.Close(); err != nil {
		return classify("service event fact", err)
	}
	for _, exit := range recorded {
		if err := s.pairInstanceExit(ctx, exit); err != nil {
			return classify("service event fact", err)
		}
	}
	return nil
}

// pairInstanceExit gives a recorded exit-caused server_failed its
// server_available when the sampler never saw the outage (w8/m158 t010): a
// crash that restarted between two passes left the checkpoint healthy, so no
// recovery edge ever came and the event stayed open. When the checkpoint's last
// healthy Ready transition is after the exit, the recovery was already seen,
// and the server_available is recorded at that transition. Otherwise the
// checkpoint takes the outage from the exit on, and the next healthy pass
// records the server_available as any outage's. A checkpoint already down has
// its outage, and its recovery pairs with this exit.
func (s *PGStore) pairInstanceExit(ctx context.Context, exit ServiceEventFact) error {
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var availability string
		var healthyAt *time.Time
		err := tx.QueryRow(ctx,
			`SELECT availability, healthy_transition_at FROM service_event_checkpoints WHERE app_id = $1 FOR UPDATE`,
			exit.AppID).Scan(&availability, &healthyAt)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && availability != "healthy" {
			return nil
		}
		if err != nil {
			return err
		}
		if healthyAt != nil && healthyAt.After(exit.At) {
			recovered := ServiceEventFact{
				SourceKey: exit.SourceKey + ":recovered",
				AppID:     exit.AppID,
				Type:      EventFactServerAvailable,
				At:        healthyAt.UTC(),
			}
			_, err := tx.Exec(ctx, insertServiceEventFactSQL, recovered.insertArgs()...)
			return err
		}
		_, err = tx.Exec(ctx,
			`UPDATE service_event_checkpoints SET availability = 'unhealthy', updated_at = $2 WHERE app_id = $1`,
			exit.AppID, exit.At)
		return err
	})
}

// insertArgs is the fact's insertServiceEventFactSQL arguments, in column order.
func (f ServiceEventFact) insertArgs() []any {
	return []any{
		f.SourceKey, f.AppID, f.Type, f.At, f.DeployID, f.Image,
		f.ReasonCode, f.InstanceID, f.FromCount, f.ToCount,
		f.BranchFrom, f.BranchTo, f.CommitID, f.CommitURL, f.Status,
		f.ExitCode, f.RunID,
	}
}

const insertServiceEventFactSQL = `
INSERT INTO service_event_facts (
    source_key, app_id, fact_type, at, deploy_id, image, reason_code,
    instance_id, from_count, to_count, branch_from, branch_to, commit_id, commit_url, status,
    exit_code, run_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
ON CONFLICT (source_key) DO NOTHING`

// BuiltDeployIDs is the set of an App's deploys that actually ran a build —
// the deploys the Events tab shows a Build started entry for. The deploy-log
// narrator reads it so it cannot tell a build story for a deploy the Events
// feed has no build record of (w4/m110 t004): a rollback and a rollout that
// reuses the active artifact both open a deploy row with real timestamps, and
// timestamps alone cannot distinguish them from a build.
//
// Scoped to one App and capped like every other feed read. An App with no
// build facts yields an empty (non-nil) set, never an error.
func (s *PGStore) BuiltDeployIDs(ctx context.Context, appID string, limit int) (map[string]bool, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, builtDeployIDsSQL, appID, limit)
	if err != nil {
		return nil, classify("built deploy ids", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, classify("built deploy ids", err)
		}
		out[id] = true
	}
	return out, classify("built deploy ids", rows.Err())
}

const builtDeployIDsSQL = `
SELECT deploy_id
FROM service_event_facts
WHERE app_id = $1 AND fact_type = 'build_started' AND deploy_id <> ''
ORDER BY at DESC
LIMIT $2`

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
	Suspended bool
	// SpecReadAt is when the pass that made this observation read the desired
	// rows its Suspended comes from (the pass start). Zero means unknown. When
	// the checkpoint changed after it, this observation's flag defers to the
	// checkpoint's for one pass (deferStaleSuspension, w5/m369).
	SpecReadAt time.Time
	// Asleep marks a Hibernated service the operator parked for idleness (its
	// current Ready reason is AutoHibernated), not for a suspension: it stays
	// down until a request wakes it. A Resume that leaves it asleep is
	// recorded when the flag falls, not at a later request's wake (w4/562).
	Asleep       bool
	Availability string
	// AvailabilityObserved distinguishes "do not advance this dimension" from
	// the observed empty state used while a service is hibernated.
	AvailabilityObserved bool
	ReasonCode           string
	// ConcreteFailure marks an unhealthy conclusion backed by a concrete
	// container failure (concreteContainerFailure: CrashLoopBackOff,
	// ImagePullBackOff, CreateContainerConfigError) rather than an instance
	// that is still starting. Only a concrete failure is an availability
	// conclusion while a service comes up from Hibernated (heldAvailability).
	ConcreteFailure bool
	// NodeLost marks an unhealthy conclusion whose Ready reason is NodeLost:
	// the instance went down with its node, so the edge is
	// server_hardware_failure rather than server_failed.
	NodeLost   bool
	InstanceID string
	// ReadyTransitionAt is the Ready condition's LastTransitionTime backing
	// this availability conclusion (w6/m41) — the operator-side timestamp the
	// reconciler's stale-conclusion guard orders an unhealthy edge against the
	// last recorded healthy checkpoint, and the value recorded as that
	// checkpoint's healthy_transition_at when the conclusion is healthy. Zero
	// when availability was not derived from a Ready condition (e.g.
	// hibernation) or the condition carried no timestamp; the guard treats
	// zero as "cannot order" and fails open toward recording. A crash under an
	// open deploy carries the Serving condition's stop instead when it is later
	// than Ready's (the rollout start), and both the edge and the stale guard
	// use it (w5/083).
	ReadyTransitionAt time.Time
	// AvailabilitySuppressed marks a pass whose availability conclusion a
	// reconciler guard blanked: the debounce's first unhealthy pass, or a
	// stale re-read (suppressAvailability). Its phase still records, but it
	// leaves the checkpoint's updated_at alone — the conclusion it carried
	// began before this pass, so this pass is no floor for the next edge. It
	// cannot be inferred from AvailabilityObserved: a pass that concluded
	// nothing (a rollout in progress) is unobserved too, and its phase change
	// must move the floor.
	AvailabilitySuppressed bool
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
	if obs.AvailabilitySuppressed && obs.AvailabilityObserved {
		return nil, fmt.Errorf("observed service state cannot be both suppressed and observed")
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
		var previousChangedAt time.Time
		err := tx.QueryRow(ctx,
			`SELECT service_phase, availability, suspended, updated_at FROM service_event_checkpoints WHERE app_id = $1 FOR UPDATE`,
			obs.AppID).Scan(&previousPhase, &previousAvailability, &previousSuspended, &previousChangedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			// The first observation is held like any other from an empty
			// baseline, so an unhealthy one latches nothing (w4/350).
			baseline := heldAvailability("", "", obs).Availability
			tag, insertErr := tx.Exec(ctx,
				`INSERT INTO service_event_checkpoints (app_id, service_phase, availability, suspended, updated_at, healthy_transition_at)
				 VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (app_id) DO NOTHING`,
				obs.AppID, obs.ServicePhase, baseline, obs.Suspended, obs.At, healthyTransition)
			if insertErr != nil {
				return insertErr
			}
			if tag.RowsAffected() > 0 {
				return nil
			}
			err = tx.QueryRow(ctx,
				`SELECT service_phase, availability, suspended, updated_at FROM service_event_checkpoints WHERE app_id = $1 FOR UPDATE`,
				obs.AppID).Scan(&previousPhase, &previousAvailability, &previousSuspended, &previousChangedAt)
		}
		if err != nil {
			return err
		}

		obs = deferStaleSuspension(obs, previousSuspended, previousChangedAt)
		obs = heldAvailability(previousPhase, previousAvailability, obs)
		availability := previousAvailability
		if obs.AvailabilityObserved {
			availability = obs.Availability
		}
		obs.Availability = availability
		facts := observedStateFacts(obs, previousPhase, previousAvailability, previousSuspended, previousChangedAt)
		for _, fact := range facts {
			// The process exiting is the outage's cause, and the exit already
			// has its own server_failed with the cause on it (w8/m158). The
			// sampled outage adds none; its server_available still follows.
			if fact.Type == EventFactServerFailed && fact.ReasonCode == EventReasonReadinessFailed {
				// The outage starts at its Ready transition, which follows the
				// process's exit within seconds; only an outage dated by the
				// pass that saw it (no transition to trust) looks a minute back.
				lead := instanceExitOutageFallbackLead
				if !obs.ReadyTransitionAt.IsZero() && fact.At.Equal(obs.ReadyTransitionAt) {
					lead = instanceExitOutageLead
				}
				var exited bool
				if err := tx.QueryRow(ctx, instanceExitExplainsOutageSQL, fact.AppID,
					fact.At.Add(-lead), fact.At.Add(instanceExitOutageLag), fact.At).Scan(&exited); err != nil {
					return err
				}
				if exited {
					continue
				}
			}
			if _, err := tx.Exec(ctx, insertServiceEventFactSQL, fact.insertArgs()...); err != nil {
				return err
			}
			inserted = append(inserted, fact)
		}
		checkpointPhase := checkpointServicePhase(previousPhase, obs.ServicePhase)
		checkpointSuspended := checkpointServiceSuspended(previousPhase, obs.ServicePhase, previousSuspended, obs.Suspended, obs.Asleep)
		// IS DISTINCT FROM makes the steady state a no-op instead of a row
		// write: the reconciler records an observation for EVERY app on every
		// resync (30s) and every Kick (after each successful API write), and
		// almost none of them changed. Same shape as SetDeployPreDeployStatus's
		// guard in store.go.
		//
		// updated_at is the floor availabilityEdgeAt orders the next edge
		// after, so it moves with every recorded change except a suppressed
		// pass's (w5/m113): on a readiness loss the debounced first pass still
		// records Running → Deploying, and that write must not lift the floor
		// past the Ready transition the confirmed server_failed carries.
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
			 SET service_phase = $2, availability = $3, suspended = $4,
			     updated_at = CASE WHEN $7 THEN updated_at ELSE $5 END,
			     healthy_transition_at = COALESCE($6, healthy_transition_at)
			 WHERE app_id = $1
			   AND (service_phase, availability, suspended) IS DISTINCT FROM ($2, $3, $4)`,
			obs.AppID, checkpointPhase, availability, checkpointSuspended, obs.At, healthyTransition, obs.AvailabilitySuppressed)
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

// heldAvailability leaves availability unobserved where a pass's conclusion is
// no availability edge, so server_available stays the partner of a recorded
// server_failed (w4/m230, w4/350):
//
//   - Waking. An idle wake or a user Resume reads Deploying with
//     Ready=False/RolloutProgressing until its instance is ready, and no deploy
//     row is open to exempt it, so the reconciler concludes readiness_failed.
//     Recorded, that sighting turned the wake's healthy pass into an unpaired
//     server_available ("Service recovered"). The rule keys on the Hibernated
//     baseline, never on the reason alone: a running service whose only
//     instance goes unready without a deploy still records server_failed. A
//     concrete container failure is a real conclusion even here — a service
//     that cannot wake is down — so it is kept, and observedStateFacts records
//     it.
//   - Sleeping while failed. A recorded failure stays open through the
//     hibernation, so the wake that brings the service back healthy records the
//     recovery that closes it.
//   - Never seen healthy (w4/350). An empty baseline outside a wake has no
//     recorded availability to lose, so an unhealthy conclusion there records
//     no failure, and latching it turned the first healthy pass into an
//     unpaired server_available. That is a first deploy crash-looping after
//     its row closed, or a checkpoint first written mid-outage. The datastore
//     projector refuses the same (nextDatastoreAvailability). The cost: a
//     service whose only healthy window fell between two passes now records
//     neither edge, where it used to record a lone recovery.
func heldAvailability(previousPhase, previousAvailability string, obs ObservedServiceState) ObservedServiceState {
	hibernated := string(appv1alpha1.PhaseHibernated)
	unhealthy := obs.AvailabilityObserved && obs.Availability == "unhealthy"
	waking := previousPhase == hibernated && obs.ServicePhase != hibernated && unhealthy && !obs.ConcreteFailure
	sleepingFailed := obs.ServicePhase == hibernated && previousAvailability == "unhealthy"
	neverHealthy := previousAvailability == "" && previousPhase != hibernated && unhealthy
	if waking || sleepingFailed || neverHealthy {
		obs = clearAvailability(obs)
	}
	return obs
}

// checkpointServiceSuspended pins the recorded suspend reason to whatever phase
// checkpointServicePhase kept. When a Hibernated baseline is held through the
// transient startup phases, the reason it hibernated must be held with it —
// otherwise the eventual Running observation (spec.suspended already back to
// false) reads every user Resume as an idle wake.
//
// The same holds while the phase stays Hibernated (w4/508): a Resume clears
// spec.suspended before the phase moves, so a pass that lands between the flag
// flip and the wake sees Hibernated -> Hibernated with suspended=false. Writing
// that down erased the reason, and the following Hibernated -> Running pass
// read a user Resume as an idle wake (service_woken, no resume push). While
// the phase is Hibernated the recorded reason may therefore rise (suspending
// an already-sleeping App) but never fall; the wake edge consumes it. It
// falls only once the operator has re-parked the resumed service for
// idleness (asleep): that Resume is recorded then, and the service's next
// wake is an idle one (w4/562).
func checkpointServiceSuspended(previousPhase, observedPhase string, previousSuspended, observedSuspended, asleep bool) bool {
	if checkpointServicePhase(previousPhase, observedPhase) != observedPhase {
		return previousSuspended
	}
	hibernated := string(appv1alpha1.PhaseHibernated)
	if observedPhase == hibernated {
		if asleep && !observedSuspended {
			return false
		}
		// Entering Hibernated keeps a reason recorded while still Running, so
		// a Resume that cleared the flag first cannot turn the episode into an
		// idle sleep (w4/527).
		return previousSuspended || observedSuspended
	}
	return observedSuspended
}

// availabilityEdgeAt is when an availability edge actually happened (w4/196):
// the operator's Ready transition backing the conclusion, not the pass that
// noticed it — that pass runs a reconcile poll and a debounce tick later, and
// stamping it understated a live ~45 s outage as 13 s. The transition is used
// only when it falls after the checkpoint's last change (an edge cannot
// predate the state it leaves; Ready's LastTransitionTime moves on status
// flips only, so it can be older than a Serving-backed healthy conclusion, or
// than a rollout's undecided phase write) and not after the observation
// itself; otherwise the observation stands. A guard-suppressed pass is not a
// change for this purpose (ObservedServiceState.AvailabilitySuppressed).
func availabilityEdgeAt(transition, previousChangedAt, observed time.Time) time.Time {
	if transition.IsZero() || !transition.After(previousChangedAt) || transition.After(observed) {
		return observed
	}
	return transition
}

// An instance's exit and a sampled outage are one outage when the exit comes
// at most instanceExitOutageLead before the outage's start (the operator
// writes Ready=False on its pass after the process died) or at most
// instanceExitOutageLag after it (the two clocks, to the second), and no
// server_available lies between them (w8/m158 t009). An outage with no Ready
// transition to date it is dated by the pass that confirmed it, up to a
// resync and a debounce late, so it looks instanceExitOutageFallbackLead back;
// so does an exit read after an outage was recorded, which cannot tell how
// that outage was dated. Either fact may be recorded first; whichever comes
// second adds no server_failed. An exit further away, or one whose outage has
// already recovered, is not this outage's cause: a separate outage half a
// minute after a crash is its own. A crash that restarted before the sampler
// confirmed an outage gets its server_available from pairInstanceExit (t010).
const (
	instanceExitOutageLead         = 15 * time.Second
	instanceExitOutageFallbackLead = time.Minute
	instanceExitOutageLag          = 5 * time.Second
)

// isInstanceExit reports whether f is an exit-caused server_failed of a
// running service (instanceExitFacts), not the sampled outage.
func (f ServiceEventFact) isInstanceExit() bool {
	if f.Type != EventFactServerFailed || f.InstanceID == "" {
		return false
	}
	switch f.ReasonCode {
	case EventReasonNonZeroExit, EventReasonOOMKilled, EventReasonEarlyExit:
		return true
	}
	return false
}

// instanceExitExplainsOutageSQL reports whether app $1 has an exit-caused
// server_failed between $2 and $3 with no server_available between it and
// the outage starting at $4.
const instanceExitExplainsOutageSQL = `SELECT EXISTS (
	SELECT 1 FROM service_event_facts e
	WHERE e.app_id = $1 AND e.fact_type = 'server_failed'
	  AND e.reason_code IN ('non_zero_exit', 'oom_killed', 'early_exit')
	  AND e.at >= $2 AND e.at <= $3
	  AND NOT EXISTS (
	      SELECT 1 FROM service_event_facts a
	      WHERE a.app_id = $1 AND a.fact_type = 'server_available'
	        AND a.at > LEAST(e.at, $4::timestamptz) AND a.at <= GREATEST(e.at, $4::timestamptz)))`

// insertInstanceExitFactSQL is insertServiceEventFactSQL for an exit fact: it
// inserts nothing when a sampled outage starting between $18 and $19, with no
// server_available between it and the exit, already has its server_failed.
const insertInstanceExitFactSQL = `
INSERT INTO service_event_facts (
    source_key, app_id, fact_type, at, deploy_id, image, reason_code,
    instance_id, from_count, to_count, branch_from, branch_to, commit_id, commit_url, status,
    exit_code, run_id
)
SELECT $1, $2, $3, $4::timestamptz, $5, $6, $7, $8, $9::integer, $10::integer, $11, $12, $13, $14, $15, $16::integer, $17
WHERE NOT EXISTS (
    SELECT 1 FROM service_event_facts o
    WHERE o.app_id = $2 AND o.fact_type = 'server_failed' AND o.reason_code = 'readiness_failed'
      AND o.at >= $18 AND o.at <= $19
      AND NOT EXISTS (
          SELECT 1 FROM service_event_facts a
          WHERE a.app_id = $2 AND a.fact_type = 'server_available'
            AND a.at > LEAST(o.at, $4::timestamptz) AND a.at <= GREATEST(o.at, $4::timestamptz)))
ON CONFLICT (source_key) DO NOTHING`

// failureFactType names an availability failure edge: an instance lost with
// its node is Render's server_hardware_failure, anything else server_failed.
func failureFactType(obs ObservedServiceState) ServiceEventFactType {
	if obs.NodeLost {
		return EventFactServerHardwareFailure
	}
	return EventFactServerFailed
}

// deferStaleSuspension keeps the checkpoint's suspension flag when this
// observation read the spec before the checkpoint last changed (w5/m369). A
// Suspend marks the checkpoint as it is accepted; a pass that read the desired
// rows just before that write still carries suspended=false, and taking it at
// its word would turn the mark into a resume that never happened.
//
// The checkpoint keeps one change time (updated_at) for every recorded change,
// so another replica's phase or availability write also defers a flag change
// read before it. Either way the cost is one pass, never a fact: the next pass
// reads the spec after the change. The mark moving updated_at also lifts the
// availability-edge floor (availabilityEdgeAt) to the Suspend, so an outage
// that began moments before it is dated at its observation instead.
func deferStaleSuspension(obs ObservedServiceState, previousSuspended bool, previousChangedAt time.Time) ObservedServiceState {
	if obs.Suspended != previousSuspended && !obs.SpecReadAt.IsZero() && previousChangedAt.After(obs.SpecReadAt) {
		obs.Suspended = previousSuspended
	}
	return obs
}

// RecordServiceSuspensionRequested records an accepted Suspend on the event
// checkpoint at once (w5/m369, from w4/604). The pair Render records for a
// suspension undone quickly, service_suspended then service_resumed, comes from
// observedStateFacts reading a checkpoint that saw the flag rise; a Resume that
// lands before any pass saw the Suspend used to record neither, so no push or
// webhook went out. With the mark, the next pass that sees the flag down
// reports both, the suspension dated at the mark.
//
// Only a Running-side checkpoint is marked. A Hibernated one (a sleeping Free
// service) records the rising flag itself (w4/562), and an already-suspended
// or missing checkpoint has nothing to add. at should follow the Suspend's
// audit row, so the feed lists "Suspended by" before "Service suspended".
func (s *PGStore) RecordServiceSuspensionRequested(ctx context.Context, appID string, at time.Time) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE service_event_checkpoints SET suspended = true, updated_at = $2
		 WHERE app_id = $1 AND NOT suspended AND service_phase <> $3`,
		appID, at.UTC(), string(appv1alpha1.PhaseHibernated))
	if err != nil {
		return fmt.Errorf("record suspension request: %w", err)
	}
	return nil
}

// quickSuspendPair dates a Suspend undone before the service stopped (w4/555).
// The pass that notices the Resume reports both, but the suspension happened
// no later than the pass that first recorded it, the checkpoint's last change.
// The resume keeps this pass's time and stays at least a microsecond, the
// precision timestamptz keeps, after the suspension, so a feed ordered newest
// first reads it on top. Source keys keep this pass's time, so a replay is
// still a no-op.
func quickSuspendPair(suspended, resumed ServiceEventFact, suspensionRecordedAt time.Time) []ServiceEventFact {
	if !suspensionRecordedAt.IsZero() && suspensionRecordedAt.Before(suspended.At) {
		suspended.At = suspensionRecordedAt
	}
	if latest := resumed.At.Add(-time.Microsecond); suspended.At.After(latest) {
		suspended.At = latest
	}
	return []ServiceEventFact{suspended, resumed}
}

func observedStateFacts(obs ObservedServiceState, previousPhase, previousAvailability string, previousSuspended bool, previousChangedAt time.Time) []ServiceEventFact {
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
	// A Suspend undone before the service ever stopped (w4/527): the flag rose
	// and fell while the phase stayed up, so no phase edge will report it.
	// Render records both actions however quickly they follow each other.
	hibernated := string(appv1alpha1.PhaseHibernated)
	if previousSuspended && !obs.Suspended && previousPhase != hibernated && obs.ServicePhase != hibernated {
		facts = append(facts, quickSuspendPair(makeFact("suspended", EventFactServiceSuspended), makeFact("resumed", EventFactServiceResumed), previousChangedAt)...)
	}
	// A sleeping Free service has no phase edge to report either action
	// (w4/562): the owner's Suspend is recorded when the flag rises, and a
	// Resume that leaves it asleep when the operator re-parks it for idleness.
	// A Resume that wakes it is still the wake edge's to report, at Ready.
	if previousPhase == hibernated && obs.ServicePhase == hibernated {
		switch {
		case !previousSuspended && obs.Suspended:
			facts = append(facts, makeFact("suspended", EventFactServiceSuspended))
		case previousSuspended && !obs.Suspended && obs.Asleep:
			facts = append(facts, makeFact("resumed", EventFactServiceResumed))
		}
	}
	if obs.ServicePhase != previousPhase {
		switch obs.ServicePhase {
		case string(appv1alpha1.PhaseHibernated):
			// Both mechanisms land on Hibernated; spec.suspended is what tells
			// them apart. Keep the source-key suffixes distinct so an App that
			// sleeps and is later suspended at the same instant cannot collide.
			// The reason may already be recorded: the pass a Suspend's write
			// kicks sees the service still Running with spec.suspended set, and
			// a Resume can clear the flag before any pass sees Hibernated
			// (w4/527).
			switch {
			case previousSuspended && !obs.Suspended && obs.Asleep:
				// Suspended, and resumed before any pass saw it down, and the
				// operator has already re-parked it for idleness: both
				// actions happened, and no wake will report the resume
				// (w4/562).
				facts = append(facts, quickSuspendPair(makeFact("suspended", EventFactServiceSuspended), makeFact("resumed", EventFactServiceResumed), previousChangedAt)...)
			case obs.Suspended || previousSuspended:
				facts = append(facts, makeFact("suspended", EventFactServiceSuspended))
			default:
				facts = append(facts, makeFact("hibernated", EventFactServiceHibernated))
			}
		case string(appv1alpha1.PhaseRunning):
			if previousPhase == string(appv1alpha1.PhaseHibernated) {
				// Symmetric with the sleep side: how it went down decides how
				// coming back up reads. spec.suspended is false in BOTH cases by
				// the time Running is observed, so only the checkpoint knows.
				fact := makeFact("woken", EventFactServiceWoken)
				if previousSuspended {
					fact = makeFact("resumed", EventFactServiceResumed)
				}
				// The service came back when its instance went Ready, not
				// when this pass noticed — 3–15 s later live (w4/m230). The
				// source key keeps obs.At, so a replay stays a no-op.
				fact.At = availabilityEdgeAt(obs.ReadyTransitionAt, previousChangedAt, obs.At)
				facts = append(facts, fact)
			}
		}
	}
	if obs.Availability != previousAvailability {
		edge := func(suffix string, typ ServiceEventFactType) ServiceEventFact {
			fact := makeFact(suffix, typ)
			fact.At = availabilityEdgeAt(obs.ReadyTransitionAt, previousChangedAt, obs.At)
			return fact
		}
		switch {
		case previousAvailability == "healthy" && obs.Availability == "unhealthy":
			facts = append(facts, edge("failed", failureFactType(obs)))
		case previousAvailability == "" && obs.Availability == "unhealthy" && previousPhase == string(appv1alpha1.PhaseHibernated):
			// A wake or resume whose instance fails concretely
			// (heldAvailability keeps only those): the service is down to
			// every caller, and the recovery that follows needs a failure to
			// pair with (w4/m230). Ready has been False since the service
			// slept, so its transition dates the sleep, not this failure: the
			// observation is the edge's time.
			fact := makeFact("failed", failureFactType(obs))
			facts = append(facts, fact)
		case previousAvailability == "unhealthy" && obs.Availability == "healthy":
			facts = append(facts, edge("available", EventFactServerAvailable))
		}
	}
	return facts
}
