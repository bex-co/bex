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
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// render.yaml's `disk` block used to be fail-closed (ADR018's stateless-first
// non-goal). ADR082 D7 turned it into a translated handler, so these pin the
// four semantics the registry now promises: the create default, what an
// omission means on sync, that a shrink is refused before any write, and that
// an ineligible service is refused rather than silently given a disk.

// TestBlueprintDiskSemantics is the fixture the capability registry names for
// the four #/definitions/disk entries.
func TestBlueprintDiskSemantics(t *testing.T) {
	t.Run("size defaults to Render's 10GB", func(t *testing.T) {
		view := blueprintDisk(&bexDisk{Name: "data", MountPath: "/var/data"})
		if view == nil || view.SizeGB != diskDefaultSizeGB {
			t.Fatalf("disk = %+v, want the 10GB Blueprint default", view)
		}
	})

	t.Run("an absent block declares no disk", func(t *testing.T) {
		if got := blueprintDisk(nil); got != nil {
			t.Fatalf("blueprintDisk(nil) = %+v, want nil", got)
		}
	})

	t.Run("an explicit size is carried through", func(t *testing.T) {
		view := blueprintDisk(&bexDisk{Name: " data ", MountPath: " /var/data ", SizeGB: 50})
		if view.SizeGB != 50 || view.MountPath != "/var/data" || view.Name != "data" {
			t.Fatalf("disk = %+v, want the declared size and trimmed paths", view)
		}
	})
}

// Omitting `disk` must PRESERVE the volume. Render's sync never deletes a
// resource a file stopped naming, and for a disk that deletion is irreversible.
func TestBlueprintOmittingDiskPreservesTheVolume(t *testing.T) {
	if got := blueprintServiceOmission("disk"); got != BlueprintPreserveOnOmission {
		t.Fatalf("omission policy = %v, want preserve — a dropped block must not destroy a volume", got)
	}

	existing := appv1alpha1.AppSpec{
		Image: "nginx:1", Tier: "starter",
		Disk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 25},
	}
	got := *existing.DeepCopy()
	// A sync that mentions other fields but not `disk`.
	changed, err := ApplyBlueprintServiceSpec(&got, appv1alpha1.AppSpec{HealthCheckPath: "/healthz"},
		map[string]BlueprintField{"healthCheckPath": {}})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !changed {
		t.Fatal("expected the health-check change to register")
	}
	if got.Disk == nil || got.Disk.SizeGB != 25 {
		t.Fatalf("disk = %+v, want the existing 25GB volume untouched", got.Disk)
	}
}

func TestBlueprintDiskGrowsButNeverShrinks(t *testing.T) {
	existing := appv1alpha1.AppSpec{
		Disk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 25},
	}

	grown := *existing.DeepCopy()
	if _, err := ApplyBlueprintServiceSpec(&grown,
		appv1alpha1.AppSpec{Disk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 100}},
		map[string]BlueprintField{"disk": {}}); err != nil {
		t.Fatalf("grow: %v", err)
	}
	if grown.Disk.SizeGB != 100 {
		t.Fatalf("size after grow = %d, want 100", grown.Disk.SizeGB)
	}

	shrunk := *existing.DeepCopy()
	_, err := ApplyBlueprintServiceSpec(&shrunk,
		appv1alpha1.AppSpec{Disk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 5}},
		map[string]BlueprintField{"disk": {}})
	if err == nil {
		t.Fatal("a Blueprint shrink was accepted; it must fail before any write")
	}
	var conflict *BlueprintFieldConflictError
	if !errors.As(err, &conflict) || conflict.Path != "disk.sizeGB" {
		t.Fatalf("error = %v, want a disk.sizeGB conflict with a source path", err)
	}
	if shrunk.Disk.SizeGB != 25 {
		t.Fatalf("a refused shrink still changed the spec to %dGB", shrunk.Disk.SizeGB)
	}
}

// A Blueprint declares a disk inline with its service, so the eligibility rules
// have to be enforced on the way in — there is no later attach to refuse.
func TestBlueprintDiskRefusesIneligibleServices(t *testing.T) {
	disk := &ServiceDiskView{Name: "data", MountPath: "/var/data", SizeGB: 10}
	for name, tc := range map[string]struct {
		svcType, tier string
		replicas      int32
		wantMsg       string
	}{
		"cron job":      {appv1alpha1.TypeCronJob, "starter", 1, "cannot have a disk"},
		"static site":   {appv1alpha1.TypeStaticSite, "starter", 1, "cannot have a disk"},
		"free tier":     {appv1alpha1.TypeWebService, "free", 1, "paid instance type"},
		"two instances": {appv1alpha1.TypeWebService, "starter", 2, "more than one instance"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := validateCreateDisk(tc.svcType, tc.tier, tc.replicas, disk)
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("validateCreateDisk = %v, want bad request", err)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("error %q does not explain the refusal (%q)", err, tc.wantMsg)
			}
		})
	}

	// The eligible shape is accepted and defaults applied.
	spec, err := validateCreateDisk(appv1alpha1.TypeWebService, "starter", 1,
		&ServiceDiskView{Name: "data", MountPath: "/var/data"})
	if err != nil {
		t.Fatalf("eligible service refused: %v", err)
	}
	if spec == nil || spec.SizeGB != diskDefaultSizeGB {
		t.Fatalf("spec = %+v, want the 10GB default", spec)
	}
}

// templateSmallDisk is shaped like Render's own Prometheus and Tailscale
// templates (render-examples): a paid Docker private service with a 1 GB disk.
func templateSmallDisk(sizeGB int) string {
	return fmt.Sprintf(`services:
  - type: pserv
    name: prometheus
    runtime: docker
    repo: https://github.com/render-examples/prometheus
    plan: starter
    disk:
      name: prometheus-data
      mountPath: /var/lib/prometheus
      sizeGB: %d
`, sizeGB)
}

// w8/215: 11 of Render's official templates declare a 1 or 5 GB disk, and
// every Blueprint stage refused them under w1/078's 10 GB floor. Validate,
// plan and apply now take the file unchanged, price and record the size it
// asks for, and keep the disk grow-only across later syncs.
func TestBlueprintAcceptsRenderTemplateOneGBDisk(t *testing.T) {
	manifest := templateSmallDisk(1)

	v, err := workspaceValidator(t).ValidateBlueprint(ownershipCtx(), connOwner, manifest, "")
	if err != nil || !v.Valid {
		t.Fatalf("validate = %+v, %v; want valid", v.Errors, err)
	}
	if v.Plan == nil || len(v.Plan.Actions) != 1 || v.Plan.Actions[0].Operation != BlueprintPlanCreate {
		t.Fatalf("plan = %+v, want one create", v.Plan)
	}
	if v.EstimatedPricing == nil || len(v.EstimatedPricing.Lines) != 1 || v.EstimatedPricing.Lines[0].StorageGB != 1 {
		t.Fatalf("estimated pricing = %+v, want the disk priced at the 1 GB it asks for", v.EstimatedPricing)
	}

	rec := &recordingStore{}
	svc, ctx := ownSourceService(newFakeBlueprintStore(), rec)
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	disk := func() *appv1alpha1.DiskSpec {
		t.Helper()
		var apps appv1alpha1.AppList
		if err := svc.Client.List(ctx, &apps); err != nil || len(apps.Items) != 1 {
			t.Fatalf("apps = %d, %v", len(apps.Items), err)
		}
		return apps.Items[0].Spec.Disk
	}
	if got := disk(); got == nil || got.SizeGB != 1 || got.MountPath != "/var/lib/prometheus" {
		t.Fatalf("spec.disk = %+v, want the template's 1 GB disk", got)
	}
	if len(rec.disks) != 1 {
		t.Fatalf("disk rows = %d, want the one billing record", len(rec.disks))
	}
	for _, row := range rec.disks {
		if row.SizeGB != 1 {
			t.Fatalf("billed disk row = %d GB, want 1", row.SizeGB)
		}
	}

	// A later sync grows 1 → 5 (still inside the 10 GB volume); a sync back to
	// 1 is a shrink and fails before any write.
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: templateSmallDisk(5)}); err != nil {
		t.Fatalf("grow sync: %v", err)
	}
	if got := disk(); got.SizeGB != 5 {
		t.Fatalf("spec.disk.sizeGB after grow = %d, want 5", got.SizeGB)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err == nil {
		t.Fatal("a sync shrinking 5 → 1 GB was accepted")
	}
	if got := disk(); got.SizeGB != 5 {
		t.Fatalf("a refused shrink changed spec.disk.sizeGB to %d", got.SizeGB)
	}
}

// The registry is the contract the compiler enforces; a stale "unsupported"
// entry would fail-close a field the compiler now handles.
func TestBlueprintDiskCapabilitiesAreTranslated(t *testing.T) {
	registry, err := RenderBlueprintCapabilityRegistry()
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	for _, pointer := range []string{
		"#/definitions/disk/properties/name",
		"#/definitions/disk/properties/mountPath",
		"#/definitions/disk/properties/sizeGB",
		"#/definitions/serverService/properties/disk",
	} {
		entry, ok := registry.Fields[pointer]
		if !ok {
			t.Fatalf("%s missing from the capability registry", pointer)
		}
		if entry.State != "translated" {
			t.Errorf("%s state = %q, want translated (ADR082 D7 reversed the non-goal)", pointer, entry.State)
		}
		// The reason may narrate the reversal; it must not still CLAIM the
		// field is unsupported because disks are a non-goal.
		if strings.Contains(strings.ToLower(entry.Reason), "are a deliberate non-goal") {
			t.Errorf("%s still cites the reversed non-goal as its reason: %s", pointer, entry.Reason)
		}
	}
}
