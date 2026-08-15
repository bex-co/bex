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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const sshServiceID = "srv-d5t5d4v8g3c73f5m9peg"

type sshAuthzCall struct {
	relation string
	object   string
}

type sshAuthzRecorder struct {
	allow bool
	// freshDeny denies on the uncached path while the cached path still allows
	// — a member revoked on another replica whose positive is still warm here.
	freshDeny  bool
	calls      []sshAuthzCall
	freshCalls []sshAuthzCall
}

func (r *sshAuthzRecorder) Check(_ context.Context, _, relation, object string) (bool, error) {
	r.calls = append(r.calls, sshAuthzCall{relation: relation, object: object})
	return r.allow, nil
}

func (r *sshAuthzRecorder) CheckFresh(_ context.Context, _, relation, object string) (bool, error) {
	r.freshCalls = append(r.freshCalls, sshAuthzCall{relation: relation, object: object})
	if r.freshDeny {
		return false, nil
	}
	return r.allow, nil
}

func sshApp() *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Labels: map[string]string{
			core.LabelAppID: sshServiceID, core.LabelTenant: "tea-workspace",
		}},
		Spec:   appv1alpha1.AppSpec{Image: "example/web:v2", Type: appv1alpha1.TypeWebService, Tier: "starter", Replicas: 2},
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, Image: "example/web:v2", ActiveRevision: "rev-2"},
	}
}

func sshPod(name, image string, ready bool) *corev1.Pod {
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: map[string]string{
			core.PodLabelApp: "web", core.PodLabelRevision: "rev-2",
		}, CreationTimestamp: metav1.NewTime(time.Now().UTC())},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: core.AppContainer, Image: image}}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{
			Type: corev1.PodReady, Status: status,
		}}},
	}
}

func TestSSHAddressInstancesAndSpecificResolution(t *testing.T) {
	app := sshApp()
	current := sshPod("web-rs-pod01", "example/web:v2", true)
	stale := sshPod("web-rs-old01", "example/web:v1", true)
	notReady := sshPod("web-rs-wait1", "example/web:v2", false)
	service := &Service{Base: &core.Base{Client: fakeClient(app, current, stale, notReady), Namespace: "default"}, SSHHost: "ssh.bex.co"}

	view, err := service.Get(context.Background(), sshServiceID)
	if err != nil {
		t.Fatal(err)
	}
	if view.SSHAddress != sshServiceID+"@ssh.bex.co" || toRenderService(view).ServiceDetails["sshAddress"] != view.SSHAddress {
		t.Fatalf("SSH address projection = %q / %+v", view.SSHAddress, toRenderService(view).ServiceDetails)
	}
	instances, err := service.ListInstances(context.Background(), sshServiceID)
	if err != nil || len(instances) != 3 {
		t.Fatalf("public instances = %+v, %v", instances, err)
	}
	currentID := ids.DeriveServiceInstance(sshServiceID, current.Name)
	target, err := service.ResolveSSHSession(context.Background(), currentID)
	if err != nil || target.PodName != current.Name || target.ServiceID != sshServiceID {
		t.Fatalf("specific target = %+v, %v", target, err)
	}
	for _, excludedID := range []string{
		ids.DeriveServiceInstance(sshServiceID, stale.Name),
		ids.DeriveServiceInstance(sshServiceID, notReady.Name),
	} {
		if _, err := service.ResolveSSHSession(context.Background(), excludedID); !errors.Is(err, core.ErrNotFound) {
			t.Fatalf("excluded instance %q resolved with %v", excludedID, err)
		}
	}
}

// Native SSH reaches the same pods/exec sink as the Browser Web Shell, so it
// must enforce the same can_view_sensitive relation (codex round-4 #8). A
// contributor holds can_operate and can_manage_ssh_keys but NOT
// can_view_sensitive, so this pins the transport that could otherwise be used to
// walk around CreateShellSession's boundary.
func TestResolveSSHSessionAuthorizesCanViewSensitiveOnTargetWorkspace(t *testing.T) {
	app := sshApp()
	recorder := &sshAuthzRecorder{allow: true}
	service := &Service{Base: &core.Base{
		Client:    fakeClient(app, sshPod("web-rs-pod01", "example/web:v2", true)),
		Namespace: "default",
		Workspace: twoWorkspaces{"user-a": {"tea-home", "tea-workspace"}},
		Authz:     recorder,
	}}

	if _, err := service.ResolveSSHSession(ctxAs("user-a"), sshServiceID); err != nil {
		t.Fatal(err)
	}
	// One cached decision from the auditing AuthorizeApp, one uncached
	// re-assert from the round-7 F7 fresh gate — same relation, same workspace.
	if len(recorder.calls) != 1 || recorder.calls[0] != (sshAuthzCall{
		relation: core.RelCanViewSensitive,
		object:   core.WorkspaceObject("tea-workspace"),
	}) {
		t.Fatalf("SSH authorization calls = %+v", recorder.calls)
	}
	if len(recorder.freshCalls) != 1 || recorder.freshCalls[0] != (sshAuthzCall{
		relation: core.RelCanViewSensitive,
		object:   core.WorkspaceObject("tea-workspace"),
	}) {
		t.Fatalf("SSH fresh re-assertion calls = %+v", recorder.freshCalls)
	}

	// A member who can see the target but lacks can_view_sensitive must fail
	// before a session target is returned. This is the viewer/contributor denial
	// the public matrix later repeats against real OpenFGA.
	recorder.allow = false
	recorder.calls = nil
	recorder.freshCalls = nil
	if _, err := service.ResolveSSHSession(ctxAs("user-a"), sshServiceID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("under-privileged resolution = %v, want forbidden", err)
	}
	if len(recorder.calls) != 1 || recorder.calls[0].relation != core.RelCanViewSensitive {
		t.Fatalf("under-privileged authorization calls = %+v", recorder.calls)
	}
	if len(recorder.freshCalls) != 0 {
		t.Fatalf("cached denial must not reach the fresh re-assertion: %+v", recorder.freshCalls)
	}
}

// TestResolveSSHSessionReassertsFreshAuthorization pins codex round-7 F7: the
// gateway's target resolution converts a verified SSH key or browser ticket
// into a NEW hours-long pods/exec session, so the relation is consulted
// uncached — a member revoked on another replica, whose cached positive is
// still warm on this gateway process, must not open one last shell.
func TestResolveSSHSessionReassertsFreshAuthorization(t *testing.T) {
	recorder := &sshAuthzRecorder{allow: true, freshDeny: true}
	service := &Service{Base: &core.Base{
		Client:    fakeClient(sshApp(), sshPod("web-rs-pod01", "example/web:v2", true)),
		Namespace: "default",
		Authz:     recorder,
	}}
	if _, err := service.ResolveSSHSession(ctxAs("user-a"), sshServiceID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("stale-positive resolution = %v, want forbidden", err)
	}
}

func TestSSHEligibleServiceTypesResolve(t *testing.T) {
	for _, serviceType := range []string{
		appv1alpha1.TypeWebService,
		appv1alpha1.TypePrivateService,
		appv1alpha1.TypeBackgroundWorker,
	} {
		t.Run(string(serviceType), func(t *testing.T) {
			app := sshApp()
			app.Spec.Type = serviceType
			service := &Service{Base: &core.Base{Client: fakeClient(app, sshPod("web-rs-pod01", "example/web:v2", true)), Namespace: "default"}, SSHHost: "ssh.bex.co"}
			view, err := service.Get(context.Background(), sshServiceID)
			if err != nil || view.SSHAddress == "" {
				t.Fatalf("eligible %s address = %q, %v", serviceType, view.SSHAddress, err)
			}
			if _, err := service.ResolveSSHSession(context.Background(), sshServiceID); err != nil {
				t.Fatalf("eligible %s resolution = %v", serviceType, err)
			}
		})
	}
}

func TestSSHEligibilityFailsClosed(t *testing.T) {
	for name, mutate := range map[string]func(*appv1alpha1.App){
		"free":         func(app *appv1alpha1.App) { app.Spec.Tier = "free" },
		"missing plan": func(app *appv1alpha1.App) { app.Spec.Tier = "" },
		"unknown plan": func(app *appv1alpha1.App) { app.Spec.Tier = "mystery" },
		"suspended":    func(app *appv1alpha1.App) { app.Spec.Suspended = true },
		"cron":         func(app *appv1alpha1.App) { app.Spec.Type = appv1alpha1.TypeCronJob },
		"static":       func(app *appv1alpha1.App) { app.Spec.Type = appv1alpha1.TypeStaticSite },
		"not running":  func(app *appv1alpha1.App) { app.Status.Phase = appv1alpha1.PhaseDeploying },
		"no revision":  func(app *appv1alpha1.App) { app.Status.ActiveRevision = "" },
		"no image":     func(app *appv1alpha1.App) { app.Status.Image = "" },
	} {
		t.Run(name, func(t *testing.T) {
			app := sshApp()
			mutate(app)
			service := &Service{Base: &core.Base{Client: fakeClient(app, sshPod("web-rs-pod01", "example/web:v2", true)), Namespace: "default"}, SSHHost: "ssh.bex.co"}
			view, err := service.Get(context.Background(), sshServiceID)
			if err != nil {
				t.Fatal(err)
			}
			if (name == "not running" || name == "no revision" || name == "no image") && view.SSHAddress == "" {
				t.Fatal("configured eligible service should advertise address before it reaches Running")
			}
			if name != "not running" && name != "no revision" && name != "no image" && view.SSHAddress != "" {
				t.Fatalf("ineligible service advertised %q", view.SSHAddress)
			}
			if _, err := service.ResolveSSHSession(context.Background(), sshServiceID); !errors.Is(err, core.ErrConflict) {
				t.Fatalf("ResolveSSHSession = %v, want fail-closed conflict", err)
			}
		})
	}
}

func TestSSHAddressRejectsMalformedConfiguredHost(t *testing.T) {
	for _, host := range []string{
		"https://ssh.bex.co", "user@ssh.bex.co", "ssh.bex.co:22", "ssh.bex.co/path", "single-label",
	} {
		t.Run(host, func(t *testing.T) {
			service := &Service{Base: &core.Base{
				Client: fakeClient(sshApp()), Namespace: "default",
			}, SSHHost: host}
			view, err := service.Get(context.Background(), sshServiceID)
			if err != nil {
				t.Fatal(err)
			}
			if view.SSHAddress != "" {
				t.Fatalf("malformed host advertised as %q", view.SSHAddress)
			}
		})
	}

	service := &Service{Base: &core.Base{
		Client: fakeClient(sshApp()), Namespace: "default",
	}, SSHHost: "SSH.BEX.CO"}
	view, err := service.Get(context.Background(), sshServiceID)
	if err != nil || view.SSHAddress != sshServiceID+"@ssh.bex.co" {
		t.Fatalf("normalized host address = %q, %v", view.SSHAddress, err)
	}
}

func TestReadyCurrentAppPodRejectsTerminatingAndWrongContainer(t *testing.T) {
	now := metav1.Now()
	terminating := sshPod("web-rs-term1", "example/web:v2", true)
	terminating.DeletionTimestamp = &now
	wrongContainer := sshPod("web-rs-other", "example/web:v2", true)
	wrongContainer.Spec.Containers[0].Name = "sidecar"
	wrongRevision := sshPod("web-rs-oldrev", "example/web:v2", true)
	wrongRevision.Labels[core.PodLabelRevision] = "rev-1"
	for name, pod := range map[string]*corev1.Pod{
		"terminating":     terminating,
		"wrong container": wrongContainer,
		"wrong revision":  wrongRevision,
	} {
		t.Run(name, func(t *testing.T) {
			if readyCurrentAppPod(pod, "example/web:v2", "rev-2") {
				t.Fatal("pod unexpectedly eligible for SSH")
			}
		})
	}
}

func TestSSHInstanceIDNeverLeaksRawPodNames(t *testing.T) {
	app := sshApp()
	pod := sshPod("raw-pod-name", "example/web:v2", true)
	service := &Service{Base: &core.Base{
		Client: fakeClient(app, pod), Namespace: "default",
	}}
	instances, err := service.ListInstances(context.Background(), sshServiceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(instances) != 1 || strings.Contains(instances[0].ID, pod.Name) {
		t.Fatalf("pod name leaked through instance id: %+v", instances)
	}
	if _, err := service.ResolveSSHSession(context.Background(), instances[0].ID); err != nil {
		t.Fatalf("derived instance id did not resolve: %v", err)
	}
}

func TestSSHInstanceIDCollisionFailsClosed(t *testing.T) {
	app := sshApp()
	first := sshPod("web-rs1-a1b2c", "example/web:v2", true)
	second := sshPod("web-rs2-a1b2c", "example/web:v2", true)
	first.UID = types.UID("same-pod-identity")
	second.UID = types.UID("same-pod-identity")
	service := &Service{Base: &core.Base{
		Client: fakeClient(app, first, second), Namespace: "default",
	}}
	targets, err := service.readySSHInstances(context.Background(), app, service.view(app))
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 0 {
		t.Fatalf("ambiguous instance id was exposed: %+v", targets)
	}
	ambiguousID := ids.DeriveServiceInstance(sshServiceID, string(first.UID))
	if _, err := service.ResolveSSHSession(context.Background(), ambiguousID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("ambiguous instance resolved with %v", err)
	}
}
