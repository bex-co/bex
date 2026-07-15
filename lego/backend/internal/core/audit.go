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
	"log"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// AuditOutcome is the result of an authorized write attempt (w4/m10).
type AuditOutcome string

const (
	AuditAllowed AuditOutcome = "allowed"
	AuditDenied  AuditOutcome = "denied"
)

// AuditEvent is one recorded write-verb authorization decision: who attempted
// what, against which object, and whether it was allowed. Never carries
// request bodies or values — Verb/Resource/Target are derived from the call
// site, the OpenFGA object string, and the verb's resource NAME, never from a
// verb's value-bearing arguments, so there is no path for a secret to reach an
// event.
type AuditEvent struct {
	Caller       string // core.Identity.Subject; empty for an unauthenticated caller
	CallerMethod string // core.Identity.Method ("oauth2" | "session")
	Verb         string // the verb's short name, e.g. "apps.Suspend" (derived, never hand-set)
	Resource     string // the OpenFGA object authorized against, e.g. "workspace:tea-abc"
	Target       string // the resource acted ON, e.g. "service:my-api"; empty for a workspace-wide verb
	Outcome      AuditOutcome
	At           time.Time
}

// ServiceTarget is the AuditEvent.Target of a verb acting on one service — the
// dimension the audit log lacked until w3/m7 (Resource is the OpenFGA object,
// i.e. the WORKSPACE a verb was authorized against, which cannot say WHICH
// service was suspended). With it, the per-service events feed (internal/events)
// is a pure VIEW over audit_events + deploys: no second write path, and the
// structural redaction holds — a target is a resource name, never a value.
//
// Note the App CR name is globally unique (all Apps live in one namespace), so
// a service target is unambiguous across tenants; internal/events still scopes
// its read by workspace so a cross-tenant caller cannot inject rows into
// someone else's feed (see store.ListServiceEvents).
func ServiceTarget(name string) string { return "service:" + name }

// DatabaseTarget is ServiceTarget's sibling for a managed Postgres Database
// (AuthorizeDatabase).
func DatabaseTarget(name string) string { return "database:" + name }

// KeyValueTarget is ServiceTarget's sibling for a managed KeyValue
// (AuthorizeKeyValue).
func KeyValueTarget(name string) string { return "keyvalue:" + name }

// AuditSink persists audit events. Base.emit bounds every call to
// auditRecordTimeout and always swallows a Record error (logged, never
// returned) — a sink outage adds at most that much latency to the verb it's
// recording, never fails or indefinitely stalls it.
type AuditSink interface {
	Record(ctx context.Context, ev AuditEvent) error
}

// auditRecordTimeout bounds Base.emit's sink call: recording must never
// indefinitely block the verb path it's instrumenting (a stalled sink adds at
// most this much latency, not an unbounded stall). Short relative to
// apikeys.Service.TouchAPIKey's fully-detached 10s fire-and-forget — this call
// is still synchronous on the verb's own path, so it stays tight.
const auditRecordTimeout = 2 * time.Second

// noopAuditSink is the store-less default: recording is a no-op, the same
// degrade-by-omission every other store-backed feature uses.
type noopAuditSink struct{}

func (noopAuditSink) Record(context.Context, AuditEvent) error { return nil }

// NoopAuditSink is Base's effective sink when Audit is nil.
var NoopAuditSink AuditSink = noopAuditSink{}

// writeRelations are the Rel… constants that gate a mutation. These emit an
// audit event on every call, allowed or denied.
var writeRelations = map[string]bool{
	RelCanOperate:       true,
	RelCanCreate:        true,
	RelCanManageKeys:    true,
	RelCanManageSSHKeys: true,
	RelCanManage:        true,
}

// readRelations are the Rel… constants that gate a read. A successful read
// stays unrecorded (volume-prohibitive — every list/get/metrics call would
// double audit_events' write rate); a DENIED read is a security-relevant
// event (someone probed a resource they can't see) and is recorded exactly
// like a denied write (w4/m20/t001, closing the deliberate w4/m10 cut noted
// in docs/ADR006-bex-api.md § Audit log). No entry-point split: Authorize,
// AuthorizeTarget/AuthorizeApp-family and AuthorizeOn all funnel through
// authorizeAndAudit, so a denied AuthorizeOn-style workspace-wide read check
// records exactly like a denied resource-scoped one — Target is simply
// whatever the entry point already passes (empty for workspace-wide, the
// resource name for AuthorizeApp/AuthorizeDatabase/AuthorizeKeyValue), the
// same split write-verb recording already uses.
var readRelations = map[string]bool{
	RelCanView:          true,
	RelCanViewLogs:      true,
	RelCanViewSensitive: true,
}

// receiverRE strips a method's pointer-receiver type (e.g. "(*Service).") out
// of its runtime-reported name, so "apps.(*Service).Suspend" reads as
// "apps.Suspend" — a short, greppable verb.
var receiverRE = regexp.MustCompile(`\(\*?\w+\)\.`)

// verbFrameSkip is the runtime.Caller depth from inside callerVerb up to the
// verb method that called Authorize/AuthorizeTarget/AuthorizeOn: 0 is
// callerVerb's own frame, 1 is the entry point (callerVerb's direct caller), 2
// is the verb that called it — CLAUDE.md's "every verb starts with s.Authorize"
// invariant is what makes this constant, not a per-call guess.
//
// Named so all THREE entry points in base.go can't drift to different skip
// counts — and note this is why each of them calls callerVerb(verbFrameSkip)
// itself rather than delegating to a sibling: one entry point implemented in
// terms of another would add a frame, silently rename every recorded verb to
// "Base.Authorize…", and (since w3/m7 keys the events feed off the verb name)
// empty every service's activity feed without failing anything.
const verbFrameSkip = 2

// helperWalkLimit bounds how many unexported-helper frames callerVerb walks
// past looking for the exported verb — deeper than any real helper chain
// (setSuspended → writeThroughStore is 2), small enough that a pathological
// stack can't make every authorize call crawl it.
const helperWalkLimit = 8

// callerVerb resolves the short "<package>.<Method>" name of the verb that
// called into an authorize entry point, starting skip frames up the stack.
//
// The frame at skip is usually the verb itself ("every verb starts with
// s.Authorize…"). But a verb may share its write path with siblings through an
// UNEXPORTED helper method (apps.Suspend → setSuspended → writeThroughStore →
// AuthorizeApp, the w2/m30 consolidation), and recording the helper's name
// would collapse distinct verbs into one meaningless "apps.writeThroughStore"
// row — un-mapping every one of them from the events feed's and the outbound
// webhooks' verb-keyed vocabularies at once (found live by w3/m11/t008). So:
// walk upward past unexported *methods* — shared helpers by construction —
// until the exported verb method appears. Plain functions and closures are
// never skipped: a test's stand-in verb (audit_test.go's suspendLikeVerb) and
// a handler closure are terminal callers, not shared write paths.
//
// A bad skip count is a bug caught by audit_test.go's expected-verb
// assertions, not a panic (an unresolved frame degrades to "unknown", never
// blocks the verb).
func callerVerb(skip int) string {
	// One unwind for the whole candidate window (runtime.Callers), not one
	// full-stack runtime.Caller per candidate — this runs on every write-verb
	// authorize. +1: Callers counts skip from itself where Caller counts from
	// its caller.
	pcs := make([]uintptr, helperWalkLimit+1)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(skip+1, pcs)])
	first := ""
	for {
		frame, more := frames.Next()
		name := frame.Function
		if name == "" {
			break
		}
		if j := strings.LastIndex(name, "/"); j >= 0 {
			name = name[j+1:]
		}
		short := receiverRE.ReplaceAllString(name, "")
		if first == "" {
			first = short
		}
		if !unexportedHelperMethod(name, short) {
			return short
		}
		if !more {
			break
		}
	}
	if first == "" {
		return "unknown"
	}
	return first // nothing but helpers in budget: degrade to the nearest frame
}

// unexportedHelperMethod reports whether a resolved frame is an unexported
// METHOD ("apps.(*Service).writeThroughStore" → "apps.writeThroughStore") —
// the shared-helper shape callerVerb walks past. name is the runtime name
// after path-trimming (receiver still present); short is the receiver-stripped
// "<pkg>.<func>" form.
func unexportedHelperMethod(name, short string) bool {
	if !receiverRE.MatchString(name) {
		return false // plain function (or nothing to strip): terminal
	}
	_, method, ok := strings.Cut(short, ".")
	if !ok || strings.Contains(method, ".") {
		return false // closures ("….func1") are terminal, never walked past
	}
	r := rune(method[0])
	return r >= 'a' && r <= 'z'
}

// emit records one audit event for an authorize call: a write relation's
// success and denial alike, a read relation's denial only —
// authorizeAndAudit filters before calling. A sink error is logged and
// swallowed, never returned: audit recording must never fail the verb it's
// recording.
func (b *Base) emit(ctx context.Context, verb, resource, target string, authzErr error) {
	sink := b.Audit
	if sink == nil {
		sink = NoopAuditSink
	}
	outcome := AuditAllowed
	if authzErr != nil {
		outcome = AuditDenied
	}
	ev := AuditEvent{Verb: verb, Resource: resource, Target: target, Outcome: outcome, At: b.Now()}
	if id, ok := IdentityFrom(ctx); ok {
		ev.Caller, ev.CallerMethod = id.Subject, id.Method
	}
	recordCtx, cancel := context.WithTimeout(ctx, auditRecordTimeout)
	defer cancel()
	if err := sink.Record(recordCtx, ev); err != nil {
		log.Printf("audit: record %s %s: %v", verb, resource, err)
	}
}
