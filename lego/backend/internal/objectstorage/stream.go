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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Errors of the streamed contract (the Cloudflare-shaped routes, ADR112 D3).
var (
	// ErrMissingContentLength refuses a put whose length is not known up front.
	ErrMissingContentLength = errors.New("the request has no Content-Length")
	// ErrIncompleteBody is a put whose body ended before its Content-Length.
	ErrIncompleteBody = errors.New("the request body ended before its Content-Length")
)

// ResolveBucket returns the bucket a Cloudflare route names. A render-<region>
// name is the workspace's system bucket, created on first use as Render's
// routes do, so wrangler can reach what the Render CLI wrote and the other way
// round before either has touched it.
func (s *Service) ResolveBucket(ctx context.Context, workspaceID, name string) (store.ObjectBucket, error) {
	if !s.Enabled() {
		return store.ObjectBucket{}, ErrNotConfigured
	}
	for _, r := range Regions {
		if name == SystemBucketName(r) {
			return s.SystemBucket(ctx, workspaceID, r)
		}
	}
	return s.GetBucket(ctx, workspaceID, name)
}

// BucketUsage returns the last sweep's reading of one bucket. ok is false for a
// bucket the sweep has not seen.
func (s *Service) BucketUsage(ctx context.Context, workspaceID, bucketID string) (usage store.ObjectStorageUsage, ok bool, err error) {
	if !s.Enabled() {
		return store.ObjectStorageUsage{}, false, ErrNotConfigured
	}
	all, err := s.repo.ListObjectStorageUsage(ctx, workspaceID)
	if err != nil {
		return store.ObjectStorageUsage{}, false, err
	}
	for _, u := range all {
		if u.BucketID == bucketID {
			return u, true, nil
		}
	}
	return store.ObjectStorageUsage{}, false, nil
}

// SetCORS validates a bucket's CORS rules and stores them; a set that fails
// ValidateCORS is stored nowhere. An empty set clears the configuration.
func (s *Service) SetCORS(ctx context.Context, workspaceID string, b store.ObjectBucket, rules []CORSRule) error {
	if err := ValidateCORS(rules); err != nil {
		return err
	}
	if rules == nil {
		rules = []CORSRule{} // stored as [], never null
	}
	return s.setRules(ctx, workspaceID, b, rules, s.repo.SetObjectBucketCORS)
}

// SetLifecycle validates a bucket's lifecycle rules and stores them for the
// hourly sweep to run; a set that fails ValidateLifecycle is stored nowhere.
func (s *Service) SetLifecycle(ctx context.Context, workspaceID string, b store.ObjectBucket, rules []LifecycleRule) error {
	if err := ValidateLifecycle(rules); err != nil {
		return err
	}
	if rules == nil {
		rules = []LifecycleRule{}
	}
	return s.setRules(ctx, workspaceID, b, rules, s.repo.SetObjectBucketLifecycle)
}

func (s *Service) setRules(ctx context.Context, workspaceID string, b store.ObjectBucket, rules any,
	set func(context.Context, string, string, json.RawMessage) error) error {
	if !s.Enabled() {
		return ErrNotConfigured
	}
	raw, err := json.Marshal(rules)
	if err != nil {
		return err
	}
	if err := set(ctx, workspaceID, b.ID, raw); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrBucketNotFound
		}
		return err
	}
	return nil
}

// locate is where key of bucket b lives: its region's store and its Wasabi key.
// ObjectKey validates the key and both ids, so nothing that reaches Wasabi can
// leave the bucket's prefix.
func (s *Service) locate(workspaceID string, b store.ObjectBucket, key string) (Store, string, error) {
	if !s.Enabled() {
		return nil, "", ErrNotConfigured
	}
	st, err := s.Store(Region(b.Region))
	if err != nil {
		return nil, "", err
	}
	wasabiKey, err := ObjectKey(workspaceID, b.ID, key)
	if err != nil {
		return nil, "", err
	}
	return st, wasabiKey, nil
}

// PutStream stores size bytes from body at key in bucket b, overwriting. It
// refuses a bad key, an unknown length, an object over the streamed cap and a
// write that would pass a quota before it reads a byte, then takes a stream
// slot (the workspace's and the replica's) for as long as the bytes move, at
// most StreamDeadline. The bytes go to Wasabi as they arrive: nothing is
// buffered or spooled.
//
// A put never succeeds into a prefix no row maps (w4/678). It refuses a bucket
// a delete is checking (ErrBucketDeleting) before its first byte, and confirms
// the bucket's row once the object is in Wasabi: a bucket deleted while the
// bytes moved loses the object again and answers ErrBucketNotFound.
func (s *Service) PutStream(ctx context.Context, workspaceID string, b store.ObjectBucket, key string, size int64, body io.Reader, headers ObjectHeaders) (etag string, err error) {
	return s.putStream(ctx, workspaceID, b, key, size, body, headers, MaxObjectBytesStreamed)
}

// PutObject is PutStream for the S3 surface: an object of up to 5 GiB, the S3
// single-PUT ceiling (ADR112 D3), where the Cloudflare-shaped routes stop at
// wrangler's 300 MiB.
func (s *Service) PutObject(ctx context.Context, workspaceID string, b store.ObjectBucket, key string, size int64, body io.Reader, headers ObjectHeaders) (etag string, err error) {
	return s.putStream(ctx, workspaceID, b, key, size, body, headers, MaxObjectBytesPresigned)
}

func (s *Service) putStream(ctx context.Context, workspaceID string, b store.ObjectBucket, key string, size int64, body io.Reader, headers ObjectHeaders, maxBytes int64) (etag string, err error) {
	st, wasabiKey, err := s.locate(workspaceID, b, key)
	if err != nil {
		return "", err
	}
	if size < 0 {
		return "", ErrMissingContentLength
	}
	if err := CheckObjectSize(size, maxBytes); err != nil {
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
	etag, err = st.PutStream(ctx, wasabiKey, size, counted, headers)
	if err != nil {
		// A body that ended early is the caller's. A failure before the body was
		// read to its end (Wasabi unreachable, say) is not.
		if counted.ended && counted.n < size {
			return "", fmt.Errorf("%w: got %d of %d bytes", ErrIncompleteBody, counted.n, size)
		}
		return "", err
	}
	s.RecordIngress(ctx, workspaceID, size)
	s.RecordWrite(ctx, workspaceID, Region(b.Region), size)
	if err := s.confirmOrDiscard(ctx, workspaceID, b, st, wasabiKey, key, "put"); err != nil {
		return "", err
	}
	return etag, nil
}

// confirmOrDiscard confirms a bucket's row once an object is in Wasabi under its prefix.
// The confirmation outlives a caller that hung up: the bytes are stored. Only a row that
// is gone loses them again; any other failure keeps the object, which may have overwritten
// one the customer still owns.
func (s *Service) confirmOrDiscard(ctx context.Context, workspaceID string, b store.ObjectBucket, st Store, wasabiKey, key, op string) error {
	err := s.repo.ConfirmObjectBucketLive(context.WithoutCancel(ctx), workspaceID, b.ID)
	if err == nil || !errors.Is(err, store.ErrNotFound) {
		return err
	}
	rmCtx, rmCancel := context.WithTimeout(context.WithoutCancel(ctx), orphanRemoveTimeout)
	defer rmCancel()
	if rmErr := st.Delete(rmCtx, wasabiKey); rmErr != nil {
		return fmt.Errorf("remove %s from a bucket deleted during the %s: %w (after %w)", key, op, rmErr, ErrBucketNotFound)
	}
	return ErrBucketNotFound
}

// bucketWriteError is the service error for a store answer about a put's bucket.
func bucketWriteError(err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return ErrBucketNotFound
	case errors.Is(err, store.ErrObjectBucketDeleting):
		return ErrBucketDeleting
	}
	return err
}

// ObjectReader is an object open for reading. Closing it ends the read,
// returns the stream slot and records the bytes read as egress.
type ObjectReader struct {
	Info ObjectInfo
	body io.ReadCloser
	n    int64
	once sync.Once
	done func(bytes int64)
}

// Read reads the object's bytes.
func (o *ObjectReader) Read(p []byte) (int, error) {
	n, err := o.body.Read(p)
	o.n += int64(n)
	return n, err
}

// Close ends the read; it is safe to call more than once.
func (o *ObjectReader) Close() error {
	err := o.body.Close()
	o.once.Do(func() { o.done(o.n) })
	return err
}

// OpenObject opens key of bucket b for streaming back to the caller, or returns
// ErrObjectNotFound. It takes a stream slot that Close gives back, and the read
// ends after StreamDeadline whatever Wasabi does.
func (s *Service) OpenObject(ctx context.Context, workspaceID string, b store.ObjectBucket, key string) (*ObjectReader, error) {
	st, wasabiKey, err := s.locate(workspaceID, b, key)
	if err != nil {
		return nil, err
	}
	release, err := s.AcquireStream(workspaceID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, s.streamDeadline)
	obj, err := st.GetObject(ctx, wasabiKey)
	if err != nil {
		cancel()
		release()
		return nil, err
	}
	obj.ObjectInfo.Key = key
	return &ObjectReader{Info: obj.ObjectInfo, body: obj.Body, done: func(bytes int64) {
		cancel()
		release()
		s.RecordEgress(ctx, workspaceID, bytes)
	}}, nil
}

// StatObject is the headers and size of key of bucket b without its bytes (a
// HEAD request), or ErrObjectNotFound. It holds no stream slot and meters
// nothing.
func (s *Service) StatObject(ctx context.Context, workspaceID string, b store.ObjectBucket, key string) (ObjectInfo, error) {
	st, wasabiKey, err := s.locate(workspaceID, b, key)
	if err != nil {
		return ObjectInfo{}, err
	}
	info, err := st.Head(ctx, wasabiKey)
	info.Key = key
	return info, err
}

// RemoveObject deletes key of bucket b. A missing key is not an error: S3 and
// Cloudflare answer success, where Render's routes answer 404 (ADR112 D2, D3).
func (s *Service) RemoveObject(ctx context.Context, workspaceID string, b store.ObjectBucket, key string) error {
	st, wasabiKey, err := s.locate(workspaceID, b, key)
	if err != nil {
		return err
	}
	return st.Delete(ctx, wasabiKey)
}

// countingReader counts the bytes a put took from the caller and notes whether
// the caller's body ended, so a body that ended early can be told from a Wasabi
// failure that came before the body was read.
type countingReader struct {
	r     io.Reader
	n     int64
	ended bool
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		c.ended = true
	}
	return n, err
}
