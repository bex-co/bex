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
	"fmt"
)

// CronRunActor names who triggered and who canceled one cron run (w4/631),
// as identity subjects. Either is empty when no user did: a scheduled run has
// no TriggeredBy, a run nobody canceled no CanceledBy.
type CronRunActor struct {
	TriggeredBy string
	CanceledBy  string
}

// CronRunActors records the user behind a cron run's trigger and cancel,
// keyed by the run's public crr- id. The Kubernetes Job keeps no caller, so
// bex-api writes the actor here at the request, where the caller is known.
type CronRunActors interface {
	RecordCronRunTrigger(ctx context.Context, appID, runID, subject string) error
	// RecordCronRunCancel names the latest canceler: a cancel that did not take
	// (the run finished first) is overwritten by the one that did.
	RecordCronRunCancel(ctx context.Context, appID, runID, subject string) error
	// CronRunActors returns the recorded actors of the given runs; a run with
	// none is absent.
	CronRunActors(ctx context.Context, appID string, runIDs []string) (map[string]CronRunActor, error)
}

func (s *PGStore) RecordCronRunTrigger(ctx context.Context, appID, runID, subject string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO cron_run_actors (app_id, run_id, triggered_by) VALUES ($1, $2, $3)
		ON CONFLICT (app_id, run_id) DO UPDATE SET triggered_by = EXCLUDED.triggered_by`,
		appID, runID, subject)
	if err != nil {
		return fmt.Errorf("record trigger of cron run %s: %w", runID, err)
	}
	return nil
}

func (s *PGStore) RecordCronRunCancel(ctx context.Context, appID, runID, subject string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO cron_run_actors (app_id, run_id, canceled_by) VALUES ($1, $2, $3)
		ON CONFLICT (app_id, run_id) DO UPDATE SET canceled_by = EXCLUDED.canceled_by`,
		appID, runID, subject)
	if err != nil {
		return fmt.Errorf("record cancel of cron run %s: %w", runID, err)
	}
	return nil
}

func (s *PGStore) CronRunActors(ctx context.Context, appID string, runIDs []string) (map[string]CronRunActor, error) {
	if len(runIDs) == 0 {
		return nil, nil
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT run_id, triggered_by, canceled_by FROM cron_run_actors
		WHERE app_id = $1 AND run_id = ANY($2::text[])`, appID, runIDs)
	if err != nil {
		return nil, fmt.Errorf("cron run actors of %s: %w", appID, err)
	}
	defer rows.Close()
	out := map[string]CronRunActor{}
	for rows.Next() {
		var (
			runID string
			actor CronRunActor
		)
		if err := rows.Scan(&runID, &actor.TriggeredBy, &actor.CanceledBy); err != nil {
			return nil, err
		}
		out[runID] = actor
	}
	return out, rows.Err()
}
