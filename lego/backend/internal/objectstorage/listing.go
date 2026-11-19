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
	"unicode/utf8"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The S3 listings (ADR112 D3): ListObjectsV2 and ListObjects over Wasabi under
// <tea-id>/<obk-id>/ plus the client's prefix, with the physical prefix stripped from
// every key returned.

// ErrDelimiterUnsupported is a delimiter that is not one printable ASCII byte: v1 keeps
// the increment of a common prefix's last byte valid UTF-8 (ADR112 D3), so it serves
// only those.
var ErrDelimiterUnsupported = errors.New("only a delimiter of one ASCII character is implemented")

// ListRequest is an S3 listing as the client asked for it.
type ListRequest struct {
	// V2 selects ListObjectsV2's continuation token and start-after; false is the
	// original ListObjects and its marker.
	V2         bool
	Prefix     string
	Delimiter  string
	MaxKeys    int
	Token      string // V2 continuation-token
	StartAfter string // V2 start-after
	Marker     string // V1 marker, a raw client key
}

// ListResult is one page.
type ListResult struct {
	Objects        []ObjectItem
	CommonPrefixes []string
	Truncated      bool
	// NextToken continues a V2 listing; NextMarker a V1 one.
	NextToken, NextMarker string
}

// ValidatePrefix checks a listing prefix. A prefix is a string match and cannot leave the
// bucket (the physical key is <tea-id>/<obk-id>/ followed by it), but it follows D7 all the
// same: bounded, UTF-8, no control character or backslash, no . or .. segment.
func ValidatePrefix(prefix string) error { return validateName(prefix) }

// kindKey and kindPrefix say what a continuation token's last entry was: a key, or a common
// prefix that the next page must skip past.
const (
	kindKey    = 'K'
	kindPrefix = 'P'
)

// EncodeListToken is a V2 continuation token: the base64url, without padding, of the last
// client key or common prefix the page returned, behind one byte that says which. It is never
// Wasabi's own opaque token, so a forged one can start a listing at another point of the same
// bucket and nowhere else.
func EncodeListToken(isPrefix bool, last string) string {
	kind := byte(kindKey)
	if isPrefix {
		kind = kindPrefix
	}
	return base64.RawURLEncoding.EncodeToString(append([]byte{kind}, last...))
}

func decodeListToken(token string) (isPrefix bool, last string, err error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) < 2 || len(raw) > MaxKeyBytes+1 || (raw[0] != kindKey && raw[0] != kindPrefix) || !utf8.Valid(raw[1:]) {
		return false, "", ErrInvalidCursor
	}
	return raw[0] == kindPrefix, string(raw[1:]), nil
}

// maxRune is the largest code point, which sorts after every character a key can hold.
const maxRune = "\U0010FFFF"

// startAfter is the client key a page resumes after and the prefix whose entries it must
// then skip. A common prefix p that ended the previous page is continued past every key under
// p: continuing from p itself would return that prefix's keys again, and the ADR's p with its
// last byte incremented ("dir/" to "dir0") would skip a real key named dir0, because
// StartAfter is exclusive. So the page resumes after p followed by the largest code point, and
// a key that begins with p and sorts after even that is dropped as already represented by p.
func startAfter(isPrefix bool, last, delimiter string) (after, skip string) {
	if isPrefix && delimiter != "" && strings.HasSuffix(last, delimiter) {
		return last + maxRune, last
	}
	return last, ""
}

// isCommonPrefix reports whether marker is what a delimiter folds keys under prefix into:
// prefix, one or more characters without the delimiter, then the delimiter.
func isCommonPrefix(marker, prefix, delimiter string) bool {
	rest, ok := strings.CutPrefix(marker, prefix)
	if !ok || delimiter == "" || !strings.HasSuffix(rest, delimiter) || len(rest) == len(delimiter) {
		return false
	}
	return !strings.Contains(strings.TrimSuffix(rest, delimiter), delimiter)
}

// ListS3 lists bucket b for the S3 surface. Wasabi counts what a listing reads as egress, so
// the response's estimated size is added to the workspace's egress meter (ADR112 D8).
func (s *Service) ListS3(ctx context.Context, workspaceID string, b store.ObjectBucket, req ListRequest) (ListResult, error) {
	st, err := s.Store(Region(b.Region))
	if err != nil {
		return ListResult{}, err
	}
	if err := ValidatePrefix(req.Prefix); err != nil {
		return ListResult{}, err
	}
	if req.Delimiter != "" && (len(req.Delimiter) != 1 || req.Delimiter[0] < 0x20 || req.Delimiter[0] > 0x7e) {
		return ListResult{}, ErrDelimiterUnsupported
	}
	if req.MaxKeys < 0 {
		return ListResult{}, fmt.Errorf("%w: max-keys must not be negative", ErrInvalidCursor)
	}
	maxKeys := min(req.MaxKeys, ListMaxLimit)
	base := Prefix(workspaceID, b.ID)
	var after, skip string
	switch {
	case req.V2 && req.Token != "":
		isPrefix, last, err := decodeListToken(req.Token)
		if err != nil {
			return ListResult{}, err
		}
		after, skip = startAfter(isPrefix, last, req.Delimiter)
	case req.V2:
		after = req.StartAfter
	default:
		// V1's marker is a raw client key, as S3 defines it, so nothing is decoded. One that
		// is a single segment past the prefix and ends in the delimiter is the common prefix
		// a page ended on; a key that equals the prefix is a key.
		after, skip = startAfter(isCommonPrefix(req.Marker, req.Prefix, req.Delimiter), req.Marker, req.Delimiter)
	}
	if len(after) > MaxKeyBytes+1 {
		return ListResult{}, ErrInvalidCursor
	}
	if after != "" {
		after = base + after
	}
	page, err := st.ListDelimited(ctx, base+req.Prefix, req.Delimiter, after, maxKeys)
	if err != nil {
		return ListResult{}, fmt.Errorf("list objects: %w", err)
	}
	out := ListResult{Truncated: page.Truncated, Objects: make([]ObjectItem, 0, len(page.Objects))}
	for _, o := range page.Objects {
		key := strings.TrimPrefix(o.Key, base)
		if skip != "" && strings.HasPrefix(key, skip) {
			continue
		}
		out.Objects = append(out.Objects, ObjectItem{Key: key, SizeBytes: o.Size, LastModified: o.LastModified.UTC(), ETag: o.ETag})
	}
	for _, p := range page.CommonPrefixes {
		if prefix := strings.TrimPrefix(p, base); skip == "" || !strings.HasPrefix(prefix, skip) {
			out.CommonPrefixes = append(out.CommonPrefixes, prefix)
		}
	}
	if page.Truncated && page.Last != "" {
		last := strings.TrimPrefix(page.Last, base)
		isPrefix := false
		for _, p := range out.CommonPrefixes {
			isPrefix = isPrefix || p == last
		}
		out.NextToken, out.NextMarker = EncodeListToken(isPrefix, last), last
	}
	// What the listing read is Wasabi's egress, listed or not.
	wasabiObjects := page.Objects
	for _, p := range page.CommonPrefixes {
		wasabiObjects = append(wasabiObjects, ObjectInfo{Key: p})
	}
	s.RecordEgress(ctx, workspaceID, listResponseBytes(base+req.Prefix, wasabiObjects))
	return out, nil
}
