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
	"os"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestBlueprintLegacyRuntimeAlias(t *testing.T) {
	for _, tc := range []struct{ name, manifest string }{
		{"root", "services: [{type: pserv, name: clickhouse, env: docker}]"},
		{"equal", "services: [{type: pserv, name: clickhouse, env: docker, runtime: docker}]"},
		{"ungrouped", "ungrouped: {services: [{type: pserv, name: clickhouse, env: docker}]}"},
		{"environment", "projects: [{name: analytics, environments: [{name: qa, services: [{type: pserv, name: clickhouse, env: docker}]}]}]"},
		{"static", "services: [{type: web, name: site, env: static, buildCommand: npm run build, staticPublishPath: dist}]"},
		{"cron", "services: [{type: cron, name: job, env: docker, schedule: '0 * * * *', dockerCommand: echo ok}]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, ir, problems := CompileBlueprintIR(tc.manifest)
			if len(problems) != 0 {
				t.Fatalf("alias rejected: %+v", problems)
			}
			if len(ir.Resources) != 1 {
				t.Fatalf("resources = %+v", ir.Resources)
			}
			fields := ir.Resources[0].Fields
			if _, exists := fields["env"]; exists {
				t.Fatal("alias leaked into canonical IR")
			}
			runtime, exists := fields["runtime"]
			if !exists || runtime.Location.Line == 0 {
				t.Fatalf("runtime presence/location lost: %+v", runtime)
			}
			if _, err := decodeCompiledBlueprintManifest(source); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRenderClickHouseBlueprintUnchanged(t *testing.T) {
	manifest, err := os.ReadFile("testdata/render-clickhouse/render.yaml")
	if err != nil {
		t.Fatal(err)
	}
	// Bytes come from render-examples/clickhouse commit
	// 355817b1d95c1a9f2b088474437593234f4027f4, including the legacy env spelling.
	repo := "https://github.com/render-examples/clickhouse"
	stack, err := parseStack(DeployRequest{Manifest: string(manifest), Repo: repo, Branch: "master"})
	if err != nil {
		t.Fatal(err)
	}
	if len(stack.services) != 1 {
		t.Fatalf("services = %+v", stack.services)
	}
	request := stack.services[0].req
	if request.Repo != repo || request.Branch != "master" || request.Type != appv1alpha1.TypePrivateService || request.Runtime != "docker" {
		t.Fatalf("Git context or service semantics lost: %+v", request)
	}
	if request.Disk == nil || request.Disk.MountPath != "/var/lib/clickhouse" || request.Disk.SizeGB != 10 {
		t.Fatalf("disk = %+v", request.Disk)
	}
	if request.AutoDeploy == nil || *request.AutoDeploy {
		t.Fatalf("autoDeploy = %v", request.AutoDeploy)
	}
}

func TestBlueprintLegacyRuntimeAliasDiagnostics(t *testing.T) {
	for _, tc := range []struct{ name, fields string }{
		{"conflicting", "env: docker\n    runtime: node"},
		{"boolean", "env: false"},
		{"null", "env: null"},
		{"unknown", "env: mystery"},
		{"array", "env: [docker]"},
		{"duplicate", "env: docker\n    env: node"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := "services:\n  - type: pserv\n    name: clickhouse\n    " + tc.fields + "\n"
			_, problems := CompileBlueprintSource(manifest)
			if len(problems) == 0 {
				t.Fatal("invalid alias accepted")
			}
			for _, p := range problems {
				if strings.HasPrefix(p.Path, "#/services/0/env") && p.Line >= 4 {
					return
				}
			}
			t.Fatalf("no diagnostic at authored alias: %+v", problems)
		})
	}
}

func TestBlueprintLegacyAliasDoesNotRelaxUnknownFields(t *testing.T) {
	for _, manifest := range []string{
		"env: docker\nservices: [{type: pserv, name: clickhouse, runtime: docker}]",
		"services: [{type: pserv, name: clickhouse, env: docker, runtim: docker}]",
		"services: [{type: pserv, name: clickhouse, env: docker, disk: {env: docker, name: data, mountPath: /data, sizeGB: 10}}]",
	} {
		if _, problems := CompileBlueprintSource(manifest); len(problems) == 0 {
			t.Fatalf("unknown field accepted: %s", manifest)
		}
	}
}

func TestBlueprintLegacyAliasConflictWritesNothing(t *testing.T) {
	service, client := newService(nil)
	_, err := service.DeployStack(context.Background(), DeployRequest{
		Repo: "https://github.com/render-examples/clickhouse", Branch: "master",
		Manifest: "services: [{type: pserv, name: clickhouse, env: docker, runtime: node}]",
	})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("error = %v", err)
	}
	var apps appv1alpha1.AppList
	if err := client.List(context.Background(), &apps); err != nil {
		t.Fatal(err)
	}
	if len(apps.Items) != 0 {
		t.Fatalf("invalid manifest created %d services", len(apps.Items))
	}
}

func TestBlueprintAliasRegistryRejectsUnreviewedTargets(t *testing.T) {
	registry := decodeCapabilityRegistry(t)
	path := "#/definitions/serverService/properties/env"
	alias := registry.Aliases[path]
	alias.Target = "#/definitions/serverService/properties/plan"
	registry.Aliases[path] = alias
	if err := validateRenderBlueprintCapabilityRegistry(renderBlueprintSchemaSource, &registry); err == nil {
		t.Fatal("unreviewed alias target accepted")
	}
}

// w33/m435: Render documents afterFirstDeployCommand as "a deprecated alias
// for" initialDeployHook. The pinned schema has no such property and the
// compiler is fail-closed, so a file using it was refused as an unknown field.
func TestBlueprintAfterFirstDeployCommandAlias(t *testing.T) {
	for _, kind := range []string{"web", "pserv", "worker"} {
		t.Run(kind, func(t *testing.T) {
			manifest := "services:\n  - type: " + kind + "\n    name: app\n    runtime: image\n    image: {url: nginx:1}\n" +
				"    preDeployCommand: npm run migrate\n    afterFirstDeployCommand: npm run seed\n"
			source, ir, problems := CompileBlueprintIR(manifest)
			if len(problems) != 0 {
				t.Fatalf("alias rejected: %+v", problems)
			}
			fields := ir.Resources[0].Fields
			if _, exists := fields["afterFirstDeployCommand"]; exists {
				t.Fatal("alias leaked into canonical IR")
			}
			if hook, exists := fields["initialDeployHook"]; !exists || hook.Location.Line != 7 {
				t.Fatalf("initialDeployHook presence/location lost: %+v", hook)
			}
			if _, err := decodeCompiledBlueprintManifest(source); err != nil {
				t.Fatal(err)
			}
			stack, err := parseStack(DeployRequest{Manifest: manifest})
			if err != nil {
				t.Fatal(err)
			}
			if req := stack.services[0].req; req.InitialDeployHook != "npm run seed" || req.PreDeployCommand != "npm run migrate" {
				t.Fatalf("hook %q, pre-deploy %q; want the alias to carry the hook beside the pre-deploy command", req.InitialDeployHook, req.PreDeployCommand)
			}
		})
	}
}

// A valid file's plan carries the hook under its canonical name, with a
// warning located at the deprecated key.
func TestBlueprintAfterFirstDeployCommandValidatesWithADeprecationWarning(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	manifest := "services:\n  - type: web\n    name: app\n    runtime: image\n    image: {url: nginx:1}\n    afterFirstDeployCommand: npm run seed\n"
	v, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
	if err != nil || !v.Valid || len(v.Errors) != 0 || v.Plan == nil || len(v.Plan.Actions) != 1 {
		t.Fatalf("ValidateBlueprint = %+v, %v; want valid with one create action", v, err)
	}
	if len(v.warnings) != 1 || v.warnings[0].Path == nil || *v.warnings[0].Path != "services[0].afterFirstDeployCommand" ||
		v.warnings[0].Line == nil || *v.warnings[0].Line != 6 ||
		v.warnings[0].Error != "afterFirstDeployCommand is deprecated; use initialDeployHook" {
		t.Fatalf("warnings = %+v, want one deprecation located at services[0].afterFirstDeployCommand line 6", v.warnings)
	}
	notices := blueprintWarningNotices(v)
	if len(notices) != 1 || notices[0].Severity != "warning" || notices[0].Path == nil || *notices[0].Path != "services[0].afterFirstDeployCommand" {
		t.Fatalf("plan notices = %+v, want the deprecation as a located warning", notices)
	}

	// The canonical spelling earns none.
	canonical, err := svc.ValidateBlueprint(context.Background(), "", strings.Replace(manifest, "afterFirstDeployCommand", "initialDeployHook", 1), "")
	if err != nil || !canonical.Valid || len(canonical.warnings) != 0 {
		t.Fatalf("canonical spelling = %+v, %v; want valid with no warning", canonical, err)
	}

	// Applied, the service carries the hook exactly as the canonical spelling does.
	service, client := newService(nil)
	if _, err := service.DeployStack(context.Background(), DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	var apps appv1alpha1.AppList
	if err := client.List(context.Background(), &apps); err != nil {
		t.Fatal(err)
	}
	if len(apps.Items) != 1 || apps.Items[0].Spec.InitialDeployHook != "npm run seed" || apps.Items[0].Spec.PreDeployCommand != "" {
		t.Fatalf("created services = %+v, want one carrying the hook and no pre-deploy command", apps.Items)
	}
}

// Both spellings on one service are refused at the deprecated key, whether or
// not their values agree; unlike env/runtime, nothing says which would win.
func TestBlueprintAfterFirstDeployCommandWithInitialDeployHookIsRefused(t *testing.T) {
	for name, fields := range map[string]string{
		"different values": "initialDeployHook: npm run seed\n    afterFirstDeployCommand: npm run other",
		"equal values":     "initialDeployHook: npm run seed\n    afterFirstDeployCommand: npm run seed",
		"alias first":      "afterFirstDeployCommand: npm run seed\n    initialDeployHook: npm run seed",
	} {
		t.Run(name, func(t *testing.T) {
			manifest := "services:\n  - type: web\n    name: app\n    runtime: image\n    image: {url: nginx:1}\n    " + fields + "\n"
			_, problems := CompileBlueprintSource(manifest)
			if len(problems) != 1 || problems[0].Code != "BLUEPRINT_ALIAS_CONFLICT" ||
				problems[0].Path != "#/services/0/afterFirstDeployCommand" || problems[0].Line < 6 ||
				!strings.Contains(problems[0].Message, "afterFirstDeployCommand is the deprecated alias of initialDeployHook") {
				t.Fatalf("problems = %+v, want one alias conflict located at the deprecated key", problems)
			}
			svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
			v, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
			if err != nil || v.Valid || len(v.Errors) != 1 || v.Errors[0].Path == nil || *v.Errors[0].Path != "services[0].afterFirstDeployCommand" {
				t.Fatalf("ValidateBlueprint = %+v, %v; want one refusal at services[0].afterFirstDeployCommand", v, err)
			}
		})
	}
}

// Render offers the hook on web and private services and background workers
// only: a cron job or a static site refuses either spelling.
func TestBlueprintAfterFirstDeployCommandStaysRefusedWhereTheHookIs(t *testing.T) {
	for name, service := range map[string]string{
		"cron":   "{type: cron, name: job, runtime: image, image: {url: nginx:1}, schedule: '0 * * * *', %s: npm run seed}",
		"static": "{type: web, name: site, runtime: static, buildCommand: npm run build, staticPublishPath: dist, %s: npm run seed}",
	} {
		for _, field := range []string{"afterFirstDeployCommand", "initialDeployHook"} {
			t.Run(name+"/"+field, func(t *testing.T) {
				manifest := "services: [" + strings.Replace(service, "%s", field, 1) + "]"
				_, problems := CompileBlueprintSource(manifest)
				if len(problems) == 0 {
					t.Fatalf("%s accepted on a %s service", field, name)
				}
				for _, p := range problems {
					if strings.HasPrefix(p.Path, "#/services/0/"+field) {
						return
					}
				}
				t.Fatalf("no diagnostic at the authored key: %+v", problems)
			})
		}
	}
}

// The registry accepts exactly the reviewed aliases: one more, one fewer or a
// retargeted one fails the load.
func TestBlueprintAliasRegistryMatchesTheReviewedSet(t *testing.T) {
	registry := decodeCapabilityRegistry(t)
	if got, want := len(registry.Aliases), len(reviewedBlueprintAliases); got != want || want != 4 {
		t.Fatalf("registered aliases = %d, reviewed = %d; want 4 of each", got, want)
	}
	hook := "#/definitions/serverService/properties/afterFirstDeployCommand"
	for _, kind := range []string{"cronService", "staticService"} {
		if _, ok := registry.Aliases["#/definitions/"+kind+"/properties/afterFirstDeployCommand"]; ok {
			t.Fatalf("the hook alias is registered on %s, which refuses the hook", kind)
		}
	}
	alias := registry.Aliases[hook]
	alias.Target = "#/definitions/serverService/properties/preDeployCommand"
	registry.Aliases[hook] = alias
	if err := validateRenderBlueprintCapabilityRegistry(renderBlueprintSchemaSource, &registry); err == nil {
		t.Fatal("a hook alias retargeted at preDeployCommand was accepted")
	}
	registry = decodeCapabilityRegistry(t)
	delete(registry.Aliases, hook)
	if err := validateRenderBlueprintCapabilityRegistry(renderBlueprintSchemaSource, &registry); err == nil {
		t.Fatal("a registry without the hook alias was accepted")
	}
}
