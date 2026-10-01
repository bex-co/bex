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
