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
// request bodies or values — Verb/Resource are derived from the call site and
// the OpenFGA object string, never from a verb's arguments, so there is no
// path for a secret to reach an event.
type AuditEvent struct {
	Caller       string // core.Identity.Subject; empty for an unauthenticated caller
	CallerMethod string // core.Identity.Method ("oauth2" | "session")
	Verb         string // the verb's short name, e.g. "apps.Suspend" (derived, never hand-set)
	Resource     string // the OpenFGA object authorized against, e.g. "workspace:tea-abc"
	Outcome      AuditOutcome
	At           time.Time
}

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

// writeRelations are the Rel… constants that gate a mutation. Only these emit
// audit events — read relations (RelCanView/RelCanViewLogs/RelCanViewSensitive)
// are out of scope by default (volume); AuditOutcome already distinguishes
// allowed/denied, so recording read denials later needs no schema change.
var writeRelations = map[string]bool{
	RelCanOperate:    true,
	RelCanCreate:     true,
	RelCanManageKeys: true,
	RelCanManage:     true,
}

// receiverRE strips a method's pointer-receiver type (e.g. "(*Service).") out
// of its runtime-reported name, so "apps.(*Service).Suspend" reads as
// "apps.Suspend" — a short, greppable verb.
var receiverRE = regexp.MustCompile(`\(\*?\w+\)\.`)

// verbFrameSkip is the runtime.Caller depth from inside callerVerb up to the
// verb method that called Authorize/AuthorizeOn: 0 is callerVerb's own frame,
// 1 is Authorize/AuthorizeOn (callerVerb's direct caller), 2 is the verb that
// called one of those — CLAUDE.md's "every verb starts with s.Authorize"
// invariant is what makes this constant, not a per-call guess. Named so both
// call sites in base.go can't drift to different skip counts.
const verbFrameSkip = 2

// callerVerb resolves the short "<package>.<Method>" name of the function skip
// frames up the stack — the verb method that called into Authorize/AuthorizeOn.
// Unexported so only this file's two entry points use it; a bad skip count is a
// bug caught by audit_test.go's expected-verb assertions, not a panic (an
// unresolved frame degrades to "unknown", never blocks the verb).
func callerVerb(skip int) string {
	pc, _, _, ok := runtime.Caller(skip)
	if !ok {
		return "unknown"
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown"
	}
	name := fn.Name()
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return receiverRE.ReplaceAllString(name, "")
}

// emit records one audit event for a write-relation authorize call — success
// and denial alike; read relations never reach here (authorizeAndAudit filters
// before calling). A sink error is logged and swallowed, never returned: audit
// recording must never fail the verb it's recording.
func (b *Base) emit(ctx context.Context, verb, resource string, authzErr error) {
	sink := b.Audit
	if sink == nil {
		sink = NoopAuditSink
	}
	outcome := AuditAllowed
	if authzErr != nil {
		outcome = AuditDenied
	}
	ev := AuditEvent{Verb: verb, Resource: resource, Outcome: outcome, At: b.Now()}
	if id, ok := IdentityFrom(ctx); ok {
		ev.Caller, ev.CallerMethod = id.Subject, id.Method
	}
	recordCtx, cancel := context.WithTimeout(ctx, auditRecordTimeout)
	defer cancel()
	if err := sink.Record(recordCtx, ev); err != nil {
		log.Printf("audit: record %s %s: %v", verb, resource, err)
	}
}
