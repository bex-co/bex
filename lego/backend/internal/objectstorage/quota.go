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
)

// ErrObjectTooLarge refuses one object above the contract's per-object cap.
var ErrObjectTooLarge = errors.New("object is too large")

// QuotaResource names the cap a write would pass.
type QuotaResource string

const (
	QuotaBytes   QuotaResource = "bytes"
	QuotaObjects QuotaResource = "objects"
)

// QuotaError refuses a write that would take a workspace past a cap. Resource
// says which one, so the answer can name it.
type QuotaError struct {
	Resource QuotaResource
	Limit    int64
}

func (e *QuotaError) Error() string {
	return fmt.Sprintf("this workspace's object storage in this region would exceed its %s limit of %d", e.Resource, e.Limit)
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
// per-object cap on Contract B. A put over an existing key counts as a new
// object, which errs on the safe side.
func (s *Service) Gate(ctx context.Context, workspaceID string, region Region, addBytes int64) error {
	if !s.Enabled() {
		return ErrNotConfigured
	}
	bytes, objects, err := s.repo.ObjectStorageUsageTotals(ctx, workspaceID, string(region))
	if err != nil {
		return fmt.Errorf("read object storage usage: %w", err)
	}
	if bytes+addBytes > MaxWorkspaceBytes {
		return &QuotaError{Resource: QuotaBytes, Limit: MaxWorkspaceBytes}
	}
	if objects+1 > MaxWorkspaceObjects {
		return &QuotaError{Resource: QuotaObjects, Limit: MaxWorkspaceObjects}
	}
	return nil
}
