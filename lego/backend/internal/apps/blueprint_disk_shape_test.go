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
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/279: a Blueprint that gave one service a disk and a scaling block was
// answered with the App resource's admission rule, `spec: Invalid value:
// "object": a service with spec.disk cannot use autoscaling`. The create-time
// disk check knew the type, the plan and the instance count, but not that the
// same entry declared scaling.

const (
	diskRuleWeb = "services:\n  - type: web\n    name: qa-bp-web\n    runtime: node\n    plan: starter\n" +
		"    repo: https://github.com/acme/api\n    buildCommand: npm install\n    startCommand: npm start\n"
	diskRuleDisk    = "    disk:\n      name: data\n      mountPath: /var/data\n      sizeGB: 1\n"
	diskRuleScaling = "    scaling:\n      minInstances: 1\n      maxInstances: 2\n      targetCPUPercent: 60\n"
)

// clusterWording are the marks of an admission answer: the API server's own
// framing, and a field named as the App resource names it.
var clusterWording = []string{"Invalid value:", "spec."}

func TestBlueprintDiskWithScalingIsRefusedInProductWords(t *testing.T) {
	svc, _ := newService(nil)
	ctx := context.Background()
	manifest := diskRuleWeb + diskRuleScaling + diskRuleDisk
	const want = `service "qa-bp-web": a service with a disk cannot use autoscaling`

	validation, err := svc.ValidateBlueprint(ctx, "", manifest, "")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if validation.Valid || len(validation.Errors) != 1 {
		t.Fatalf("validation = valid %v, errors %+v, want the one refusal", validation.Valid, validation.Errors)
	}
	got := validation.Errors[0]
	if got.Error != want || got.Path == nil || *got.Path != "services[0].scaling" {
		t.Fatalf("error = %q at %v, want %q at services[0].scaling", got.Error, got.Path, want)
	}

	_, err = svc.DeployStack(ctx, DeployRequest{Manifest: manifest})
	if err == nil || !strings.Contains(err.Error(), `service "qa-bp-web"`) || !strings.Contains(err.Error(), "a service with a disk cannot use autoscaling") {
		t.Fatalf("sync = %v, want the same refusal", err)
	}
	for _, mark := range clusterWording {
		if strings.Contains(err.Error(), mark) {
			t.Fatalf("sync error %q carries cluster wording %q", err, mark)
		}
	}

	// Each half alone is a valid service.
	for name, half := range map[string]string{"disk": diskRuleDisk, "scaling": diskRuleScaling} {
		if v, err := svc.ValidateBlueprint(ctx, "", diskRuleWeb+half, ""); err != nil || !v.Valid {
			t.Fatalf("a service with only %s = valid %v, errors %+v, err %v", name, v.Valid, v.Errors, err)
		}
	}
}

// A sync judges the service it leaves, not only the fields one file declares
// together: the disk, or the scaling range, may have been set outside the
// Blueprint.
func TestBlueprintSyncKeepsTheDiskRulesOnAnExistingService(t *testing.T) {
	disk := &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10}
	scaling := &appv1alpha1.AutoscalingSpec{Enabled: true, MinReplicas: 1, MaxReplicas: 2}
	declares := func(names ...string) map[string]BlueprintField {
		fields := map[string]BlueprintField{}
		for _, name := range names {
			fields[name] = BlueprintField{}
		}
		return fields
	}
	for name, tc := range map[string]struct {
		existing, want appv1alpha1.AppSpec
		fields         map[string]BlueprintField
		path, message  string // empty: the sync is accepted
	}{
		"scaling declared for a service that has a disk": {
			existing: appv1alpha1.AppSpec{Tier: "starter", Replicas: 1, Disk: disk},
			want:     appv1alpha1.AppSpec{Autoscaling: scaling},
			fields:   declares("scaling"),
			path:     "scaling", message: "a service with a disk cannot use autoscaling",
		},
		// The scaling range is the live service's, so the file is told how
		// to turn it off.
		"a disk declared for a service that autoscales": {
			existing: appv1alpha1.AppSpec{Tier: "starter", Replicas: 1, Autoscaling: scaling},
			want:     appv1alpha1.AppSpec{Disk: disk},
			fields:   declares("disk"),
			path:     "disk", message: "a service with a disk cannot use autoscaling; declare numInstances in the file to turn autoscaling off",
		},
		"a disk and one instance declared together for a service that autoscales": {
			existing: appv1alpha1.AppSpec{Tier: "starter", Replicas: 1, Autoscaling: scaling},
			want:     appv1alpha1.AppSpec{Replicas: 1, Disk: disk},
			fields:   declares("disk", "numInstances"),
		},
		"a disk declared for a service on two instances": {
			existing: appv1alpha1.AppSpec{Tier: "starter", Replicas: 2},
			want:     appv1alpha1.AppSpec{Disk: disk},
			fields:   declares("disk"),
			path:     "disk", message: "a service with a disk cannot run more than one instance (currently 2)",
		},
		"two instances declared for a service that has a disk": {
			existing: appv1alpha1.AppSpec{Tier: "starter", Replicas: 1, Disk: disk},
			want:     appv1alpha1.AppSpec{Replicas: 2},
			fields:   declares("numInstances"),
			path:     "numInstances", message: "a service with a disk cannot run more than one instance (currently 2)",
		},
		"the free plan declared for a service that has a disk": {
			existing: appv1alpha1.AppSpec{Tier: "starter", Replicas: 1, Disk: disk},
			want:     appv1alpha1.AppSpec{Tier: "free"},
			fields:   declares("plan"),
			path:     "plan", message: "disks require a paid instance type",
		},
		"a disk and one instance declared together for a service on two": {
			existing: appv1alpha1.AppSpec{Tier: "starter", Replicas: 2},
			want:     appv1alpha1.AppSpec{Replicas: 1, Disk: disk},
			fields:   declares("disk", "numInstances"),
		},
		// Admission accepts a disk on a legacy App with no recorded plan. A
		// sync that touches something else is not the one to refuse it.
		"an unrelated field on a legacy service with a disk and no plan": {
			existing: appv1alpha1.AppSpec{Replicas: 1, Disk: disk},
			want:     appv1alpha1.AppSpec{HealthCheckPath: "/healthz"},
			fields:   declares("healthCheckPath"),
		},
		// Each rule is judged on its own: the plan rule the legacy service
		// already breaks does not excuse the autoscaling one.
		"scaling declared for a legacy service with a disk and no plan": {
			existing: appv1alpha1.AppSpec{Replicas: 1, Disk: disk},
			want:     appv1alpha1.AppSpec{Autoscaling: scaling},
			fields:   declares("scaling"),
			path:     "scaling", message: "a service with a disk cannot use autoscaling",
		},
	} {
		t.Run(name, func(t *testing.T) {
			dst := *tc.existing.DeepCopy()
			_, err := ApplyBlueprintServiceSpec(&dst, tc.want, tc.fields)
			if tc.message == "" {
				if err != nil {
					t.Fatalf("sync refused: %v", err)
				}
				// The plan probes one field at a time, and must not refuse a
				// combination the file never asks for.
				for field := range tc.fields {
					probe := *tc.existing.DeepCopy()
					if _, err := probeBlueprintServiceSpec(&probe, tc.want, tc.fields, field); err != nil {
						t.Fatalf("the %s probe refused: %v", field, err)
					}
				}
				return
			}
			var conflict *BlueprintFieldConflictError
			if !errors.As(err, &conflict) || conflict.Path != tc.path || conflict.Message != tc.message {
				t.Fatalf("sync = %v, want %s: %s", err, tc.path, tc.message)
			}
			if !reflect.DeepEqual(dst, tc.existing) {
				t.Fatalf("a refused sync still changed the spec: %+v", dst)
			}
		})
	}
}

// Through validate and sync, the refusal on an existing service is the file's
// fault, a 400 naming the service, never a 500 "internal error".
func TestBlueprintSyncOntoAServiceWithADiskAnswersInProductWords(t *testing.T) {
	ctx := context.Background()
	svc, cl := newService(&recordingStore{})
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: diskRuleWeb}); err != nil {
		t.Fatal(err)
	}
	existing := manage(getApp(t, cl, "qa-bp-web"), id.New(id.Service))
	existing.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 1}
	svc.Client = fakeClient(serializedBlueprintApp(t, existing))
	manifest := diskRuleWeb + diskRuleScaling
	const sentence = "a service with a disk cannot use autoscaling"

	validation, err := svc.ValidateBlueprint(ctx, "", manifest, "")
	if err != nil {
		t.Fatalf("validate answered %v (status %d), want the refusal as a finding", err, core.ErrorStatus(err))
	}
	if validation.Valid || len(validation.Errors) != 1 || !strings.Contains(validation.Errors[0].Error, sentence) {
		t.Fatalf("validation = valid %v, errors %+v, want the one refusal", validation.Valid, validation.Errors)
	}

	_, err = svc.DeployStack(ctx, DeployRequest{Manifest: manifest})
	if core.ErrorStatus(err) != http.StatusBadRequest || !strings.Contains(err.Error(), `service "qa-bp-web"`) || !strings.Contains(err.Error(), sentence) {
		t.Fatalf("sync = %v (status %d), want a 400 naming the service and the rule", err, core.ErrorStatus(err))
	}
	if got := getApp(t, svc.Client, "qa-bp-web"); got.Spec.Autoscaling != nil {
		t.Fatalf("a refused sync still set autoscaling: %+v", got.Spec.Autoscaling)
	}
}

// Every validation rule on the App resource, and what keeps a Blueprint from
// being answered with it. A rule a Blueprint entry can break carries a
// manifest that breaks it: the fake cluster here runs no admission rules, so a
// manifest the compiler lets through validates, and fails this test. A rule
// only a later sync can break names the check that stands in front of it.
//
// A new rule on the App fails this test until it is added here, which is the
// moment to give it a product sentence.
var appRuleBlueprintCover = map[string]struct{ manifest, refusal, elsewhere string }{
	"spec.type is immutable; delete and recreate the service to change type": {
		elsewhere: "updateStackService refuses a changed type first (TestBlueprintRejectsEveryExistingServiceTypeTransition)"},
	"spec.disk is supported only on web_service, private_service and background_worker": {
		manifest: "services:\n  - type: web\n    name: site\n    runtime: static\n    repo: https://github.com/acme/site\n" +
			"    staticPublishPath: public\n" + diskRuleDisk,
		refusal: "additional properties 'disk' not allowed"},
	"spec.disk requires a paid tier": {
		manifest: strings.Replace(diskRuleWeb, "plan: starter", "plan: free", 1) + diskRuleDisk,
		refusal:  "disks require a paid instance type"},
	"a service with spec.disk cannot run more than one instance": {
		manifest: diskRuleWeb + "    numInstances: 2\n" + diskRuleDisk,
		refusal:  "cannot run more than one instance"},
	"a service with spec.disk cannot use autoscaling": {
		manifest: diskRuleWeb + diskRuleScaling + diskRuleDisk,
		refusal:  "cannot use autoscaling"},
	"spec.disk.sizeGB can only grow; a disk is never shrunk": {
		elsewhere: "ApplyBlueprintServiceSpec refuses a shrink (TestBlueprintDiskGrowsButNeverShrinks)"},
	"a static_site builds from spec.repo; spec.image is not supported": {
		manifest: "services:\n  - type: web\n    name: site\n    runtime: static\n    image:\n      url: docker.io/library/nginx:1.27\n" +
			"    staticPublishPath: public\n",
		refusal: "image"},
	"a service linked to a build source takes its source from it; spec.repo and spec.image must be empty": {
		manifest: diskRuleBuildSource + "services:\n  - type: web\n    name: api\n    runtime: node\n" +
			"    repo: https://github.com/acme/other\n    buildSource: mono\n    startCommand: npm start\n",
		refusal: "at '/services/0/repo'"},
	"spec.buildSource is supported only on web_service, private_service, background_worker and cron_job": {
		manifest: diskRuleBuildSource + "services:\n  - type: web\n    name: site\n    runtime: static\n" +
			"    buildSource: mono\n    staticPublishPath: public\n",
		refusal: "buildSource"},
	"disk.mountPath must be an absolute path": {
		manifest: diskRuleWeb + strings.Replace(diskRuleDisk, "/var/data", "data", 1),
		refusal:  "mountPath"},
	"disk.mountPath must not be the root directory or end with a slash": {
		manifest: diskRuleWeb + strings.Replace(diskRuleDisk, "/var/data", "/var/data/", 1),
		refusal:  "mountPath"},
	"disk.mountPath is a reserved platform path; mount a subdirectory of it instead": {
		manifest: diskRuleWeb + strings.Replace(diskRuleDisk, "/var/data", "/etc/secrets", 1),
		refusal:  "mountPath"},
}

const diskRuleBuildSource = "buildSources:\n  - name: mono\n    git:\n      repo: https://github.com/acme/mono\n" +
	"      branch: main\n      runtime: node\n      buildCommand: npm ci\n"

func TestBlueprintIsNeverAnsweredWithAnAdmissionRule(t *testing.T) {
	source, err := os.ReadFile("../../../types/v1alpha1/app_types.go")
	if err != nil {
		t.Fatalf("reading the App types: %v", err)
	}
	rules := regexp.MustCompile(`(?m)^// \+kubebuilder:validation:XValidation:rule=.*,message="([^"]+)"$`).
		FindAllStringSubmatch(string(source), -1)
	if markers := strings.Count(string(source), "+kubebuilder:validation:XValidation"); len(rules) != markers {
		t.Fatalf("read %d of the %d App validation markers; one is written in a form this test does not parse", len(rules), markers)
	}
	declared := map[string]bool{}
	for _, rule := range rules {
		declared[rule[1]] = true
		if _, ok := appRuleBlueprintCover[rule[1]]; !ok {
			t.Errorf("App rule %q is not covered: say what keeps a Blueprint from being answered with it", rule[1])
		}
	}

	svc, _ := newService(nil)
	for rule, cover := range appRuleBlueprintCover {
		if !declared[rule] {
			t.Errorf("covered rule %q is no longer on the App; drop it here", rule)
		}
		if cover.manifest == "" {
			if cover.elsewhere == "" {
				t.Errorf("rule %q names neither a manifest nor the check in front of it", rule)
			}
			continue
		}
		validation, err := svc.ValidateBlueprint(context.Background(), "", cover.manifest, "")
		if err != nil {
			t.Errorf("rule %q: validate: %v", rule, err)
			continue
		}
		if validation.Valid || len(validation.Errors) == 0 {
			t.Errorf("rule %q: the Blueprint compiler accepted a manifest that breaks it, so admission would answer", rule)
		}
		// Refused for this rule, not for something else the manifest got wrong.
		if !slices.ContainsFunc(validation.Errors, func(e BlueprintValidationError) bool { return strings.Contains(e.Error, cover.refusal) }) {
			t.Errorf("rule %q: refused with %+v, none of which says %q", rule, validation.Errors, cover.refusal)
		}
		for _, e := range validation.Errors {
			for _, mark := range clusterWording {
				if strings.Contains(e.Error, mark) {
					t.Errorf("rule %q: the answer %q carries cluster wording %q", rule, e.Error, mark)
				}
			}
		}
	}
}
