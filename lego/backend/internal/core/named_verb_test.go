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
	"strings"
	"testing"
	"time"
)

// w21/093: callerVerb names a verb after the first frame above an authorize entry
// point that is not an unexported method, so a closure, a goroutine, a worker's
// step and a handler mounted as a method value recorded their own names
// ("agentsessions.RunOnce.func1", "core.runStep"). These pin the two ways a path
// now names its verb instead — an ...As entry point, and WithAuditVerb for a chain
// too deep to hand a verb down — and that neither renames a verb method.

// denyingBase is a Base over one service, one Database and one KeyValue of
// workspace tea-a that refuses every check; the caller is a member of it.
func denyingBase() *Base {
	b := resolvingBase(tenantDatabase("tea-a", "dpg-x", "tea-a"), tenantKeyValue("tea-a", "red-x", "tea-a"), sampleApp("web", "tea-a"))
	b.Authz = fakeDenyChecker{}
	return b
}

// namedVerbService stands in for a feature service: Suspend is a verb method that
// authorizes through an unexported helper, as apps.Service.Suspend does.
type namedVerbService struct{ *Base }

func (s *namedVerbService) Suspend(ctx context.Context) error { return s.suspendHelper(ctx) }

func (s *namedVerbService) suspendHelper(ctx context.Context) error {
	return s.Authorize(ctx, RelCanOperate)
}

// step is a worker's step: no verb method is on its stack.
func (s *namedVerbService) step(ctx context.Context) error { return s.suspendHelper(ctx) }

func (s *namedVerbService) runWorker(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// The step is a method value handed to core, as a worker's Run hands its tick
	// to core.Poll: core's loop is the frame above it.
	PollTicks(ctx, "named-verb-test", make(chan time.Time), func(ctx context.Context) error {
		cancel()
		return s.step(ctx)
	})
}

func (s *namedVerbService) goroutine(ctx context.Context) error {
	done := make(chan error)
	go func() { done <- s.suspendHelper(ctx) }()
	return <-done
}

func recordedVerbs(sink *fakeAuditSink) []string {
	var out []string
	for _, ev := range sink.events {
		out = append(out, ev.Verb)
	}
	return out
}

func TestAsEntryPointsRecordTheVerbTheyAreGiven(t *testing.T) {
	const verb = "feature.NamedAction"
	for _, tc := range []struct {
		name string
		call func(*Base, context.Context) error
	}{
		{"AuthorizeAs", func(b *Base, ctx context.Context) error { return b.AuthorizeAs(ctx, verb, RelCanOperate) }},
		{"AuthorizeTargetAs", func(b *Base, ctx context.Context) error {
			return b.AuthorizeTargetAs(ctx, verb, RelCanOperate, ServiceTarget("web"))
		}},
		{"AuthorizeOnAs", func(b *Base, ctx context.Context) error {
			return b.AuthorizeOnAs(ctx, verb, RelCanOperate, WorkspaceObject("tea-a"))
		}},
		{"AuthorizeWorkspaceAs", func(b *Base, ctx context.Context) error {
			_, err := b.AuthorizeWorkspaceAs(ctx, verb, RelCanOperate, "tea-a")
			return err
		}},
		{"AuthorizeWorkspaceTargetAs", func(b *Base, ctx context.Context) error {
			_, err := b.AuthorizeWorkspaceTargetAs(ctx, verb, RelCanOperate, "tea-a", BucketTarget("assets"))
			return err
		}},
		{"AuthorizeAppAs", func(b *Base, ctx context.Context) error {
			_, err := b.AuthorizeAppAs(ctx, verb, RelCanOperate, "web")
			return err
		}},
		{"AuthorizeDatabaseAs", func(b *Base, ctx context.Context) error {
			_, err := b.AuthorizeDatabaseAs(ctx, verb, RelCanOperate, "dpg-x")
			return err
		}},
		{"AuthorizeKeyValueAs", func(b *Base, ctx context.Context) error {
			_, err := b.AuthorizeKeyValueAs(ctx, verb, RelCanOperate, "red-x")
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, allow := range []bool{true, false} {
				b := denyingBase()
				if allow {
					b.Authz = &fakeAllowChecker{}
				}
				sink := &fakeAuditSink{}
				b.Audit = sink
				// From a goroutine's closure, where a derived verb is the closure's name.
				done := make(chan error)
				go func() { done <- tc.call(b, actingCtx()) }()
				<-done
				if got := recordedVerbs(sink); len(got) != 1 || got[0] != verb {
					t.Errorf("allowed=%v recorded %v, want exactly [%s]", allow, got, verb)
				}
			}
		})
	}
}

// Every ...As entry point authorizes exactly as its derived twin does: only the
// recorded verb differs.
func TestAsEntryPointsAuthorizeLikeTheirTwins(t *testing.T) {
	b := denyingBase()
	b.Authz = denyAllButChecker{allowed: WorkspaceObject("tea-a")}
	ctx := actingCtx()
	if _, err := b.AuthorizeDatabaseAs(ctx, "feature.Named", RelCanOperate, "dpg-x"); err != nil {
		t.Fatalf("AuthorizeDatabaseAs on the resource's own workspace: %v", err)
	}
	if _, err := b.AuthorizeDatabaseAs(ctx, "feature.Named", RelCanOperate, "absent"); err == nil {
		t.Fatal("AuthorizeDatabaseAs found a database that does not exist")
	}
	b.Authz = fakeDenyChecker{}
	if _, err := b.AuthorizeKeyValueAs(ctx, "feature.Named", RelCanOperate, "red-x"); err == nil {
		t.Fatal("AuthorizeKeyValueAs allowed a denied caller")
	}
}

func TestWithAuditVerbNamesWhatNoVerbMethodNames(t *testing.T) {
	const named = "feature.NamedOperation"
	for _, tc := range []struct {
		name string
		ctx  func(context.Context) context.Context
		call func(*namedVerbService, context.Context) error
		want func(got string) bool
	}{
		{"a worker's step", func(c context.Context) context.Context { return WithAuditVerb(c, named) },
			func(s *namedVerbService, ctx context.Context) error { s.runWorker(ctx); return nil },
			func(got string) bool { return got == named }},
		{"a goroutine's closure", func(c context.Context) context.Context { return WithAuditVerb(c, named) },
			(*namedVerbService).goroutine,
			func(got string) bool { return got == named }},
		{"a plain function", func(c context.Context) context.Context { return WithAuditVerb(c, named) },
			func(s *namedVerbService, ctx context.Context) error { return suspendLikeVerb(ctx, s.Base) },
			func(got string) bool { return got == named }},
		// A verb method names its own verb, whatever the context says: naming an
		// operation never renames a verb it calls.
		{"a verb method beneath it", func(c context.Context) context.Context { return WithAuditVerb(c, named) },
			(*namedVerbService).Suspend,
			func(got string) bool { return got == "core.Suspend" }},
		// Without a name nothing changes.
		{"a worker's step, unnamed", func(c context.Context) context.Context { return c },
			func(s *namedVerbService, ctx context.Context) error { s.runWorker(ctx); return nil },
			func(got string) bool { return strings.Contains(got, "func") || got == "core.runStep" }},
		{"a goroutine's closure, unnamed", func(c context.Context) context.Context { return c },
			(*namedVerbService).goroutine,
			func(got string) bool { return strings.Contains(got, "func") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sink := &fakeAuditSink{}
			b := denyingBase()
			b.Audit = sink
			_ = tc.call(&namedVerbService{Base: b}, tc.ctx(actingCtx()))
			got := recordedVerbs(sink)
			if len(got) != 1 || !tc.want(got[0]) {
				t.Fatalf("recorded %v", got)
			}
		})
	}
}

// A context without an operation name leaves an empty verb to the walk, and an
// empty name is no name.
func TestWithAuditVerbIgnoresAnEmptyName(t *testing.T) {
	sink := &fakeAuditSink{}
	b := denyingBase()
	b.Audit = sink
	_ = (&namedVerbService{Base: b}).goroutine(WithAuditVerb(actingCtx(), ""))
	if got := recordedVerbs(sink); len(got) != 1 || !strings.Contains(got[0], "func") {
		t.Fatalf("recorded %v, want the closure's name", got)
	}
}
