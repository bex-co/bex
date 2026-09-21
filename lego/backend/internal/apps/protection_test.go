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

// protection_test.go exercises w6/m19's protected-environment guard
// (protection.go): a member App of a protectedStatus=protected Environment
// refuses Delete/Suspend/an existing-service DeployStack override without a
// matching ProtectedConfirmation phrase — enforcement, not just the storage
// round-trip environments' own tests cover.

func TestDelete_BlockedWhenProtectedWithoutConfirm(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if err := svc.Delete(context.Background(), "web"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("Delete on a protected member: got %v, want ErrBadRequest", err)
	}
	// The App must survive an unconfirmed, blocked delete.
	getApp(t, cl, "web")
}

func TestDelete_SucceedsWithCorrectConfirm(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	want := ProtectedConfirmation("delete", "web")
	ctx := core.WithConfirm(context.Background(), want)
	if err := svc.Delete(ctx, "web"); err != nil {
		t.Fatalf("Delete with correct confirm: %v", err)
	}
	var got appv1alpha1.AppList
	if err := cl.List(context.Background(), &got); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got.Items) != 0 {
		t.Fatalf("App should be deleted, got %d remaining", len(got.Items))
	}
}

func TestDelete_WrongConfirmStillBlocked(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	svc, _ := newService(rec, managedApp("web", "srv-1"))

	ctx := core.WithConfirm(context.Background(), "sudo delete service someone-else")
	if err := svc.Delete(ctx, "web"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("Delete with wrong confirm: got %v, want ErrBadRequest", err)
	}
}

func TestDelete_UnprotectedNeedsNoConfirm(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "unprotected"}}
	svc, _ := newService(rec, managedApp("web", "srv-1"))

	if err := svc.Delete(context.Background(), "web"); err != nil {
		t.Fatalf("Delete on an unprotected member: %v", err)
	}
}

func TestDelete_RetryRemovesCRAfterSourceRowIsGone(t *testing.T) {
	rec := &recordingStore{err: store.ErrNotFound}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if err := svc.Delete(context.Background(), "web"); err != nil {
		t.Fatalf("Delete after row-first interruption: %v", err)
	}
	gone(t, cl, "web")
}

func TestSuspend_BlockedWhenProtectedWithoutConfirm(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if _, err := svc.Suspend(context.Background(), "web"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("Suspend on a protected member: got %v, want ErrBadRequest", err)
	}
	if getApp(t, cl, "web").Spec.Suspended {
		t.Fatal("a blocked suspend must not patch the CR")
	}
}

func TestSuspend_SucceedsWithCorrectConfirm(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	want := ProtectedConfirmation("suspend", "web")
	ctx := core.WithConfirm(context.Background(), want)
	if _, err := svc.Suspend(ctx, "web"); err != nil {
		t.Fatalf("Suspend with correct confirm: %v", err)
	}
	if !getApp(t, cl, "web").Spec.Suspended {
		t.Fatal("App should be suspended")
	}
}

// TestResume_NeverBlockedEvenWhenProtected proves Resume is exempt: a
// protected environment blocks taking availability AWAY, not restoring it.
func TestResume_NeverBlockedEvenWhenProtected(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	app := managedApp("web", "srv-1")
	app.Spec.Suspended = true
	svc, cl := newService(rec, app)

	if _, err := svc.Resume(context.Background(), "web"); err != nil {
		t.Fatalf("Resume on a protected member must never be blocked: %v", err)
	}
	if getApp(t, cl, "web").Spec.Suspended {
		t.Fatal("App should be resumed")
	}
}

func TestDeployStack_DirectOverrideBlockedWhenProtected(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	existing := managedApp("web", "srv-1")
	existing.Spec.Image = "old:1"
	svc, cl := newService(rec, existing)

	changed := "services:\n  - {name: web, type: web, runtime: image, image: {url: new:1}}\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: changed}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("DeployStack override on a protected member: got %v, want ErrBadRequest", err)
	}
	if got := getApp(t, cl, "web").Spec.Image; got != "old:1" {
		t.Fatalf("image must not change on a blocked override, got %q", got)
	}
}

func TestDeployStack_DirectOverrideSucceedsWithConfirm(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	existing := managedApp("web", "srv-1")
	existing.Spec.Image = "old:1"
	svc, cl := newService(rec, existing)

	changed := "services:\n  - {name: web, type: web, runtime: image, image: {url: new:1}}\n"
	confirm := ProtectedConfirmation("deploy", "web")
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: changed, Confirm: confirm}); err != nil {
		t.Fatalf("DeployStack override with correct confirm: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Image; got != "new:1" {
		t.Fatalf("image = %q, want new:1", got)
	}
}

func TestSyncBlueprint_ProtectedOverrideSucceedsWithConfirm(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	existing := managedApp("web", "srv-1")
	existing.Spec.Image = "old:1"
	svc, cl := newService(rec, existing)
	svc.Blueprints = newFakeBlueprintStore(store.Blueprint{
		ID:       "blp-1",
		Manifest: "services:\n  - {name: web, type: web, runtime: image, image: {url: new:1}}\n",
		Status:   "active",
	})

	if _, err := svc.SyncBlueprint(context.Background(), "blp-1", "", "", "", nil); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("SyncBlueprint override on a protected member: got %v, want ErrBadRequest", err)
	}
	if got := getApp(t, cl, "web").Spec.Image; got != "old:1" {
		t.Fatalf("blocked sync changed image to %q", got)
	}

	confirm := ProtectedConfirmation("deploy", "web")
	if _, err := svc.SyncBlueprint(context.Background(), "blp-1", "", "", confirm, nil); err != nil {
		t.Fatalf("SyncBlueprint with correct confirm: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Image; got != "new:1" {
		t.Fatalf("image = %q, want new:1", got)
	}
}

// TestDeployStack_NewServiceNeverBlocked proves a brand-new service (no
// existing App to override) is exempt — only an override of something
// already running is guarded.
func TestDeployStack_NewServiceNeverBlocked(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec)

	manifest := "services:\n  - {name: web, type: web, runtime: image, image: {url: new:1}}\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("DeployStack for a brand-new service: %v", err)
	}
	getApp(t, cl, "web")
}

// --- w4/m126: the guard covers the verbs that redefine or take offline -------
//
// The milestone's finding: a protected environment refused suspend and delete
// while setImage was accepted, changing which executable the service runs on
// its next deploy with no confirmation asked and no confirm argument to give.
// These are the service-layer half — one case per verb class, each asserting
// the refusal, the phrase that clears it, and that an unprotected member is
// untouched.

// protectedCall is one guarded verb bound to a fixture: build the App, run the
// verb against a Service, and report whether the spec actually changed.
type protectedCall struct {
	name    string
	verb    string // the word inside ProtectedConfirmation
	app     func() *appv1alpha1.App
	call    func(ctx context.Context, svc *Service, name string) error
	applied func(a *appv1alpha1.App) bool
}

func m126Calls() []protectedCall {
	str := func(s string) *string { return &s }
	docker := func() *appv1alpha1.App {
		a := managedRepoApp("web")
		a.Spec.Runtime = "docker"
		a.Spec.BuildCommand = ""
		return a
	}
	return []protectedCall{{
		name: "setImage",
		verb: "repoint",
		app:  func() *appv1alpha1.App { return managedApp("web", "srv-test") },
		call: func(ctx context.Context, svc *Service, n string) error {
			_, err := svc.SetSourceAndRegistryCredential(ctx, n, sourcePatch{Image: str("mendhak/http-https-echo:35")})
			return err
		},
		applied: func(a *appv1alpha1.App) bool { return a.Spec.Image == "mendhak/http-https-echo:35" },
	}, {
		name: "setBranch",
		verb: "repoint",
		app:  func() *appv1alpha1.App { return managedRepoApp("web") },
		call: func(ctx context.Context, svc *Service, n string) error {
			_, err := svc.SetSourceAndRegistryCredential(ctx, n, sourcePatch{Branch: str("release")})
			return err
		},
		applied: func(a *appv1alpha1.App) bool { return a.Spec.Branch == "release" },
	}, {
		name: "setStartCommand",
		verb: "redefine",
		app:  func() *appv1alpha1.App { return managedRepoApp("web") },
		call: func(ctx context.Context, svc *Service, n string) error {
			_, err := svc.SetCommands(ctx, n, nil, str("./app --serve"))
			return err
		},
		applied: func(a *appv1alpha1.App) bool { return a.Spec.StartCommand == "./app --serve" },
	}, {
		name: "setRootDir",
		verb: "redefine",
		app:  func() *appv1alpha1.App { return managedRepoApp("web") },
		call: func(ctx context.Context, svc *Service, n string) error {
			_, err := svc.SetRootDir(ctx, n, "services/api")
			return err
		},
		applied: func(a *appv1alpha1.App) bool { return a.Spec.RootDir == "services/api" },
	}, {
		name: "setDockerfilePath",
		verb: "redefine",
		app:  docker,
		call: func(ctx context.Context, svc *Service, n string) error {
			_, err := svc.SetDockerfilePath(ctx, n, "ops/Dockerfile")
			return err
		},
		applied: func(a *appv1alpha1.App) bool { return a.Spec.DockerfilePath == "ops/Dockerfile" },
	}, {
		name: "setPreDeployCommand",
		verb: "redefine",
		app:  func() *appv1alpha1.App { return managedRepoApp("web") },
		call: func(ctx context.Context, svc *Service, n string) error {
			_, err := svc.SetPreDeployCommand(ctx, n, "./migrate")
			return err
		},
		applied: func(a *appv1alpha1.App) bool { return a.Spec.PreDeployCommand == "./migrate" },
	}, {
		name: "updateCronJob command",
		verb: "redefine",
		app: func() *appv1alpha1.App {
			a := managedRepoApp("web")
			a.Spec.Type = appv1alpha1.TypeCronJob
			a.Spec.Schedule = "0 0 * * *"
			return a
		},
		call: func(ctx context.Context, svc *Service, n string) error {
			_, err := svc.SetCronJob(ctx, n, str("0 0 * * *"), str("/bin/exfiltrate"))
			return err
		},
		applied: func(a *appv1alpha1.App) bool { return a.Spec.Command == "/bin/exfiltrate" },
	}, {
		name: "enable maintenance mode",
		verb: "take offline",
		app: func() *appv1alpha1.App {
			a := managedApp("web", "srv-test")
			a.Spec.Type = appv1alpha1.TypeWebService
			a.Spec.Tier = "standard"
			return a
		},
		call: func(ctx context.Context, svc *Service, n string) error {
			_, err := svc.SetMaintenanceMode(ctx, n, MaintenanceModeView{Enabled: true})
			return err
		},
		applied: func(a *appv1alpha1.App) bool {
			return a.Spec.MaintenanceMode != nil && a.Spec.MaintenanceMode.Enabled
		},
	}}
}

func TestM126_ProtectedMemberRefusesWithoutConfirm(t *testing.T) {
	for _, c := range m126Calls() {
		t.Run(c.name, func(t *testing.T) {
			rec := &recordingStore{protectedStatus: map[string]string{"srv-test": "protected"}}
			svc, cl := newService(rec, c.app())

			err := c.call(context.Background(), svc, "web")
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("on a protected member: got %v, want ErrBadRequest", err)
			}
			// The refusal has to carry the phrase — a caller with no way to
			// learn it is refused permanently, which is the bug one level down.
			if want := ProtectedConfirmation(c.verb, "web"); !strings.Contains(err.Error(), want) {
				t.Fatalf("error %q must name the phrase %q", err, want)
			}
			if c.applied(getApp(t, cl, "web")) {
				t.Fatal("a refused verb must not have changed the spec")
			}
		})
	}
}

func TestM126_ProtectedMemberProceedsWithConfirm(t *testing.T) {
	for _, c := range m126Calls() {
		t.Run(c.name, func(t *testing.T) {
			rec := &recordingStore{protectedStatus: map[string]string{"srv-test": "protected"}}
			svc, cl := newService(rec, c.app())

			ctx := core.WithConfirm(context.Background(), ProtectedConfirmation(c.verb, "web"))
			if err := c.call(ctx, svc, "web"); err != nil {
				t.Fatalf("with the confirmation phrase: %v", err)
			}
			if !c.applied(getApp(t, cl, "web")) {
				t.Fatal("a confirmed verb must have applied")
			}
		})
	}
}

func TestM126_UnprotectedMemberNeedsNoConfirm(t *testing.T) {
	for _, c := range m126Calls() {
		t.Run(c.name, func(t *testing.T) {
			// No protectedStatus entry at all: the store's default, an App in
			// no environment or an unprotected one.
			svc, cl := newService(&recordingStore{}, c.app())

			if err := c.call(context.Background(), svc, "web"); err != nil {
				t.Fatalf("unprotected member: %v", err)
			}
			if !c.applied(getApp(t, cl, "web")) {
				t.Fatal("an unprotected verb must have applied unchanged")
			}
		})
	}
}

// TestM126_WrongVerbPhraseDoesNotArmAnother is why the phrase is per class and
// not per resource: a confirm the user typed to repoint a service must not
// silently authorize taking it offline.
func TestM126_WrongVerbPhraseDoesNotArmAnother(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-test": "protected"}}
	a := managedApp("web", "srv-test")
	a.Spec.Type = appv1alpha1.TypeWebService
	a.Spec.Tier = "standard"
	svc, _ := newService(rec, a)

	ctx := core.WithConfirm(context.Background(), ProtectedConfirmation("repoint", "web"))
	if _, err := svc.SetMaintenanceMode(ctx, "web", MaintenanceModeView{Enabled: true}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("repoint's phrase must not arm take-offline: got %v", err)
	}
}

// TestM126_RestoringVerbsStayUngated mirrors Resume's exclusion: a protected
// environment blocks taking availability away, not giving it back, and a cron
// reschedule changes when the job runs rather than what it runs.
func TestM126_RestoringVerbsStayUngated(t *testing.T) {
	t.Run("disable maintenance mode", func(t *testing.T) {
		rec := &recordingStore{protectedStatus: map[string]string{"srv-test": "protected"}}
		a := managedApp("web", "srv-test")
		a.Spec.Type = appv1alpha1.TypeWebService
		a.Spec.Tier = "standard"
		a.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: true}
		svc, cl := newService(rec, a)

		if _, err := svc.SetMaintenanceMode(context.Background(), "web", MaintenanceModeView{}); err != nil {
			t.Fatalf("turning maintenance mode off must not need a confirmation: %v", err)
		}
		if m := getApp(t, cl, "web").Spec.MaintenanceMode; m != nil && m.Enabled {
			t.Fatal("maintenance mode should be off")
		}
	})

	t.Run("cron reschedule only", func(t *testing.T) {
		rec := &recordingStore{protectedStatus: map[string]string{"srv-test": "protected"}}
		a := managedRepoApp("web")
		a.Spec.Type = appv1alpha1.TypeCronJob
		a.Spec.Schedule = "0 0 * * *"
		svc, cl := newService(rec, a)

		sched := "*/5 * * * *"
		if _, err := svc.SetCronJob(context.Background(), "web", &sched, nil); err != nil {
			t.Fatalf("rescheduling must not need a confirmation: %v", err)
		}
		if got := getApp(t, cl, "web").Spec.Schedule; got != sched {
			t.Fatalf("schedule = %q, want %q", got, sched)
		}
	})
}

// TestM126_ProtectionLookupFailureFailsClosed: the guard is only as good as the
// lookup behind it, so a control-plane outage must refuse the verb rather than
// wave it through. Delete and suspend already behave this way.
func TestM126_ProtectionLookupFailureFailsClosed(t *testing.T) {
	rec := &recordingStore{protectedErr: errors.New("control plane unavailable")}
	svc, cl := newService(rec, managedApp("web", "srv-test"))

	image := "mendhak/http-https-echo:35"
	if _, err := svc.SetSourceAndRegistryCredential(context.Background(), "web", sourcePatch{Image: &image}); err == nil {
		t.Fatal("want an error when the protection lookup fails")
	}
	if got := getApp(t, cl, "web").Spec.Image; got == image {
		t.Fatal("the image must not have changed when protection could not be read")
	}
}

// TestM126_GuardedMutationsAcceptConfirm is the milestone's second finding, and
// the one a service-layer test cannot catch: the guard was not merely absent on
// these verbs, it was *unreachable*, because none of them had a `confirm`
// argument for a caller who wanted to supply the phrase. A guarded verb with no
// way to confirm it is a verb permanently refused on a protected member.
func TestM126_GuardedMutationsAcceptConfirm(t *testing.T) {
	svc := &Service{}
	mutations := svc.GraphQLMutation()
	for _, name := range []string{
		// w6/m19's originals, so a refactor cannot quietly drop them.
		"deleteService", "suspendService",
		// w4/m126's additions.
		"setImage", "setRepo", "setBranch", "setRegistryCredential",
		"setBuildCommand", "setStartCommand", "setPreDeployCommand",
		"setRootDir", "setDockerfilePath",
		"updateCronJob", "setMaintenanceMode",
	} {
		field, ok := mutations[name]
		if !ok {
			t.Errorf("mutation %s is missing", name)
			continue
		}
		if _, ok := field.Args["confirm"]; !ok {
			t.Errorf("mutation %s has no confirm argument, so its protected-environment refusal cannot be cleared", name)
		}
	}
}
