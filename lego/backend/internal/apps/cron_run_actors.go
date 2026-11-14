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

package apps

import (
	"context"
	"log"
	"slices"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// EmailLookup resolves identity subjects to their emails; a subject without
// one is absent.
type EmailLookup interface {
	LookupEmails(ctx context.Context, subjects []string) (map[string]string, error)
}

// recordCronRunTrigger names the caller as the user who triggered runID
// (w4/631). A caller-less request (none in production) and store-less mode
// record nothing, and the run reads with no triggeredBy.
func (s *Service) recordCronRunTrigger(ctx context.Context, appID, runID string) error {
	subject := core.SubjectFrom(ctx)
	if s.CronRunActors == nil || appID == "" || subject == "" {
		return nil
	}
	return s.CronRunActors.RecordCronRunTrigger(ctx, appID, runID, subject)
}

// recordCronRunCancel names the caller as the user who canceled runID.
func (s *Service) recordCronRunCancel(ctx context.Context, appID, runID string) error {
	subject := core.SubjectFrom(ctx)
	if s.CronRunActors == nil || appID == "" || subject == "" {
		return nil
	}
	return s.CronRunActors.RecordCronRunCancel(ctx, appID, runID, subject)
}

// withCronRunActors fills each run's triggeredBy and canceledBy with the email
// of the user recorded for it (w4/631). Render types both as a bare "user"
// string; bex answers the email, which is what Render's dashboard shows for
// the person (ADR038). canceledBy is named only on a run that did end
// canceled: a cancel that lost the race to the run's own finish canceled
// nothing. Best-effort, like the events feed's user objects: a lookup failure
// leaves the runs as they were rather than failing the read.
func (s *Service) withCronRunActors(ctx context.Context, appID string, runs []CronRunView) []CronRunView {
	if s.CronRunActors == nil || appID == "" || len(runs) == 0 {
		return runs
	}
	runIDs := make([]string, len(runs))
	for i, run := range runs {
		runIDs[i] = run.ID
	}
	actors, err := s.CronRunActors.CronRunActors(ctx, appID, runIDs)
	if err != nil {
		log.Printf("apps: cron run actors of %s: %v", appID, err)
		return runs
	}
	if len(actors) == 0 {
		return runs
	}
	var subjects []string
	for _, actor := range actors {
		for _, subject := range []string{actor.TriggeredBy, actor.CanceledBy} {
			if subject != "" && !slices.Contains(subjects, subject) {
				subjects = append(subjects, subject)
			}
		}
	}
	if s.ActorEmails == nil || len(subjects) == 0 {
		return runs
	}
	emails, err := s.ActorEmails.LookupEmails(ctx, subjects)
	if err != nil {
		log.Printf("apps: cron run actor emails of %s: %v", appID, err)
		return runs
	}
	out := slices.Clone(runs)
	for i := range out {
		actor, ok := actors[out[i].ID]
		if !ok {
			continue
		}
		out[i].TriggeredBy = emails[actor.TriggeredBy]
		if out[i].Status == cronRunCanceled {
			out[i].CanceledBy = emails[actor.CanceledBy]
		}
	}
	return out
}
