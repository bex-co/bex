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

package objectstorage

import (
	"context"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// RecordIngress adds bytes a workspace sent into object storage to its
// object_storage_ingress_bytes shadow meter (ADR112 D8); PutStream calls it with
// the exact length of a completed put. It sums in memory and never fails or
// blocks the request; it does nothing until SetUsageSink turns the meters on.
func (s *Service) RecordIngress(ctx context.Context, workspaceID string, bytes int64) {
	s.record(workspaceID, store.UsageKindObjectStorageIngressBytes, bytes)
}

// RecordEgress adds bytes a workspace read out of object storage to its
// object_storage_egress_bytes shadow meter (ADR112 D8): OpenObject calls it
// with the bytes a get streamed, and MintGet with the size of the object a GET
// URL was minted for (an estimate). Like RecordIngress it never fails the
// request.
func (s *Service) RecordEgress(ctx context.Context, workspaceID string, bytes int64) {
	s.record(workspaceID, store.UsageKindObjectStorageEgressBytes, bytes)
}

func (s *Service) record(workspaceID, kind string, bytes int64) {
	if s != nil && s.meter != nil {
		s.meter.add(workspaceID, kind, bytes, time.Now())
	}
}
