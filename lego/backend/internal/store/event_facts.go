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
)

// ServiceEventFactType is the closed vocabulary persisted outside deploy and
// audit rows. Keep this in the store package: the control-plane reconciler and
// signed Git webhook produce facts without importing the presentation feature.
type ServiceEventFactType string

const (
	EventFactImagePullFailed    ServiceEventFactType = "image_pull_failed"
	EventFactServiceSuspended   ServiceEventFactType = "service_suspended"
	EventFactServiceResumed     ServiceEventFactType = "service_resumed"
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
)

var serviceEventFactTypes = map[ServiceEventFactType]bool{
	EventFactImagePullFailed:    true,
	EventFactServiceSuspended:   true,
	EventFactServiceResumed:     true,
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
	Availability string
	// AvailabilityObserved distinguishes "do not advance this dimension" from
	// the observed empty state used while a service is hibernated.
	AvailabilityObserved bool
	ReasonCode           string
	InstanceID           string
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

	var inserted []ServiceEventFact
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var previousPhase, previousAvailability string
		err := tx.QueryRow(ctx,
			`SELECT service_phase, availability FROM service_event_checkpoints WHERE app_id = $1 FOR UPDATE`,
			obs.AppID).Scan(&previousPhase, &previousAvailability)
		if errors.Is(err, pgx.ErrNoRows) {
			tag, insertErr := tx.Exec(ctx,
				`INSERT INTO service_event_checkpoints (app_id, service_phase, availability, updated_at)
				 VALUES ($1, $2, $3, $4) ON CONFLICT (app_id) DO NOTHING`,
				obs.AppID, obs.ServicePhase, obs.Availability, obs.At)
			if insertErr != nil {
				return insertErr
			}
			if tag.RowsAffected() > 0 {
				return nil
			}
			err = tx.QueryRow(ctx,
				`SELECT service_phase, availability FROM service_event_checkpoints WHERE app_id = $1 FOR UPDATE`,
				obs.AppID).Scan(&previousPhase, &previousAvailability)
		}
		if err != nil {
			return err
		}

		availability := previousAvailability
		if obs.AvailabilityObserved {
			availability = obs.Availability
		}
		obs.Availability = availability
		facts := observedStateFacts(obs, previousPhase, previousAvailability)
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
		_, err = tx.Exec(ctx,
			`UPDATE service_event_checkpoints
			 SET service_phase = $2, availability = $3, updated_at = $4
			 WHERE app_id = $1`,
			obs.AppID, checkpointPhase, availability, obs.At)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("record observed service state: %w", err)
	}
	return inserted, nil
}

// checkpointServicePhase keeps a suspended edge pending through the transient
// startup phases that can appear between a resume request and Running. Without
// this, a polling reconciler that samples Hibernated -> Deploying -> Running
// forgets the Hibernated baseline and loses the real service_resumed edge.
func checkpointServicePhase(previous, observed string) string {
	if previous == "Hibernated" && observed != "Hibernated" && observed != "Running" {
		return previous
	}
	return observed
}

func observedStateFacts(obs ObservedServiceState, previousPhase, previousAvailability string) []ServiceEventFact {
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
		case "Hibernated":
			facts = append(facts, makeFact("suspended", EventFactServiceSuspended))
		case "Running":
			if previousPhase == "Hibernated" {
				facts = append(facts, makeFact("resumed", EventFactServiceResumed))
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
