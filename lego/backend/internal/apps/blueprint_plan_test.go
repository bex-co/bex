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
	"reflect"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestApplyBlueprintServiceSpecOmissionAndExplicitValues(t *testing.T) {
	t.Parallel()
	cpu := int32(70)
	initial := appv1alpha1.AppSpec{
		Repo:       "https://example.test/old.git",
		Branch:     "release",
		Tier:       "pro",
		Replicas:   4,
		AutoDeploy: true,
		Host:       "old.example.test",
		Hosts:      []string{"also-old.example.test"},
		BuildFilter: &appv1alpha1.BuildFilterSpec{
			Paths: []string{"old/**"},
		},
		Env: []appv1alpha1.EnvVar{{Name: "DECLARED", Value: "old"}, {Name: "DASHBOARD", Value: "keep"}},
		Autoscaling: &appv1alpha1.AutoscalingSpec{
			Enabled: true, MinReplicas: 1, MaxReplicas: 3, TargetCPUPercent: &cpu,
		},
	}

	t.Run("omitted service fields preserve current values except build filter", func(t *testing.T) {
		got := *initial.DeepCopy()
		changed := ApplyBlueprintServiceSpec(&got, appv1alpha1.AppSpec{}, nil)
		if !changed {
			t.Fatal("buildFilter omission must be a change when a filter exists")
		}
		if got.BuildFilter != nil {
			t.Fatalf("omitted buildFilter = %#v, want nil", got.BuildFilter)
		}
		want := *initial.DeepCopy()
		want.BuildFilter = nil
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("omission changed a field other than buildFilter:\n got: %#v\nwant: %#v", got, want)
		}
	})

	t.Run("explicit empty values apply only to their own field families", func(t *testing.T) {
		got := *initial.DeepCopy()
		want := appv1alpha1.AppSpec{
			Tier:       "starter",
			AutoDeploy: false,
			Env:        []appv1alpha1.EnvVar{{Name: "DECLARED", Value: "new"}},
		}
		fields := map[string]BlueprintField{
			"plan":        {},
			"domains":     {},
			"ipAllowList": {},
			"autoDeploy":  {},
			"envVars":     {},
		}
		ApplyBlueprintServiceSpec(&got, want, fields)
		if got.Tier != "starter" || got.AutoDeploy || got.Host != "" || len(got.Hosts) != 0 {
			t.Fatalf("declared plan/autoDeploy/domains not applied: %#v", got)
		}
		if len(got.Env) != 2 || got.Env[0].Name != "DECLARED" || got.Env[0].Value != "new" || got.Env[1].Name != "DASHBOARD" {
			t.Fatalf("envVars should upsert declared values and retain dashboard values: %#v", got.Env)
		}
		if got.Replicas != 4 || got.Autoscaling == nil {
			t.Fatalf("omitted manual/autoscaling fields reset scaling state: %#v", got)
		}
	})
}

func TestApplyBlueprintServiceSpecScalingOwnership(t *testing.T) {
	t.Parallel()
	cpu := int32(60)
	initial := appv1alpha1.AppSpec{
		Replicas: 2,
		Autoscaling: &appv1alpha1.AutoscalingSpec{
			Enabled: true, MinReplicas: 1, MaxReplicas: 4, TargetCPUPercent: &cpu,
		},
	}

	t.Run("manual numInstances disables declared autoscaling mode", func(t *testing.T) {
		got := *initial.DeepCopy()
		ApplyBlueprintServiceSpec(&got, appv1alpha1.AppSpec{Replicas: 5}, map[string]BlueprintField{"numInstances": {}})
		if got.Autoscaling != nil || got.Replicas != 5 {
			t.Fatalf("manual scaling = %#v, want replicas 5 with nil autoscaling", got)
		}
	})

	t.Run("scaling block owns autoscaling without changing omitted manual count", func(t *testing.T) {
		got := *initial.DeepCopy()
		memory := int32(75)
		ApplyBlueprintServiceSpec(&got, appv1alpha1.AppSpec{Autoscaling: &appv1alpha1.AutoscalingSpec{
			Enabled: true, MinReplicas: 2, MaxReplicas: 8, TargetMemoryPercent: &memory,
		}}, map[string]BlueprintField{"scaling": {}})
		if got.Autoscaling == nil || got.Autoscaling.MaxReplicas != 8 || got.Replicas != 2 {
			t.Fatalf("autoscaling block was not applied independently: %#v", got)
		}
	})
}

func TestBlueprintServiceFieldPoliciesHaveKnownOmission(t *testing.T) {
	t.Parallel()
	for _, policy := range BlueprintServiceFieldPolicies {
		if policy.Name == "" || (policy.Omission != BlueprintPreserveOnOmission && policy.Omission != BlueprintClearOnOmission) {
			t.Fatalf("invalid policy: %#v", policy)
		}
	}
}

func TestApplyBlueprintDatabaseSpecPresenceAndConstraints(t *testing.T) {
	t.Parallel()
	initial := appv1alpha1.DatabaseSpec{
		Name:            "orders",
		DatabaseName:    "orders_db",
		DatabaseUser:    "orders_user",
		Plan:            "basic-256mb",
		Version:         "16",
		StorageGB:       10,
		DiskAutoscaling: true,
		Pooler:          true,
		IPAllowList:     []appv1alpha1.IPAllowEntry{{CIDR: "10.0.0.0/8"}},
	}

	t.Run("omission leaves an existing database unchanged", func(t *testing.T) {
		got := *initial.DeepCopy()
		changed, err := ApplyBlueprintDatabaseSpec(&got, appv1alpha1.DatabaseSpec{}, nil)
		if err != nil || changed || !reflect.DeepEqual(got, initial) {
			t.Fatalf("omission = changed %v err %v spec %#v, want unchanged", changed, err, got)
		}
	})

	t.Run("declared empty list clears only the allow list", func(t *testing.T) {
		got := *initial.DeepCopy()
		changed, err := ApplyBlueprintDatabaseSpec(&got, appv1alpha1.DatabaseSpec{}, map[string]BlueprintField{"ipAllowList": {}})
		if err != nil || !changed || got.IPAllowList != nil || got.Plan != initial.Plan {
			t.Fatalf("explicit empty allow list = changed %v err %v spec %#v", changed, err, got)
		}
	})

	t.Run("declared false and none disable database features", func(t *testing.T) {
		got := *initial.DeepCopy()
		changed, err := ApplyBlueprintDatabaseSpec(&got, appv1alpha1.DatabaseSpec{}, map[string]BlueprintField{
			"storageAutoscalingEnabled": {}, "connectionPool": {},
		})
		if err != nil || !changed || got.DiskAutoscaling || got.Pooler {
			t.Fatalf("explicit disables = changed %v err %v spec %#v", changed, err, got)
		}
	})

	t.Run("immutable and shrink transitions name the offending field", func(t *testing.T) {
		got := *initial.DeepCopy()
		_, err := ApplyBlueprintDatabaseSpec(&got, appv1alpha1.DatabaseSpec{DatabaseName: "other"}, map[string]BlueprintField{"databaseName": {}})
		if conflict, ok := err.(*BlueprintFieldConflictError); !ok || conflict.Path != "databaseName" {
			t.Fatalf("immutable databaseName error = %v", err)
		}
		_, err = ApplyBlueprintDatabaseSpec(&got, appv1alpha1.DatabaseSpec{StorageGB: 5}, map[string]BlueprintField{"diskSizeGB": {}})
		if conflict, ok := err.(*BlueprintFieldConflictError); !ok || conflict.Path != "diskSizeGB" {
			t.Fatalf("shrink error = %v", err)
		}
		_, err = ApplyBlueprintDatabaseSpec(&got, appv1alpha1.DatabaseSpec{}, map[string]BlueprintField{"diskSizeGB": {}})
		if conflict, ok := err.(*BlueprintFieldConflictError); !ok || conflict.Path != "diskSizeGB" {
			t.Fatalf("explicit zero disk error = %v", err)
		}
	})
}

func TestParseBlueprintDatabaseMapsAutoscalingAndPooler(t *testing.T) {
	t.Parallel()
	enabled := true
	parsed, err := parseDatabase(bexDatabase{
		Name: "orders", StorageAutoscalingEnabled: &enabled, ConnectionPool: "pgbouncer",
	})
	if err != nil {
		t.Fatalf("parseDatabase: %v", err)
	}
	if parsed.spec.Plan != "basic-256mb" || !parsed.spec.DiskAutoscaling || !parsed.spec.Pooler {
		t.Fatalf("database mapping = %#v", parsed.spec)
	}
	parsed, err = parseDatabase(bexDatabase{Name: "orders", ConnectionPool: "none"})
	if err != nil || parsed.spec.Pooler {
		t.Fatalf("connectionPool none = %#v, err %v", parsed.spec, err)
	}
}

func TestApplyBlueprintKeyValueSpecPresence(t *testing.T) {
	t.Parallel()
	initial := appv1alpha1.KeyValueSpec{
		Plan:            "starter",
		MaxmemoryPolicy: "allkeys-lru",
		PersistenceMode: "journal-snapshot",
		IPAllowList:     []appv1alpha1.IPAllowEntry{{CIDR: "192.0.2.0/24"}},
	}
	got := *initial.DeepCopy()
	if changed := ApplyBlueprintKeyValueSpec(&got, appv1alpha1.KeyValueSpec{}, nil); changed || !reflect.DeepEqual(got, initial) {
		t.Fatalf("omitted Key Value fields changed spec: %#v", got)
	}
	if changed := ApplyBlueprintKeyValueSpec(&got, appv1alpha1.KeyValueSpec{PersistenceMode: "off"}, map[string]BlueprintField{"persistenceMode": {}}); !changed || got.PersistenceMode != "off" || got.Plan != "starter" {
		t.Fatalf("declared persistence mode did not apply narrowly: %#v", got)
	}
}

func TestParseBlueprintServiceMapsDockerAndStaticFields(t *testing.T) {
	t.Parallel()
	docker, _, err := parseService(DeployRequest{}, bexService{
		Name: "api", Type: "web", Runtime: "docker", Repo: "https://example.test/api.git", DockerCommand: "./serve",
	})
	if err != nil || docker.StartCommand != "./serve" {
		t.Fatalf("docker command = %#v, err %v", docker, err)
	}
	static, _, err := parseService(DeployRequest{}, bexService{
		Name: "site", Type: "web", Runtime: "static", Repo: "https://example.test/site.git", StaticPublishPath: "dist",
		RenderSubdomainPolicy: "disabled",
		Routes:                []StaticRouteView{{Type: "rewrite", Source: "/*", Destination: "/index.html"}},
		Headers:               []StaticHeaderView{{Path: "/*", Name: "X-Frame-Options", Value: "DENY"}},
	})
	if err != nil || static.SubdomainPolicy != "disabled" || len(static.Routes) != 1 || len(static.Headers) != 1 {
		t.Fatalf("static fields = %#v, err %v", static, err)
	}
	if _, _, err := parseService(DeployRequest{}, bexService{Name: "api", Type: "web", Runtime: "image", Image: &bexImage{URL: "nginx:1"}, DockerCommand: "bad"}); err == nil {
		t.Fatal("dockerCommand outside docker runtime was accepted")
	}
}

func TestParseBlueprintServiceRejectsUnsupportedFieldsAndUsesXBexBuilder(t *testing.T) {
	t.Parallel()
	parsed, _, err := parseService(DeployRequest{}, bexService{
		Name: "api", Type: "web", Runtime: "docker", Repo: "https://example.test/api.git",
		XBex: &bexExtension{Builder: "dockerfile"},
	})
	if err != nil || parsed.Builder != "dockerfile" {
		t.Fatalf("x-bex.builder = %q, err %v", parsed.Builder, err)
	}
	if _, _, err := parseService(DeployRequest{}, bexService{
		Name: "api", Type: "web", Runtime: "docker", Repo: "https://example.test/api.git", Builder: "dockerfile",
	}); err == nil {
		t.Fatal("unnamespaced builder was accepted")
	}
	if _, _, err := parseService(DeployRequest{}, bexService{
		Name: "api", Type: "web", Runtime: "docker", Repo: "https://example.test/api.git", AutoDeployTrigger: "checksPass",
	}); err == nil {
		t.Fatal("checksPass was accepted")
	}
}

func TestParseBlueprintServiceTranslatesDeprecatedDomain(t *testing.T) {
	t.Parallel()
	parsed, _, err := parseService(DeployRequest{}, bexService{
		Name: "api", Type: "web", Runtime: "docker", Repo: "https://example.test/api.git", Domain: "api.example.test",
	})
	if err != nil || !reflect.DeepEqual(parsed.Hosts, []string{"api.example.test"}) {
		t.Fatalf("domain translation = %#v, err %v", parsed.Hosts, err)
	}
}

func TestResolveDatabasePoolerReferenceStaysSecretBacked(t *testing.T) {
	t.Parallel()
	value, err := resolveDatabaseRef(bexEnvVar{Key: "DATABASE_POOL_URL", FromDatabase: &bexFromRef{
		Name: "orders", Property: "connectionPoolString",
	}}, map[string]string{"orders": "dpg-orders"})
	if err != nil {
		t.Fatalf("resolveDatabaseRef: %v", err)
	}
	if value.ValueFrom == nil || value.ValueFrom.SecretKeyRef == nil || value.ValueFrom.SecretKeyRef.Name != "dpg-orders-pooler-app" || value.ValueFrom.SecretKeyRef.Key != "uri" {
		t.Fatalf("pooled reference = %#v, want pooler Secret uri ref", value)
	}

	_, err = parseStack(DeployRequest{Manifest: `
databases:
  - name: orders
services:
  - name: api
    image: {url: nginx}
    envVars:
      - key: DATABASE_POOL_URL
        fromDatabase: {name: orders, property: connectionPoolString}
`})
	if err == nil {
		t.Fatal("connectionPoolString without pgbouncer was accepted")
	}
}
