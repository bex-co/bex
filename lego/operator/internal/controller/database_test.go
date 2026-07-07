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

package controller

import (
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestNormalizeIdent(t *testing.T) {
	cases := map[string]string{
		"bex-mvp-smoketest": "bex_mvp_smoketest", // hyphens -> underscores (valid unquoted identifier)
		"MyDB":              "mydb",              // lowercased
		"plain":             "plain",
	}
	for in, want := range cases {
		if got := normalizeIdent(in); got != want {
			t.Errorf("normalizeIdent(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolvePlan(t *testing.T) {
	// known plan
	if p, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}); p.mem != "1Gi" || gb != 5 {
		t.Errorf("basic-1gb => mem %q storage %d, want 1Gi/5", p.mem, gb)
	}
	// unknown plan falls back to free
	if p, _ := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "nonsense"}); p.mem != "256Mi" {
		t.Errorf("unknown plan should default to free (256Mi), got %q", p.mem)
	}
	// storage only grows: a request below the plan floor is raised to the floor
	if _, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "basic-1gb", StorageGB: 2}); gb != 5 {
		t.Errorf("storage below plan floor should be raised to 5, got %d", gb)
	}
	// a larger request is honored
	if _, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "free", StorageGB: 10}); gb != 10 {
		t.Errorf("larger storage request should be honored, got %d", gb)
	}
}

func TestCnpgClusterSpec(t *testing.T) {
	plan, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "free"})
	spec := cnpgClusterSpec(plan, gb, "", "my_db", "my_db_user")

	if spec["instances"] != int64(1) {
		t.Errorf("free instances = %v, want 1", spec["instances"])
	}
	storage := spec["storage"].(map[string]any)
	if storage["size"] != "1Gi" || storage["storageClass"] != dbStorageClass {
		t.Errorf("storage = %v", storage)
	}
	req := spec["resources"].(map[string]any)["requests"].(map[string]any)
	lim := spec["resources"].(map[string]any)["limits"].(map[string]any)
	if req["cpu"] != "100m" || req["memory"] != "256Mi" || lim["memory"] != "256Mi" {
		t.Errorf("resources requests=%v limits=%v (want Guaranteed 100m/256Mi)", req, lim)
	}
	initdb := spec["bootstrap"].(map[string]any)["initdb"].(map[string]any)
	if initdb["database"] != "my_db" || initdb["owner"] != "my_db_user" {
		t.Errorf("initdb = %v, want database=my_db owner=my_db_user", initdb)
	}
	if _, hasImage := spec["imageName"]; hasImage {
		t.Errorf("no version => no imageName override, got %v", spec["imageName"])
	}

	// version pins the image
	withVer := cnpgClusterSpec(plan, gb, "16", "d", "d_user")
	if withVer["imageName"] != "ghcr.io/cloudnative-pg/postgresql:16" {
		t.Errorf("version image = %v", withVer["imageName"])
	}
}

func TestIngressRouteTCPSpec(t *testing.T) {
	spec := ingressRouteTCPSpec("smoke-db.db.bex.co", "smoke-db-rw")

	if ep := spec["entryPoints"].([]any); len(ep) != 1 || ep[0] != pgEntryPoint {
		t.Errorf("entryPoints = %v, want [%s]", ep, pgEntryPoint)
	}
	// TLS passthrough: Postgres terminates its own TLS.
	if pt := spec["tls"].(map[string]any)["passthrough"]; pt != true {
		t.Errorf("tls.passthrough = %v, want true", pt)
	}
	route := spec["routes"].([]any)[0].(map[string]any)
	if route["match"] != "HostSNI(`smoke-db.db.bex.co`)" {
		t.Errorf("match = %v", route["match"])
	}
	svc := route["services"].([]any)[0].(map[string]any)
	if svc["name"] != "smoke-db-rw" || svc["port"] != int64(5432) {
		t.Errorf("service = %v, want smoke-db-rw:5432", svc)
	}
}
