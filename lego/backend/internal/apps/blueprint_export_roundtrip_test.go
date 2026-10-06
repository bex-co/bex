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

	"sigs.k8s.io/yaml"

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
		svc.RegistryCreds = probeRegistryCreds()
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
		if err := svc.resolveBlueprintRegistryCredentials(ctx, &stack); err != nil {
			t.Errorf("%s: the export's registry credential does not resolve: %v\n%s", req.Name, err, out.Manifest)
			continue
		}
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
	"RegistryCredentialID": {
		reason: "render.yaml has no spelling for binding no credential at all: the export reads as unbound, so a pull falls back to a credential matching the image host",
		applies: func(spec appv1alpha1.AppSpec) bool {
			return spec.RegistryCredentialID != nil && *spec.RegistryCredentialID == ""
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

// probeRegistryCreds is a workspace whose credential "rc-probe" is named
// "probe-creds": an export names it, and the parse resolves the name back.
func probeRegistryCreds() *fakePullSecrets {
	return &fakePullSecrets{
		credNames:           map[string]string{"rc-probe": "probe-creds"},
		credentialIDsByName: map[string]string{"probe-creds": "rc-probe"},
	}
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
		req.RegistryCredentialID = nil // the probe's native runtime takes none; see rt-private-*
		out = append(out, req)
	}
	// Values the probes leave at their defaults.
	off := false
	out = append(out,
		CreateRequest{Name: "rt-hidden", Image: "nginx:1.27", Hosts: []string{"hidden.example.com"}, SubdomainPolicy: appv1alpha1.SubdomainPolicyDisabled},
		CreateRequest{Name: "rt-manual", Repo: "https://github.com/acme/app", AutoDeploy: &off},
		// Commands a build never runs, which create and update both still
		// accept: w5/090 refused only the settings update already refused.
		CreateRequest{Name: "rt-docker-build-command", Repo: "https://github.com/acme/app", Runtime: "docker", BuildCommand: "make"},
		CreateRequest{Name: "rt-buildpack-commands", Repo: "https://github.com/acme/app", Builder: "buildpack", BuildCommand: "make", StartCommand: "./serve"},
		// An image's command round-trips (w5/080).
		CreateRequest{Name: "rt-image-command", Image: "nginx:1.27", StartCommand: "nginx -g 'daemon off;'"},
		// A bound registry credential exports by name, on image.creds for an
		// image and on registryCredential for a build (w5/089); binding none
		// at all cannot be spelled.
		CreateRequest{Name: "rt-private-image", Image: "ghcr.io/acme/private:1", RegistryCredentialID: strp("rc-probe")},
		CreateRequest{Name: "rt-private-build", Repo: "https://github.com/acme/app", Runtime: "docker", RegistryCredentialID: strp("rc-probe")},
		CreateRequest{Name: "rt-no-credential", Image: "ghcr.io/acme/public:1", RegistryCredentialID: strp("")},
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
					// A static site serves files and runs no start command,
					// which create refuses (w5/090).
					req.PublishPath, req.StartCommand = "dist", ""
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

// TestGenerateBlueprintNamesTheBoundRegistryCredential (w5/089): a bound
// credential exports by its workspace name where Render spells it — on
// image.creds for a prebuilt image, on registryCredential for a build. One the
// file could not sync exports nothing: a credential since deleted, a name
// another credential shares, or a binding the build no longer reads.
func TestGenerateBlueprintNamesTheBoundRegistryCredential(t *testing.T) {
	ctx := context.Background()
	image := CreateRequest{Name: "private-image", Image: "ghcr.io/acme/private:1", RegistryCredentialID: strp("rc-probe")}
	build := CreateRequest{Name: "private-build", Repo: "https://github.com/acme/app", Runtime: "docker", RegistryCredentialID: strp("rc-probe")}
	imageCreds := func(service map[string]any) any { image, _ := service["image"].(map[string]any); return image["creds"] }
	buildCreds := func(service map[string]any) any { return service["registryCredential"] }
	named := map[string]any{"fromRegistryCreds": map[string]any{"name": "probe-creds"}}
	for _, tc := range []struct {
		name   string
		create CreateRequest
		creds  func(service map[string]any) any
		after  func(*fakePullSecrets, *appv1alpha1.App) // what changed since the create
		want   any
	}{
		{"a prebuilt image", image, imageCreds, nil, named},
		{"a Dockerfile build", build, buildCreds, nil, named},
		{"a deleted credential", image, imageCreds, func(f *fakePullSecrets, _ *appv1alpha1.App) { f.credNames = nil }, nil},
		{"a shared name", image, imageCreds, func(f *fakePullSecrets, _ *appv1alpha1.App) {
			f.ambiguousNames = map[string]bool{"probe-creds": true}
		}, nil},
		{"a build moved to a native runtime", build, buildCreds, func(_ *fakePullSecrets, a *appv1alpha1.App) {
			a.Spec.Runtime, a.Spec.BuildCommand, a.Spec.StartCommand = "node", "npm ci", "npm start"
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService(nil)
			creds := probeRegistryCreds()
			svc.RegistryCreds = creds
			if _, err := svc.Create(ctx, tc.create); err != nil {
				t.Fatalf("create: %v", err)
			}
			if tc.after != nil {
				app := getApp(t, cl, tc.create.Name)
				tc.after(creds, app)
				if err := cl.Update(ctx, app); err != nil {
					t.Fatal(err)
				}
			}
			out, err := svc.GenerateBlueprint(ctx, GenerateBlueprintRequest{ServiceIDs: []string{tc.create.Name}})
			if err != nil {
				t.Fatalf("GenerateBlueprint: %v", err)
			}
			var doc struct {
				Services []map[string]any `json:"services"`
			}
			if err := yaml.Unmarshal([]byte(out.Manifest), &doc); err != nil || len(doc.Services) != 1 {
				t.Fatalf("export does not hold one service: %v\n%s", err, out.Manifest)
			}
			if got := tc.creds(doc.Services[0]); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("exported credential = %v, want %v\n%s", got, tc.want, out.Manifest)
			}
		})
	}
}
