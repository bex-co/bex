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
	"net/http"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestConfiguredWebhookRequiresDurableReplayStore(t *testing.T) {
	if err := validateWebhookReplayConfig("manual", "", false); err == nil {
		t.Fatal("manual webhook without replay store was accepted")
	}
	if err := validateWebhookReplayConfig("", "github", false); err == nil {
		t.Fatal("GitHub webhook without replay store was accepted")
	}
	if err := validateWebhookReplayConfig("manual", "", true); err != nil {
		t.Fatalf("configured replay store rejected: %v", err)
	}
	if err := validateWebhookReplayConfig("", "", false); err != nil {
		t.Fatalf("disabled webhook should not need a store: %v", err)
	}
}

// Every sandbox template's writable layer must fit the sandbox namespace's
// LimitRange ceiling, or the pod is refused at admission; the base template
// carries Render's 40 GB (w22/008).
func TestSandboxTemplatesFitTheSandboxLimitRange(t *testing.T) {
	ceiling := resource.MustParse(store.SandboxEphemeralMax)
	templates := sandboxTemplateRegistry("alpine:3", "agent:test")
	if got := templates["base"].EphemeralStorage; got != "40Gi" {
		t.Fatalf("base template ephemeral storage = %q, want 40Gi", got)
	}
	for name, tmpl := range templates {
		if tmpl.EphemeralStorage == "" {
			t.Errorf("template %q has no ephemeral-storage limit: a sandbox could fill the node's disk", name)
			continue
		}
		if q := resource.MustParse(tmpl.EphemeralStorage); q.Cmp(ceiling) > 0 {
			t.Errorf("template %q asks for %s, above the sandbox LimitRange ceiling %s", name, q.String(), ceiling.String())
		}
	}
}

func TestSandboxTemplateRegistryIncludesPluggableAgentDriver(t *testing.T) {
	templates := sandboxTemplateRegistry("alpine:3", "agent:test")
	agent, ok := templates["agent"]
	if !ok {
		t.Fatal("agent template is not registered")
	}
	if agent.Image != "agent:test" || len(agent.Entrypoint) != 1 || agent.Entrypoint[0] != "bex-agent-driver" {
		t.Fatalf("agent template = %+v", agent)
	}
	if agent.CPU != "2" || agent.Memory != "4Gi" {
		t.Fatalf("agent resource limits = %s/%s, want 2/4Gi", agent.CPU, agent.Memory)
	}
	large, ok := templates["agent-4c-8g"]
	if !ok {
		t.Fatal("agent-4c-8g template is not registered")
	}
	if large.Image != agent.Image || !slices.Equal(large.Entrypoint, agent.Entrypoint) {
		t.Fatalf("agent-4c-8g template = %+v, want the agent template's image and driver", large)
	}
	if large.CPU != "4" || large.Memory != "8Gi" {
		t.Fatalf("agent-4c-8g resource limits = %s/%s, want 4/8Gi", large.CPU, large.Memory)
	}
	// Above the namespace's 10Gi default, within the sandbox LimitRange's 40Gi max.
	for name, tmpl := range map[string]sandbox.Template{"agent": agent, "agent-4c-8g": large} {
		if tmpl.EphemeralStorage != "30Gi" {
			t.Errorf("%s ephemeral storage = %q, want 30Gi", name, tmpl.EphemeralStorage)
		}
	}
	// w7/063: only a session's sandbox runs the agent driver; the public
	// create refuses these templates, and the base one stays public.
	for name, tmpl := range templates {
		if want := name != "base"; tmpl.SessionOnly != want {
			t.Errorf("%s SessionOnly = %v, want %v", name, tmpl.SessionOnly, want)
		}
	}
	base := templates["base"]
	if base.Image != "alpine:3" {
		t.Fatalf("base template image = %q", base.Image)
	}
	// w32/m348: the base entrypoint links /bin/bash, then sleeps. OpenSandbox's
	// bootstrap runs a multi-word entrypoint as argv, so it must be one sh -c
	// script that parses.
	if !slices.Equal(base.Entrypoint, []string{"/bin/sh", "-c", baseEntrypoint}) || !strings.HasSuffix(baseEntrypoint, "\nexec sleep infinity") {
		t.Fatalf("base template entrypoint = %q", base.Entrypoint)
	}
	if out, err := exec.Command("/bin/sh", "-n", "-c", baseEntrypoint).CombinedOutput(); err != nil {
		t.Fatalf("base entrypoint does not parse: %v: %s", err, out)
	}
	// w32/019: Render's home directory, which the entrypoint creates.
	if base.Env["HOME"] != "/home/render" || !strings.Contains(baseEntrypoint, `mkdir -p -- "${HOME:-/root}"`) {
		t.Fatalf("base template env = %v, entrypoint %q; want HOME=/home/render, created at start", base.Env, baseEntrypoint)
	}
	// w8/247: a sandbox created with no plan runs the base template labelled
	// with the default plan, so the base template is that plan's size:
	// Render's 1c-2g, not the deprecated starter's 500m / 512Mi.
	cpu, memory, _ := sandbox.PlanLimits(sandbox.DefaultPlan)
	if sandbox.DefaultPlan != sandbox.Plan1C2G || base.CPU != cpu || base.Memory != memory || cpu != "1" || memory != "2Gi" {
		t.Fatalf("base template = %s/%s under default plan %q, want 1/2Gi under 1c-2g", base.CPU, base.Memory, sandbox.DefaultPlan)
	}
}

// requireCPAuth is the w1/m53 fail-closed gate on the internal control-plane
// API (:8091): an empty BEX_CP_TOKEN must abort startup unless the explicit
// local-dev override is set.
func TestRequireCPAuth(t *testing.T) {
	cases := []struct {
		name     string
		token    string
		insecure string
		wantErr  bool
	}{
		{"token set", "s3kret", "", false},
		{"token set, insecure ignored", "s3kret", "1", false},
		{"empty token fails closed", "", "", true},
		{"empty token, insecure!=1 still fails", "", "0", true},
		{"empty token, insecure!=1 word still fails", "", "yes", true},
		{"empty token, insecure=1 override", "", "1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := requireCPAuth(tc.token, tc.insecure)
			if (err != nil) != tc.wantErr {
				t.Fatalf("requireCPAuth(%q,%q) err=%v, wantErr=%v", tc.token, tc.insecure, err, tc.wantErr)
			}
		})
	}
}

func TestParseMigrateContainerPolicyArgs(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want migrateContainerPolicyArgs
		ok   bool
	}{
		{[]string{"srv-a"}, migrateContainerPolicyArgs{serviceID: "srv-a"}, true},
		{[]string{"srv-a", "--apply"}, migrateContainerPolicyArgs{serviceID: "srv-a", apply: true}, true},
		{[]string{"--apply", "srv-a"}, migrateContainerPolicyArgs{serviceID: "srv-a", apply: true}, true},
		// w32/m462: every native runtime service at once.
		{[]string{"--all-native"}, migrateContainerPolicyArgs{allNative: true}, true},
		{[]string{"--apply", "--all-native"}, migrateContainerPolicyArgs{allNative: true, apply: true}, true},
		{nil, migrateContainerPolicyArgs{}, false},
		{[]string{"--apply"}, migrateContainerPolicyArgs{}, false},
		{[]string{"srv-a", "srv-b"}, migrateContainerPolicyArgs{}, false},
		{[]string{"srv-a", "--force"}, migrateContainerPolicyArgs{}, false},
		// One service or all of them, never both.
		{[]string{"srv-a", "--all-native"}, migrateContainerPolicyArgs{}, false},
		{[]string{"--all-native", "srv-a"}, migrateContainerPolicyArgs{}, false},
	} {
		got, err := parseMigrateContainerPolicyArgs(tc.args)
		if (err == nil) != tc.ok || got != tc.want {
			t.Errorf("%v: %+v err=%v", tc.args, got, err)
		}
	}
}

// The server must never close an idle keep-alive connection before the edge
// stops reusing it; otherwise a request written onto it races the close
// (w3/044). Zero would silently fall back to ReadTimeout.
func TestNewHTTPServerIdleTimeoutOutlivesEdgePool(t *testing.T) {
	srv := newHTTPServer(":0", http.NotFoundHandler())
	if srv.IdleTimeout <= edgeUpstreamIdleTimeout {
		t.Fatalf("IdleTimeout = %v, want > edge upstream idle timeout %v", srv.IdleTimeout, edgeUpstreamIdleTimeout)
	}
	if srv.IdleTimeout <= srv.ReadTimeout {
		t.Fatalf("IdleTimeout = %v must exceed ReadTimeout %v (the implicit fallback that caused w3/044)", srv.IdleTimeout, srv.ReadTimeout)
	}
}
