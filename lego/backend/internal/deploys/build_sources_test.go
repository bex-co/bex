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

package deploys

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func sampleBuildSource(id, repo string) *appv1alpha1.App {
	a := sampleApp(id, id)
	a.Spec.Type = appv1alpha1.TypeBuildSource
	if repo != "" {
		a.Spec.Image = ""
		a.Spec.Repo = repo
		a.Spec.Runtime = "node"
	}
	return a
}

func TestTriggerBuildDeploysTheCarrierAndReportsABuild(t *testing.T) {
	ds := newFakeStore()
	ds.mintIDs = true
	svc, _ := newService(ds, sampleBuildSource("bds-git", "https://github.com/acme/web"))
	ctx := context.Background()

	b, err := svc.TriggerBuild(ctx, "bds-git")
	if err != nil {
		t.Fatalf("TriggerBuild: %v", err)
	}
	rows := ds.byApp["bds-git"]
	if len(rows) != 1 {
		t.Fatalf("carrier deploy rows = %d, want 1", len(rows))
	}
	if b.ID != BuildID(rows[0].ID) || b.BuildSourceID != "bds-git" || b.BuildRun == nil || b.BuildRun.ID != rows[0].ID || b.BuildRun.Status != "created" || b.BuildRun.Runtime != "node" || b.Image != nil {
		t.Fatalf("build = %+v (run %+v), want the derived id of %s, created, node", b, b.BuildRun, rows[0].ID)
	}

	list, err := svc.ListBuilds(ctx, "bds-git", "", core.DefaultPageLimit)
	if err != nil || len(list) != 1 || list[0].ID != b.ID {
		t.Fatalf("ListBuilds = %+v, %v; want [%s]", list, err, b.ID)
	}
	got, err := svc.GetBuild(ctx, "bds-git", b.ID)
	if err != nil || got.ID != b.ID {
		t.Fatalf("GetBuild = %+v, %v", got, err)
	}
}

func TestBuildRoutesRefuseServicesAndImageSources(t *testing.T) {
	ds := newFakeStore()
	ds.mintIDs = true
	svc, _ := newService(ds, sampleApp("web", "srv-1"), sampleBuildSource("bds-img", ""))
	ctx := context.Background()

	if _, err := svc.TriggerBuild(ctx, "web"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("TriggerBuild on a service: want ErrNotFound, got %v", err)
	}
	if _, err := svc.ListBuilds(ctx, "web", "", 20); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("ListBuilds on a service: want ErrNotFound, got %v", err)
	}
	if _, err := svc.TriggerBuild(ctx, "bds-img"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("TriggerBuild on an image source: want ErrBadRequest, got %v", err)
	}
	if len(ds.byApp["srv-1"])+len(ds.byApp["bds-img"]) != 0 {
		t.Errorf("a refused build wrote deploy rows: %+v", ds.byApp)
	}
}

func TestGetBuildRefusesIdsThatAreNotItsBuilds(t *testing.T) {
	ds := newFakeStore()
	ds.mintIDs = true
	svc, _ := newService(ds, sampleBuildSource("bds-git", "https://github.com/acme/web"), sampleBuildSource("bds-two", "https://github.com/acme/api"))
	ctx := context.Background()
	other, err := svc.TriggerBuild(ctx, "bds-two")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{other.ID, other.BuildRun.ID, "bld-nope", ""} {
		if _, err := svc.GetBuild(ctx, "bds-git", id); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("GetBuild(bds-git, %q): want ErrNotFound, got %v", id, err)
		}
	}
}

func TestImageRefIsTheTagOrDigest(t *testing.T) {
	for image, want := range map[string]string{
		"nginx:1.27":                      "1.27",
		"registry.local:5000/acme/web:v2": "v2",
		"registry.local:5000/acme/web":    "",
		"ghcr.io/acme/web@sha256:abc":     "sha256:abc",
		"ghcr.io/acme/web:v1@sha256:abc":  "sha256:abc",
	} {
		if got := imageRef(image); got != want {
			t.Errorf("imageRef(%q) = %q, want %q", image, got, want)
		}
	}
}

// The build verbs authorize through an unexported helper, so the audit
// record names the verb itself, never the helper.
func TestBuildVerbsAuditUnderTheirOwnNames(t *testing.T) {
	ds := newFakeStore()
	ds.mintIDs = true
	svc, _ := newService(ds, sampleBuildSource("bds-git", "https://github.com/acme/web"))
	sink := &deployHookAuditSink{}
	svc.Audit = sink
	if _, err := svc.TriggerBuild(context.Background(), "bds-git"); err != nil {
		t.Fatal(err)
	}
	if len(sink.events) == 0 || sink.events[0].Verb != "deploys.TriggerBuild" {
		t.Fatalf("audit events = %+v, want the first named deploys.TriggerBuild", sink.events)
	}
}
