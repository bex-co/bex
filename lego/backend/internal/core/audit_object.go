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

package core

import (
	"context"
	"fmt"
	"log"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The audit target of the object store's routes (w21/094, ADR112 D6).
//
// An object or bucket row used to name only the workspace, so a reader could not
// tell which bucket or key a put, a delete or a bucket change touched. The target
// is "bucket:<name>" for a route that acts on a bucket and "object:<bucket>/<key>"
// for one that acts on an object (Render's routes, which have no bucket in their
// path, name their system bucket, render-<region>). Both come from the route's
// path, never from a header, a query string or a body, so a presigned URL or a
// credential has no way in; the text a caller chose is escaped and bounded here.

const (
	// AuditVerbGetObject is a download of an object's bytes: Render's wire has no
	// such event, but Render audits the downloads of customer data it has
	// (DownloadDatabaseBackupEvent, DocumentDownloadEvent), once per request;
	// bex's own Postgres export download is the same shape (w21/105).
	// RecordObjectViewed writes it once the object opened.
	AuditVerbGetObject = "objectstorage.GetObject"

	// AuditKeyMaxBytes bounds the key in a target, after escaping and marker
	// included; an object key is at most 1024 bytes (objectstorage.MaxKeyBytes).
	AuditKeyMaxBytes = 256
	// auditBucketMaxBytes bounds a bucket name in a target. A valid name is at most
	// 63 bytes; the rest is a refused caller's garbage, which is still bounded.
	auditBucketMaxBytes = 64
	// auditMarkerReserve is room for the truncation marker: the ellipsis, the
	// words and the length of the original, which is at most ten digits.
	auditMarkerReserve = 32
)

// auditedViews are the verbs of a read that records a row when it SUCCEEDS,
// against the rule that a successful read stays unrecorded (readRelations): a view
// of secret values, a view of connection info, a download of a Postgres logical
// backup and a download of an object. It is closed on purpose. Each is a read
// Render audits as a view or a download of customer data, once per request
// (ViewEnvVarValuesEvent, ViewConnectionInfoEvent, DownloadDatabaseBackupEvent),
// or the object store's twin of it. recordView refuses any other verb, and the
// audit sweeps in internal/api fail an allowed read that records a row under a
// verb that is not listed here.
var auditedViews = map[string]bool{
	AuditVerbViewEnvVarValues:       true,
	AuditVerbViewConnectionInfo:     true,
	AuditVerbDownloadDatabaseBackup: true,
	AuditVerbGetObject:              true,
}

// AuditedViewVerbs lists the verbs of auditedViews, sorted.
func AuditedViewVerbs() []string { return slices.Sorted(maps.Keys(auditedViews)) }

// recordView writes the row of one successful audited read. verb must be one of
// auditedViews: a caller cannot widen the set by passing another.
func (b *Base) recordView(ctx context.Context, verb, workspaceID, target, targetName string) {
	if !auditedViews[verb] {
		log.Printf("audit: %q is not an audited view; a successful read of it records nothing", verb)
		return
	}
	b.RecordAllowedWrite(ctx, verb, "", workspaceID, target, targetName)
}

// RecordObjectViewed records one download of an object: Contract B's streamed
// get once the object opened and before its first byte, Contract A's mint of a
// presigned GET once it was minted. One row per request, never one per chunk or
// per range; a HEAD, a list and a get that found no object record none. target is
// the route's ObjectTarget.
func (b *Base) RecordObjectViewed(ctx context.Context, workspaceID, target string) {
	b.recordView(ctx, AuditVerbGetObject, workspaceID, target, "")
}

// BucketTarget is the AuditEvent.Target of a verb acting on one bucket, by the
// name its route's path carries. "" for an empty name.
func BucketTarget(name string) string {
	if name == "" {
		return ""
	}
	return "bucket:" + auditText(name, auditBucketMaxBytes, true)
}

// AccessKeyTarget is the target of a verb acting on one S3 access key: its id,
// which is a typed s3k- id and carries no secret.
func AccessKeyTarget(id string) string {
	if id == "" {
		return ""
	}
	return "access_key:" + auditText(id, auditBucketMaxBytes, true)
}

// ObjectTarget is the target of a verb acting on one object of a bucket. It names
// the bucket alone when key is empty. A key is a name the caller chose, so it is
// bounded to AuditKeyMaxBytes and its control characters are escaped; a signed
// query a caller pasted into a key is replaced, so a presigned URL cannot reach
// the log through the one field that is theirs to fill.
func ObjectTarget(bucket, key string) string {
	if key == "" {
		return BucketTarget(bucket)
	}
	if bucket == "" {
		return ""
	}
	return "object:" + auditText(bucket, auditBucketMaxBytes, true) + "/" + auditText(redactSignedQuery(key), AuditKeyMaxBytes, false)
}

// SplitObjectTarget is ObjectTarget's inverse for the name SplitTarget returns
// for the "object" kind. A bucket name carries no "/" (a valid one cannot, and
// auditText escapes it in an invalid one), so the first "/" ends it.
func SplitObjectTarget(name string) (bucket, key string, ok bool) {
	bucket, key, ok = strings.Cut(name, "/")
	if !ok || bucket == "" || key == "" {
		return "", "", false
	}
	return bucket, key, true
}

// signedQuery matches the parameters of a presigned URL and of a signed request:
// AWS and Google signing, a signature, a credential or a token.
var signedQuery = regexp.MustCompile(`(?i)x-amz-|x-goog-|signature=|credential=|token=|sig=`)

// redactSignedQuery cuts a key at the "?" that opens a signed query. A key may
// hold a "?" (what?.txt is a valid S3 key); only one followed by signing
// parameters is replaced.
func redactSignedQuery(key string) string {
	i := strings.IndexByte(key, '?')
	if i < 0 || !signedQuery.MatchString(key[i:]) {
		return key
	}
	return key[:i] + "?[redacted]"
}

// auditText is s made safe to store and to show: valid UTF-8 (a byte that is not
// is written \xNN), no control, format or line-separating character (written
// \uXXXX, so a bidirectional override or a newline cannot reorder or forge a line
// of the log), no backslash (written \\, so an escape is never mistaken for text),
// and at most max bytes. A text over max is cut on a character boundary and ends in
// "…[truncated, N bytes]", N being the original's length. escapeSeparators also
// escapes "/" and ":", for a name that sits before one. It reads no more of s
// than max bytes of output need, so a megabyte of garbage costs the same.
func auditText(s string, max int, escapeSeparators bool) string {
	limit := max - auditMarkerReserve
	out := make([]byte, 0, min(len(s), max))
	cut := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		piece := s[i : i+size]
		switch {
		case r == utf8.RuneError && size == 1:
			piece = fmt.Sprintf(`\x%02x`, s[i])
		case r == '\\':
			piece = `\\`
		case escapeSeparators && (r == '/' || r == ':'):
			piece = escapeRune(r)
		case unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp):
			piece = escapeRune(r)
		}
		if len(out)+len(piece) > max {
			return string(out[:cut]) + fmt.Sprintf("…[truncated, %d bytes]", len(s))
		}
		out = append(out, piece...)
		if len(out) <= limit {
			cut = len(out)
		}
		i += size
	}
	return string(out)
}

func escapeRune(r rune) string {
	if r <= 0xffff {
		return fmt.Sprintf(`\u%04x`, r)
	}
	return fmt.Sprintf(`\U%08x`, r)
}
