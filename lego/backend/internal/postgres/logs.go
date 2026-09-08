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

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/datastorelogs"
)

// The managed-Postgres log shapes are the shared datastore ones: the wire
// contract and the read path are identical to Key Value's, so they live in one
// place. Aliases keep the Postgres-facing names the adapters register.
type (
	// DatabaseLogQuery is the filter set for QueryDatabaseLogs.
	DatabaseLogQuery = datastorelogs.Query
	// DatabaseLogEntry is one CNPG log line in Render's log shape.
	DatabaseLogEntry = datastorelogs.Entry
	// DatabaseLogQuerySource lets the dedicated Postgres compatibility adapters
	// reuse the generic logs core without importing the logs package here.
	DatabaseLogQuerySource = datastorelogs.Source
)

// QueryDatabaseLogs is the dedicated Postgres adapters' compatibility verb.
// Production delegates every canonical resource id to the generic durable logs
// core; isolated tests can omit that seam and exercise the direct CNPG pod
// path. Results are oldest-first and capped at q.Limit. ErrLogsUnavailable is
// returned when the selected path has no source; unknown instances return
// ErrNotFound.
func (s *Service) QueryDatabaseLogs(ctx context.Context, name string, q DatabaseLogQuery) ([]DatabaseLogEntry, error) {
	if s.DatabaseLogs != nil {
		return s.DatabaseLogs(ctx, name, q)
	}
	d, err := s.fetchDatabaseForRead(ctx, core.RelCanViewLogs, name)
	if err != nil {
		return nil, err
	}
	if s.PodLogs == nil {
		return nil, core.ErrLogsUnavailable
	}
	pods, err := s.DatabasePods(ctx, d.Namespace, d.Name)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(pods))
	for i := range pods {
		names = append(names, pods[i].Name)
	}
	return datastorelogs.Collect(ctx, datastorelogs.Instance{
		// The Database's OWN namespace (ADR043 D8), not the shared one: pod logs
		// are read from where the CNPG pods actually run.
		Namespace: d.Namespace,
		Name:      d.Name,
		Kind:      datastorelogs.KindPostgres,
		Container: core.CNPGPostgresContainer,
		Pods:      names,
		PodLogs:   s.PodLogs,
	}, q)
}
