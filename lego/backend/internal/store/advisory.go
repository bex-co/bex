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

	"github.com/jackc/pgx/v5"
)

// WithTenantAdvisoryLock runs fn while holding a transaction-scoped Postgres
// advisory lock for tenantID. It coordinates count-then-write admission paths
// across bex-api replicas even when the write itself reaches an external
// provider before persisting its tenant binding.
func (s *PGStore) WithTenantAdvisoryLock(ctx context.Context, tenantID string, fn func() error) error {
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, tenantID); err != nil {
			return err
		}
		return fn()
	})
}
