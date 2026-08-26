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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// update_service_mcp_test.go covers the w1/m71 fold: eighteen per-field MCP
// setters collapsed into one patch-shaped tool. The risk the fold introduces is
// not "does a field still write" — each field reaches the verb it always did —
// it is that ONE call now carries many fields, so a bug can write a field the
// caller never mentioned. Every test here is aimed at that.

// populatedService returns a Dockerfile-built repo service with every folded
// setting already carrying a distinctive value, so "unchanged" is observable
// rather than vacuously empty.
func populatedService(t *testing.T) (*Service, *appv1alpha1.App) {
	t.Helper()
	a := dockerRepoApp("web")
	a.Spec.DisplayName = "Original Label"
	a.Spec.Branch = "main"
	a.Spec.RootDir = "services/api"
	a.Spec.BuildCommand = "make build"
	a.Spec.StartCommand = "bin/original"
	a.Spec.DockerfilePath = "docker/Dockerfile"
	a.Spec.HealthCheckPath = "/healthz"
	a.Spec.PreDeployCommand = "bin/migrate"
	delay := int32(45)
	a.Spec.MaxShutdownDelaySeconds = &delay
	a.Spec.AutoDeploy = true
	a.Spec.BuildFilter = &appv1alpha1.BuildFilterSpec{Paths: []string{"services/**"}}
	a.Spec.NotifyOnFail = "notify"
	a.Spec.NotificationsToSend = "failure"
	a.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: false, URI: "https://status.example.com/original"}
	a.Spec.Hosts = []string{"api.example.com"} // lets subdomainPolicy=disabled validate
	a.Spec.SubdomainPolicy = appv1alpha1.SubdomainPolicyEnabled
	a.Spec.IPAllowListEntries = []appv1alpha1.IPAllowEntry{{CIDR: "203.0.113.0/24", Description: "office"}}
	a.Spec.Type = appv1alpha1.TypeWebService
	a.Spec.Tier = "starter" // maintenanceMode is a paid-plan web-service setting
	// No RegistryCredentialID here: binding one needs a credential store, and
	// registry-credential reach through update_service is covered by
	// TestMCPDockerBuildRegistryCredentialCreateUpdateAndEcho, which wires one.
	svc, _ := newService(nil, a)
	return svc, a
}

// TestUpdateServiceReachesEveryFoldedField is the positive half: each folded
// argument, passed alone, reaches the same Service verb its retired set_* tool
// called and lands in the CR spec.
func TestUpdateServiceReachesEveryFoldedField(t *testing.T) {
	tests := []struct {
		name  string
		arg   string
		value any
		check func(spec appv1alpha1.AppSpec) bool
	}{
		{"displayName", "displayName", "Customer API", func(s appv1alpha1.AppSpec) bool { return s.DisplayName == "Customer API" }},
		{"branch", "branch", "release", func(s appv1alpha1.AppSpec) bool { return s.Branch == "release" }},
		{"rootDir", "rootDir", "services/web", func(s appv1alpha1.AppSpec) bool { return s.RootDir == "services/web" }},
		{"buildCommand", "buildCommand", "npm run build", func(s appv1alpha1.AppSpec) bool { return s.BuildCommand == "npm run build" }},
		{"startCommand", "startCommand", "bin/server", func(s appv1alpha1.AppSpec) bool { return s.StartCommand == "bin/server" }},
		{"dockerfilePath", "dockerfilePath", "docker/Dockerfile.prod", func(s appv1alpha1.AppSpec) bool {
			return s.DockerfilePath == "docker/Dockerfile.prod"
		}},
		{"healthCheckPath", "healthCheckPath", "/livez", func(s appv1alpha1.AppSpec) bool { return s.HealthCheckPath == "/livez" }},
		{"preDeployCommand", "preDeployCommand", "bin/seed", func(s appv1alpha1.AppSpec) bool { return s.PreDeployCommand == "bin/seed" }},
		{"maxShutdownDelaySeconds", "maxShutdownDelaySeconds", 120, func(s appv1alpha1.AppSpec) bool {
			return s.MaxShutdownDelaySeconds != nil && *s.MaxShutdownDelaySeconds == 120
		}},
		{"autoDeploy", "autoDeploy", false, func(s appv1alpha1.AppSpec) bool { return !s.AutoDeploy }},
		{"buildFilter", "buildFilter", map[string]any{"paths": []string{"api/**"}, "ignoredPaths": []string{"docs/**"}}, func(s appv1alpha1.AppSpec) bool {
			return s.BuildFilter != nil && len(s.BuildFilter.Paths) == 1 && s.BuildFilter.Paths[0] == "api/**" &&
				len(s.BuildFilter.IgnoredPaths) == 1 && s.BuildFilter.IgnoredPaths[0] == "docs/**"
		}},
		{"notifyOnFail", "notifyOnFail", "ignore", func(s appv1alpha1.AppSpec) bool { return s.NotifyOnFail == "ignore" }},
		{"notificationsToSend", "notificationsToSend", "all", func(s appv1alpha1.AppSpec) bool { return s.NotificationsToSend == "all" }},
		{"maintenanceMode", "maintenanceMode", map[string]any{"enabled": true, "uri": "https://status.example.com/down"}, func(s appv1alpha1.AppSpec) bool {
			return s.MaintenanceMode != nil && s.MaintenanceMode.Enabled && s.MaintenanceMode.URI == "https://status.example.com/down"
		}},
		{"renderSubdomainPolicy", "renderSubdomainPolicy", "disabled", func(s appv1alpha1.AppSpec) bool {
			return s.SubdomainPolicy == appv1alpha1.SubdomainPolicyDisabled
		}},
		{"ipAllowList", "ipAllowList", []map[string]any{{"cidrBlock": "198.51.100.0/24", "description": "vpn"}}, func(s appv1alpha1.AppSpec) bool {
			return len(s.IPAllowListEntries) == 1 && s.IPAllowListEntries[0].CIDR == "198.51.100.0/24" &&
				s.IPAllowListEntries[0].Description == "vpn"
		}},
		{"autoscaling", "autoscaling", map[string]any{"minInstances": 2, "maxInstances": 5, "targetCPUPercent": 70}, func(s appv1alpha1.AppSpec) bool {
			return s.Autoscaling != nil && s.Autoscaling.Enabled && s.Autoscaling.MinReplicas == 2 && s.Autoscaling.MaxReplicas == 5
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := populatedService(t)
			call, cleanup := appsMCPClient(t, svc)
			defer cleanup()

			call("update_service", map[string]any{"serviceId": "web", tc.arg: tc.value})
			spec := getApp(t, svc.Client, "web").Spec
			if !tc.check(spec) {
				t.Fatalf("update_service %s did not reach the spec: %#v", tc.arg, spec)
			}
		})
	}
}

// TestUpdateServiceLeavesOmittedFieldsAlone is the fold's central invariant,
// asserted against a service where every folded field already has a value: a
// one-field call must not clear, default, or rewrite any of the other
// seventeen. Before the fold this could not regress; now it is the whole risk.
func TestUpdateServiceLeavesOmittedFieldsAlone(t *testing.T) {
	svc, before := populatedService(t)
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()

	call("update_service", map[string]any{"serviceId": "web", "displayName": "Renamed"})

	after := getApp(t, svc.Client, "web").Spec
	if after.DisplayName != "Renamed" {
		t.Fatalf("displayName = %q, want Renamed", after.DisplayName)
	}
	checks := []struct {
		field string
		ok    bool
	}{
		{"branch", after.Branch == before.Spec.Branch},
		{"rootDir", after.RootDir == before.Spec.RootDir},
		{"buildCommand", after.BuildCommand == before.Spec.BuildCommand},
		{"startCommand", after.StartCommand == before.Spec.StartCommand},
		{"dockerfilePath", after.DockerfilePath == before.Spec.DockerfilePath},
		{"healthCheckPath", after.HealthCheckPath == before.Spec.HealthCheckPath},
		{"preDeployCommand", after.PreDeployCommand == before.Spec.PreDeployCommand},
		{"maxShutdownDelaySeconds", after.MaxShutdownDelaySeconds != nil && *after.MaxShutdownDelaySeconds == 45},
		{"autoDeploy", after.AutoDeploy},
		{"buildFilter", after.BuildFilter != nil && len(after.BuildFilter.Paths) == 1},
		{"notifyOnFail", after.NotifyOnFail == "notify"},
		{"notificationsToSend", after.NotificationsToSend == "failure"},
		{"maintenanceMode", after.MaintenanceMode != nil && after.MaintenanceMode.URI == "https://status.example.com/original"},
		{"subdomainPolicy", after.SubdomainPolicy == appv1alpha1.SubdomainPolicyEnabled},
		{"ipAllowListEntries", len(after.IPAllowListEntries) == 1 && after.IPAllowListEntries[0].Description == "office"},
	}
	for _, c := range checks {
		if !c.ok {
			t.Errorf("update_service displayName=… changed the omitted %s: %#v", c.field, after)
		}
	}
}

// TestUpdateServiceAppliesEveryFieldInOneCall covers the other direction: a
// multi-field call must apply all of them, not just the first or last op.
func TestUpdateServiceAppliesEveryFieldInOneCall(t *testing.T) {
	svc, _ := populatedService(t)
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()

	call("update_service", map[string]any{
		"serviceId":               "web",
		"displayName":             "Combined",
		"branch":                  "release",
		"buildCommand":            "make ci",
		"startCommand":            "bin/combined",
		"healthCheckPath":         "/ready",
		"maxShutdownDelaySeconds": 90,
		"autoDeploy":              false,
		"notifyOnFail":            "ignore",
	})

	spec := getApp(t, svc.Client, "web").Spec
	if spec.DisplayName != "Combined" || spec.Branch != "release" || spec.BuildCommand != "make ci" ||
		spec.StartCommand != "bin/combined" || spec.HealthCheckPath != "/ready" ||
		spec.MaxShutdownDelaySeconds == nil || *spec.MaxShutdownDelaySeconds != 90 ||
		spec.AutoDeploy || spec.NotifyOnFail != "ignore" {
		t.Fatalf("combined update_service left a field unapplied: %#v", spec)
	}
}

// TestUpdateServiceEmptyValuesStillClear pins the second half of the pointer
// contract: absent leaves alone, but an explicitly empty value still clears,
// which is how every retired setter cleared a field.
func TestUpdateServiceEmptyValuesStillClear(t *testing.T) {
	svc, _ := populatedService(t)
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()

	call("update_service", map[string]any{
		"serviceId":        "web",
		"buildCommand":     "",
		"preDeployCommand": "",
		"healthCheckPath":  "",
		"ipAllowList":      []map[string]any{},
	})

	spec := getApp(t, svc.Client, "web").Spec
	if spec.BuildCommand != "" || spec.PreDeployCommand != "" || spec.HealthCheckPath != "" || len(spec.IPAllowListEntries) != 0 {
		t.Fatalf("empty arguments did not clear their fields: %#v", spec)
	}
	// …while a field that was not mentioned in the same call is untouched.
	if spec.StartCommand != "bin/original" {
		t.Fatalf("clearing other fields also cleared startCommand: %q", spec.StartCommand)
	}
}

// TestUpdateServiceBuildTriggeringFieldsStillTriggerABuild guards the
// behaviour the fold could most easily lose: rootDir and dockerfilePath do not
// merely write, they bump restartedAt so the service rebuilds. buildFilter
// deliberately does not (it only changes which FUTURE pushes deploy).
func TestUpdateServiceBuildTriggeringFieldsStillTriggerABuild(t *testing.T) {
	for _, tc := range []struct {
		name        string
		args        map[string]any
		wantRebuild bool
	}{
		{"rootDir rebuilds", map[string]any{"rootDir": "services/other"}, true},
		{"dockerfilePath rebuilds", map[string]any{"dockerfilePath": "docker/Dockerfile.ci"}, true},
		{"buildFilter does not", map[string]any{"buildFilter": map[string]any{"paths": []string{"api/**"}}}, false},
		{"displayName does not", map[string]any{"displayName": "Just A Label"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := populatedService(t)
			call, cleanup := appsMCPClient(t, svc)
			defer cleanup()

			if got := getApp(t, svc.Client, "web").Spec.RestartedAt; got != "" {
				t.Fatalf("fixture already carries restartedAt %q", got)
			}
			args := map[string]any{"serviceId": "web"}
			for k, v := range tc.args {
				args[k] = v
			}
			call("update_service", args)

			restartedAt := getApp(t, svc.Client, "web").Spec.RestartedAt
			if tc.wantRebuild && restartedAt == "" {
				t.Fatalf("update_service %v did not trigger a build (restartedAt empty)", tc.args)
			}
			if !tc.wantRebuild && restartedAt != "" {
				t.Fatalf("update_service %v triggered a build it should not have (restartedAt %q)", tc.args, restartedAt)
			}
		})
	}
}

// TestUpdateServiceAutoscalingKeepsItsDisableCounterpart: the patch tool
// enables/updates autoscaling, and disable_autoscaling — a delete verb, not a
// setter — still turns it off. The PUT/DELETE split REST has survives the fold.
func TestUpdateServiceAutoscalingKeepsItsDisableCounterpart(t *testing.T) {
	svc, _ := populatedService(t)
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()

	call("update_service", map[string]any{
		"serviceId":   "web",
		"autoscaling": map[string]any{"minInstances": 2, "maxInstances": 6, "targetCPUPercent": 65},
	})
	if as := getApp(t, svc.Client, "web").Spec.Autoscaling; as == nil || !as.Enabled || as.MaxReplicas != 6 {
		t.Fatalf("update_service autoscaling = %#v", as)
	}
	// The enable call answers with the service, and the autoscaling view still
	// has its own reader.
	if view := call("get_autoscaling", map[string]any{"serviceId": "web"}); view["enabled"] != true {
		t.Fatalf("get_autoscaling after update_service = %#v", view)
	}
	call("disable_autoscaling", map[string]any{"serviceId": "web"})
	if as := getApp(t, svc.Client, "web").Spec.Autoscaling; as != nil && as.Enabled {
		t.Fatalf("disable_autoscaling left autoscaling enabled: %#v", as)
	}
}

// TestUpdateServiceRejectsInvalidValues: validation still happens per verb, and
// a rejected op fails the call — the caller gets the error instead of a
// half-applied patch silently reporting success.
func TestUpdateServiceRejectsInvalidValues(t *testing.T) {
	svc, _ := populatedService(t)
	ctx := context.Background()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "update_service",
		Arguments: map[string]any{
			"serviceId":               "web",
			"maxShutdownDelaySeconds": 9000, // outside the 1-300 range
		},
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("out-of-range maxShutdownDelaySeconds should be a tool error: %#v", res)
	}
	if got := getApp(t, svc.Client, "web").Spec.MaxShutdownDelaySeconds; got == nil || *got != 45 {
		t.Fatalf("rejected update still wrote maxShutdownDelaySeconds: %v", got)
	}
}

// TestUpdateServiceMatchesRESTPatchFieldForField is the adapter-parity check the
// fold has to keep true: MCP no longer has one tool per field, so the question
// "does MCP still reach what REST reaches" stops being obvious. For each folded
// setting it applies the change through update_service and through
// PATCH /v1/services/{id} on two identical fixtures and requires the resulting
// CR spec to be identical — same value, same field, same side effects.
func TestUpdateServiceMatchesRESTPatchFieldForField(t *testing.T) {
	tests := []struct {
		name     string
		mcpArgs  map[string]any
		restBody string
	}{
		{"displayName", map[string]any{"displayName": "Customer API"}, `{"name":"Customer API"}`},
		{"repo", map[string]any{"repo": "https://github.com/acme/next", "branch": "release"}, `{"repo":"https://github.com/acme/next","branch":"release"}`},
		{"image", map[string]any{"image": "nginx:stable"}, `{"image":{"imagePath":"nginx:stable"}}`},
		{"rootDir", map[string]any{"rootDir": "services/web"}, `{"rootDir":"services/web"}`},
		{"buildCommand", map[string]any{"buildCommand": "npm run build"}, `{"serviceDetails":{"envSpecificDetails":{"buildCommand":"npm run build"}}}`},
		{"startCommand", map[string]any{"startCommand": "bin/server"}, `{"serviceDetails":{"envSpecificDetails":{"dockerCommand":"bin/server"}}}`},
		{"dockerfilePath", map[string]any{"dockerfilePath": "docker/Dockerfile.prod"}, `{"serviceDetails":{"envSpecificDetails":{"dockerfilePath":"docker/Dockerfile.prod"}}}`},
		{"healthCheckPath", map[string]any{"healthCheckPath": "/livez"}, `{"serviceDetails":{"healthCheckPath":"/livez"}}`},
		{"preDeployCommand", map[string]any{"preDeployCommand": "bin/seed"}, `{"serviceDetails":{"preDeployCommand":"bin/seed"}}`},
		{"maxShutdownDelaySeconds", map[string]any{"maxShutdownDelaySeconds": 120}, `{"serviceDetails":{"maxShutdownDelaySeconds":120}}`},
		{"autoDeploy", map[string]any{"autoDeploy": false}, `{"autoDeploy":"no"}`},
		{"notifyOnFail", map[string]any{"notifyOnFail": "ignore"}, `{"notifyOnFail":"ignore"}`},
		{"renderSubdomainPolicy", map[string]any{"renderSubdomainPolicy": "disabled"}, `{"renderSubdomainPolicy":"disabled"}`},
		{"ipAllowList", map[string]any{"ipAllowList": []map[string]any{{"cidrBlock": "198.51.100.0/24", "description": "vpn"}}},
			`{"serviceDetails":{"ipAllowList":[{"cidrBlock":"198.51.100.0/24","description":"vpn"}]}}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mcpSvc, _ := populatedService(t)
			call, cleanup := appsMCPClient(t, mcpSvc)
			defer cleanup()
			args := map[string]any{"serviceId": "web"}
			for k, v := range tc.mcpArgs {
				args[k] = v
			}
			call("update_service", args)
			viaMCP := getApp(t, mcpSvc.Client, "web").Spec

			restSvc, _ := populatedService(t)
			mux := http.NewServeMux()
			restSvc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", strings.NewReader(tc.restBody)))
			if rec.Code != http.StatusOK {
				t.Fatalf("REST PATCH %s => %d: %s", tc.restBody, rec.Code, rec.Body)
			}
			viaREST := getApp(t, restSvc.Client, "web").Spec

			// restartedAt is a timestamp; compare its emptiness (did this field
			// trigger a build?) rather than the instant.
			mcpRebuilt, restRebuilt := viaMCP.RestartedAt != "", viaREST.RestartedAt != ""
			viaMCP.RestartedAt, viaREST.RestartedAt = "", ""
			if mcpRebuilt != restRebuilt {
				t.Errorf("build trigger differs: MCP rebuilt=%v, REST rebuilt=%v", mcpRebuilt, restRebuilt)
			}
			if !reflect.DeepEqual(viaMCP, viaREST) {
				mcpJSON, _ := json.Marshal(viaMCP)
				restJSON, _ := json.Marshal(viaREST)
				t.Errorf("MCP and REST disagree on %s:\n  MCP:  %s\n  REST: %s", tc.name, mcpJSON, restJSON)
			}
		})
	}
}

// --- w1/m74: the second fold (per-field update_* tools) ---

// TestUpdateServiceCarriesTheSecondFoldsFields covers what update_service_plan,
// update_idle_timeout, update_publish_path and update_cron_job used to own. Each
// is asserted alone, because the risk the fold adds is a field being written
// when the caller did not ask for it.
func TestUpdateServiceCarriesTheSecondFoldsFields(t *testing.T) {
	t.Run("idleTTLSeconds", func(t *testing.T) {
		svc, _ := populatedService(t)
		call, cleanup := appsMCPClient(t, svc)
		defer cleanup()
		call("update_service", map[string]any{"serviceId": "web", "idleTTLSeconds": 900})
		if got := getApp(t, svc.Client, "web").Spec.IdleTTLSeconds; got != 900 {
			t.Fatalf("idleTTLSeconds = %d, want 900", got)
		}
	})

	t.Run("plan", func(t *testing.T) {
		svc, _ := populatedService(t)
		call, cleanup := appsMCPClient(t, svc)
		defer cleanup()
		call("update_service", map[string]any{"serviceId": "web", "plan": "standard"})
		if got := getApp(t, svc.Client, "web").Spec.Tier; got != "standard" {
			t.Fatalf("plan = %q, want standard", got)
		}
	})

	t.Run("publishPath", func(t *testing.T) {
		a := repoApp("site", "https://github.com/x/site", "main")
		a.Spec.Type = "static_site"
		a.Spec.PublishPath = "dist"
		svc, _ := newService(nil, a)
		call, cleanup := appsMCPClient(t, svc)
		defer cleanup()
		call("update_service", map[string]any{"serviceId": "site", "publishPath": "build"})
		if got := getApp(t, svc.Client, "site").Spec.PublishPath; got != "build" {
			t.Fatalf("publishPath = %q, want build", got)
		}
	})

	t.Run("cron schedule and command", func(t *testing.T) {
		svc, _ := newService(nil, cronApp("nightly"))
		call, cleanup := appsMCPClient(t, svc)
		defer cleanup()
		call("update_service", map[string]any{"serviceId": "nightly", "schedule": "0 6 * * *", "command": "node daily.js"})
		spec := getApp(t, svc.Client, "nightly").Spec
		if spec.Schedule != "0 6 * * *" || spec.Command != "node daily.js" {
			t.Fatalf("cron fields = %q / %q", spec.Schedule, spec.Command)
		}
		// Changing only the schedule leaves the command alone — the pair shares
		// one verb, which is exactly where a fold can lose a field.
		call("update_service", map[string]any{"serviceId": "nightly", "schedule": "30 7 * * *"})
		spec = getApp(t, svc.Client, "nightly").Spec
		if spec.Schedule != "30 7 * * *" || spec.Command != "node daily.js" {
			t.Fatalf("schedule-only update disturbed the command: %q / %q", spec.Schedule, spec.Command)
		}
	})
}

// TestUpdateServiceDryRunPreviewsThePlanOnly pins the rule w1/m74 took from
// PATCH /v1/services/{id}, including the one place MCP deliberately diverges:
// REST silently drops the other fields of a dry-run body, this refuses.
func TestUpdateServiceDryRunPreviewsThePlanOnly(t *testing.T) {
	svc, _ := populatedService(t)
	ctx := context.Background()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	// A dry-run plan change previews and writes nothing.
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "update_service", Arguments: map[string]any{
		"serviceId": "web", "plan": "standard", "dryRun": true,
	}})
	if err != nil || res.IsError {
		t.Fatalf("dry-run plan preview: err=%v isError=%v", err, res != nil && res.IsError)
	}
	if got := getApp(t, svc.Client, "web").Spec.Tier; got == "standard" {
		t.Fatalf("dry run wrote the plan: %q", got)
	}

	// A dry run carrying another field is refused, and names it.
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "update_service", Arguments: map[string]any{
		"serviceId": "web", "plan": "standard", "dryRun": true, "startCommand": "bin/other",
	}})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("dryRun with a non-plan field should be refused, not silently dropped")
	}
	if got := getApp(t, svc.Client, "web").Spec.StartCommand; got != "bin/original" {
		t.Fatalf("refused dry run still wrote startCommand: %q", got)
	}

	// A dry run with no plan at all is a read-only reflect, as REST does.
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "update_service", Arguments: map[string]any{
		"serviceId": "web", "dryRun": true,
	}})
	if err != nil || res.IsError {
		t.Fatalf("bare dry run should reflect current state: err=%v isError=%v", err, res != nil && res.IsError)
	}
}
