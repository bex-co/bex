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
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrInvalidCursor refuses a list cursor bex did not hand out.
var ErrInvalidCursor = errors.New("invalid cursor")

// PresignedURL is a URL the caller uses to move the bytes straight to or from
// Wasabi, so they never pass through bex-api (Contract A, ADR112 D2).
type PresignedURL struct {
	URL       string
	ExpiresAt time.Time
	// MaxSizeBytes is what a PUT URL allows: the size asked for, which the URL
	// signs as its Content-Length. Zero for a GET URL.
	MaxSizeBytes int64
}

// ObjectItem is one object in a listing, with the customer's key (not the
// Wasabi key) and the cursor that resumes after it.
type ObjectItem struct {
	Key          string
	SizeBytes    int64
	LastModified time.Time
	Cursor       string
}

// ObjectPage is one page of a listing. NextCursor is set only when HasNext is.
type ObjectPage struct {
	Items      []ObjectItem
	HasNext    bool
	NextCursor string
}

// The Render routes work in the workspace's system bucket render-<region>, so
// an object put with the Render CLI is readable with wrangler and the other
// way round (ADR112 D1).

// MintPut returns a URL that uploads exactly size bytes to key in the
// workspace's render-<region> bucket. It refuses a bad key, an object over the
// 5 GiB cap, a mint over the workspace's rate and a write that would pass a
// quota, before it creates or signs anything.
func (s *Service) MintPut(ctx context.Context, workspaceID string, region Region, key string, size int64) (PresignedURL, error) {
	if !s.Enabled() {
		return PresignedURL{}, ErrNotConfigured
	}
	if err := ValidateKey(key); err != nil {
		return PresignedURL{}, err
	}
	if err := CheckObjectSize(size, MaxObjectBytesPresigned); err != nil {
		return PresignedURL{}, err
	}
	st, err := s.Store(region)
	if err != nil {
		return PresignedURL{}, err
	}
	if err := s.AllowMint(workspaceID); err != nil {
		return PresignedURL{}, err
	}
	if err := s.Gate(ctx, workspaceID, region, size); err != nil {
		return PresignedURL{}, err
	}
	wasabiKey, err := s.systemObjectKey(ctx, workspaceID, region, key)
	if err != nil {
		return PresignedURL{}, err
	}
	url, err := st.PresignPut(ctx, wasabiKey, size, PresignTTL)
	if err != nil {
		return PresignedURL{}, fmt.Errorf("presign put: %w", err)
	}
	return PresignedURL{URL: url, ExpiresAt: time.Now().Add(PresignTTL).UTC(), MaxSizeBytes: size}, nil
}

// MintGet returns a URL that downloads key, or ErrObjectNotFound: it HEADs the
// key first because Render's clients treat a 404 from the mint as "object not
// found", and a presigned URL for a missing key would only fail later.
func (s *Service) MintGet(ctx context.Context, workspaceID string, region Region, key string) (PresignedURL, error) {
	if !s.Enabled() {
		return PresignedURL{}, ErrNotConfigured
	}
	if err := ValidateKey(key); err != nil {
		return PresignedURL{}, err
	}
	st, err := s.Store(region)
	if err != nil {
		return PresignedURL{}, err
	}
	if err := s.AllowMint(workspaceID); err != nil {
		return PresignedURL{}, err
	}
	wasabiKey, err := s.systemObjectKey(ctx, workspaceID, region, key)
	if err != nil {
		return PresignedURL{}, err
	}
	info, err := st.Head(ctx, wasabiKey)
	if err != nil {
		return PresignedURL{}, err
	}
	url, err := st.PresignGet(ctx, wasabiKey, PresignTTL)
	if err != nil {
		return PresignedURL{}, fmt.Errorf("presign get: %w", err)
	}
	// The bytes bypass bex-api, so egress is an estimate: the object's size,
	// exact for a complete download and an upper bound for a ranged one
	// (ADR112 D2).
	s.RecordEgress(ctx, workspaceID, info.Size)
	return PresignedURL{URL: url, ExpiresAt: time.Now().Add(PresignTTL).UTC()}, nil
}

// DeleteObject removes key, or returns ErrObjectNotFound: S3's DeleteObject
// answers 204 for a missing key, while Render's clients treat 404 as "object
// not found", so the key is HEADed first.
func (s *Service) DeleteObject(ctx context.Context, workspaceID string, region Region, key string) error {
	if !s.Enabled() {
		return ErrNotConfigured
	}
	if err := ValidateKey(key); err != nil {
		return err
	}
	st, err := s.Store(region)
	if err != nil {
		return err
	}
	wasabiKey, err := s.systemObjectKey(ctx, workspaceID, region, key)
	if err != nil {
		return err
	}
	if _, err := st.Head(ctx, wasabiKey); err != nil {
		return err
	}
	return st.Delete(ctx, wasabiKey)
}

// ListObjects returns one page of the workspace's render-<region> bucket.
// limit is honored up to ListMaxLimit, the S3 page size: the CLI's own help
// example asks for 200, so a lower cap would truncate silently. The cursor is
// the base64 of the last key returned, fed to StartAfter, so the listing reads
// Wasabi directly and no database mirror can drift from it.
func (s *Service) ListObjects(ctx context.Context, workspaceID string, region Region, cursor string, limit int) (ObjectPage, error) {
	st, err := s.Store(region)
	if err != nil {
		return ObjectPage{}, err
	}
	startAfter := ""
	if cursor != "" {
		last, err := decodeCursor(cursor)
		if err != nil {
			return ObjectPage{}, err
		}
		startAfter = last
	}
	b, err := s.SystemBucket(ctx, workspaceID, region)
	if err != nil {
		return ObjectPage{}, err
	}
	prefix := Prefix(workspaceID, b.ID)
	if startAfter != "" {
		startAfter = prefix + startAfter
	}
	limit = min(max(limit, 1), ListMaxLimit)
	page, err := st.List(ctx, prefix, startAfter, limit)
	if err != nil {
		return ObjectPage{}, fmt.Errorf("list objects: %w", err)
	}
	out := ObjectPage{Items: make([]ObjectItem, 0, len(page.Objects))}
	for _, o := range page.Objects {
		key := strings.TrimPrefix(o.Key, prefix)
		out.Items = append(out.Items, ObjectItem{Key: key, SizeBytes: o.Size, LastModified: o.LastModified.UTC(), Cursor: encodeCursor(key)})
	}
	if page.Next != "" && len(out.Items) > 0 {
		out.HasNext, out.NextCursor = true, out.Items[len(out.Items)-1].Cursor
	}
	return out, nil
}

// systemObjectKey resolves the workspace's render-<region> bucket and builds
// the Wasabi key of key in it.
func (s *Service) systemObjectKey(ctx context.Context, workspaceID string, region Region, key string) (string, error) {
	b, err := s.SystemBucket(ctx, workspaceID, region)
	if err != nil {
		return "", err
	}
	return ObjectKey(workspaceID, b.ID, key)
}

func encodeCursor(key string) string { return base64.RawURLEncoding.EncodeToString([]byte(key)) }

func decodeCursor(cursor string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || ValidateKey(string(raw)) != nil {
		return "", ErrInvalidCursor
	}
	return string(raw), nil
}
