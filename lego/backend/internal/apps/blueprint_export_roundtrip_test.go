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
	"reflect"
	"strconv"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestGenerateBlueprintRoundTripsRealCreates (w5/m117): Generate Blueprint
// exports what a real create stored. Each create — the create probes and every
// type × runtime × builder combination create accepts — runs through Create,
// GenerateBlueprint, the parse a Blueprint sync applies, and specFromCreate:
// the spec that apply would create must equal the stored one field for field,
// except the fields blueprintNotExported names with the reason. The earlier
// round-trip used hand-built Apps with explicit runtimes, which is how a
// Dockerfile build with no spec.runtime failed its export (w4/193).
func TestGenerateBlueprintRoundTripsRealCreates(t *testing.T) {
	allowHighAvailabilityOnEveryPlan(t)
	ctx := context.Background()
	differs := map[string]bool{}
	for _, req := range roundTripCreates(t) {
		svc, cl := newService(nil)
		if _, err := svc.Create(ctx, req); err != nil {
			t.Errorf("%s: create: %v", req.Name, err)
			continue
		}
		stored := getApp(t, cl, req.Name).Spec
		out, err := svc.GenerateBlueprint(ctx, GenerateBlueprintRequest{ServiceIDs: []string{req.Name}})
		if err != nil {
			t.Errorf("%s: GenerateBlueprint: %v", req.Name, err)
			continue
		}
		stack := parseBlueprintStackForTest(t, out.Manifest) // GenerateBlueprint's self-check already parsed it
		parsed, err := specFromCreate(stack.services[0].req)
		if err != nil {
			t.Errorf("%s: the exported service does not create: %v\n%s", req.Name, err, out.Manifest)
			continue
		}
		want, got := exportSpelling(stored), exportSpelling(parsed)
		for _, field := range changedFields(want, got) {
			if gap, ok := blueprintNotExported[field]; ok && gap.applies(stored) {
				differs[field] = true
				continue
			}
			t.Errorf("%s: spec.%s was stored as %s, but its export creates %s\n%s", req.Name, field,
				wireJSON(reflect.ValueOf(want).FieldByName(field).Interface()),
				wireJSON(reflect.ValueOf(got).FieldByName(field).Interface()), out.Manifest)
		}

		// The export also describes its source: syncing it back changes nothing.
		v, err := svc.ValidateBlueprint(ctx, "", out.Manifest, "")
		if err != nil || !v.Valid || v.Plan == nil {
			t.Errorf("%s: the export does not validate: %+v err=%v\n%s", req.Name, v, err, out.Manifest)
			continue
		}
		for _, action := range v.Plan.Actions {
			if action.Operation == BlueprintPlanNoop {
				continue
			}
			for _, change := range action.ChangedFields {
				if !gapReplans(stored, change.Path) {
					t.Errorf("%s: the export re-plans as %q, changing %v\n%s", req.Name, action.Operation, fieldPaths(action.ChangedFields), out.Manifest)
					break
				}
			}
		}
	}
	for field, gap := range blueprintNotExported {
		if !differs[field] {
			t.Errorf("blueprintNotExported lists spec.%s (%s), but every export now reproduces it: remove it", field, gap.reason)
		}
	}
}

// blueprintNotExported names the stored fields an export cannot reproduce,
// where, and why.
var blueprintNotExported = map[string]exportGap{
	"Port": {
		reason:  "render.yaml has no port and reserves PORT: a Blueprint create listens on the default port, and the port verb sets it",
		applies: func(appv1alpha1.AppSpec) bool { return true },
	},
	"NotifyOnFail": {
		reason:  "not a Blueprint field: a sync keeps the service's own setting",
		applies: func(appv1alpha1.AppSpec) bool { return true },
	},
	"Replicas": {
		reason: "an autoscaled service exports its scaling block; render.yaml ignores numInstances beside one, so the manual count kept for a later manual mode cannot travel",
		applies: func(spec appv1alpha1.AppSpec) bool {
			return spec.Autoscaling != nil && spec.Autoscaling.Enabled
		},
	},
	"Runtime": {
		reason: "render.yaml spells every static site runtime: static, so a static site's native toolchain is the build plane's choice again",
		applies: func(spec appv1alpha1.AppSpec) bool {
			return spec.Type == appv1alpha1.TypeStaticSite && blueprintNativeRuntime(spec.Runtime)
		},
		replans: "runtime",
	},
	"BuildCommand": {
		reason: "only a native build runs a build command, and render.yaml refuses one beside the runtime the export writes for any other",
		applies: func(spec appv1alpha1.AppSpec) bool {
			return !strings.HasPrefix(buildStrategy(spec), buildNative)
		},
	},
	"StartCommand": {
		reason: "a static site serves files and never runs a start command, and render.yaml has none for it",
		applies: func(spec appv1alpha1.AppSpec) bool {
			return spec.Type == appv1alpha1.TypeStaticSite
		},
	},
}

type exportGap struct {
	reason  string
	applies func(stored appv1alpha1.AppSpec) bool
	// replans is the Blueprint field a sync of the export changes because of
	// the gap, if any.
	replans string
}

// gapReplans reports whether a gap of the stored spec explains a re-plan's
// change to path.
func gapReplans(stored appv1alpha1.AppSpec, path string) bool {
	for _, gap := range blueprintNotExported {
		if gap.replans == path && gap.applies(stored) {
			return true
		}
	}
	return false
}

// exportSpelling folds the spellings an export rewrites by design onto one, so
// the comparison sees only what a re-created service would do differently:
//   - a repo build with no runtime builds its Dockerfile, which render.yaml
//     spells runtime docker (w4/193), and which creates as builder dockerfile;
//   - a prebuilt image builds nothing, whatever builder it was created with;
//   - a static site's auto builder is its default, and its docker runtime a
//     Dockerfile build;
//   - a cron runs spec.command, else spec.startCommand, and render.yaml has
//     one field for it (a native cron's start command beside a command never
//     runs).
func exportSpelling(spec appv1alpha1.AppSpec) appv1alpha1.AppSpec {
	switch {
	case spec.Image != "" && spec.Repo == "":
		spec.Runtime, spec.Builder = "image", "auto"
	case spec.Type == appv1alpha1.TypeStaticSite:
		if strings.EqualFold(spec.Runtime, "docker") {
			spec.Runtime, spec.Builder = "", "dockerfile"
		}
		if spec.Builder == "auto" {
			spec.Builder = ""
		}
	case spec.Runtime == "" && effectiveRuntime(spec, effectiveType(spec.Type)) == "docker":
		spec.Runtime, spec.Builder = "docker", "dockerfile"
	}
	if spec.Type == appv1alpha1.TypeCronJob && spec.Command != "" {
		spec.StartCommand, spec.Command = spec.Command, ""
	}
	return spec
}

// roundTripCreates is the create probes, minus what a storeless create cannot
// take, plus every type × runtime × builder combination create accepts.
func roundTripCreates(t *testing.T) []CreateRequest {
	t.Helper()
	var out []CreateRequest
	for _, req := range createProbeRequests() {
		// Consumed by Create rather than stored on the spec, or needing the
		// store and secret seams this test runs without.
		req.OwnerID, req.EnvironmentID, req.EnvironmentSpecified = "", "", false
		req.SecretFiles, req.InitialDeployHook, req.DryRun = nil, "", false
		req.RegistryCredentialID = nil
		out = append(out, req)
	}
	// Values the probes leave at their defaults.
	off := false
	out = append(out,
		CreateRequest{Name: "rt-hidden", Image: "nginx:1.27", Hosts: []string{"hidden.example.com"}, SubdomainPolicy: appv1alpha1.SubdomainPolicyDisabled},
		CreateRequest{Name: "rt-manual", Repo: "https://github.com/acme/app", AutoDeploy: &off},
		// Commands a build never runs, which create accepts (w5/090).
		CreateRequest{Name: "rt-docker-build-command", Repo: "https://github.com/acme/app", Runtime: "docker", BuildCommand: "make"},
		CreateRequest{Name: "rt-buildpack-commands", Repo: "https://github.com/acme/app", Builder: "buildpack", BuildCommand: "make", StartCommand: "./serve"},
		// An image's command round-trips (w5/080).
		CreateRequest{Name: "rt-image-command", Image: "nginx:1.27", StartCommand: "nginx -g 'daemon off;'"},
	)
	types := []string{appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService, appv1alpha1.TypeBackgroundWorker, appv1alpha1.TypeCronJob, appv1alpha1.TypeStaticSite}
	sources := []struct {
		name string
		set  func(*CreateRequest)
	}{
		{"repo", func(r *CreateRequest) { r.Repo = "https://github.com/acme/app" }},
		{"docker", func(r *CreateRequest) { r.Repo, r.Runtime = "https://github.com/acme/app", "docker" }},
		{"native", func(r *CreateRequest) {
			r.Repo, r.Runtime, r.BuildCommand, r.StartCommand = "https://github.com/acme/app", "node", "npm ci", "npm start"
		}},
		{"image", func(r *CreateRequest) { r.Image, r.Runtime = "nginx:1.27", "image" }},
		{"bare-image", func(r *CreateRequest) { r.Image = "nginx:1.27" }},
	}
	n := 0
	for _, typ := range types {
		for _, source := range sources {
			for _, builder := range []string{"", "auto", "buildpack", "dockerfile"} {
				n++
				req := CreateRequest{Name: "rt-" + strconv.Itoa(n), Type: typ, Builder: builder}
				source.set(&req)
				switch typ {
				case appv1alpha1.TypeCronJob:
					req.Schedule, req.Command = "*/5 * * * *", "bin/report"
				case appv1alpha1.TypeStaticSite:
					req.PublishPath = "dist"
				}
				if _, err := specFromCreate(req); err != nil {
					continue // a combination create refuses has nothing to export
				}
				out = append(out, req)
			}
		}
	}
	if len(out) < 30 {
		t.Fatalf("only %d round-trip creates: the matrix lost its coverage", len(out))
	}
	return out
}

// TestBlueprintSyncKeepsAnOmittedBuilder pins x-bex.builder's preserve-on-
// omission policy beside w5/m117's build-strategy comparison: a hand-written
// render.yaml that never mentions the builder keeps it. A Dockerfile-built
// static site stays one rather than turning into a no-build publish, and a
// buildpack service that declares runtime docker changes its runtime only.
func TestBlueprintSyncKeepsAnOmittedBuilder(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		create      CreateRequest
		manifest    string
		wantRuntime string // "" when the sync changes nothing
	}{
		{
			create:   CreateRequest{Name: "site", Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/acme/app", Builder: buildDockerfile, PublishPath: "dist"},
			manifest: "services:\n  - type: web\n    name: site\n    runtime: static\n    repo: https://github.com/acme/app\n    staticPublishPath: dist\n",
		},
		{
			create:      CreateRequest{Name: "api", Repo: "https://github.com/acme/app", Builder: buildBuildpack},
			manifest:    "services:\n  - type: web\n    name: api\n    runtime: docker\n    repo: https://github.com/acme/app\n",
			wantRuntime: "docker",
		},
	} {
		t.Run(tc.create.Name, func(t *testing.T) {
			svc, cl := newService(nil)
			if _, err := svc.Create(ctx, tc.create); err != nil {
				t.Fatal(err)
			}
			stored := getApp(t, cl, tc.create.Name).Spec
			declared := parseBlueprintStackForTest(t, tc.manifest).services[0]
			want, err := specFromCreate(declared.req)
			if err != nil {
				t.Fatal(err)
			}
			synced := *stored.DeepCopy()
			changed, err := ApplyBlueprintServiceSpec(&synced, want, declared.fields)
			if err != nil {
				t.Fatal(err)
			}
			if synced.Builder != stored.Builder {
				t.Errorf("the sync moved the omitted builder %q to %q", stored.Builder, synced.Builder)
			}
			if changed != (tc.wantRuntime != "") || (changed && synced.Runtime != tc.wantRuntime) {
				t.Errorf("sync changed = %v, runtime %q; want changed %v, runtime %q", changed, synced.Runtime, tc.wantRuntime != "", tc.wantRuntime)
			}
		})
	}
}
