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

package secrets

import (
	"context"
	"errors"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// core.Base names the audit verb by walking the call stack (core/audit.go's
// callerVerb), and the events feed is keyed off that name, so a verb's recorded
// name is load-bearing wire behavior — not a log string. Two things can rename
// it silently, and neither fails anything on its own:
//
//   - adding a stack frame between the verb and AuthorizeApp (the `scope` seam
//     is exactly such a frame; callerVerb walks past it only because it is an
//     unexported METHOD), and
//   - renaming or exporting one of those helpers.
//
// Either would empty every service's activity feed while all other tests pass:
// the vocabularies in internal/events assert against these same strings, but as
// TABLE LITERALS — nothing else in the repo checks that the running code still
// produces them. core/audit_test.go covers the resolver with stand-in verbs;
// this covers the real ones.

// recordingAuditSink keeps whole events, not just verb names: the feed's query
// filters on verb AND target AND outcome (store/events.go), so a verb that
// regressed from AuthorizeApp to a plain Authorize would keep its name, lose
// its target, and still vanish from the feed.
type recordingAuditSink struct{ events []core.AuditEvent }

func (r *recordingAuditSink) Record(_ context.Context, ev core.AuditEvent) error {
	r.events = append(r.events, ev)
	return nil
}

// auditedService builds a Service whose authorize decisions land in sink.
func auditedService(allow bool, sink *recordingAuditSink) *Service {
	svc := newService(newFakeSecretStore(), tenantApp("web", "tea-a"))
	svc.Authz = &fakeChecker{allow: allow}
	svc.Audit = sink
	return svc
}

func auditCtx() context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: "user-1", Method: "oauth2"})
}

// Every write verb records its own name. A write relation is audited on success,
// so these run against an allowing checker.
func TestWriteVerbsRecordTheirOwnAuditVerbName(t *testing.T) {
	for _, tc := range []struct {
		want string
		// setup runs first and its events are discarded, so the assertion below
		// can demand EXACTLY ONE event from the verb under test — which also
		// catches a verb that authorizes twice.
		setup func(*Service, context.Context) error
		call  func(*Service, context.Context) error
	}{
		{want: "secrets.SetEnvVar", call: func(s *Service, ctx context.Context) error {
			_, err := s.SetEnvVar(ctx, "web", "K", EnvVarWrite{Value: "v"})
			return err
		}},
		{want: "secrets.SetEnvVars", call: func(s *Service, ctx context.Context) error {
			_, err := s.SetEnvVars(ctx, "web", []EnvVarView{{Key: "K", Value: "v"}})
			return err
		}},
		{
			want: "secrets.DeleteEnvVar",
			setup: func(s *Service, ctx context.Context) error {
				_, err := s.SetEnvVar(ctx, "web", "K", EnvVarWrite{Value: "v"})
				return err
			},
			call: func(s *Service, ctx context.Context) error {
				return s.DeleteEnvVar(ctx, "web", "K")
			},
		},
		{want: "secrets.SeedEnvVars", call: func(s *Service, ctx context.Context) error {
			return s.SeedEnvVars(ctx, "web", map[string]string{"K": "v"}, nil)
		}},
		{want: "secrets.SetSecretFile", call: func(s *Service, ctx context.Context) error {
			_, err := s.SetSecretFile(ctx, "web", "cert.pem", "body")
			return err
		}},
		{want: "secrets.SetSecretFiles", call: func(s *Service, ctx context.Context) error {
			_, err := s.SetSecretFiles(ctx, "web", []SecretFileView{{Name: "cert.pem", Content: "body"}})
			return err
		}},
		{want: "secrets.SeedSecretFiles", call: func(s *Service, ctx context.Context) error {
			return s.SeedSecretFiles(ctx, "web", []core.SecretFile{{Name: "cert.pem", Content: "body"}})
		}},
		{
			want: "secrets.DeleteSecretFile",
			setup: func(s *Service, ctx context.Context) error {
				_, err := s.SetSecretFile(ctx, "web", "cert.pem", "body")
				return err
			},
			call: func(s *Service, ctx context.Context) error {
				return s.DeleteSecretFile(ctx, "web", "cert.pem")
			},
		},
		{want: "secrets.PatchEnvironment", call: func(s *Service, ctx context.Context) error {
			_, err := s.PatchEnvironment(ctx, "web", EnvironmentPatch{
				SaveMode: SaveModeOnly,
				EnvVars:  []EnvVarPatch{{Key: "K", Value: "v"}},
			})
			return err
		}},
	} {
		t.Run(tc.want, func(t *testing.T) {
			sink := &recordingAuditSink{}
			svc := auditedService(true, sink)
			ctx := auditCtx()
			if tc.setup != nil {
				if err := tc.setup(svc, ctx); err != nil {
					t.Fatalf("setup: %v", err)
				}
				sink.events = nil
			}
			if err := tc.call(svc, ctx); err != nil {
				t.Fatalf("verb returned %v", err)
			}
			if len(sink.events) != 1 {
				t.Fatalf("recorded %d audit events, want exactly 1: %+v", len(sink.events), sink.events)
			}
			ev := sink.events[0]
			if ev.Verb != tc.want {
				t.Errorf("recorded verb = %q, want %q — the events feed is keyed off this name", ev.Verb, tc.want)
			}
			// The feed also filters on target and outcome, so a verb that kept its
			// name but stopped targeting its service would still vanish from it.
			if want := core.ServiceTarget("web"); ev.Target != want {
				t.Errorf("recorded target = %q, want %q", ev.Target, want)
			}
			if ev.Outcome != core.AuditAllowed {
				t.Errorf("recorded outcome = %q, want %q", ev.Outcome, core.AuditAllowed)
			}
		})
	}
}

// Sensitive reads are audited only when DENIED, so these run against a denying
// checker. Their names matter for the same reason: a denial has to be
// attributable to the verb the caller actually invoked.
func TestDeniedSensitiveReadsRecordTheirOwnAuditVerbName(t *testing.T) {
	for _, tc := range []struct {
		name string // the verb invoked; want is the name it RECORDS
		want string
		call func(*Service, context.Context) error
	}{
		{"ListEnvVars", "secrets.ListEnvVars", func(s *Service, ctx context.Context) error {
			_, err := s.ListEnvVars(ctx, "web")
			return err
		}},
		{"EnvVarValue", "secrets.EnvVarValue", func(s *Service, ctx context.Context) error {
			_, err := s.EnvVarValue(ctx, "web", "K")
			return err
		}},
		{"ListSecretFiles", "secrets.ListSecretFiles", func(s *Service, ctx context.Context) error {
			_, err := s.ListSecretFiles(ctx, "web")
			return err
		}},
		{"GetSecretFile", "secrets.GetSecretFile", func(s *Service, ctx context.Context) error {
			_, err := s.GetSecretFile(ctx, "web", "cert.pem")
			return err
		}},
		// A verb that delegates through another EXPORTED verb records the INNER
		// name: callerVerb walks past unexported helpers only, so the walk stops
		// at ListSecretFiles rather than reaching its caller. Harmless while no
		// vocabulary names the outer verb — pinned here so that stays a choice
		// rather than a surprise for whoever adds `secrets.SecretFileNames` to a
		// table and finds nothing ever produces it.
		{"SecretFileNames", "secrets.ListSecretFiles", func(s *Service, ctx context.Context) error {
			_, err := s.SecretFileNames(ctx, "web")
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sink := &recordingAuditSink{}
			svc := auditedService(false, sink)
			if err := tc.call(svc, auditCtx()); err == nil {
				t.Fatal("denied read returned no error")
			}
			if len(sink.events) != 1 {
				t.Fatalf("recorded %d audit events, want exactly 1: %+v", len(sink.events), sink.events)
			}
			ev := sink.events[0]
			if ev.Verb != tc.want {
				t.Errorf("recorded verb = %q, want %q", ev.Verb, tc.want)
			}
			if ev.Outcome != core.AuditDenied {
				t.Errorf("recorded outcome = %q, want %q", ev.Outcome, core.AuditDenied)
			}
		})
	}
}

// An unauthorized caller must not learn whether a secret store is configured:
// scope authorizes BEFORE it checks s.Store, so a denied caller gets the
// authorization error either way. Reversing those two statements would turn the
// verb into a probe for the platform's configuration.
func TestUnauthorizedCallerCannotProbeStoreConfiguration(t *testing.T) {
	withStore := newService(newFakeSecretStore(), tenantApp("web", "tea-a"))
	withStore.Authz = &fakeChecker{allow: false}

	withoutStore := newService(nil, tenantApp("web", "tea-a"))
	withoutStore.Authz = &fakeChecker{allow: false}

	_, errWith := withStore.SetEnvVar(auditCtx(), "web", "K", EnvVarWrite{Value: "v"})
	_, errWithout := withoutStore.SetEnvVar(auditCtx(), "web", "K", EnvVarWrite{Value: "v"})

	// Both must be the authorization refusal specifically — identical is not
	// enough if both regressed to reporting the store's state.
	if !errors.Is(errWith, core.ErrForbidden) || !errors.Is(errWithout, core.ErrForbidden) {
		t.Fatalf("configured store = %v, unconfigured = %v; both should be ErrForbidden", errWith, errWithout)
	}
	if errWith.Error() != errWithout.Error() {
		t.Errorf("configured store = %v, unconfigured = %v; a denied caller can tell the two apart", errWith, errWithout)
	}
}

// w4/m321: every read that returns env var values records one Render
// ViewEnvVarValuesEvent naming the service; the keys-only list (the
// Environment tab) records nothing, and a denied read records its denial, not
// a view.
func TestValueReadsRecordOneViewEnvVarValuesEvent(t *testing.T) {
	for _, tc := range []struct {
		name  string
		call  func(*Service, context.Context) error
		views int
	}{
		{"list", func(s *Service, ctx context.Context) error { _, err := s.ListEnvVars(ctx, "web"); return err }, 1},
		{"list page", func(s *Service, ctx context.Context) error {
			_, err := s.ListEnvVarsPage(ctx, "web", "", 1)
			return err
		}, 1},
		{"get", func(s *Service, ctx context.Context) error { _, err := s.GetEnvVar(ctx, "web", "K"); return err }, 1},
		{"reveal", func(s *Service, ctx context.Context) error { _, err := s.EnvVarValue(ctx, "web", "K"); return err }, 1},
		{"keys only", func(s *Service, ctx context.Context) error { _, err := s.EnvVarKeys(ctx, "web"); return err }, 0},
		// The Blueprint generator's names-only read and a copy between
		// services show no one a value.
		{"names for a Blueprint", func(s *Service, ctx context.Context) error { _, err := s.ListEnvVarNames(ctx, "web"); return err }, 0},
		{"copy", func(s *Service, ctx context.Context) error {
			return envCopier{service: s}.CopyEnvVars(ctx, "web", []core.EnvVarCopy{{From: "web", FromKey: "K", ToKey: "K2"}})
		}, 0},
	} {
		sink := &recordingAuditSink{}
		svc := auditedService(true, sink)
		if _, err := svc.SetEnvVar(auditCtx(), "web", "K", EnvVarWrite{Value: "v"}); err != nil {
			t.Fatal(err)
		}
		sink.events = nil
		if err := tc.call(svc, auditCtx()); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		views := 0
		for _, ev := range sink.events {
			if ev.Verb == core.AuditVerbViewEnvVarValues {
				views++
				if ev.Target != core.ServiceTarget("web") || ev.Outcome != core.AuditAllowed || ev.Caller != "user-1" || ev.Resource != core.WorkspaceObject("tea-a") {
					t.Errorf("%s: view event %+v", tc.name, ev)
				}
			}
		}
		// A copy's own write row is not a view.
		if views != tc.views || (tc.name != "copy" && len(sink.events) != tc.views) {
			t.Errorf("%s: recorded %+v, want %d view event(s) and nothing else", tc.name, sink.events, tc.views)
		}
	}

	sink := &recordingAuditSink{}
	svc := auditedService(false, sink)
	if _, err := svc.ListEnvVars(auditCtx(), "web"); err == nil {
		t.Fatal("a denied list succeeded")
	}
	for _, ev := range sink.events {
		if ev.Verb == core.AuditVerbViewEnvVarValues || ev.Outcome != core.AuditDenied {
			t.Errorf("denied list recorded %+v, want its denial only", ev)
		}
	}
	if len(sink.events) == 0 {
		t.Error("a denied list recorded nothing")
	}
}

// The dashboard's Environment tab lists keys through GraphQL envVars: only a
// query that selects values records a view, however it selects them (w4/m321).
func TestGraphQLEnvVarsRecordsAViewOnlyWhenValuesAreSelected(t *testing.T) {
	for _, tc := range []struct {
		query string
		views int
	}{
		{`{ envVars(serviceId:"web") { envVar { key } cursor } }`, 0},
		{`{ envVars(serviceId:"web") { envVar { key value } } }`, 1},
		{`{ envVars(serviceId:"web") { ...row } } fragment row on EnvVarWithCursor { envVar { value } }`, 1},
	} {
		sink := &recordingAuditSink{}
		svc := auditedService(true, sink)
		if _, err := svc.SetEnvVar(auditCtx(), "web", "K", EnvVarWrite{Value: "v"}); err != nil {
			t.Fatal(err)
		}
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		})
		if err != nil {
			t.Fatal(err)
		}
		sink.events = nil
		if res := graphql.Do(graphql.Params{Schema: schema, Context: auditCtx(), RequestString: tc.query}); res.HasErrors() {
			t.Fatalf("%s: %v", tc.query, res.Errors)
		}
		views := 0
		for _, ev := range sink.events {
			if ev.Verb == core.AuditVerbViewEnvVarValues {
				views++
			}
		}
		if views != tc.views {
			t.Errorf("%s: %d view events, want %d", tc.query, views, tc.views)
		}
	}
}
