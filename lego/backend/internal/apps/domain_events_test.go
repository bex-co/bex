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

package apps

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// auditedDomainService is the domain harness with a capturing audit sink, so a
// test can assert how many rows a sequence of calls produced. The events feed
// projects allowed audit rows one-for-one into past-tense event types, so the
// audit count IS the feed count for these verbs.
func auditedDomainService(t *testing.T) (*Service, *memoryDomainClaimStore, *captureAuditSink) {
	t.Helper()
	claims := newMemoryDomainClaimStore()
	svc, _ := newService(claims, managedApp("web", "srv-1"))
	sink := &captureAuditSink{}
	svc.Audit = sink
	return svc, claims, sink
}

// TestFailedVerificationLeavesNoTraceInTheFeed is w4/m122's headline. Live on
// 2026-09-21, five Re-checks on a domain whose TXT record had not propagated
// produced five "Custom domain verified" rows in the Activity feed while the
// domain's own row still read Pending — because the audit row was emitted at
// AUTHORIZE time, and the feed projects an allowed audit row into a past-tense
// fact regardless of what the verb then did.
//
// Verification failing is the NORMAL first outcome, so this was the common
// path, not an edge case — and the feed is exactly where a user goes when the
// domain row only says "Pending".
func TestFailedVerificationLeavesNoTraceInTheFeed(t *testing.T) {
	svc, _, sink := auditedDomainService(t)
	ctx := context.Background()
	if _, err := svc.AddDomain(ctx, "web", "app.example.com"); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	pending := errors.New("txt record absent")
	svc.DomainOwnership = domainOwnershipFunc(func(context.Context, string, string) error { return pending })

	for i := range 5 {
		if _, err := svc.VerifyDomain(ctx, "web", "app.example.com"); err == nil {
			t.Fatalf("re-check %d unexpectedly succeeded", i+1)
		}
	}
	if n := sink.countVerb(core.AuditVerbVerifyDomain); n != 0 {
		t.Errorf("five failed re-checks recorded %d verification events, want 0", n)
	}

	// And a real verification still records exactly one.
	svc.DomainOwnership = domainOwnershipFunc(func(context.Context, string, string) error { return nil })
	if _, err := svc.VerifyDomain(ctx, "web", "app.example.com"); err != nil {
		t.Fatalf("VerifyDomain after the record landed: %v", err)
	}
	if n := sink.countVerb(core.AuditVerbVerifyDomain); n != 1 {
		t.Fatalf("a real verification recorded %d events, want exactly 1", n)
	}

	// Re-checking an already-verified domain returns the claim without
	// re-promoting — a no-op, and the case an error-vs-success flag alone would
	// have missed.
	if _, err := svc.VerifyDomain(ctx, "web", "app.example.com"); err != nil {
		t.Fatalf("re-check of a verified domain: %v", err)
	}
	if n := sink.countVerb(core.AuditVerbVerifyDomain); n != 1 {
		t.Errorf("re-checking a verified domain recorded %d events, want still 1", n)
	}
}

// TestRefusedAddLeavesNoTraceInTheFeed: live, one refused add (a reserved
// platform hostname belonging to a DIFFERENT service) plus one accepted add
// produced two identical `custom_domain_added` rows at the same second.
func TestRefusedAddLeavesNoTraceInTheFeed(t *testing.T) {
	svc, _, sink := auditedDomainService(t)
	svc.BaseDomain = "onbex.co"
	ctx := context.Background()

	if _, err := svc.AddDomain(ctx, "web", "someone-elses-service.onbex.co"); err == nil {
		t.Fatal("adding a reserved platform hostname must be refused")
	}
	if n := sink.countVerb(core.AuditVerbAddDomain); n != 0 {
		t.Errorf("a refused add recorded %d events, want 0", n)
	}

	if _, err := svc.AddDomain(ctx, "web", "app.example.com"); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if n := sink.countVerb(core.AuditVerbAddDomain); n != 1 {
		t.Fatalf("a real add recorded %d events, want exactly 1", n)
	}

	// Re-adding the same host is idempotent — nothing changed, so no event.
	if _, err := svc.AddDomain(ctx, "web", "app.example.com"); err != nil {
		t.Fatalf("idempotent re-add: %v", err)
	}
	if n := sink.countVerb(core.AuditVerbAddDomain); n != 1 {
		t.Errorf("an idempotent re-add recorded %d events, want still 1", n)
	}
}

// TestDeleteRecordsOnlyARealRemoval covers the third verb, and pins that the
// pre-existing add/delete pairing asymmetry is NOT changed here: an add can
// attach a `www.` sibling alongside the primary, and deleting the primary takes
// the sibling with it in ONE call and one row (.pm/w7/done/050.md:259). This
// milestone gates whether a row is written, not how many.
func TestDeleteRecordsOnlyARealRemoval(t *testing.T) {
	svc, _, sink := auditedDomainService(t)
	ctx := context.Background()

	// Deleting a host that was never there is idempotent — and must be silent.
	if err := svc.DeleteDomain(ctx, "web", "never-added.example.com"); err != nil {
		t.Fatalf("idempotent delete: %v", err)
	}
	if n := sink.countVerb(core.AuditVerbDeleteDomain); n != 0 {
		t.Errorf("deleting an absent host recorded %d events, want 0", n)
	}

	if _, err := svc.AddDomain(ctx, "web", "foo.com"); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if err := svc.DeleteDomain(ctx, "web", "foo.com"); err != nil {
		t.Fatalf("DeleteDomain: %v", err)
	}
	if n := sink.countVerb(core.AuditVerbDeleteDomain); n != 1 {
		t.Errorf("a real removal recorded %d events, want exactly 1", n)
	}
}

// TestDomainVerbsRecordNothingAtAuthorizeTime is the structural guard. The
// three verbs must authorize under WithDeferredAllowedWriteAudit — if one
// reverts to the default, its refusals start producing events again and only a
// live QA pass would notice. Asserted through behavior a caller can see: an
// operation that fails for a reason EVERY verb shares (an unknown service)
// leaves no row at all.
func TestDomainVerbsRecordNothingAtAuthorizeTime(t *testing.T) {
	svc, _, sink := auditedDomainService(t)
	ctx := context.Background()

	if _, err := svc.AddDomain(ctx, "nope", "app.example.com"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("AddDomain on an unknown service = %v, want ErrNotFound", err)
	}
	if _, err := svc.VerifyDomain(ctx, "nope", "app.example.com"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("VerifyDomain on an unknown service = %v, want ErrNotFound", err)
	}
	if err := svc.DeleteDomain(ctx, "nope", "app.example.com"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("DeleteDomain on an unknown service = %v, want ErrNotFound", err)
	}
	if len(sink.events) != 0 {
		t.Errorf("verbs against an unknown service recorded %d audit rows: %+v", len(sink.events), sink.events)
	}
}

// TestRefusedConfigWritesLeaveNoTraceInTheFeed is the generalized regression for
// w4/m122's t004 audit. The custom-domain verbs were the ones proven live, but
// the mechanism was never domain-specific: every verb in the events vocabulary
// is audited inside its AUTHORIZE call, and this repo's contract is
// authorize-BEFORE-validate — so every verb with a type gate, a range check or
// a quota was reporting its refusals as accomplished facts.
//
// Each case below is an ordinary mistake an ordinary UI or CLI flow makes, not
// a hand-built bad request.
func TestRefusedConfigWritesLeaveNoTraceInTheFeed(t *testing.T) {
	for _, tc := range []struct {
		name string
		verb string
		// setup returns the service to act on.
		setup func() *appv1alpha1.App
		call  func(*Service) error
	}{{
		name: "a port on a service that binds none",
		verb: core.AuditVerbSetPort,
		setup: func() *appv1alpha1.App {
			a := sampleApp("worker")
			a.Spec.Type = appv1alpha1.TypeBackgroundWorker
			return a
		},
		call: func(s *Service) error {
			_, err := s.SetPort(context.Background(), "worker", 8080)
			return err
		},
	}, {
		name:  "a port the container could never bind",
		verb:  core.AuditVerbSetPort,
		setup: func() *appv1alpha1.App { return sampleApp("web") },
		call: func(s *Service) error {
			_, err := s.SetPort(context.Background(), "web", 80)
			return err
		},
	}, {
		name:  "a dockerfile path on an image-backed service",
		verb:  core.AuditVerbSetDockerfilePath,
		setup: func() *appv1alpha1.App { return sampleApp("web") }, // image-backed
		call: func(s *Service) error {
			_, err := s.SetDockerfilePath(context.Background(), "web", "Dockerfile.prod")
			return err
		},
	}, {
		name:  "a root directory on an image-backed service",
		verb:  core.AuditVerbSetRootDir,
		setup: func() *appv1alpha1.App { return sampleApp("web") },
		call: func(s *Service) error {
			_, err := s.SetRootDir(context.Background(), "web", "packages/api")
			return err
		},
	}, {
		name:  "a publish path on a web service",
		verb:  core.AuditVerbSetPublishPath,
		setup: func() *appv1alpha1.App { return sampleApp("web") },
		call: func(s *Service) error {
			_, err := s.SetPublishPath(context.Background(), "web", "dist")
			return err
		},
	}, {
		name: "a pre-deploy command on a cron job",
		verb: core.AuditVerbSetPreDeployCommand,
		setup: func() *appv1alpha1.App {
			a := sampleApp("nightly")
			a.Spec.Type = appv1alpha1.TypeCronJob
			return a
		},
		call: func(s *Service) error {
			_, err := s.SetPreDeployCommand(context.Background(), "nightly", "./migrate")
			return err
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			sink := &captureAuditSink{}
			cl := fakeClient(tc.setup())
			svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", Audit: sink}}

			if err := tc.call(svc); err == nil {
				t.Fatal("the write was expected to be refused")
			}
			if n := sink.countVerb(tc.verb); n != 0 {
				t.Errorf("a refused %s recorded %d events, want 0", tc.verb, n)
			}
		})
	}
}

// TestAcceptedConfigWritesStillRecordExactlyOne is the other half: gating must
// not cost the feed the events it is supposed to carry. A verb that defers its
// audit and then forgets to record is the mirror-image bug — and it is not
// hypothetical, since the four disk verbs had exactly that defect (they
// deferred from the day they landed and no recorder was ever written), which
// this milestone also fixes.
func TestAcceptedConfigWritesStillRecordExactlyOne(t *testing.T) {
	for _, tc := range []struct {
		name string
		verb string
		call func(*Service) error
	}{{
		name: "set port",
		verb: core.AuditVerbSetPort,
		call: func(s *Service) error {
			_, err := s.SetPort(context.Background(), "web", 8080)
			return err
		},
	}, {
		name: "set max shutdown delay",
		verb: core.AuditVerbSetMaxShutdownDelay,
		call: func(s *Service) error {
			_, err := s.SetMaxShutdownDelay(context.Background(), "web", 45)
			return err
		},
	}, {
		name: "set commands",
		verb: core.AuditVerbSetCommands,
		call: func(s *Service) error {
			start := "./server"
			_, err := s.SetCommands(context.Background(), "web", nil, &start)
			return err
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			sink := &captureAuditSink{}
			cl := fakeClient(sampleApp("web"))
			svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", Audit: sink}}

			if err := tc.call(svc); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if n := sink.countVerb(tc.verb); n != 1 {
				t.Fatalf("an accepted %s recorded %d events, want exactly 1", tc.verb, n)
			}
		})
	}
}
