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
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// w21/094: an object or bucket row names its bucket, and an object row its key, in
// the audit target. The key is the one field a caller fills, so it is escaped and
// bounded, and a presigned URL pasted into it is cut.

func TestObjectTargetNamesBucketAndKey(t *testing.T) {
	for _, tc := range []struct{ bucket, key, want string }{
		{"assets", "logo.png", "object:assets/logo.png"},
		{"assets", "a/b/c.txt", "object:assets/a/b/c.txt"},
		{"assets", "what?.txt", "object:assets/what?.txt"}, // a "?" is valid in a key
		{"render-oregon", "export.csv", "object:render-oregon/export.csv"},
		{"assets", "", "bucket:assets"},
		{"assets", "日本語/ファイル.txt", "object:assets/日本語/ファイル.txt"},
		{"", "k", ""},
		{"", "", ""},
	} {
		if got := ObjectTarget(tc.bucket, tc.key); got != tc.want {
			t.Errorf("ObjectTarget(%q, %q) = %q, want %q", tc.bucket, tc.key, got, tc.want)
		}
	}
	if got := BucketTarget("assets"); got != "bucket:assets" {
		t.Errorf("BucketTarget = %q", got)
	}
	if got := BucketTarget(""); got != "" {
		t.Errorf("BucketTarget(\"\") = %q, want none", got)
	}
}

// The target is decoded as every other: SplitTarget cuts at the first colon, and
// SplitObjectTarget cuts the object kind's name at the first slash.
func TestObjectTargetRoundTrips(t *testing.T) {
	kind, name, ok := SplitTarget(ObjectTarget("assets", "a:b/c d.txt"))
	if !ok || kind != "object" {
		t.Fatalf("SplitTarget = %q %q %v", kind, name, ok)
	}
	bucket, key, ok := SplitObjectTarget(name)
	if !ok || bucket != "assets" || key != "a:b/c d.txt" {
		t.Fatalf("SplitObjectTarget = %q %q %v", bucket, key, ok)
	}
	if kind, name, ok := SplitTarget(BucketTarget("assets")); !ok || kind != "bucket" || name != "assets" {
		t.Fatalf("bucket target decodes to %q %q %v", kind, name, ok)
	}
	for _, bad := range []string{"", "assets", "/key", "assets/"} {
		if _, _, ok := SplitObjectTarget(bad); ok {
			t.Errorf("SplitObjectTarget(%q) accepted it", bad)
		}
	}
}

// A refused caller's bucket and key are not validated yet, so each must come out
// harmless: valid UTF-8, no control character, no way to end the bucket early.
func TestObjectTargetEscapesWhatACallerChose(t *testing.T) {
	for _, tc := range []struct{ name, bucket, key, want string }{
		{"newline", "assets", "a\nb", `object:assets/a\u000ab`},
		{"carriage return and NUL", "assets", "a\r\x00b", `object:assets/a\u000d\u0000b`},
		{"DEL", "assets", "a\x7fb", `object:assets/a\u007fb`},
		{"a C1 control", "assets", "a\u0085b", `object:assets/a\u0085b`},
		{"a bidirectional override", "assets", "a\u202eb", `object:assets/a\u202eb`},
		{"a line separator", "assets", "a\u2028b", `object:assets/a\u2028b`},
		{"a zero-width joiner", "assets", "a\u200db", `object:assets/a\u200db`},
		{"a byte that is not UTF-8", "assets", "a\xffb\xc3", `object:assets/a\xffb\xc3`},
		{"a backslash", "assets", `a\nb`, `object:assets/a\\nb`},
		{"a slash in the bucket", "a/b", "k", `object:a\u002fb/k`},
		{"a colon in the bucket", "a:b", "k", `object:a\u003ab/k`},
		{"a plane-1 control", "assets", "a\U000E0001b", `object:assets/a\U000e0001b`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ObjectTarget(tc.bucket, tc.key)
			if got != tc.want {
				t.Errorf("ObjectTarget(%q, %q) = %q, want %q", tc.bucket, tc.key, got, tc.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("target %q is not valid UTF-8", got)
			}
			// However the bucket was written, the first "/" ends it.
			bucket, _, ok := SplitObjectTarget(strings.TrimPrefix(got, "object:"))
			if !ok || strings.ContainsAny(bucket, "/:") {
				t.Errorf("the bucket of %q is %q, ok %v", got, bucket, ok)
			}
			if strings.ContainsAny(got, "\n\r\x00") {
				t.Errorf("target %q holds a raw control character", got)
			}
		})
	}
}

func TestObjectTargetBoundsTheKey(t *testing.T) {
	// A key of the longest length bex stores (1024 bytes).
	long := strings.Repeat("k", 1024)
	got := ObjectTarget("assets", long)
	key := strings.TrimPrefix(got, "object:assets/")
	if len(key) > AuditKeyMaxBytes {
		t.Errorf("the key of a 1024-byte key is %d bytes, over the bound of %d", len(key), AuditKeyMaxBytes)
	}
	if !strings.HasSuffix(key, "…[truncated, 1024 bytes]") {
		t.Errorf("a truncated key must say so and how long it was: %q", key)
	}
	if !strings.HasPrefix(key, strings.Repeat("k", 200)) {
		t.Errorf("the prefix of the key was lost: %q", key)
	}

	// A key that fits is whole, however close to the bound.
	for _, n := range []int{1, 100, AuditKeyMaxBytes - 1, AuditKeyMaxBytes} {
		k := strings.Repeat("k", n)
		if got := strings.TrimPrefix(ObjectTarget("assets", k), "object:assets/"); got != k {
			t.Errorf("a %d-byte key was changed: %q", n, got)
		}
	}
	if got := strings.TrimPrefix(ObjectTarget("assets", strings.Repeat("k", AuditKeyMaxBytes+1)), "object:assets/"); !strings.Contains(got, "truncated") {
		t.Errorf("a key one byte over the bound was not marked: %q", got)
	}

	// The cut falls between characters, never inside one, and never inside an
	// escape.
	for _, unit := range []string{"é", "日", "😀", "\n", "\xff"} {
		got := ObjectTarget("assets", strings.Repeat(unit, 600))
		key := strings.TrimPrefix(got, "object:assets/")
		if len(key) > AuditKeyMaxBytes || !utf8.ValidString(key) {
			t.Errorf("a key of %q repeated: %d bytes, valid UTF-8 %v", unit, len(key), utf8.ValidString(key))
		}
		body := key[:strings.Index(key, "…")]
		if strings.HasSuffix(body, `\`) || strings.HasSuffix(body, `\u000`) || strings.HasSuffix(body, `\x`) {
			t.Errorf("the cut fell inside an escape: %q", key)
		}
	}

	// A megabyte of garbage costs the same.
	if got := ObjectTarget("assets", strings.Repeat("\x00", 1<<20)); len(got) > len("object:assets/")+AuditKeyMaxBytes {
		t.Errorf("a megabyte key gave a %d-byte target", len(got))
	}
	// A bucket is bounded too.
	if got := BucketTarget(strings.Repeat("b", 500)); len(got) > len("bucket:")+auditBucketMaxBytes {
		t.Errorf("a 500-byte bucket gave a %d-byte target", len(got))
	}
}

// A presigned URL or a signed query pasted into a key does not reach the log.
func TestObjectTargetCutsASignedQueryOutOfAKey(t *testing.T) {
	const secret = "deadbeef0123456789"
	for _, key := range []string{
		"a.txt?X-Amz-Signature=" + secret,
		"https:/bucket.s3.amazonaws.com/a.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKIAEXAMPLE%2F20261116&X-Amz-Signature=" + secret,
		"a.txt?x-goog-signature=" + secret,
		"a.txt?Signature=" + secret + "&Expires=1",
		"a.txt?token=" + secret,
	} {
		got := ObjectTarget("assets", key)
		for _, leak := range []string{secret, "AKIAEXAMPLE", "Credential", "X-Amz", "Signature", "token="} {
			if strings.Contains(got, leak) {
				t.Errorf("ObjectTarget(%q) = %q, which holds %q", key, got, leak)
			}
		}
		if !strings.Contains(got, "?[redacted]") {
			t.Errorf("ObjectTarget(%q) = %q, want the signed query replaced", key, got)
		}
	}
	// A "?" that opens nothing signed is part of the name.
	if got := ObjectTarget("assets", "what?.txt?page=2"); got != "object:assets/what?.txt?page=2" {
		t.Errorf("a key with a plain query was changed: %q", got)
	}
}

// RecordObjectViewed is one allowed row of the get verb, on the workspace, naming
// the object, with the caller; and nothing else records a view.
func TestRecordObjectViewedWritesOneRow(t *testing.T) {
	sink := &fakeAuditSink{}
	b := &Base{Audit: sink}
	ctx := WithIdentity(context.Background(), Identity{Subject: "user-1", Method: MethodSession})
	b.RecordObjectViewed(ctx, "tea-a", ObjectTarget("assets", "logo.png"))
	if sink.len() != 1 {
		t.Fatalf("recorded %d rows, want 1: %+v", sink.len(), sink.events)
	}
	ev := sink.events[0]
	if ev.Verb != "objectstorage.GetObject" || ev.Outcome != AuditAllowed ||
		ev.Resource != WorkspaceObject("tea-a") || ev.Target != "object:assets/logo.png" || ev.Caller != "user-1" {
		t.Errorf("recorded %+v", ev)
	}
}

// w21/094 overrides the rule that a successful read records nothing for exactly
// the views Render audits as views or downloads of customer data (and bex's own
// download of an object), and for no other verb; w21/105 added the download of a
// Postgres logical backup. The set is pinned here by name: widening it is a
// decision with its own ADR line, so this test fails until the line is written and
// listed.
func TestOnlyTheAuditedViewsRecordASuccessfulRead(t *testing.T) {
	want := []string{AuditVerbDownloadDatabaseBackup, AuditVerbGetObject, AuditVerbViewConnectionInfo, AuditVerbViewEnvVarValues}
	slices.Sort(want)
	if got := AuditedViewVerbs(); !slices.Equal(got, want) {
		t.Fatalf("the audited views are %v, want exactly %v", got, want)
	}
	want = []string{"datastores.ViewConnectionInfo", "objectstorage.GetObject", "postgres.DownloadDatabaseBackup", "secrets.ViewEnvVarValues"}
	if got := AuditedViewVerbs(); !slices.Equal(got, want) {
		t.Fatalf("the audited views are %v, want exactly %v", got, want)
	}

	sink := &fakeAuditSink{}
	b := &Base{Audit: sink}
	ctx := WithIdentity(context.Background(), Identity{Subject: "user-1", Method: MethodSession})
	// A caller of the one writer that takes a verb cannot widen the set.
	for _, verb := range []string{"postgres.Query", "objectstorage.ListObjects", "objectstorage.PutObject", "apps.Get", ""} {
		b.RecordSecretsViewed(ctx, verb, "tea-a", ServiceTarget("srv-1"), "web")
	}
	if sink.len() != 0 {
		t.Fatalf("a verb outside the set recorded %+v", sink.events)
	}
	for _, verb := range want {
		b.RecordSecretsViewed(ctx, verb, "tea-a", ServiceTarget("srv-1"), "web")
	}
	if sink.len() != len(want) {
		t.Fatalf("the audited views recorded %d rows, want %d", sink.len(), len(want))
	}
}

// A read relation still records a refusal only: the get's allowed row comes from
// RecordObjectViewed once the object opened, not from authorizing, so a get of an
// object that is not there, or one the limiter refused, leaves no success row.
func TestAnAllowedGetAuthorizationRecordsNothingAndADeniedOneStillDoes(t *testing.T) {
	sink := &fakeAuditSink{}
	ctx := WithIdentity(context.Background(), Identity{Subject: "user-1", Method: MethodSession})
	target := ObjectTarget("assets", "logo.png")

	allow := &Base{Authz: &fakeAllowChecker{}, Audit: sink}
	if _, err := allow.AuthorizeWorkspaceTargetAs(ctx, AuditVerbGetObject, RelCanViewSensitive, "tea-a", target); err != nil {
		t.Fatal(err)
	}
	if sink.len() != 0 {
		t.Fatalf("an allowed get authorization recorded %+v", sink.events)
	}

	deny := &Base{Authz: &fakeDenyChecker{}, Audit: sink}
	if _, err := deny.AuthorizeWorkspaceTargetAs(ctx, AuditVerbGetObject, RelCanViewSensitive, "tea-a", target); err == nil {
		t.Fatal("a denied caller was allowed")
	}
	if sink.len() != 1 {
		t.Fatalf("a denied get recorded %d rows, want 1", sink.len())
	}
	if ev := sink.events[0]; ev.Verb != AuditVerbGetObject || ev.Outcome != AuditDenied || ev.Target != target || ev.Relation != RelCanViewSensitive {
		t.Fatalf("the denied row is %+v", ev)
	}
}
