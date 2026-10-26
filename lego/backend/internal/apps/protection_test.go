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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// protection_test.go exercises the protected-environment rule (protection.go).
// It is Render's (w8/187, render.com/docs/projects): in a protectedStatus=protected
// Environment only a workspace Admin may delete, suspend or resume a service or
// turn maintenance mode on; everything else — deploys, rollbacks, settings — is
// open to anyone who may do it elsewhere, and no verb asks for a typed phrase.
// (Before w8/187 w6/m19, w4/m126 and w4/m127 gated a wider set behind
// ProtectedConfirmation phrases for every caller.)

// denyWorkspaceManage allows every relation except the admin can_manage on a
// workspace — a Developer in the role ladder. Other objects' can_manage checks
// (the ordinary per-verb authorization) are untouched.
type denyWorkspaceManage struct{}

func (denyWorkspaceManage) Check(_ context.Context, _, relation, object string) (bool, error) {
	return !(relation == core.RelCanManage && strings.HasPrefix(object, "workspace:")), nil
}

// allowAllRelations is an admin caller.
type allowAllRelations struct{}

func (allowAllRelations) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

// asNonAdmin makes svc's caller a non-admin; asAdmin an admin (the identity both
// need is ctxAs).
func asNonAdmin(svc *Service) { svc.Authz = denyWorkspaceManage{} }
func asAdmin(svc *Service)    { svc.Authz = allowAllRelations{} }

func TestDelete_NonAdminBlockedWhenProtected(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	svc, cl := newService(rec, managedApp("web", "srv-1"))
	asNonAdmin(svc)

	if err := svc.Delete(ctxAs("user-a"), "web"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("Delete on a protected member by a non-admin: got %v, want ErrForbidden", err)
	}
	// The App must survive a blocked delete.
	getApp(t, cl, "web")
}

func TestDelete_AdminSucceedsWithNoPhrase(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	svc, cl := newService(rec, managedApp("web", "srv-1"))
	asAdmin(svc)

	if err := svc.Delete(ctxAs("user-a"), "web"); err != nil {
		t.Fatalf("Delete by an admin: %v", err)
	}
	var got appv1alpha1.AppList
	if err := cl.List(ctxAs("user-a"), &got); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got.Items) != 0 {
		t.Fatalf("App should be deleted, got %d remaining", len(got.Items))
	}
}

// A typed phrase no longer clears anything for a non-admin.
func TestDelete_ConfirmPhraseDoesNotSubstituteForTheAdminRole(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	svc, _ := newService(rec, managedApp("web", "srv-1"))
	asNonAdmin(svc)

	ctx := core.WithConfirm(ctxAs("user-a"), "sudo delete service web")
	if err := svc.Delete(ctx, "web"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("Delete with the old phrase by a non-admin: got %v, want ErrForbidden", err)
	}
}

func TestDelete_UnprotectedNeedsNoAdmin(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "unprotected"}}
	svc, _ := newService(rec, managedApp("web", "srv-1"))
	asNonAdmin(svc)

	if err := svc.Delete(ctxAs("user-a"), "web"); err != nil {
		t.Fatalf("Delete on an unprotected member: %v", err)
	}
}

func TestDelete_RetryRemovesCRAfterSourceRowIsGone(t *testing.T) {
	rec := &recordingStore{err: store.ErrNotFound}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if err := svc.Delete(ctxAs("user-a"), "web"); err != nil {
		t.Fatalf("Delete after row-first interruption: %v", err)
	}
	gone(t, cl, "web")
}

func TestSuspendAndResume_NonAdminBlockedWhenProtected(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	running := managedApp("web", "srv-1")
	stopped := managedApp("idle", "srv-1")
	stopped.Spec.Suspended = true
	svc, cl := newService(rec, running, stopped)
	asNonAdmin(svc)

	if _, err := svc.Suspend(ctxAs("user-a"), "web"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("Suspend on a protected member by a non-admin: got %v, want ErrForbidden", err)
	}
	if getApp(t, cl, "web").Spec.Suspended {
		t.Fatal("a blocked suspend must not patch the CR")
	}
	if _, err := svc.Resume(ctxAs("user-a"), "idle"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("Resume on a protected member by a non-admin: got %v, want ErrForbidden", err)
	}
	if !getApp(t, cl, "idle").Spec.Suspended {
		t.Fatal("a blocked resume must not patch the CR")
	}
}

func TestSuspendAndResume_AdminSucceedsWithNoPhrase(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	app := managedApp("web", "srv-1")
	svc, cl := newService(rec, app)
	asAdmin(svc)

	if _, err := svc.Suspend(ctxAs("user-a"), "web"); err != nil {
		t.Fatalf("Suspend by an admin: %v", err)
	}
	if !getApp(t, cl, "web").Spec.Suspended {
		t.Fatal("App should be suspended")
	}
	if _, err := svc.Resume(ctxAs("user-a"), "web"); err != nil {
		t.Fatalf("Resume by an admin: %v", err)
	}
	if getApp(t, cl, "web").Spec.Suspended {
		t.Fatal("App should be resumed")
	}
}

// A deploy of a different image is not one of the actions Render restricts,
// so a Developer may run it in a protected environment with no phrase.
func TestDeployStack_OverrideNeedsNoPhraseOrAdmin(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	existing := managedApp("web", "srv-1")
	existing.Spec.Image = "old:1"
	svc, cl := newService(rec, existing)
	asNonAdmin(svc)

	changed := "services:\n  - {name: web, type: web, runtime: image, image: {url: new:1}}\n"
	if _, err := svc.DeployStack(ctxAs("user-a"), DeployRequest{Manifest: changed}); err != nil {
		t.Fatalf("DeployStack override on a protected member by a non-admin: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Image; got != "new:1" {
		t.Fatalf("image = %q, want new:1", got)
	}
}

func TestSyncBlueprint_ProtectedOverrideNeedsNoPhrase(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	existing := managedApp("web", "srv-1")
	existing.Spec.Image = "old:1"
	svc, cl := newService(rec, existing)
	asNonAdmin(svc)
	svc.Blueprints = newFakeBlueprintStore(store.Blueprint{
		ID:       "blp-1",
		Manifest: "services:\n  - {name: web, type: web, runtime: image, image: {url: new:1}}\n",
		Status:   "active",
	})

	if _, err := svc.SyncBlueprint(ctxAs("user-a"), "blp-1", "", "", "", nil); err != nil {
		t.Fatalf("SyncBlueprint override on a protected member: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Image; got != "new:1" {
		t.Fatalf("image = %q, want new:1", got)
	}
}

// TestDeployStack_NewServiceNeverBlocked proves a brand-new service (no
// existing App to override) is exempt.
func TestDeployStack_NewServiceNeverBlocked(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec)

	manifest := "services:\n  - {name: web, type: web, runtime: image, image: {url: new:1}}\n"
	if _, err := svc.DeployStack(ctxAs("user-a"), DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("DeployStack for a brand-new service: %v", err)
	}
	getApp(t, cl, "web")
}

// --- the verbs Render leaves open ---------------------------------------------

// protectedCall is one verb bound to a fixture: build the App, run the verb
// against a Service, and report whether the spec actually changed.
type protectedCall struct {
	name    string
	app     func() *appv1alpha1.App
	call    func(ctx context.Context, svc *Service, name string) error
	applied func(a *appv1alpha1.App) bool
}

func openVerbCalls() []protectedCall {
	str := func(s string) *string { return &s }
	docker := func() *appv1alpha1.App {
		a := managedRepoApp("web")
		a.Spec.Runtime = "docker"
		a.Spec.BuildCommand = ""
		return a
	}
	return []protectedCall{{
		name: "setImage",
		app:  func() *appv1alpha1.App { return managedApp("web", "srv-test") },
		call: func(ctx context.Context, svc *Service, n string) error {
			_, err := svc.SetSourceAndRegistryCredential(ctx, n, sourcePatch{Image: str("mendhak/http-https-echo:35")})
			return err
		},
		applied: func(a *appv1alpha1.App) bool { return a.Spec.Image == "mendhak/http-https-echo:35" },
	}, {
		name: "setBranch",
		app:  func() *appv1alpha1.App { return managedRepoApp("web") },
		call: func(ctx context.Context, svc *Service, n string) error {
			_, err := svc.SetSourceAndRegistryCredential(ctx, n, sourcePatch{Branch: str("release")})
			return err
		},
		applied: func(a *appv1alpha1.App) bool { return a.Spec.Branch == "release" },
	}, {
		name: "setStartCommand",
		app:  func() *appv1alpha1.App { return managedRepoApp("web") },
		call: func(ctx context.Context, svc *Service, n string) error {
			_, err := svc.SetCommands(ctx, n, nil, str("./app --serve"))
			return err
		},
		applied: func(a *appv1alpha1.App) bool { return a.Spec.StartCommand == "./app --serve" },
	}, {
		name: "setRootDir",
		app:  func() *appv1alpha1.App { return managedRepoApp("web") },
		call: func(ctx context.Context, svc *Service, n string) error {
			_, err := svc.SetRootDir(ctx, n, "services/api")
			return err
		},
		applied: func(a *appv1alpha1.App) bool { return a.Spec.RootDir == "services/api" },
	}, {
		name: "setDockerfilePath",
		app:  docker,
		call: func(ctx context.Context, svc *Service, n string) error {
			_, err := svc.SetDockerfilePath(ctx, n, "ops/Dockerfile")
			return err
		},
		applied: func(a *appv1alpha1.App) bool { return a.Spec.DockerfilePath == "ops/Dockerfile" },
	}, {
		name: "setPreDeployCommand",
		app:  func() *appv1alpha1.App { return managedRepoApp("web") },
		call: func(ctx context.Context, svc *Service, n string) error {
			_, err := svc.SetPreDeployCommand(ctx, n, "./migrate")
			return err
		},
		applied: func(a *appv1alpha1.App) bool { return a.Spec.PreDeployCommand == "./migrate" },
	}, {
		name: "updateCronJob command",
		app: func() *appv1alpha1.App {
			a := managedRepoApp("web")
			a.Spec.Type = appv1alpha1.TypeCronJob
			a.Spec.Schedule = "0 0 * * *"
			return a
		},
		call: func(ctx context.Context, svc *Service, n string) error {
			_, err := svc.SetCronJob(ctx, n, str("0 0 * * *"), str("/bin/run"))
			return err
		},
		applied: func(a *appv1alpha1.App) bool { return a.Spec.Command == "/bin/run" },
	}}
}

func maintenanceApp() *appv1alpha1.App {
	a := managedApp("web", "srv-test")
	a.Spec.Type = appv1alpha1.TypeWebService
	a.Spec.Tier = "standard"
	return a
}

// Render restricts only delete, suspend/resume and maintenance mode: a
// Developer changes the code a protected service runs, with no phrase.
func TestProtectedMemberSettingsNeedNeitherPhraseNorAdmin(t *testing.T) {
	for _, c := range openVerbCalls() {
		t.Run(c.name, func(t *testing.T) {
			rec := &recordingStore{protectedStatus: map[string]string{"srv-test": "protected"}}
			svc, cl := newService(rec, c.app())
			asNonAdmin(svc)

			if err := c.call(ctxAs("user-a"), svc, "web"); err != nil {
				t.Fatalf("on a protected member by a non-admin: %v", err)
			}
			if !c.applied(getApp(t, cl, "web")) {
				t.Fatal("the verb must have applied")
			}
		})
	}
}

func TestProtectedMemberMaintenanceModeNeedsAdmin(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-test": "protected"}}
	svc, cl := newService(rec, maintenanceApp())
	asNonAdmin(svc)

	if _, err := svc.SetMaintenanceMode(ctxAs("user-a"), "web", MaintenanceModeView{Enabled: true}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("maintenance on by a non-admin: got %v, want ErrForbidden", err)
	}
	if m := getApp(t, cl, "web").Spec.MaintenanceMode; m != nil && m.Enabled {
		t.Fatal("a refused verb must not have changed the spec")
	}

	asAdmin(svc)
	if _, err := svc.SetMaintenanceMode(ctxAs("user-a"), "web", MaintenanceModeView{Enabled: true}); err != nil {
		t.Fatalf("maintenance on by an admin: %v", err)
	}
	if m := getApp(t, cl, "web").Spec.MaintenanceMode; m == nil || !m.Enabled {
		t.Fatal("maintenance mode should be on")
	}
}

// Turning maintenance mode off restores availability; a cron reschedule changes
// when the job runs rather than what it runs.
func TestProtectedRestoringVerbsStayOpen(t *testing.T) {
	t.Run("disable maintenance mode", func(t *testing.T) {
		rec := &recordingStore{protectedStatus: map[string]string{"srv-test": "protected"}}
		a := maintenanceApp()
		a.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: true}
		svc, cl := newService(rec, a)
		asNonAdmin(svc)

		if _, err := svc.SetMaintenanceMode(ctxAs("user-a"), "web", MaintenanceModeView{}); err != nil {
			t.Fatalf("turning maintenance mode off: %v", err)
		}
		if m := getApp(t, cl, "web").Spec.MaintenanceMode; m != nil && m.Enabled {
			t.Fatal("maintenance mode should be off")
		}
	})
}

func TestUnprotectedMemberNeedsNoAdmin(t *testing.T) {
	svc, _ := newService(&recordingStore{}, maintenanceApp())
	asNonAdmin(svc)
	if _, err := svc.SetMaintenanceMode(ctxAs("user-a"), "web", MaintenanceModeView{Enabled: true}); err != nil {
		t.Fatalf("unprotected member: %v", err)
	}
}

// TestProtectionLookupFailureFailsClosed: the rule is only as good as the lookup
// behind it, so a control-plane outage must refuse a restricted verb rather than
// wave it through.
func TestProtectionLookupFailureFailsClosed(t *testing.T) {
	rec := &recordingStore{protectedErr: errors.New("control plane unavailable")}
	svc, cl := newService(rec, managedApp("web", "srv-test"))

	if _, err := svc.Suspend(ctxAs("user-a"), "web"); err == nil {
		t.Fatal("want an error when the protection lookup fails")
	}
	if getApp(t, cl, "web").Spec.Suspended {
		t.Fatal("the service must not be suspended when protection could not be read")
	}
}

// The GraphQL mutations keep their confirm argument for clients written against
// the phrase (it is now ignored), so a refactor must not drop it.
func TestGuardedMutationsKeepTheirConfirmArgument(t *testing.T) {
	svc := &Service{}
	mutations := svc.GraphQLMutation()
	for _, name := range []string{"deleteService", "suspendService", "setImage", "setMaintenanceMode"} {
		field, ok := mutations[name]
		if !ok {
			t.Errorf("mutation %s is missing", name)
			continue
		}
		if _, ok := field.Args["confirm"]; !ok {
			t.Errorf("mutation %s lost its confirm argument", name)
		}
	}
}

// Shell access to a protected environment's service is an Admin's (w8/193), on
// both transports: the browser terminal ticket and native SSH.
func TestProtectedMemberShellNeedsAdmin(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	svc, _ := newService(rec, managedApp("web", "srv-1"))
	asNonAdmin(svc)

	if _, err := svc.CreateShellSession(ctxAs("user-a"), "web", ""); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("CreateShellSession by a non-admin = %v, want ErrForbidden", err)
	}
	if _, err := svc.ResolveSSHSession(ctxAs("user-a"), "web"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("ResolveSSHSession by a non-admin = %v, want ErrForbidden", err)
	}

	asAdmin(svc)
	if _, err := svc.CreateShellSession(ctxAs("user-a"), "web", ""); errors.Is(err, core.ErrForbidden) {
		t.Fatalf("CreateShellSession by an admin was refused for protection: %v", err)
	}
}
