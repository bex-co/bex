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
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// roundTripFixture is one resource of every exportable kind, each carrying the
// fields that make the export non-trivial — a custom domain on the web service
// (the w4/m124 finding), an allow-list on the Key Value, a disk and HA on the
// Postgres.
func roundTripFixture() (*Service, GenerateBlueprintRequest) {
	svcSpec := func(t string) appv1alpha1.AppSpec {
		return appv1alpha1.AppSpec{
			Type: t, Repo: "https://github.com/acme/app", Branch: "main",
			Runtime: "docker", Tier: "starter", Replicas: 1,
		}
	}
	web := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       svcSpec(appv1alpha1.TypeWebService),
	}
	// The primary custom domain lives in Spec.Host, the shape the exporter
	// flattens and the planner used to move into Hosts on every re-plan.
	web.Spec.Host = "blockeden.xyz"
	web.Spec.HealthCheckPath = "/healthz"

	// The MIRROR shape, and the ordinary one: AddDomain appends to Spec.Hosts
	// and never sets Spec.Host, so a service that gained its domains through the
	// API has Host empty and Hosts populated. Flattened it is [a, b]; written
	// back canonically it would become Host=a, Hosts=[b] — a different split of
	// the same set, and therefore a spurious `update` unless the applier
	// compares the SETS. Without this fixture the set-comparison is untested.
	private := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "private", Namespace: "default"},
		Spec:       svcSpec(appv1alpha1.TypePrivateService),
	}
	worker := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "default"},
		Spec:       svcSpec(appv1alpha1.TypeBackgroundWorker),
	}
	cron := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "cron", Namespace: "default"},
		Spec:       svcSpec(appv1alpha1.TypeCronJob),
	}
	cron.Spec.Schedule = "0 * * * *"
	cron.Spec.Command = "./run"

	static := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "site", Namespace: "default"},
		Spec:       svcSpec(appv1alpha1.TypeStaticSite),
	}
	// The MIRROR host shape, and the ordinary one: AddDomain appends to
	// Spec.Hosts and never sets Spec.Host, so a service that gained its domains
	// through the API has Host empty and Hosts populated. Flattened it is
	// [a, b]; written back canonically it would become Host=a, Hosts=[b] — a
	// different split of the same set, and therefore a spurious `update` unless
	// the applier compares the SETS. Without this the set-comparison is
	// untested. (It goes on the static site because a private service has no
	// ingress and is refused domains outright.)
	static.Spec.Hosts = []string{"a.example.com", "b.example.com"}
	static.Spec.PublishPath = "dist"
	static.Spec.BuildCommand = "npm run build"
	static.Spec.Runtime = ""

	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-rt", Namespace: "default"},
		Spec: appv1alpha1.DatabaseSpec{
			Name: "rt-db", Plan: "basic-1gb", StorageGB: 10, Version: "16", HighAvailability: true,
		},
	}
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-rt", Namespace: "default"},
		Spec: appv1alpha1.KeyValueSpec{
			Name: "rt-cache", Plan: "standard", MaxmemoryPolicy: "allkeys-lru",
		},
	}
	cl := fakeClient(web, private, worker, cron, static, db, kv)
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}, EnvNames: fakeEnvNames{}}
	return svc, GenerateBlueprintRequest{
		ServiceIDs:  []string{"web", "private", "worker", "cron", "site"},
		PostgresIDs: []string{"dpg-rt"},
		KeyValueIDs: []string{"red-rt"},
	}
}

// TestGeneratedBlueprintRePlansAsNoop is w4/m124's definition of done, and the
// guard the milestone asks for: export → validate is a CLOSED loop.
//
// A user's first move with Blueprints is to export the stack they already run,
// commit it, and connect it. Live on 2026-09-21 that manifest re-planned as
// `update` for the web service and the Key Value — the plan proposed to modify
// the very resources it had just described. A plan nobody believes is a plan
// nobody reads, which is worse than no plan at the moment one of the actions is
// real.
//
// Two distinct causes sat behind it, both of the same shape — the exporter and
// the planner disagreeing about how a value is represented:
//   - `domains:` is one flat list, but the spec keeps the primary host in
//     Spec.Host and the rest in Spec.Hosts. The create path has no Host field,
//     so the whole list arrived in Hosts and the planner moved the domain out
//     of Host every time.
//   - a declared `ipAllowList: []` cloned to an empty NON-nil slice, while the
//     live CR reads back nil (Kubernetes drops empty slices), and
//     reflect.DeepEqual(nil, []T{}) is false.
func TestGeneratedBlueprintRePlansAsNoop(t *testing.T) {
	svc, req := roundTripFixture()
	ctx := context.Background()

	out, err := svc.GenerateBlueprint(ctx, req)
	if err != nil {
		t.Fatalf("GenerateBlueprint: %v", err)
	}
	// The finding's own trigger must actually be in the exported file, or this
	// test would pass for the wrong reason.
	for _, host := range []string{"blockeden.xyz", "a.example.com", "b.example.com"} {
		if !strings.Contains(out.Manifest, host) {
			t.Fatalf("fixture did not export %s, so the regression is not exercised:\n%s", host, out.Manifest)
		}
	}

	v, err := svc.ValidateBlueprint(ctx, "", out.Manifest)
	if err != nil || !v.Valid {
		t.Fatalf("bex's own export must validate: %+v err=%v", v, err)
	}
	if v.Plan == nil {
		t.Fatal("validation returned no plan")
	}
	plan := *v.Plan
	if len(plan.Actions) == 0 {
		t.Fatal("plan produced no actions at all — the resources were not matched")
	}
	for _, action := range plan.Actions {
		if action.Operation != BlueprintPlanNoop {
			t.Errorf("%s %q re-plans as %q with changedFields %v — bex's own export does not describe its own source",
				action.Kind, action.Name, action.Operation, fieldPaths(action.ChangedFields))
		}
	}
}

// TestPlanNamesOnlyTheFieldThatChanged is the other half: the loop being closed
// must not come from a planner that stopped noticing things. Until w4/m124
// changedFields listed every field the MANIFEST DECLARED, so two key-value
// manifests differing in maxmemoryPolicy produced byte-identical plans — a
// field list that does not depend on the diff is not a diff.
func TestPlanNamesOnlyTheFieldThatChanged(t *testing.T) {
	svc, req := roundTripFixture()
	ctx := context.Background()
	out, err := svc.GenerateBlueprint(ctx, req)
	if err != nil {
		t.Fatalf("GenerateBlueprint: %v", err)
	}

	for _, tc := range []struct {
		name      string
		from, to  string
		wantKind  string
		wantField string
	}{
		{"key value eviction policy", "maxmemoryPolicy: allkeys-lru", "maxmemoryPolicy: noeviction", "keyvalue", "maxmemoryPolicy"},
		{"postgres plan", "plan: basic-1gb", "plan: basic-256mb", "postgres", "plan"},
		{"service health check", "healthCheckPath: /healthz", "healthCheckPath: /live", "service", "healthCheckPath"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(out.Manifest, tc.from) {
				t.Skipf("export does not contain %q; nothing to perturb", tc.from)
			}
			changed := strings.Replace(out.Manifest, tc.from, tc.to, 1)
			v, err := svc.ValidateBlueprint(ctx, "", changed)
			if err != nil || v.Plan == nil {
				t.Fatalf("ValidateBlueprint: %+v err=%v", v, err)
			}
			plan := *v.Plan
			var updates int
			for _, action := range plan.Actions {
				if action.Operation == BlueprintPlanNoop {
					continue
				}
				updates++
				paths := fieldPaths(action.ChangedFields)
				// Exactly the perturbed field — not the declared set, and
				// never the match key, which cannot have changed.
				if len(paths) != 1 || paths[0] != tc.wantField {
					t.Errorf("%s %q changedFields = %v, want exactly [%s]", action.Kind, action.Name, paths, tc.wantField)
				}
			}
			if updates == 0 {
				t.Error("a real change was not detected at all — the planner has gone blind")
			}
		})
	}
}

func fieldPaths(changes []BlueprintFieldChange) []string {
	out := make([]string, 0, len(changes))
	for _, c := range changes {
		out = append(out, c.Path)
	}
	return out
}
