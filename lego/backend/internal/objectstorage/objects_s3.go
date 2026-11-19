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

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The object operations only the S3 surface needs (ADR112 D3): a ranged and
// conditional read, a server-side copy and a batch delete.

// ErrCopyAcrossRegions refuses a copy whose two buckets are in different regions: v1
// copies inside one Wasabi bucket (ADR112 D3, D10).
var ErrCopyAcrossRegions = errors.New("copying between regions is not implemented")

// ObjectOpen says how an opened read answers: the whole object or a range of it.
type ObjectOpen struct {
	// ContentRange is "bytes a-b/size" for a partial answer, "" for the whole object.
	ContentRange string
}

// OpenObjectWith opens key of bucket b with the caller's range and conditions, taking
// a stream slot that Close gives back. A condition that holds the object back is
// ErrNotModified or ErrPreconditionFailed, and a range past its end
// ErrRangeNotSatisfiable. Wasabi answers a presigned Range past the end with the whole
// object and a 200 (ADR112 § Not verified), so a 200 to a ranged read is checked
// against the object's size here: a resume at bytes=<size>- would otherwise download
// everything again.
func (s *Service) OpenObjectWith(ctx context.Context, workspaceID string, b store.ObjectBucket, key string, opts GetOptions) (*ObjectReader, ObjectOpen, error) {
	st, wasabiKey, err := s.locate(workspaceID, b, key)
	if err != nil {
		return nil, ObjectOpen{}, err
	}
	if err := s.CheckEgressBudget(ctx, workspaceID); err != nil {
		return nil, ObjectOpen{}, err
	}
	release, err := s.AcquireStream(workspaceID)
	if err != nil {
		return nil, ObjectOpen{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, s.streamDeadline)
	obj, err := st.GetObjectWith(ctx, wasabiKey, opts)
	if err != nil {
		cancel()
		release()
		return nil, ObjectOpen{}, err
	}
	if opts.Range != "" && obj.ContentRange == "" && !RangeSatisfiable(opts.Range, obj.Size) {
		_ = obj.Body.Close()
		cancel()
		release()
		return nil, ObjectOpen{}, ErrRangeNotSatisfiable
	}
	obj.ObjectInfo.Key = key
	return &ObjectReader{Info: obj.ObjectInfo, body: obj.Body, done: func(bytes int64) {
		cancel()
		release()
		s.RecordEgress(ctx, workspaceID, bytes)
	}}, ObjectOpen{ContentRange: obj.ContentRange}, nil
}

// RangeSatisfiable reports whether an HTTP Range header can be answered for an object
// of size bytes: its first byte is inside the object. A header that is not a single
// byte range is one S3 ignores, so it is satisfiable (the whole object is served).
func RangeSatisfiable(header string, size int64) bool {
	spec, ok := strings.CutPrefix(strings.TrimSpace(header), "bytes=")
	if !ok || strings.Contains(spec, ",") {
		return true
	}
	from, to, found := strings.Cut(spec, "-")
	if !found {
		return true
	}
	if from == "" {
		n, err := strconv.ParseInt(to, 10, 64)
		return err != nil || (n > 0 && size > 0)
	}
	start, err := strconv.ParseInt(from, 10, 64)
	if err != nil {
		return true
	}
	if to != "" {
		if end, err := strconv.ParseInt(to, 10, 64); err == nil && end < start {
			return true // malformed: S3 ignores it
		}
	}
	return start < size
}

// CopyObject copies an object to another key, inside Wasabi (ADR112 D3): both buckets
// are bex buckets of the workspace in one region and the source is at most 5 GiB. It
// reads the source's size with a HEAD, gates the write by it, and copies with the
// source's headers kept (replace nil) or replaced. It returns the source's metadata.
func (s *Service) CopyObject(ctx context.Context, workspaceID string, src store.ObjectBucket, srcKey string, dst store.ObjectBucket, dstKey string, replace *ObjectHeaders) (CopyResult, error) {
	if src.Region != dst.Region {
		return CopyResult{}, ErrCopyAcrossRegions
	}
	st, srcWasabi, err := s.locate(workspaceID, src, srcKey)
	if err != nil {
		return CopyResult{}, err
	}
	_, dstWasabi, err := s.locate(workspaceID, dst, dstKey)
	if err != nil {
		return CopyResult{}, err
	}
	info, err := st.Head(ctx, srcWasabi)
	if err != nil {
		return CopyResult{}, err
	}
	if err := CheckObjectSize(info.Size, MaxObjectBytesPresigned); err != nil {
		return CopyResult{}, err
	}
	releaseGate, err := s.GateAndReserve(ctx, workspaceID, Region(dst.Region), info.Size)
	if err != nil {
		return CopyResult{}, err
	}
	defer releaseGate()
	if err := s.repo.CheckObjectBucketWritable(ctx, workspaceID, dst.ID); err != nil {
		return CopyResult{}, bucketWriteError(err)
	}
	ctx, cancel := context.WithTimeout(ctx, s.streamDeadline)
	defer cancel()
	res, err := st.Copy(ctx, srcWasabi, dstWasabi, replace)
	if err != nil {
		return CopyResult{}, err
	}
	s.RecordWrite(ctx, workspaceID, Region(dst.Region), info.Size)
	if err := s.confirmOrDiscard(ctx, workspaceID, dst, st, dstWasabi, dstKey, "copy"); err != nil {
		return CopyResult{}, err
	}
	return res, nil
}

// DeleteFailure is a key a batch delete could not remove.
type DeleteFailure struct {
	Key     string
	Invalid bool // the key is not one bex stores (ADR112 D7)
}

// DeleteObjects removes up to ListMaxLimit keys of bucket b in one request and
// reports the ones it could not remove. A key that was already gone is deleted, as
// on S3.
func (s *Service) DeleteObjects(ctx context.Context, workspaceID string, b store.ObjectBucket, keys []string) (deleted []string, failed []DeleteFailure, err error) {
	if len(keys) > ListMaxLimit {
		return nil, nil, fmt.Errorf("%w: at most %d keys", ErrInvalidKey, ListMaxLimit)
	}
	st, err := s.Store(Region(b.Region))
	if err != nil {
		return nil, nil, err
	}
	wasabi := make([]string, 0, len(keys))
	byWasabi := make(map[string]string, len(keys))
	for _, k := range keys {
		w, err := ObjectKey(workspaceID, b.ID, k)
		if err != nil {
			failed = append(failed, DeleteFailure{Key: k, Invalid: true})
			continue
		}
		wasabi = append(wasabi, w)
		byWasabi[w] = k
	}
	var gone []string
	if len(wasabi) > 0 {
		undone, delErr := st.DeleteKeys(ctx, wasabi)
		failedSet := map[string]bool{}
		for _, w := range undone {
			failedSet[w] = true
			failed = append(failed, DeleteFailure{Key: byWasabi[w]})
		}
		for _, w := range wasabi {
			if !failedSet[w] {
				gone = append(gone, byWasabi[w])
			}
		}
		if delErr != nil && len(undone) == 0 {
			return nil, nil, delErr
		}
	}
	return gone, failed, nil
}
