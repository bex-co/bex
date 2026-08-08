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

package keyvalue

import (
	"context"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/datastorelogs"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// The managed Key Value log shapes are the shared datastore ones: the wire
// contract and the read path are identical to Postgres's, so they live in one
// place. Aliases keep the Key Value-facing names the adapters register.
type (
	// KeyValueLogQuery is the filter set for QueryKeyValueLogs.
	KeyValueLogQuery = datastorelogs.Query
	// KeyValueLogEntry is one Valkey log line in Render's log shape.
	KeyValueLogEntry = datastorelogs.Entry
	// KeyValueLogQuerySource lets the dedicated Key Value compatibility adapters
	// reuse the generic logs core without importing the logs package here.
	KeyValueLogQuerySource = datastorelogs.Source
)

// QueryKeyValueLogs is the dedicated Key Value adapters' compatibility verb.
// Typed red- ids delegate to the generic durable logs core in production;
// isolated tests retain the direct Valkey pod path.
// Results are oldest-first and capped at q.Limit. ErrLogsUnavailable is
// returned when the selected path has no source; unknown instances return
// ErrNotFound.
func (s *Service) QueryKeyValueLogs(ctx context.Context, name string, q KeyValueLogQuery) ([]KeyValueLogEntry, error) {
	if kind, ok := ids.KindOf(name); ok && kind == ids.KeyValue && s.KeyValueLogs != nil {
		return s.KeyValueLogs(ctx, name, q)
	}
	kv, err := s.fetchKeyValue(ctx, core.RelCanViewLogs, name)
	if err != nil {
		return nil, err
	}
	if s.PodLogs == nil {
		return nil, core.ErrLogsUnavailable
	}
	pods, err := s.KeyValuePods(ctx, kv.Name)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(pods))
	for i := range pods {
		names = append(names, pods[i].Name)
	}
	return datastorelogs.Collect(ctx, datastorelogs.Instance{
		Namespace: s.Namespace,
		Name:      kv.Name,
		Kind:      datastorelogs.KindKeyValue,
		Container: core.ValkeyContainer,
		Pods:      names,
		PodLogs:   s.PodLogs,
	}, q)
}
