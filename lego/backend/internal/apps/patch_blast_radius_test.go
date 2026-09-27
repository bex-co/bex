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
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The demonstrated bug was one invalid scalar, but the table has
// 22 rows and five CLASSES of late refusal. Each class is a different code
// path into the same failure mode, so fixing only the scalar one would leave
// the rest able to apply an earlier field and then refuse.
//
// Every case pairs a VALID displayName (table row 1, so it writes first) with
// a refusal that lands later, and asserts the rename did not happen. The
// fixture differs per class because the refusals are service-kind and
// policy-specific.
func TestLateRefusalClassesApplyNoEarlierField(t *testing.T) {
	renamed := "Renamed"

	cases := []struct {
		name  string
		app   func() *appv1alpha1.App
		store func() IntentStore
		patch func() ServicePatch
	}{
		{
			// Invalid scalar — the reported case.
			name: "invalid scalar (health path)",
			app:  func() *appv1alpha1.App { return managedRepoApp("web") },
			patch: func() ServicePatch {
				bad := "not-a-path"
				return ServicePatch{DisplayName: &renamed, HealthCheckPath: &bad}
			},
		},
		{
			// Inapplicable service kind — a cron job has no HTTP port, so the
			// health row refuses on type rather than on the value.
			name: "inapplicable service kind (health path on a cron job)",
			app:  func() *appv1alpha1.App { return cronApp("web") },
			patch: func() ServicePatch {
				ok := "/healthz"
				return ServicePatch{DisplayName: &renamed, HealthCheckPath: &ok}
			},
		},
		{
			// Plan policy — a background worker may not sit on the free plan.
			name: "plan policy (worker downgraded to free)",
			app: func() *appv1alpha1.App {
				a := sampleApp("web")
				a.Spec.Type = appv1alpha1.TypeBackgroundWorker
				a.Spec.Tier = "starter"
				return a
			},
			patch: func() ServicePatch {
				free := "free"
				return ServicePatch{DisplayName: &renamed, Plan: &free}
			},
		},
		{
			// Protected environment — redefining what the service builds needs
			// a confirmation phrase the request does not carry.
			name: "protected environment (root dir without confirmation)",
			app:  func() *appv1alpha1.App { return managedRepoApp("web") },
			store: func() IntentStore {
				return &recordingStore{protectedStatus: map[string]string{"srv-test": core.ProtectedStatusProtected}}
			},
			patch: func() ServicePatch {
				dir := "services/api"
				return ServicePatch{DisplayName: &renamed, RootDir: &dir}
			},
		},
		{
			// Source — the row that owns repo/image/credential together, which
			// still runs after the rename.
			name: "source validation (malformed repo URL)",
			app:  func() *appv1alpha1.App { return managedRepoApp("web") },
			patch: func() ServicePatch {
				repo := "not a repo url"
				return ServicePatch{DisplayName: &renamed, Repo: &repo}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var st IntentStore
			if tc.store != nil {
				st = tc.store()
			}
			svc, cl := newService(st, tc.app())
			before := getApp(t, cl, "web")

			_, err := svc.ApplyServicePatch(context.Background(), "web", tc.patch())
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("ApplyServicePatch = %v, want bad-request", err)
			}

			after := getApp(t, cl, "web")
			if after.Spec.DisplayName != before.Spec.DisplayName {
				t.Errorf("displayName = %q, want unchanged %q — the earlier row applied behind a later refusal",
					after.Spec.DisplayName, before.Spec.DisplayName)
			}
			if after.ResourceVersion != before.ResourceVersion {
				t.Errorf("rejected patch wrote to the App: resourceVersion %s -> %s",
					before.ResourceVersion, after.ResourceVersion)
			}
		})
	}
}

// The preflight must not make a refusal MORE informative than it was. An
// unauthorized caller still gets the same non-disclosing answer for a service
// that exists and one that does not — otherwise the up-front authorization
// would have turned the patch surface into an existence oracle.
func TestPreflightKeepsUnauthorizedAndMissingIndistinguishable(t *testing.T) {
	renamed := "Renamed"
	patch := ServicePatch{DisplayName: &renamed}

	denied, _ := newService(nil, sampleApp("web"))
	denied.Authz = &relationChecker{allow: map[string]bool{}}
	_, existsErr := denied.ApplyServicePatch(ctxAs("stranger"), "web", patch)

	absent, _ := newService(nil)
	absent.Authz = &relationChecker{allow: map[string]bool{}}
	_, missingErr := absent.ApplyServicePatch(ctxAs("stranger"), "web", patch)

	if existsErr == nil || missingErr == nil {
		t.Fatalf("both must refuse: exists=%v missing=%v", existsErr, missingErr)
	}
	if existsErr.Error() != missingErr.Error() {
		t.Errorf("a stranger can tell the service apart from a missing one:\n exists  = %v\n missing = %v",
			existsErr, missingErr)
	}
}

// The valid coupled cases the preflight could plausibly have broken, asserted
// on the core verb (the REST and MCP spellings are already pinned by
// TestMaintenanceMode{REST,MCP}PlanTransitions). The preflight validates
// against the PROPOSED combined state, so the free downgrade must see the
// maintenance disable queued ahead of it rather than the enabled state on
// disk.
func TestPreflightValidatesCoupledStateAsProposedNotAsStored(t *testing.T) {
	a := paidWebApp("web")
	a.Spec.Replicas = 1 // free caps at one instance (w6/m118)
	a.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: true}
	svc, cl := newService(nil, a)

	free := "free"
	if _, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{
		Plan:                           &free,
		MaintenanceMode:                &MaintenanceModeView{Enabled: false},
		MaintenanceBeforeFreeDowngrade: true,
	}); err != nil {
		t.Fatalf("disable maintenance + downgrade to free: %v", err)
	}
	spec := getApp(t, cl, "web").Spec
	if spec.Tier != "free" || spec.MaintenanceMode == nil || spec.MaintenanceMode.Enabled {
		t.Fatalf("coupled downgrade did not apply: %+v", spec)
	}
}

// The empty patch stays the read-only no-op both surfaces document: the
// preflight must add no write and no authorization beyond the can_view that
// Get already did.
func TestPreflightLeavesEmptyPatchAReadOnlyNoOp(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	svc.Authz = &relationChecker{allow: map[string]bool{core.RelCanView: true}}
	before := getApp(t, cl, "web").ResourceVersion

	if _, err := svc.ApplyServicePatch(ctxAs("viewer"), "web", ServicePatch{}); err != nil {
		t.Fatalf("empty patch: %v", err)
	}
	if after := getApp(t, cl, "web").ResourceVersion; after != before {
		t.Errorf("empty patch wrote: resourceVersion %s -> %s", before, after)
	}
}

// "Preflight" is only a real preflight if it writes nothing. This asserts that
// directly against BOTH stores the patch pipeline touches — the App CR and the
// control-plane row — for a patch whose rows would otherwise write to each.
//
// It matters beyond tidiness: the preflight runs on every PATCH, including the
// ones that go on to succeed. A check that quietly wrote would double every
// successful save rather than protect the failing ones.
func TestPreflightItselfWritesNothing(t *testing.T) {
	st := &recordingStore{}
	svc, cl := newService(st, managedRepoApp("web"))
	before := getApp(t, cl, "web").ResourceVersion

	display, start, health := "Renamed", "bin/serve", "/livez"
	delay, idle := int32(45), int32(600)
	if _, err := svc.preflightServicePatch(context.Background(), "web", ServicePatch{
		DisplayName: &display, StartCommand: &start, HealthCheckPath: &health,
		MaxShutdownDelaySeconds: &delay, IdleTTLSeconds: &idle,
	}); err != nil {
		t.Fatalf("preflight: %v", err)
	}

	if after := getApp(t, cl, "web").ResourceVersion; after != before {
		t.Errorf("preflight patched the App CR: resourceVersion %s -> %s", before, after)
	}
	if n := len(st.displayNameCalls); n != 0 {
		t.Errorf("preflight wrote %d displayName rows, want 0", n)
	}
	if n := len(st.idleTTLCalls); n != 0 {
		t.Errorf("preflight wrote %d idleTTL rows, want 0", n)
	}
	if n := len(st.deployCalls); n != 0 {
		t.Errorf("preflight opened %d deploy rows, want 0", n)
	}
}

// The fix split each setting's guards into a check the preflight
// runs and a verb the apply pass runs. GraphQL (and so the dashboard) still
// calls those verbs one setting at a time — 24 of them — so the two paths must
// stay the same rule. If a check ever became weaker than its verb, the
// preflight would pass a patch its own table then refuses mid-flight, which is
// exactly the bug this milestone closed; if it became stronger, the patch
// surface would start refusing what the single-setting surface accepts.
//
// So, per row: the same input through the check and through the verb must
// agree on whether it is refused, and refuse with the SAME message.
func TestEachRowsCheckAndVerbAgree(t *testing.T) {
	s := func(v string) *string { return &v }
	i := func(v int32) *int32 { return &v }
	b := func(v bool) *bool { return &v }

	// Values chosen to land on each row's refusal where it has one. A value
	// that turns out acceptable is still compared — agreement on "allowed" is
	// half the property.
	patches := map[string]ServicePatch{
		"refused": {
			DisplayName: s("Renamed"), Repo: s("not a repo url"),
			MaintenanceMode: &MaintenanceModeView{Enabled: true, URI: "http://%zz"},
			Plan:            s("no-such-plan"), IdleTTLSeconds: i(-1),
			MaxShutdownDelaySeconds: i(9999), RootDir: s("../escape"),
			AutoDeploy: b(false), Schedule: s("not a cron"), Command: s("bin/run"),
			HealthCheckPath: s("not-a-path"), PreDeployCommand: s("bin/migrate"),
			PublishPath: s(""), BuildCommand: s("make all"), StartCommand: s("bin/serve"),
			DockerfilePath: s("../escape"), Port: i(0),
			NotifyOnFail: s("bogus"), NotificationsToSend: s("bogus"),
			RenderSubdomainPolicy: s("bogus"),
			IPAllowList:           &[]core.IPAllowListEntry{{CIDRBlock: "not-a-cidr"}},
			Autoscaling:           &SetAutoscalingRequest{MinInstances: 9, MaxInstances: 2},
		},
		// The source quartet and autoscaling are deliberately present here,
		// not only in "refused": they own the two most complex checks in the
		// table (the source row's changed/probe folding, checkAutoscaling's
		// disk gate), so agreement in the ALLOWED direction is where they can
		// most easily drift.
		"allowed": {
			DisplayName: s("Renamed"), Branch: s("release"),
			Autoscaling: &SetAutoscalingRequest{
				MinInstances: 2, MaxInstances: 5, TargetCPUPercent: i(70),
			},
			MaintenanceMode: &MaintenanceModeView{Enabled: false},
			IdleTTLSeconds:  i(600), MaxShutdownDelaySeconds: i(45),
			RootDir: s("services/api"), AutoDeploy: b(false),
			HealthCheckPath: s("/livez"), PreDeployCommand: s("bin/migrate"),
			BuildCommand: s("make all"), StartCommand: s("bin/serve"),
			DockerfilePath: s("docker/Dockerfile"), Port: i(3000),
			NotifyOnFail: s("ignore"), NotificationsToSend: s("all"),
			RenderSubdomainPolicy: s("enabled"),
			IPAllowList:           &[]core.IPAllowListEntry{},
		},
	}

	for label, p := range patches {
		for idx, row := range servicePatchTable {
			if !row.present(p) || row.check == nil {
				continue
			}
			ctx := context.Background()

			checkSvc, _ := populatedService(t)
			probe := getApp(t, checkSvc.Client, "web").DeepCopy()
			checkErr := row.check(ctx, checkSvc, probe, "web", p)

			verbSvc, _ := populatedService(t)
			_, verbErr := row.apply(ctx, verbSvc, "web", p)

			switch {
			case checkErr == nil && verbErr != nil:
				t.Errorf("%s: servicePatchTable[%d] (%v) — the preflight ACCEPTS what its own verb refuses (%v); "+
					"a patch would apply the earlier rows and then fail here", label, idx, row.fields, verbErr)
			case checkErr != nil && verbErr == nil:
				t.Errorf("%s: servicePatchTable[%d] (%v) — the preflight REFUSES (%v) what its own verb accepts; "+
					"the patch surface would be stricter than the GraphQL one", label, idx, row.fields, checkErr)
			case checkErr != nil && verbErr != nil && checkErr.Error() != verbErr.Error():
				t.Errorf("%s: servicePatchTable[%d] (%v) — same input, different refusal:\n preflight = %v\n verb      = %v",
					label, idx, row.fields, checkErr, verbErr)
			}
		}
	}
}

// The preflight and the apply pass BOTH resolve the source, so without the
// probed-source marker every source-repointing patch would make the outbound
// registry-credential and GitHub repository calls twice — doubling the latency
// of the change and the load bex puts on someone else's API.
//
// This test exists because the first version of that marker keyed on the
// service id and matched it against `metadata.name` and LabelServiceName,
// while both surfaces address a service by its srv- id (LabelAppID). It
// therefore never fired on the real request path, and nothing noticed. The
// fixture is addressed by srv- id for exactly that reason.
func TestSourcePatchProbesOutboundCallsExactlyOnce(t *testing.T) {
	a := repoApp("web", "https://github.com/bex-co/hello.git", "main")
	a.Labels = map[string]string{
		store.LabelManagedBy: store.ManagedByValue,
		store.LabelAppID:     "srv-probeonce",
	}
	gh := &fakeCloneTokens{ok: true}
	rc := &fakePullSecrets{ok: true}
	cl := fakeClient(a)
	svc := &Service{
		Base:          &core.Base{Client: cl, Namespace: "default"},
		GitHub:        gh,
		RegistryCreds: rc,
	}

	repo := "https://github.com/bex-co/moved.git"
	if _, err := svc.ApplyServicePatch(context.Background(), "srv-probeonce", ServicePatch{Repo: &repo}); err != nil {
		t.Fatalf("ApplyServicePatch: %v", err)
	}

	if got := getApp(t, cl, "web").Spec.Repo; got != repo {
		t.Fatalf("repo = %q, want %q — the patch must still apply", got, repo)
	}
	if gh.validateCalls != 1 {
		t.Errorf("GitHub.ValidateRepo called %d times, want exactly 1 "+
			"(the preflight probed it; the apply pass must reuse that answer)", gh.validateCalls)
	}
	// The registry-credential probe is not reached by a repo-only patch (no
	// credential to resolve); it is pinned at exactly one call through a REST
	// PATCH by TestRESTRegistryCredentialCreatePatchAndClear.
	if rc.validateCalls > 1 {
		t.Errorf("ValidatePullSecret called %d times, want at most 1", rc.validateCalls)
	}
}

// Validating twice must not cost twice. Four of this patch's rows independently
// ask "is this service in a protected environment?", and each pass asks again
// — eight identical Postgres round trips for one request-invariant boolean
// before the per-request memo collapsed them.
func TestPatchAsksProtectedStatusOncePerRequest(t *testing.T) {
	st := &recordingStore{protectedStatus: map[string]string{"srv-test": "unprotected"}}
	svc, _ := newService(st, dockerRepoAppManaged("web"))

	rootDir, dockerfile := "services/api", "docker/Dockerfile"
	build, preDeploy := "make all", "bin/migrate"
	if _, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{
		RootDir: &rootDir, DockerfilePath: &dockerfile,
		BuildCommand: &build, PreDeployCommand: &preDeploy,
	}); err != nil {
		t.Fatalf("ApplyServicePatch: %v", err)
	}

	if st.protectedCalls != 1 {
		t.Errorf("GetAppProtectedStatus called %d times for one patch, want 1 "+
			"(4 guarded rows x 2 passes without the per-request memo)", st.protectedCalls)
	}
}

// dockerRepoAppManaged is dockerRepoApp with the control-plane labels, so the
// protection lookup has an app id to key on.
func dockerRepoAppManaged(name string) *appv1alpha1.App {
	a := dockerRepoApp(name)
	a.Labels = map[string]string{
		store.LabelManagedBy: store.ManagedByValue,
		store.LabelAppID:     "srv-test",
	}
	return a
}

// A name conflict is raised by the STORE during the write (w8/m47), not by a
// check the preflight can run — so it is the one patch refusal that still lands
// mid-apply. It is harmless only because displayName owns the FIRST table row:
// nothing can have been written before it. That makes this test a guard on the
// table ORDER as much as on the refusal, which is why it asserts the later
// field too.
func TestDisplayNameConflictAppliesNoOtherField(t *testing.T) {
	st := &recordingStore{takenDisplayNames: map[string]bool{"Taken": true}}
	svc, cl := newService(st, managedRepoApp("web"))
	before := getApp(t, cl, "web")

	taken, health := "Taken", "/livez"
	_, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{
		DisplayName: &taken, HealthCheckPath: &health,
	})
	if err == nil {
		t.Fatal("ApplyServicePatch succeeded, want the name conflict")
	}

	after := getApp(t, cl, "web")
	if after.Spec.DisplayName != before.Spec.DisplayName {
		t.Errorf("displayName = %q, want unchanged %q", after.Spec.DisplayName, before.Spec.DisplayName)
	}
	if after.Spec.HealthCheckPath != before.Spec.HealthCheckPath {
		t.Errorf("healthCheckPath = %q, want unchanged %q — a conflict on row 1 must not let a later row through",
			after.Spec.HealthCheckPath, before.Spec.HealthCheckPath)
	}
	if len(st.deployCalls) != 0 {
		t.Errorf("conflicted patch opened %d deploy rows, want none", len(st.deployCalls))
	}
}
