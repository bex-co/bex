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
	"net/http"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// A rejected multi-field patch must leave EVERY field it touched unchanged
// (w9/m166). The table applies each present field through its own setter, so a
// field whose row runs before the invalid one used to be persisted — and, for a
// build-relevant field, rolled the service — while the call still returned the
// 400 the invalid field earned. The caller therefore saw a failure and a
// changed, redeployed service at the same time.
func TestApplyServicePatchRejectsBeforeApplyingAnyField(t *testing.T) {
	invalid := "not-a-path"
	renamed := "renamed"
	delay := int32(8)

	cases := []struct {
		name  string
		patch ServicePatch
	}{
		{"name + invalid health path", ServicePatch{DisplayName: &renamed, HealthCheckPath: &invalid}},
		{"shutdown delay + invalid health path", ServicePatch{MaxShutdownDelaySeconds: &delay, HealthCheckPath: &invalid}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService(nil, sampleApp("web"))
			before := getApp(t, cl, "web")

			_, err := svc.ApplyServicePatch(context.Background(), "web", tc.patch)
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("ApplyServicePatch = %v, want bad-request", err)
			}

			after := getApp(t, cl, "web")
			if after.ResourceVersion != before.ResourceVersion {
				t.Errorf("rejected patch wrote to the App: resourceVersion %s -> %s",
					before.ResourceVersion, after.ResourceVersion)
			}
			if after.Spec.DisplayName != before.Spec.DisplayName {
				t.Errorf("displayName = %q, want unchanged %q", after.Spec.DisplayName, before.Spec.DisplayName)
			}
			if !sameInt32Ptr(after.Spec.MaxShutdownDelaySeconds, before.Spec.MaxShutdownDelaySeconds) {
				t.Errorf("maxShutdownDelaySeconds = %v, want unchanged %v",
					after.Spec.MaxShutdownDelaySeconds, before.Spec.MaxShutdownDelaySeconds)
			}
			if after.Spec.HealthCheckPath != before.Spec.HealthCheckPath {
				t.Errorf("healthCheckPath = %q, want unchanged %q", after.Spec.HealthCheckPath, before.Spec.HealthCheckPath)
			}
		})
	}
}

// sameInt32Ptr compares two optional int32s without collapsing nil onto a
// sentinel — "unset" and "set to -1" are different states, and the
// shutdown-delay assertions here turn on exactly that distinction.
func sameInt32Ptr(a, b *int32) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// The rejected patch must also mint no deploy row. The rollout batch is
// flushed on failure ON PURPOSE (w6/m51): a table that already rolled the
// service still owes that rollout a row. The fix is to never roll it, not to
// suppress the row — so with the preflight in place the batch has nothing to
// flush, and the service's deploy history stays clean.
//
// The field pair matters: the valid field must own a row that runs BEFORE the
// invalid one, or the test proves nothing. maxShutdownDelaySeconds does, and it
// moves the pod template, so it is exactly the pair whose rejected update was
// observed reaching `live` in production. (An earlier draft paired the health
// path with startCommand, whose row runs AFTER it — nothing was ever at risk
// and the test passed against the bug.)
func TestRejectedPatchOpensNoDeployRow(t *testing.T) {
	st := &recordingStore{}
	svc, _ := newService(st, managedRepoApp("web"))
	delay, invalid := int32(8), "not-a-path"

	_, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{
		MaxShutdownDelaySeconds: &delay, HealthCheckPath: &invalid,
	})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("ApplyServicePatch = %v, want bad-request", err)
	}
	if len(st.deployCalls) != 0 {
		t.Fatalf("rejected patch opened %d deploy rows, want none: %+v", len(st.deployCalls), st.deployCalls)
	}
}

// The REST surface is where the bug was reported from: the installed CLI sends
// one PATCH and correctly reports the 400, while the service had already moved.
func TestRESTPatchRejectsBeforeApplyingAnyField(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	before := getApp(t, cl, "web")

	rec := serveRESTPatch(t, svc, "web",
		`{"serviceDetails":{"maxShutdownDelaySeconds":8,"healthCheckPath":"not-a-path"}}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PATCH = %d, want 400: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "health check path must start with /") {
		t.Errorf("body = %s, want the existing health-path message", rec.Body)
	}
	after := getApp(t, cl, "web")
	if after.ResourceVersion != before.ResourceVersion {
		t.Errorf("rejected PATCH wrote to the App: resourceVersion %s -> %s",
			before.ResourceVersion, after.ResourceVersion)
	}
	if after.Spec.MaxShutdownDelaySeconds != nil {
		t.Errorf("maxShutdownDelaySeconds = %d, want still unset", *after.Spec.MaxShutdownDelaySeconds)
	}
}

// MCP's update_service reduces to the same table, so it must agree.
func TestMCPUpdateServiceRejectsBeforeApplyingAnyField(t *testing.T) {
	svc, _ := populatedService(t)
	before := getApp(t, svc.Client, "web").Spec.DisplayName

	msg := callAppsMCPError(t, svc, "update_service", map[string]any{
		"serviceId": "web", "displayName": "Renamed", "healthCheckPath": "not-a-path",
	})

	if !strings.Contains(msg, "health check path must start with /") {
		t.Errorf("error = %q, want the health-path refusal", msg)
	}
	if got := getApp(t, svc.Client, "web").Spec.DisplayName; got != before {
		t.Errorf("displayName = %q, want unchanged %q", got, before)
	}
}

// A refusal does not have to be a validation error. The table's rows do not
// all need the same permission — renaming is can_operate, repointing the build
// is can_create — so a contributor patching both used to get the rename
// applied and then a denial for the root directory.
func TestPatchDeniedOnALaterRowAppliesNothingEarlier(t *testing.T) {
	a := managedRepoApp("web")
	a.Spec.DisplayName = "Original"
	cl := fakeClient(a)
	svc := &Service{Base: &core.Base{
		Client:    cl,
		Namespace: "default",
		// A contributor: lifecycle yes, redefining what gets built no.
		Authz: &relationChecker{allow: map[string]bool{core.RelCanOperate: true}},
	}}

	renamed, rootDir := "Renamed", "services/api"
	_, err := svc.ApplyServicePatch(ctxAs("contributor"), "web", ServicePatch{
		DisplayName: &renamed, RootDir: &rootDir,
	})
	if err == nil {
		t.Fatal("ApplyServicePatch succeeded, want the can_create denial for rootDir")
	}
	if got := getApp(t, cl, "web").Spec.DisplayName; got != "Original" {
		t.Errorf("displayName = %q, want unchanged %q — the denied row must not let the permitted one through", got, "Original")
	}
}

// The preflight authorizes from each row's DECLARED relation, while the apply
// pass authorizes from inside each verb. If a row ever declares the weaker of
// the two, the preflight would wave a patch through that its own verb then
// refuses — mid-table, with the earlier rows already written, which is the bug
// this milestone closed. So: every relation the apply pass asks for must be
// one the preflight already asked for.
func TestPreflightAuthorizesEveryRelationTheApplyPassUses(t *testing.T) {
	// Every field present, so each row's apply can read the value it owns and
	// reach its verb's AuthorizeApp. The values need not be valid: the
	// assertion is on which relations the verb ASKS OpenFGA about, which is
	// recorded whether the verb then succeeds or refuses.
	full := func() ServicePatch {
		s := func(v string) *string { return &v }
		i := func(v int32) *int32 { return &v }
		b := func(v bool) *bool { return &v }
		return ServicePatch{
			DisplayName: s("Renamed"), Repo: s("https://github.com/bex-co/hello.git"),
			Image: s("img:v2"), ImageOwnerID: s(""), Branch: s("release"),
			RegistryCredentialID: s(""),
			MaintenanceMode:      &MaintenanceModeView{}, Plan: s("starter"),
			IdleTTLSeconds: i(600), MaxShutdownDelaySeconds: i(45),
			RootDir: s("services/api"), BuildFilter: &BuildFilterView{},
			AutoDeploy: b(false), Schedule: s("0 * * * *"), Command: s("bin/run"),
			HealthCheckPath: s("/livez"), PreDeployCommand: s("bin/migrate"),
			PublishPath: s("dist"), BuildCommand: s("make all"),
			StartCommand: s("bin/serve"), DockerfilePath: s("docker/Dockerfile"),
			Port: i(3000), NotifyOnFail: s("ignore"), NotificationsToSend: s("all"),
			RenderSubdomainPolicy: s("enabled"),
			IPAllowList:           &[]core.IPAllowListEntry{},
			Autoscaling:           &SetAutoscalingRequest{},
		}
	}
	ctx := ctxAs("owner")

	// The cron row is the one whose relation depends on the patch: supplying a
	// command is create-like, rescheduling alone is not. Both spellings.
	patches := map[string]ServicePatch{"with command": full(), "schedule only": full()}
	scheduleOnly := patches["schedule only"]
	scheduleOnly.Command = nil
	patches["schedule only"] = scheduleOnly

	for label, p := range patches {
		for i, row := range servicePatchTable {
			if !row.present(p) {
				continue
			}
			declared := row.relation(p)
			svc, _ := populatedService(t)
			// Allow everything, so the verb runs to completion and asks for
			// every relation it would ever ask for. What it ASKED is the
			// assertion — a verb that refuses for an unrelated reason has
			// still recorded its authorization by then.
			checker := &relationChecker{allow: map[string]bool{
				core.RelCanOperate: true, core.RelCanCreate: true,
				core.RelCanView: true, core.RelCanViewSensitive: true,
			}}
			svc.Authz = checker
			_, _ = row.apply(ctx, svc, "web", p)

			if len(checker.asked) == 0 {
				t.Errorf("%s: servicePatchTable[%d] (%v) authorized nothing — it cannot be preflighted",
					label, i, row.fields)
			}
			for _, asked := range checker.asked {
				// can_view is exempt: a verb may read the result back to
				// return it, which the preflight has no reason to pre-check.
				if asked == declared || asked == core.RelCanView {
					continue
				}
				t.Errorf("%s: servicePatchTable[%d] (%v) declares %q but its verb authorizes on %q — "+
					"the preflight would check the wrong permission and that denial would land "+
					"after the earlier rows had written", label, i, row.fields, declared, asked)
			}
		}
	}
}

// The counterweight to every test above: preflighting must not turn a valid
// multi-field patch into a refusal, and every field must still land.
func TestValidMultiFieldPatchStillAppliesEveryField(t *testing.T) {
	st := &recordingStore{}
	svc, cl := newService(st, managedRepoApp("web"))
	start, build, health, display := "./app --serve", "go build -o app ./cmd", "/healthz", "Web"
	delay := int32(45)

	if _, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{
		StartCommand: &start, BuildCommand: &build, HealthCheckPath: &health,
		DisplayName: &display, MaxShutdownDelaySeconds: &delay,
	}); err != nil {
		t.Fatalf("ApplyServicePatch: %v", err)
	}

	spec := getApp(t, cl, "web").Spec
	if spec.StartCommand != start || spec.BuildCommand != build ||
		spec.HealthCheckPath != health || spec.DisplayName != display ||
		!sameInt32Ptr(spec.MaxShutdownDelaySeconds, &delay) {
		t.Fatalf("a valid five-field patch did not apply every field: %+v", spec)
	}
	if len(st.deployCalls) != 1 {
		t.Fatalf("valid patch opened %d deploy rows, want exactly 1", len(st.deployCalls))
	}
}
