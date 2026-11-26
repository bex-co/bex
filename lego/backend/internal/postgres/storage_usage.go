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
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// GraphQL's storageUsedPercent, storageUsed and storageTotal on Database are
// the fields Render's dashboard `database` query reads for its almost-full
// warning (w3/m656, docs/render-artifacts/postgres-dashboard.md § Almost-full
// warning). Render's public REST and MCP objects have no usage field, so
// neither do bex's.

// StorageUsedSource reads a database's current used bytes: the fullest of its
// instance volumes. ok is false when there is no recent sample.
type StorageUsedSource func(ctx context.Context, namespace, database string) (usedBytes float64, ok bool, err error)

// storageReadTimeout bounds the usage read, so a slow Prometheus delays a
// database page by at most this much before the fields read null. A var so a
// test need not wait it out.
var storageReadTimeout = 3 * time.Second

// bytesPerGiB sizes DiskSizeGB, which the operator provisions as "<n>Gi".
const bytesPerGiB = 1 << 30

// storageUsage is one database's usage read, shared by the fields that need
// it: made once, and only when a query selects one of them.
type storageUsage struct {
	s                *Service
	namespace, owner string
	name             string

	once sync.Once
	used float64
	ok   bool
}

func (s *Service) storageUsage(namespace, owner, name string) *storageUsage {
	if s.StorageUsed == nil {
		return nil
	}
	return &storageUsage{s: s, namespace: namespace, owner: owner, name: name}
}

// usedBytes is the database's used bytes, or false when they cannot be told:
// no recent sample, a failed or slow read, or a caller who may not view the
// database's activity (its Metrics page reads the same series under
// can_view_activity).
func (u *storageUsage) usedBytes(ctx context.Context) (float64, bool) {
	if u == nil {
		return 0, false
	}
	u.once.Do(func() { u.used, u.ok = u.read(ctx) })
	return u.used, u.ok
}

func (u *storageUsage) read(ctx context.Context) (float64, bool) {
	workspace := u.owner
	if workspace == "" {
		workspace = u.s.WorkspaceOrDefault(ctx)
	}
	if !u.s.CanDecisionOn(ctx, core.RelCanViewActivity, core.WorkspaceObject(workspace)).Allowed() {
		return 0, false
	}
	ctx, cancel := context.WithTimeout(ctx, storageReadTimeout)
	defer cancel()
	used, ok, err := u.s.StorageUsed(ctx, u.namespace, u.name)
	if err != nil {
		log.Printf("postgres storage usage: %s: %v", u.name, err)
		return 0, false
	}
	return used, ok
}

// storageTotalBytes is the database's logical size, the capacity its Metrics
// page draws (w4/m91), not the larger physical volume.
func storageTotalBytes(v PostgresView) (int64, bool) {
	return int64(v.DiskSizeGB) * bytesPerGiB, v.DiskSizeGB > 0
}

// The storage fields resolve lazily, so a read that selects none of them
// makes no usage read. Byte counts are decimal strings, which Render's
// dashboard reads with Number(): a size past 2 GiB does not fit GraphQL's Int.

func resolveStorageTotal(p graphql.ResolveParams) (any, error) {
	total, ok := storageTotalBytes(p.Source.(PostgresView))
	if !ok {
		return nil, nil
	}
	return strconv.FormatInt(total, 10), nil
}

func resolveStorageUsed(p graphql.ResolveParams) (any, error) {
	used, ok := p.Source.(PostgresView).storage.usedBytes(p.Context)
	if !ok {
		return nil, nil
	}
	return strconv.FormatInt(int64(used), 10), nil
}

// resolveStorageUsedPercent is used over total. Below 10 GiB the physical
// volume is larger than the logical size, so a database can pass 100.
func resolveStorageUsedPercent(p graphql.ResolveParams) (any, error) {
	v := p.Source.(PostgresView)
	total, hasTotal := storageTotalBytes(v)
	if !hasTotal {
		return nil, nil
	}
	used, ok := v.storage.usedBytes(p.Context)
	if !ok {
		return nil, nil
	}
	return used / float64(total) * 100, nil
}
