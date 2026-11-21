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
	"strconv"
	"strings"
	"time"
)

// ErrObjectTooLarge refuses one object above the contract's per-object cap.
var ErrObjectTooLarge = errors.New("object is too large")

// QuotaResource names the cap a write would pass.
type QuotaResource string

const (
	QuotaBytes   QuotaResource = "bytes"
	QuotaObjects QuotaResource = "objects"
)

// QuotaError refuses a write that would take a workspace past a cap in a region, or
// an S3 read once the workspace's monthly egress budget is spent. Resource says
// which one, so the answer can name it.
type QuotaError struct {
	Resource QuotaResource
	Limit    int64
	// Resets is when the egress budget starts again: the next month's first instant
	// in UTC. The per-region caps have none.
	Resets time.Time
}

func (e *QuotaError) Error() string {
	if e.Resource == QuotaEgress {
		// The budget covers every region, has already been reached, and comes back
		// when the month turns, so the per-region sentence below is wrong for it.
		return fmt.Sprintf("this workspace's S3 reads this month have reached its egress limit of %s (the larger of the bytes it stores and %s, across every region); reads resume on %s UTC",
			formatGiB(e.Limit), formatGiB(S3EgressBudgetFloor), e.Resets.UTC().Format(time.DateOnly))
	}
	return fmt.Sprintf("this workspace's object storage in this region would exceed its %s limit of %d", e.Resource, e.Limit)
}

// formatGiB writes n bytes in GiB with at most two decimals: 5368709120 is "5 GiB".
func formatGiB(n int64) string {
	s := strconv.FormatFloat(float64(n)/(1<<30), 'f', 2, 64)
	return strings.TrimSuffix(strings.TrimRight(s, "0"), ".") + " GiB"
}

// CheckObjectSize refuses an object larger than max, or a negative size.
func CheckObjectSize(size, max int64) error {
	if size < 0 || size > max {
		return ErrObjectTooLarge
	}
	return nil
}

// Gate is the write-time quota check (ADR112 D8). It adds the size of the write
// to the last usage the hourly sweep observed for the workspace in region and
// refuses a write that would pass a cap. It does not reserve anything: the
// sweep's reading is up to an hour old, so the overshoot is bounded by the
// mint rate limit on Contract A and by the concurrent streams times the
// per-object cap on the S3 surface. A put over an existing key counts as a new
// object, which errs on the safe side.
func (s *Service) Gate(ctx context.Context, workspaceID string, region Region, addBytes int64) error {
	if !s.Enabled() {
		return ErrNotConfigured
	}
	bytes, objects, err := s.repo.ObjectStorageUsageTotals(ctx, workspaceID, string(region))
	if err != nil {
		return fmt.Errorf("read object storage usage: %w", err)
	}
	// What bex has itself written since that observation, and what is in flight.
	pb, po, err := s.pendingUsage(ctx, workspaceID, region)
	if err != nil {
		return err
	}
	bytes, objects = bytes+pb, objects+po
	if bytes+addBytes > MaxWorkspaceBytes {
		return &QuotaError{Resource: QuotaBytes, Limit: MaxWorkspaceBytes}
	}
	if objects+1 > MaxWorkspaceObjects {
		return &QuotaError{Resource: QuotaObjects, Limit: MaxWorkspaceObjects}
	}
	return nil
}
