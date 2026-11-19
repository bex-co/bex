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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestArtifactSourceChangedLinkLifecycle pins w4/m510 t001: linking a service
// to a build source, switching it to a second, and unlinking it back to its
// own source each record one apps.LinkBuildSource row with the right
// BuildSourceFrom/BuildSourceTo sides and no apps.SetSourceAndRegistryCredential
// row; a branch or repository change on an unlinked service still records
// SetSourceAndRegistryCredential; refused and no-op writes record nothing.
func TestArtifactSourceChangedLinkLifecycle(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	sink := &captureAuditSink{}
	svc.Audit = sink

	src1, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{
		OwnerID: "tea-a", Name: "Img1", Image: &BuildSourceImage{ImageURL: "nginx:1"},
	})
	if err != nil {
		t.Fatalf("create src1: %v", err)
	}
	src2, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{
		OwnerID: "tea-a", Name: "Img2", Image: &BuildSourceImage{ImageURL: "nginx:2"},
	})
	if err != nil {
		t.Fatalf("create src2: %v", err)
	}
	web, err := svc.Create(ctx, CreateRequest{
		OwnerID: "tea-a", Name: "web", Type: appv1alpha1.TypeWebService,
		Repo: "https://github.com/acme/web", Branch: "main", Runtime: "docker",
	})
	if err != nil {
		t.Fatalf("create web: %v", err)
	}

	assertRows := func(step string, wantFrom, wantTo string, wantLinkCount, wantSourceCount int) {
		t.Helper()
		links := allowedRows(sink, core.AuditVerbLinkBuildSource)
		sources := allowedRows(sink, core.AuditVerbSetSourceAndRegistryCredential)
		if len(links) != wantLinkCount {
			t.Fatalf("%s: LinkBuildSource rows = %+v, want %d", step, links, wantLinkCount)
		}
		if len(sources) != wantSourceCount {
			t.Fatalf("%s: SetSourceAndRegistryCredential rows = %+v, want %d", step, sources, wantSourceCount)
		}
		if wantLinkCount == 1 {
			if links[0].BuildSourceFrom != wantFrom || links[0].BuildSourceTo != wantTo {
				t.Errorf("%s: sides = %q -> %q, want %q -> %q", step, links[0].BuildSourceFrom, links[0].BuildSourceTo, wantFrom, wantTo)
			}
			if links[0].FeedOnly {
				t.Errorf("%s: LinkBuildSource row must not be feed_only", step)
			}
		}
	}

	// 1. New link: from="" -> to=src1.ID
	sink.events = nil
	if _, err := svc.LinkBuildSource(ctx, web.ID, src1.ID, nil, false); err != nil {
		t.Fatalf("link src1: %v", err)
	}
	assertRows("new link", "", src1.ID, 1, 0)

	// 2. No-op link to same build source: records nothing.
	sink.events = nil
	if _, err := svc.LinkBuildSource(ctx, web.ID, src1.ID, nil, false); err != nil {
		t.Fatalf("no-op link: %v", err)
	}
	assertRows("no-op link", "", "", 0, 0)

	// 3. Dry run to src2: records nothing.
	sink.events = nil
	if _, err := svc.LinkBuildSource(ctx, web.ID, src2.ID, nil, true); err != nil {
		t.Fatalf("dry-run relink: %v", err)
	}
	assertRows("dry-run relink", "", "", 0, 0)

	// 4. Refused link (nonexistent build source): records nothing.
	sink.events = nil
	if _, err := svc.LinkBuildSource(ctx, web.ID, "bds-missing", nil, false); err == nil {
		t.Fatal("refused link unexpectedly succeeded")
	}
	assertRows("refused link", "", "", 0, 0)

	// 5. Relink: from=src1.ID -> to=src2.ID
	sink.events = nil
	if _, err := svc.LinkBuildSource(ctx, web.ID, src2.ID, nil, false); err != nil {
		t.Fatalf("relink src2: %v", err)
	}
	assertRows("relink", src1.ID, src2.ID, 1, 0)

	// 6. Unlink by giving the service its own repo: from=src2.ID -> to=""
	sink.events = nil
	repo := "https://github.com/acme/web"
	if _, err := svc.SetSourceAndRegistryCredential(ctx, web.ID, sourcePatch{Repo: &repo}); err != nil {
		t.Fatalf("unlink to repo: %v", err)
	}
	assertRows("unlink", src2.ID, "", 1, 0)

	// 7. Repository / branch change on unlinked service: still source_changed, no LinkBuildSource.
	sink.events = nil
	branch := "release"
	if _, err := svc.SetSourceAndRegistryCredential(ctx, web.ID, sourcePatch{Branch: &branch}); err != nil {
		t.Fatalf("branch change: %v", err)
	}
	assertRows("branch change", "", "", 0, 1)

	// 8. CreateBuildSource with serviceIds records one link with from="" -> to=new.
	sink.events = nil
	src3, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{
		OwnerID: "tea-a", Name: "Img3", Image: &BuildSourceImage{ImageURL: "nginx:3"},
		ServiceIDs: []string{web.ID},
	})
	if err != nil {
		t.Fatalf("create src3 with serviceIds: %v", err)
	}
	assertRows("create with serviceIds", "", src3.ID, 1, 0)

	// 9. Refused unlink (invalid repo URL) records nothing.
	sink.events = nil
	badRepo := "not-a-repo"
	if _, err := svc.SetSourceAndRegistryCredential(ctx, web.ID, sourcePatch{Repo: &badRepo}); err == nil {
		t.Fatal("refused unlink unexpectedly succeeded")
	}
	assertRows("refused unlink", "", "", 0, 0)
}

// TestArtifactSourceChangedBlueprintRelinkAndUnlink pins Blueprint applies
// switching between two build sources and detaching back to an own image: each
// writes one apps.LinkBuildSource row with the exact sides and no source_changed.
func TestArtifactSourceChangedBlueprintRelinkAndUnlink(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	sink := &captureAuditSink{}
	svc.Audit = sink

	const twoSources = `buildSources:
  - name: bs-one
    image:
      url: nginx:1
  - name: bs-two
    image:
      url: nginx:2
services:
  - type: web
    name: api
    buildSource: bs-one
`
	created, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: twoSources})
	if err != nil {
		t.Fatalf("create with bs-one: %v", err)
	}
	var bs1ID, bs2ID string
	for _, bs := range created.BuildSources {
		switch bs.Name {
		case "bs-one":
			bs1ID = bs.ID
		case "bs-two":
			bs2ID = bs.ID
		}
	}
	if bs1ID == "" || bs2ID == "" {
		t.Fatalf("build sources = %+v, want bs-one and bs-two", created.BuildSources)
	}

	// Relink bs-one -> bs-two via Blueprint.
	sink.events = nil
	const relinked = `buildSources:
  - name: bs-one
    image:
      url: nginx:1
  - name: bs-two
    image:
      url: nginx:2
services:
  - type: web
    name: api
    buildSource: bs-two
`
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: relinked}); err != nil {
		t.Fatalf("relink to bs-two: %v", err)
	}
	links := allowedRows(sink, core.AuditVerbLinkBuildSource)
	sources := allowedRows(sink, core.AuditVerbSetSourceAndRegistryCredential)
	if len(links) != 1 || links[0].BuildSourceFrom != bs1ID || links[0].BuildSourceTo != bs2ID || len(sources) != 0 {
		t.Fatalf("blueprint relink: links=%+v sources=%+v, want %s->%s and no source_changed", links, sources, bs1ID, bs2ID)
	}

	// Unlink bs-two -> own image via Blueprint.
	sink.events = nil
	const unlinked = `buildSources:
  - name: bs-one
    image:
      url: nginx:1
  - name: bs-two
    image:
      url: nginx:2
services:
  - type: web
    name: api
    runtime: image
    image:
      url: nginx:3
`
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: unlinked}); err != nil {
		t.Fatalf("unlink to image: %v", err)
	}
	links = allowedRows(sink, core.AuditVerbLinkBuildSource)
	sources = allowedRows(sink, core.AuditVerbSetSourceAndRegistryCredential)
	if len(links) != 1 || links[0].BuildSourceFrom != bs2ID || links[0].BuildSourceTo != "" || len(sources) != 0 {
		t.Fatalf("blueprint unlink: links=%+v sources=%+v, want %s->\"\" and no source_changed", links, sources, bs2ID)
	}
}
