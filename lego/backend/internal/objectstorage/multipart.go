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
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Multipart upload on the S3 surface (ADR112 D3). The upload id bex hands a client is the
// row's typed id; Wasabi's own stays in the row. Parts are rows too, so a part number sent
// again replaces the earlier one, and the unfinished parts' bytes count against the
// workspace's quota until the upload completes or is aborted (the gate reads them through
// MultipartBytes), because no object listing sees them.

var (
	// ErrTooManyUploads refuses a workspace's hundred-and-first open upload.
	ErrTooManyUploads = errors.New("too many open multipart uploads")
	// ErrInvalidPartNumber is a part number outside 1 to 10,000.
	ErrInvalidPartNumber = errors.New("a part number is 1 to 10000")
	// ErrInvalidPart is a part CompleteMultipartUpload names that the upload does not hold
	// or whose ETag differs.
	ErrInvalidPart = errors.New("a listed part was not uploaded or its ETag differs")
	// ErrInvalidPartOrder is a part list that is not in ascending order.
	ErrInvalidPartOrder = errors.New("the part list is not in ascending order")
	// ErrEntityTooSmall is a part other than the last that is under 5 MiB.
	ErrEntityTooSmall = errors.New("a part other than the last is under the 5 MiB minimum")
	// ErrNoParts is a CompleteMultipartUpload that lists none.
	ErrNoParts = errors.New("the part list is empty")
)

// MultipartRepository is the part of the store multipart upload uses; *store.PGStore
// implements it, and a repository without it leaves the service refusing multipart.
type MultipartRepository interface {
	CreateObjectMultipartUpload(ctx context.Context, in store.ObjectMultipartUpload, maxOpen int) (store.ObjectMultipartUpload, error)
	GetObjectMultipartUpload(ctx context.Context, workspaceID, id string) (store.ObjectMultipartUpload, error)
	ListObjectMultipartUploads(ctx context.Context, workspaceID, bucketID, prefix, afterKey, afterID string, limit int) ([]store.ObjectMultipartUpload, error)
	UpsertObjectMultipartPart(ctx context.Context, uploadID string, part int, size int64, etag string) error
	ListObjectMultipartParts(ctx context.Context, uploadID string, after, limit int) ([]store.ObjectMultipartPart, error)
	ObjectMultipartUploadBytes(ctx context.Context, id string, exceptPart int) (int64, error)
	DeleteObjectMultipartUpload(ctx context.Context, id string) error
	ObjectBucketHasMultipartUploads(ctx context.Context, bucketID string) (bool, error)
}

// MultipartSweepRepository is what aborting uploads in bulk reads: the uploads with no part
// for a while, a deleted workspace's, and a bucket's older than a lifecycle rule allows.
type MultipartSweepRepository interface {
	MultipartRepository
	StaleObjectMultipartUploads(ctx context.Context, cutoff time.Time, limit int) ([]store.ObjectMultipartUpload, error)
	ObjectMultipartUploadsOfWorkspace(ctx context.Context, workspaceID string, limit int) ([]store.ObjectMultipartUpload, error)
	ObjectMultipartUploadsStartedBefore(ctx context.Context, bucketID, prefix string, cutoff time.Time, limit int) ([]store.ObjectMultipartUpload, error)
}

func (s *Service) multipart() (MultipartRepository, error) {
	repo, ok := s.repo.(MultipartRepository)
	if !ok {
		return nil, ErrNotConfigured
	}
	return repo, nil
}

// openUpload is the workspace's open upload id on key of bucket b. Another workspace's id, a
// bucket's or key's mismatch and an id that never existed are all ErrNoSuchUpload.
func (s *Service) openUpload(ctx context.Context, repo MultipartRepository, workspaceID string, b store.ObjectBucket, key, uploadID string) (store.ObjectMultipartUpload, error) {
	u, err := repo.GetObjectMultipartUpload(ctx, workspaceID, uploadID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return store.ObjectMultipartUpload{}, ErrNoSuchUpload
	case err != nil:
		return store.ObjectMultipartUpload{}, err
	case u.BucketID != b.ID || u.Key != key:
		return store.ObjectMultipartUpload{}, ErrNoSuchUpload
	}
	return u, nil
}

// CreateMultipartUpload starts an upload of key in bucket b and returns its id. createdBy is
// the access key id that asked.
func (s *Service) CreateMultipartUpload(ctx context.Context, workspaceID string, b store.ObjectBucket, key, createdBy string, headers ObjectHeaders) (string, error) {
	repo, err := s.multipart()
	if err != nil {
		return "", err
	}
	st, wasabiKey, err := s.locate(workspaceID, b, key)
	if err != nil {
		return "", err
	}
	if err := s.repo.CheckObjectBucketWritable(ctx, workspaceID, b.ID); err != nil {
		return "", bucketWriteError(err)
	}
	ctx, cancel := context.WithTimeout(ctx, s.streamDeadline)
	defer cancel()
	wasabiID, err := st.CreateMultipart(ctx, wasabiKey, headers)
	if err != nil {
		return "", err
	}
	row, err := repo.CreateObjectMultipartUpload(ctx, store.ObjectMultipartUpload{
		ID: ids.New(ids.ObjectMultipartUpload), WorkspaceID: workspaceID, BucketID: b.ID, Region: b.Region, Key: key,
		WasabiUploadID: wasabiID, CreatedBy: createdBy,
	}, MaxOpenMultipartUploads)
	if err != nil {
		s.abortAtWasabi(ctx, st, wasabiKey, wasabiID)
		if errors.Is(err, store.ErrMultipartQuota) {
			return "", ErrTooManyUploads
		}
		return "", err
	}
	// A bucket deleted while the upload started takes it with it.
	if err := s.repo.ConfirmObjectBucketLive(context.WithoutCancel(ctx), workspaceID, b.ID); err != nil {
		s.discardUpload(ctx, repo, st, row)
		if errors.Is(err, store.ErrNotFound) {
			return "", ErrBucketNotFound
		}
		return "", err
	}
	return row.ID, nil
}

// abortAtWasabi aborts an upload at Wasabi on a detached context, logging what it cannot:
// the parts of an upload nobody can name stay billed until the lifecycle of the region's
// bucket or an operator removes them.
func (s *Service) abortAtWasabi(ctx context.Context, st Store, wasabiKey, wasabiID string) {
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), orphanRemoveTimeout)
	defer cancel()
	if err := st.AbortMultipart(rmCtx, wasabiKey, wasabiID); err != nil && !errors.Is(err, ErrNoSuchUpload) {
		log.Printf("object storage: abort multipart upload %s of %s: %v", wasabiID, wasabiKey, err)
	}
}

// discardUpload aborts an upload at Wasabi and drops its row.
func (s *Service) discardUpload(ctx context.Context, repo MultipartRepository, st Store, u store.ObjectMultipartUpload) {
	if wasabiKey, err := ObjectKey(u.WorkspaceID, u.BucketID, u.Key); err == nil {
		s.abortAtWasabi(ctx, st, wasabiKey, u.WasabiUploadID)
	}
	if err := repo.DeleteObjectMultipartUpload(context.WithoutCancel(ctx), u.ID); err != nil {
		log.Printf("object storage: delete multipart upload row %s: %v", u.ID, err)
	}
}

// checkPart validates a part of size bytes numbered part against an upload: the number, the
// part cap, and the upload's total, so a part that would take the upload past 5 GiB is refused
// when it starts, not at the end.
func (s *Service) checkPart(ctx context.Context, repo MultipartRepository, u store.ObjectMultipartUpload, part int, size int64) error {
	if part < 1 || part > MaxParts {
		return ErrInvalidPartNumber
	}
	if err := CheckObjectSize(size, MaxPartBytes); err != nil {
		return err
	}
	others, err := repo.ObjectMultipartUploadBytes(ctx, u.ID, part)
	if err != nil {
		return err
	}
	return CheckObjectSize(others+size, MaxObjectBytesPresigned)
}

// UploadPart streams size bytes from body as part number part of an upload.
func (s *Service) UploadPart(ctx context.Context, workspaceID string, b store.ObjectBucket, key, uploadID string, part int, size int64, body io.Reader) (etag string, err error) {
	repo, err := s.multipart()
	if err != nil {
		return "", err
	}
	st, wasabiKey, err := s.locate(workspaceID, b, key)
	if err != nil {
		return "", err
	}
	if size < 0 {
		return "", ErrMissingContentLength
	}
	u, err := s.openUpload(ctx, repo, workspaceID, b, key, uploadID)
	if err != nil {
		return "", err
	}
	if err := s.checkPart(ctx, repo, u, part, size); err != nil {
		return "", err
	}
	releaseGate, err := s.GateAndReserve(ctx, workspaceID, Region(b.Region), size)
	if err != nil {
		return "", err
	}
	defer releaseGate()
	if err := s.repo.CheckObjectBucketWritable(ctx, workspaceID, b.ID); err != nil {
		return "", bucketWriteError(err)
	}
	release, err := s.AcquireStream(workspaceID)
	if err != nil {
		return "", err
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, s.streamDeadline)
	defer cancel()

	counted := &countingReader{r: body}
	etag, err = st.UploadPartStream(ctx, wasabiKey, u.WasabiUploadID, part, size, counted)
	switch {
	case errors.Is(err, ErrNoSuchUpload):
		return "", ErrNoSuchUpload
	case err != nil && counted.ended && counted.n < size:
		return "", fmt.Errorf("%w: got %d of %d bytes", ErrIncompleteBody, counted.n, size)
	case err != nil:
		return "", err
	}
	s.RecordIngress(ctx, workspaceID, size)
	return etag, s.recordPart(ctx, repo, u.ID, part, size, etag)
}

func (s *Service) recordPart(ctx context.Context, repo MultipartRepository, uploadID string, part int, size int64, etag string) error {
	err := repo.UpsertObjectMultipartPart(context.WithoutCancel(ctx), uploadID, part, size, etag)
	if errors.Is(err, store.ErrNotFound) {
		return ErrNoSuchUpload
	}
	return err
}

// PartCopyOptions say what a part copy takes of its source.
type PartCopyOptions struct {
	// Range is the x-amz-copy-source-range value, "" for the whole object.
	Range string
	// Conditions are the x-amz-copy-source-if-* conditions the source must meet.
	Conditions GetOptions
}

// UploadPartCopy copies a source object, or a range of it, into part number part of an
// upload. The source is a bex bucket of the same workspace and region, at most 5 GiB, as for
// CopyObject.
func (s *Service) UploadPartCopy(ctx context.Context, workspaceID string, src store.ObjectBucket, srcKey string, dst store.ObjectBucket, dstKey, uploadID string, part int, opts PartCopyOptions) (CopyResult, error) {
	repo, err := s.multipart()
	if err != nil {
		return CopyResult{}, err
	}
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
	u, err := s.openUpload(ctx, repo, workspaceID, dst, dstKey, uploadID)
	if err != nil {
		return CopyResult{}, err
	}
	info, err := st.Head(ctx, srcWasabi)
	if err != nil {
		return CopyResult{}, err
	}
	if opts.Conditions.ConditionStatus(info.ETag, info.LastModified) != 0 {
		return CopyResult{}, ErrPreconditionFailed
	}
	size := info.Size
	if opts.Range != "" {
		if !RangeSatisfiable(opts.Range, info.Size) {
			return CopyResult{}, ErrRangeNotSatisfiable
		}
		size = rangeLength(opts.Range, info.Size)
	}
	if err := s.checkPart(ctx, repo, u, part, size); err != nil {
		return CopyResult{}, err
	}
	releaseGate, err := s.GateAndReserve(ctx, workspaceID, Region(dst.Region), size)
	if err != nil {
		return CopyResult{}, err
	}
	defer releaseGate()
	if err := s.repo.CheckObjectBucketWritable(ctx, workspaceID, dst.ID); err != nil {
		return CopyResult{}, bucketWriteError(err)
	}
	ctx, cancel := context.WithTimeout(ctx, s.streamDeadline)
	defer cancel()
	res, err := st.UploadPartCopy(ctx, srcWasabi, dstWasabi, u.WasabiUploadID, part, opts.Range)
	if errors.Is(err, ErrNoSuchUpload) {
		return CopyResult{}, ErrNoSuchUpload
	}
	if err != nil {
		return CopyResult{}, err
	}
	return res, s.recordPart(ctx, repo, u.ID, part, size, res.ETag)
}

// rangeLength is the bytes an HTTP range of the form bytes=a-b (b optional) takes of an
// object of size bytes, after RangeSatisfiable accepted it.
func rangeLength(header string, size int64) int64 {
	spec, _ := strings.CutPrefix(strings.TrimSpace(header), "bytes=")
	from, to, _ := strings.Cut(spec, "-")
	start, _ := strconv.ParseInt(from, 10, 64)
	end := size - 1
	if v, err := strconv.ParseInt(to, 10, 64); err == nil {
		end = min(v, size-1)
	}
	return end - start + 1
}

// CompletedPartRef is one entry of a CompleteMultipartUpload's part list.
type CompletedPartRef struct {
	Number int
	ETag   string
}

// CompleteMultipartUpload assembles an upload from the listed parts. The list is rebuilt
// from the part numbers and ETags alone, in ascending order, every one of which must be a
// part the upload holds; each but the last must be at least 5 MiB and the total at most
// 5 GiB. Parts uploaded and not listed are dropped, as on S3.
func (s *Service) CompleteMultipartUpload(ctx context.Context, workspaceID string, b store.ObjectBucket, key, uploadID string, parts []CompletedPartRef) (etag string, err error) {
	repo, err := s.multipart()
	if err != nil {
		return "", err
	}
	st, wasabiKey, err := s.locate(workspaceID, b, key)
	if err != nil {
		return "", err
	}
	u, err := s.openUpload(ctx, repo, workspaceID, b, key, uploadID)
	if err != nil {
		return "", err
	}
	listed, total, err := s.checkPartList(ctx, repo, u, parts)
	if err != nil {
		return "", err
	}
	if err := s.repo.CheckObjectBucketWritable(ctx, workspaceID, b.ID); err != nil {
		return "", bucketWriteError(err)
	}
	ctx, cancel := context.WithTimeout(ctx, s.streamDeadline)
	defer cancel()
	etag, err = st.CompleteMultipart(ctx, wasabiKey, u.WasabiUploadID, listed)
	if errors.Is(err, ErrNoSuchUpload) {
		// Wasabi lost it (aborted by an operator, or completed by a request that raced this
		// one): the row has nothing left to track.
		_ = repo.DeleteObjectMultipartUpload(context.WithoutCancel(ctx), u.ID)
		return "", ErrNoSuchUpload
	}
	if err != nil {
		return "", err
	}
	s.RecordWrite(ctx, workspaceID, Region(b.Region), total)
	if err := repo.DeleteObjectMultipartUpload(context.WithoutCancel(ctx), u.ID); err != nil {
		log.Printf("object storage: delete the row of completed multipart upload %s: %v", u.ID, err)
	}
	if err := s.confirmOrDiscard(ctx, workspaceID, b, st, wasabiKey, key, "multipart upload"); err != nil {
		return "", err
	}
	return etag, nil
}

// checkPartList validates a CompleteMultipartUpload's list against the upload's rows and
// returns what to send Wasabi and the object's size.
func (s *Service) checkPartList(ctx context.Context, repo MultipartRepository, u store.ObjectMultipartUpload, parts []CompletedPartRef) ([]CompletedPart, int64, error) {
	if len(parts) == 0 {
		return nil, 0, ErrNoParts
	}
	if len(parts) > MaxParts {
		return nil, 0, ErrInvalidPartNumber
	}
	for i, p := range parts {
		if p.Number < 1 || p.Number > MaxParts {
			return nil, 0, ErrInvalidPartNumber
		}
		if i > 0 && p.Number <= parts[i-1].Number {
			return nil, 0, ErrInvalidPartOrder
		}
	}
	held := map[int]store.ObjectMultipartPart{}
	for after := 0; ; {
		page, err := repo.ListObjectMultipartParts(ctx, u.ID, after, MaxParts)
		if err != nil {
			return nil, 0, err
		}
		for _, p := range page {
			held[p.PartNumber] = p
			after = p.PartNumber
		}
		if len(page) < MaxParts {
			break
		}
	}
	listed := make([]CompletedPart, 0, len(parts))
	var total int64
	for i, p := range parts {
		have, ok := held[p.Number]
		if !ok || strings.Trim(p.ETag, `"`) != strings.Trim(have.ETag, `"`) {
			return nil, 0, ErrInvalidPart
		}
		if i < len(parts)-1 && have.Size < MinPartBytes {
			return nil, 0, ErrEntityTooSmall
		}
		total += have.Size
		listed = append(listed, CompletedPart{Number: p.Number, ETag: have.ETag})
	}
	if err := CheckObjectSize(total, MaxObjectBytesPresigned); err != nil {
		return nil, 0, err
	}
	return listed, total, nil
}

// AbortMultipartUpload discards an upload and its parts. An upload Wasabi no longer holds is
// still aborted: the row goes.
func (s *Service) AbortMultipartUpload(ctx context.Context, workspaceID string, b store.ObjectBucket, key, uploadID string) error {
	repo, err := s.multipart()
	if err != nil {
		return err
	}
	st, wasabiKey, err := s.locate(workspaceID, b, key)
	if err != nil {
		return err
	}
	u, err := s.openUpload(ctx, repo, workspaceID, b, key, uploadID)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, s.streamDeadline)
	defer cancel()
	// A failure that is not "gone" keeps the row, so the client's retry or the sweep tries again.
	if err := st.AbortMultipart(ctx, wasabiKey, u.WasabiUploadID); err != nil && !errors.Is(err, ErrNoSuchUpload) {
		return err
	}
	return repo.DeleteObjectMultipartUpload(context.WithoutCancel(ctx), u.ID)
}

// PartInfo is one part of an upload as ListParts shows it.
type PartInfo struct {
	Number       int
	Size         int64
	ETag         string
	LastModified time.Time
}

// PartsPage is one page of ListParts.
type PartsPage struct {
	Parts      []PartInfo
	Truncated  bool
	NextMarker int
}

// ListParts lists an upload's parts after a part number, at most maxParts (1,000 at most).
func (s *Service) ListParts(ctx context.Context, workspaceID string, b store.ObjectBucket, key, uploadID string, maxParts, marker int) (PartsPage, error) {
	repo, err := s.multipart()
	if err != nil {
		return PartsPage{}, err
	}
	u, err := s.openUpload(ctx, repo, workspaceID, b, key, uploadID)
	if err != nil {
		return PartsPage{}, err
	}
	if maxParts <= 0 {
		return PartsPage{}, nil
	}
	maxParts = min(maxParts, ListMaxLimit)
	rows, err := repo.ListObjectMultipartParts(ctx, u.ID, max(marker, 0), maxParts+1)
	if err != nil {
		return PartsPage{}, err
	}
	page := PartsPage{Truncated: len(rows) > maxParts}
	for _, r := range rows[:min(len(rows), maxParts)] {
		page.Parts = append(page.Parts, PartInfo{Number: r.PartNumber, Size: r.Size, ETag: r.ETag, LastModified: r.UploadedAt.UTC()})
	}
	if page.Truncated {
		page.NextMarker = page.Parts[len(page.Parts)-1].Number
	}
	return page, nil
}

// UploadInfo is one open upload as ListMultipartUploads shows it.
type UploadInfo struct {
	Key, UploadID string
	Initiated     time.Time
}

// UploadsPage is one page of ListMultipartUploads.
type UploadsPage struct {
	Uploads                           []UploadInfo
	Truncated                         bool
	NextKeyMarker, NextUploadIDMarker string
}

// ListMultipartUploads lists a bucket's open uploads under a key prefix, ordered by key and
// upload id, after the (key, upload id) marker, at most maxUploads (1,000 at most).
func (s *Service) ListMultipartUploads(ctx context.Context, workspaceID string, b store.ObjectBucket, prefix, keyMarker, uploadIDMarker string, maxUploads int) (UploadsPage, error) {
	repo, err := s.multipart()
	if err != nil {
		return UploadsPage{}, err
	}
	if err := ValidatePrefix(prefix); err != nil {
		return UploadsPage{}, err
	}
	if maxUploads <= 0 {
		return UploadsPage{}, nil
	}
	maxUploads = min(maxUploads, ListMaxLimit)
	rows, err := repo.ListObjectMultipartUploads(ctx, workspaceID, b.ID, prefix, keyMarker, uploadIDMarker, maxUploads+1)
	if err != nil {
		return UploadsPage{}, err
	}
	page := UploadsPage{Truncated: len(rows) > maxUploads}
	for _, r := range rows[:min(len(rows), maxUploads)] {
		page.Uploads = append(page.Uploads, UploadInfo{Key: r.Key, UploadID: r.ID, Initiated: r.CreatedAt.UTC()})
	}
	if page.Truncated {
		last := page.Uploads[len(page.Uploads)-1]
		page.NextKeyMarker, page.NextUploadIDMarker = last.Key, last.UploadID
	}
	return page, nil
}

// sweepAbortBatch and sweepAbortBatches bound what one sweep pass aborts, so a workspace
// that opened thousands of uploads cannot hold the pass: the rest wait for the next.
const (
	sweepAbortBatch   = 100
	sweepAbortBatches = 20
)

// abortUploads aborts each upload at Wasabi and drops its row, and returns how many went and
// every failure. A row whose abort failed stays, so the next pass tries again; an upload
// Wasabi no longer holds is already gone.
func (s *Service) abortUploads(ctx context.Context, repo MultipartRepository, uploads []store.ObjectMultipartUpload) (done int, err error) {
	var errs []error
	for _, u := range uploads {
		st, e := s.Store(Region(u.Region))
		if e != nil {
			errs = append(errs, e)
			continue
		}
		wasabiKey, e := ObjectKey(u.WorkspaceID, u.BucketID, u.Key)
		if e != nil {
			errs = append(errs, e)
			continue
		}
		if e := st.AbortMultipart(ctx, wasabiKey, u.WasabiUploadID); e != nil && !errors.Is(e, ErrNoSuchUpload) {
			errs = append(errs, fmt.Errorf("abort %s: %w", u.ID, e))
			continue
		}
		if e := repo.DeleteObjectMultipartUpload(context.WithoutCancel(ctx), u.ID); e != nil {
			errs = append(errs, e)
			continue
		}
		done++
	}
	return done, errors.Join(errs...)
}

// abortBatches aborts what list returns until it returns nothing, a batch makes no progress
// or maxBatches have run.
func (s *Service) abortBatches(ctx context.Context, repo MultipartRepository, maxBatches int, list func(limit int) ([]store.ObjectMultipartUpload, error)) (aborted int, err error) {
	for range maxBatches {
		batch, err := list(sweepAbortBatch)
		if err != nil || len(batch) == 0 {
			return aborted, err
		}
		done, err := s.abortUploads(ctx, repo, batch)
		aborted += done
		if err != nil || done == 0 {
			return aborted, err
		}
		if ctx.Err() != nil {
			return aborted, ctx.Err()
		}
	}
	return aborted, nil
}

func (s *Service) sweepRepo() (MultipartSweepRepository, bool) {
	repo, ok := s.repo.(MultipartSweepRepository)
	return repo, ok && s.Enabled()
}

// AbortStaleUploads aborts the uploads that have had no part for MultipartAbortAfter, the
// sweep's backstop for a client that never finished or aborted (ADR112 D3).
func (s *Service) AbortStaleUploads(ctx context.Context, now time.Time) (int, error) {
	repo, ok := s.sweepRepo()
	if !ok {
		return 0, nil
	}
	cutoff := now.Add(-MultipartAbortAfter)
	return s.abortBatches(ctx, repo, sweepAbortBatches, func(limit int) ([]store.ObjectMultipartUpload, error) {
		return repo.StaleObjectMultipartUploads(ctx, cutoff, limit)
	})
}

// AbortBucketUploads runs a bucket's AbortIncompleteMultipartUpload lifecycle rules: an upload
// under a rule's prefix that was started longer ago than the rule's age is aborted.
func (s *Service) AbortBucketUploads(ctx context.Context, b store.ObjectBucket, now time.Time, logf func(string, ...any)) (int, error) {
	repo, ok := s.sweepRepo()
	if !ok {
		return 0, nil
	}
	var total int
	var errs []error
	for _, rule := range abortRules(b.ID, b.Lifecycle, logf) {
		cutoff := now.Add(-rule.maxAge)
		n, err := s.abortBatches(ctx, repo, sweepAbortBatches, func(limit int) ([]store.ObjectMultipartUpload, error) {
			return repo.ObjectMultipartUploadsStartedBefore(ctx, b.ID, rule.prefix, cutoff, limit)
		})
		total += n
		if err != nil {
			errs = append(errs, err)
		}
	}
	return total, errors.Join(errs...)
}

// abortWorkspaceUploads aborts every upload of a workspace being purged, which no object
// listing of its prefix would show.
func (s *Service) abortWorkspaceUploads(ctx context.Context, workspaceID string) error {
	repo, ok := s.sweepRepo()
	if !ok {
		return nil
	}
	_, err := s.abortBatches(ctx, repo, 1<<20, func(limit int) ([]store.ObjectMultipartUpload, error) {
		return repo.ObjectMultipartUploadsOfWorkspace(ctx, workspaceID, limit)
	})
	return err
}
