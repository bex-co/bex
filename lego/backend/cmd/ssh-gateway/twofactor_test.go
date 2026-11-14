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

package main

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/agentsessions"
	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
	sandboxfilesticket "github.com/bex-co/bex/lego/backend/internal/sandboxfiles"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
	sandboxfilegateway "github.com/bex-co/bex/lego/backend/internal/sshgateway/sandboxfiles"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/sandboxsse"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/twofactor"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// twofactor_test.go wires the gateway exactly as main() does, over the REAL
// verb handler and the REAL twofactor.Gate, and proves every gateway door that
// asks core.Base — native SSH key resolution (apps and ags-… targets), the web
// shell's resolution, agent attach, sandbox exec and sandbox files — refuses an
// unenrolled member of an enforcing workspace with the coded error and admits an
// enrolled one (w22/m402, docs/ADR012-auth.md §9a). The gateway's own pieces
// (cache, banner, revalidation) are tested beside their code; this pins the
// wiring and that every identity the gateway builds is one the verb accepts.

var _ agentsession.TwoFactorPolicy = (*twofactor.Gate)(nil)

type kratosFake struct{ enrolled map[string]bool } // subject -> enrolled; absent = not a Kratos identity

func (k kratosFake) MFAEnrolled(_ context.Context, subject string) (bool, bool, error) {
	enrolled, human := k.enrolled[subject]
	return enrolled, human, nil
}

type settingFake map[string]bool

func (s settingFake) RequireTwoFactor(_ context.Context, workspaceID string) (bool, error) {
	return s[workspaceID], nil
}

type sessionRows map[string]store.AgentSession

func (r sessionRows) GetAgentSession(_ context.Context, id string) (store.AgentSession, error) {
	row, ok := r[id]
	if !ok {
		return store.AgentSession{}, store.ErrNotFound
	}
	return row, nil
}

// AgentSessionWorkspace is the gate's one-column read of the same rows.
func (r sessionRows) AgentSessionWorkspace(_ context.Context, id string) (string, error) {
	return r[id].WorkspaceID, nil
}

type allowEverything struct{}

func (allowEverything) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

const (
	twoFactorOn      = "tea-enforcing"
	twoFactorOff     = "tea-relaxed"
	twoFactorService = "srv-d5t5d4v8g3c73f5m9peg"
)

type gatewayFixture struct {
	base     *core.Base
	sessions sessionRows
	session  string // an agent session of the enforcing workspace
	registry *prometheus.Registry
}

func newGatewayFixture(t *testing.T) *gatewayFixture {
	t.Helper()
	secret := []byte("exec-secret")
	// bex-api's side: the real gate over fake settings and Kratos, behind the
	// real verb handler.
	gate := twofactor.New(twofactor.Config{
		Settings:   settingFake{twoFactorOn: true},
		Enrollment: kratosFake{enrolled: map[string]bool{"enrolled": true, "unenrolled": false}},
	})
	verb := httptest.NewServer(&agentsession.TwoFactorHandler{Secret: secret, Policy: gate})
	t.Cleanup(verb.Close)

	session := ids.New(ids.AgentSession)
	f := &gatewayFixture{
		session: session, registry: prometheus.NewRegistry(),
		sessions: sessionRows{session: {
			ID: session, WorkspaceID: twoFactorOn, Phase: agentsessions.PhaseRunning, SandboxID: "os-sandbox",
		}},
	}
	// The gateway's side, built by the same constructor main() uses.
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(rendertest.Stamped(
		twoFactorApp(twoFactorOn), twoFactorPod(twoFactorOn),
		twoFactorApp(twoFactorOff), twoFactorPod(twoFactorOff),
	)...).WithInterceptorFuncs(rendertest.StampOnCreate).WithReturnManagedFields().Build()
	f.base = &core.Base{
		Client: cl, Namespace: "default", Authz: allowEverything{},
		Workspace: coretest.Workspaces{twoFactorOn, twoFactorOff},
		TwoFactor: newTwoFactorGate(string(secret), verb.URL, f.sessions, sshgateway.NewMetrics(f.registry)),
	}
	return f
}

func twoFactorApp(workspace string) client.Object {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web-" + workspace, Namespace: "default", Labels: map[string]string{
			core.LabelAppID: serviceFor(workspace), core.LabelTenant: workspace,
		}},
		Spec:   appv1alpha1.AppSpec{Image: "example/web:v2", Type: appv1alpha1.TypeWebService, Tier: "starter", Replicas: 1},
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, Image: "example/web:v2", ActiveRevision: "rev-2"},
	}
}

func twoFactorPod(workspace string) client.Object {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-" + workspace + "-pod01", Namespace: "default", Labels: map[string]string{
			core.PodLabelApp: "web-" + workspace, core.PodLabelRevision: "rev-2",
		}, CreationTimestamp: metav1.NewTime(time.Now().UTC())},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: core.AppContainer, Image: "example/web:v2"}}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{
			Type: corev1.PodReady, Status: corev1.ConditionTrue,
		}}},
	}
}

// serviceFor gives each fixture workspace its own well-formed service id.
func serviceFor(workspace string) string {
	if workspace == twoFactorOn {
		return twoFactorService
	}
	return "srv-d5t5d4v8g3c73f5m9pe0"
}

func asMember(subject, method string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: method})
}

func wantTwoFactorRefusal(t *testing.T, door string, err error) {
	t.Helper()
	var coded *core.CodedError
	if !errors.Is(err, core.ErrForbidden) || !errors.As(err, &coded) || coded.Code != core.CodeWorkspaceTwoFactorRequired {
		t.Fatalf("%s as an unenrolled member = %v, want the coded %s 403", door, err, core.CodeWorkspaceTwoFactorRequired)
	}
}

func TestEveryGatewayDoorRefusesAnUnenrolledMemberAndAdmitsAnEnrolledOne(t *testing.T) {
	f := newGatewayFixture(t)
	appResolver := &apps.Service{Base: f.base}
	agentResolver := &agentsessions.SSHResolver{Base: f.base, Store: f.sessions}
	attach := &agentsessions.AttachRevalidator{Base: f.base, Store: f.sessions}
	exec := &sandboxsse.ExecRevalidator{Base: f.base}
	files := &sandboxfilegateway.ExecRevalidator{Base: f.base}

	doors := map[string]func(subject string) error{
		// Native SSH key login to a service, and the web shell's redemption of
		// its ticket, resolve the same way with their own identity method.
		"native ssh to a service": func(subject string) error {
			_, err := appResolver.ResolveSSHSession(asMember(subject, "ssh"), twoFactorService)
			return err
		},
		"web shell": func(subject string) error {
			_, err := appResolver.ResolveSSHSession(asMember(subject, "shell"), twoFactorService)
			return err
		},
		"native ssh to an agent-session sandbox": func(subject string) error {
			_, err := agentResolver.ResolveSSHSession(asMember(subject, "ssh"), f.session)
			return err
		},
		"agent attach read": func(subject string) error {
			return attach.RevalidateAttach(context.Background(), subject, f.session, agentsessionticket.ActionRead)
		},
		"agent attach turn": func(subject string) error {
			return attach.RevalidateAttach(context.Background(), subject, f.session, agentsessionticket.ActionTurn)
		},
		"sandbox exec": func(subject string) error {
			return exec.RevalidateExec(context.Background(), sandboxexec.Claims{Subject: subject, Workspace: twoFactorOn})
		},
		"sandbox exec of an agent session's sandbox": func(subject string) error {
			return exec.RevalidateExec(context.Background(), sandboxexec.Claims{Subject: subject, Workspace: twoFactorOn, AgentSessionID: f.session})
		},
		"sandbox files": func(subject string) error {
			return files.RevalidateFiles(context.Background(), sandboxfilesticket.Claims{Subject: subject, Workspace: twoFactorOn})
		},
	}
	for door, open := range doors {
		t.Run(door, func(t *testing.T) {
			wantTwoFactorRefusal(t, door, open("unenrolled"))
			if err := open("enrolled"); err != nil {
				t.Fatalf("%s as an enrolled member = %v", door, err)
			}
		})
	}

	// The platform's own system ticket subject is never a member: the revalidator
	// exempts it before any check, and a subject that is not a Kratos identity is
	// exempt in the gate, so neither is refused.
	if err := exec.RevalidateExec(context.Background(), sandboxexec.Claims{Subject: sandboxexec.SystemSubject, Workspace: twoFactorOn}); err != nil {
		t.Fatalf("system exec ticket = %v", err)
	}
	if err := exec.RevalidateExec(context.Background(), sandboxexec.Claims{Subject: "platform-machine", Workspace: twoFactorOn}); err != nil {
		t.Fatalf("a subject that is not a Kratos identity = %v, want exempt", err)
	}
}

// A workspace that does not require 2FA is untouched for everyone, and the
// refusal is counted without any workspace or subject label.
func TestWorkspaceWithoutTheSettingAdmitsAnUnenrolledMember(t *testing.T) {
	f := newGatewayFixture(t)
	resolver := &apps.Service{Base: f.base}
	if _, err := resolver.ResolveSSHSession(asMember("unenrolled", "ssh"), serviceFor(twoFactorOff)); err != nil {
		t.Fatalf("a setting-off workspace refused an unenrolled member: %v", err)
	}
	if got := gatewaytest.MetricValue(t, f.registry, "bex_ssh_gateway_two_factor_refusals_total", nil); got != 0 {
		t.Fatalf("refusals counted for an admitted member: %v", got)
	}
	_, _ = resolver.ResolveSSHSession(asMember("unenrolled", "ssh"), twoFactorService)
	if got := gatewaytest.MetricValue(t, f.registry, "bex_ssh_gateway_two_factor_refusals_total", map[string]string{"result": "not_enrolled"}); got < 1 {
		t.Fatalf("the refusal was not counted: %v", got)
	}
}

// With bex-api unreachable, or the gateway unable to authenticate to it, the
// gate cannot know the setting, so nothing it guards is admitted.
func TestGatewayFailsClosedWhenTheVerbCannotAnswer(t *testing.T) {
	secret := []byte("exec-secret")
	unreachable := httptest.NewServer(&agentsession.TwoFactorHandler{Secret: secret})
	url := unreachable.URL
	unreachable.Close()
	wrongSecret := httptest.NewServer(&agentsession.TwoFactorHandler{Secret: []byte("another"), Policy: twofactor.New(twofactor.Config{Settings: settingFake{}})})
	t.Cleanup(wrongSecret.Close)

	for name, gate := range map[string]*sshgateway.TwoFactorGate{
		"bex-api unreachable":  newTwoFactorGate(string(secret), url, nil, nil),
		"signature rejected":   newTwoFactorGate(string(secret), wrongSecret.URL, nil, nil),
		"no exec secret wired": newTwoFactorGate("", wrongSecret.URL, nil, nil),
	} {
		t.Run(name, func(t *testing.T) {
			err := gate.Check(context.Background(), twoFactorOff, core.Identity{Subject: "enrolled", Method: "ssh"})
			if !errors.Is(err, core.ErrAuthzUnavailable) {
				t.Fatalf("Check = %v, want ErrAuthzUnavailable (fail closed)", err)
			}
		})
	}
}
