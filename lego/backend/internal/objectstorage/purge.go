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
	"errors"
	"fmt"
	"log"
	"time"
)

// Deleting a workspace or an account purges its objects (ADR112 D9, ADR086).
//
// Bucket rows have no tenant foreign key, so deleting the tenant row would leave
// the rows, and the objects the rows alone map, behind. PGStore.DeleteTenant
// therefore retires the workspace's rows (deleted_at) in the same transaction as
// the tenant row, which stops any pub- host answering at once. The purge then
// has two tries: WorkspacePurger, right after the deletion, and the hourly
// sweep, which retries until every region is empty. Only when both regions are
// empty are the retired rows and the usage rows dropped, so a purge that fails
// is never forgotten: the rows are the record that it is still owed.
//
// What this does not cover: a presigned PUT URL minted just before the deletion
// stays valid for PresignTTL, so an upload that lands after the purge leaves an
// object no row names. Rows dropped now cannot find it; ADR112 does not record
// the case.

// ImmediatePurgeTimeout bounds the purge a deletion attempts itself, so a large
// workspace does not hold the delete request open; the sweep finishes it.
const ImmediatePurgeTimeout = 10 * time.Second

// WorkspacePurger deletes a deleted workspace's objects as the last step of the
// workspace teardown, after its tenant row (and with it the live bucket rows) is
// gone. It satisfies workspaces.WorkspacePurger, which this package does not
// import. Wasabi being unavailable never fails the deletion: the purge is
// logged, the retired rows stay stamped, and the sweep retries.
type WorkspacePurger struct {
	Service *Service
	Repo    PurgeRepository
	// Timeout bounds the attempt; zero is ImmediatePurgeTimeout.
	Timeout time.Duration
}

// PurgeWorkspace implements workspaces.WorkspacePurger. It always returns nil.
func (p *WorkspacePurger) PurgeWorkspace(ctx context.Context, workspaceID string) error {
	if p == nil || !p.Service.Enabled() {
		return nil
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = ImmediatePurgeTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if _, err := purgeWorkspace(ctx, p.Service, p.Repo, workspaceID, time.Now()); err != nil {
		log.Printf("object storage: the purge of deleted workspace %s is left to the hourly sweep: %v", workspaceID, err)
	}
	return nil
}

// purgeWorkspace removes every object under <tea-id>/ in every region, then
// drops the workspace's rows, and reports how many objects it removed. It
// retires any live row first (a no-op after DeleteTenant), so no pub- host
// answers while the objects go, and stamps the attempt so a purge that keeps
// failing waits behind the others. It tries every region and returns every
// failure, leaving the rows in place unless all of them succeeded.
func purgeWorkspace(ctx context.Context, svc *Service, repo PurgeRepository, workspaceID string, now time.Time) (removed int, err error) {
	if !svc.Enabled() {
		return 0, ErrNotConfigured
	}
	if _, err := repo.RetireObjectBuckets(ctx, workspaceID); err != nil {
		return 0, fmt.Errorf("retire buckets: %w", err)
	}
	if err := repo.MarkObjectStoragePurgeAttempt(ctx, workspaceID, now); err != nil {
		return 0, fmt.Errorf("stamp purge attempt: %w", err)
	}
	var errs []error
	// The parts of an unfinished upload are in no listing of the prefix: abort them first,
	// so none can complete into it after it is emptied.
	if err := svc.abortWorkspaceUploads(ctx, workspaceID); err != nil {
		errs = append(errs, fmt.Errorf("abort multipart uploads: %w", err))
	}
	for _, region := range Regions {
		st, ok := svc.stores[region]
		if !ok {
			continue
		}
		n, err := st.DeletePrefix(ctx, WorkspacePrefix(workspaceID))
		removed += n
		if err != nil {
			errs = append(errs, fmt.Errorf("purge %s: %w", region, err))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return removed, err
	}
	if err := repo.FinishObjectStoragePurge(ctx, workspaceID); err != nil {
		return removed, fmt.Errorf("drop rows: %w", err)
	}
	return removed, nil
}
