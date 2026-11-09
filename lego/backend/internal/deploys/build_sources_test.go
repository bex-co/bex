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
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	"github.com/bex-co/bex/lego/backend/internal/store"
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

// ADR097: a build source owns its linked env groups and reads them at build
// time. A link is staged on the carrier until its next build (w2/m254), and
// that build's release carries it: the group's Secret lands in the carrier's
// spec and the staged annotation clears (w9/m180 production check).
func TestTriggerBuildShipsAStagedGroupLink(t *testing.T) {
	ds := newFakeStore()
	ds.mintIDs = true
	carrier := sampleBuildSource("bds-git", "https://github.com/acme/web")
	rollout.StageGroupLink(carrier, "evg-shared-env", "evg-shared-files", true)
	svc, cl := newService(ds, carrier)
	ctx := context.Background()

	if _, err := svc.TriggerBuild(ctx, "bds-git"); err != nil {
		t.Fatalf("TriggerBuild: %v", err)
	}
	var got appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKeyFromObject(carrier), &got); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(got.Spec.EnvFromSecrets, "evg-shared-env") {
		t.Fatalf("carrier envFrom = %v, want the staged group's Secret in the build's release", got.Spec.EnvFromSecrets)
	}
	if got.Annotations[appv1alpha1.AnnotationPendingGroupLinks] != "" {
		t.Fatalf("staged links still pending after the build: %q", got.Annotations[appv1alpha1.AnnotationPendingGroupLinks])
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

func builtCarrier(id, image string) *appv1alpha1.App {
	a := sampleBuildSource(id, "https://github.com/acme/web")
	a.Status.Image, a.Status.ActiveRevision = image, "r1"
	return a
}

func linkedService(name, storeID, carrier, image string, autoDeploy bool) *appv1alpha1.App {
	a := sampleApp(name, storeID)
	a.Spec.Image = ""
	a.Spec.AutoDeploy = autoDeploy
	a.Spec.BuildSource = &appv1alpha1.BuildSourceLink{Name: carrier, Image: image}
	return a
}

// A deploy of a linked service ships the source's current build, recorded in
// the row the projector owns and on the CR together.
func TestTriggerOfALinkedServiceSelectsTheCurrentBuild(t *testing.T) {
	ds := newFakeStore()
	svc, cl := newService(ds, builtCarrier("bds-1", "zot/src@sha256:new"), linkedService("web", "srv-1", "bds-1", "zot/src@sha256:old", false))
	if _, err := svc.Trigger(context.Background(), "web", TriggerParams{}); err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.BuildSource; got == nil || got.Image != "zot/src@sha256:new" {
		t.Errorf("spec.buildSource = %+v, want the current build", got)
	}
	if ds.builds["srv-1"] != "zot/src@sha256:new" || len(ds.byApp["srv-1"]) != 1 {
		t.Errorf("row build = %q, deploys = %d; want the current build and one deploy", ds.builds["srv-1"], len(ds.byApp["srv-1"]))
	}
}

// A deploy of a linked service whose source has not built yet is refused
// rather than opening a deploy that can never finish; a restart keeps the
// build the service is serving.
func TestLinkedTriggerRefusesAnUnbuiltSourceAndRestartKeepsTheBuild(t *testing.T) {
	ds := newFakeStore()
	svc, cl := newService(ds, sampleBuildSource("bds-new", "https://github.com/acme/web"), builtCarrier("bds-1", "zot/src@sha256:new"),
		linkedService("waiting", "srv-1", "bds-new", "", false),
		linkedService("web", "srv-2", "bds-1", "zot/src@sha256:old", false))
	ctx := context.Background()
	if _, err := svc.Trigger(ctx, "waiting", TriggerParams{}); !errors.Is(err, core.ErrConflict) {
		t.Errorf("deploy from an unbuilt source: want ErrConflict, got %v", err)
	}
	if _, err := svc.triggerFetched(ctx, "web", getApp(t, cl, "web"), TriggerParams{restart: true}, store.TriggerRestart); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.BuildSource.Image; got != "zot/src@sha256:old" || ds.builds["srv-2"] != "" {
		t.Errorf("restart moved the build to %q (row %q); it re-runs the serving build", got, ds.builds["srv-2"])
	}
}

// A succeeded build deploys each linked auto-deploy service once, completes
// the first deploy of a service waiting on the source, and leaves
// auto-deploy-off, suspended, already-current and other sources' services
// alone.
func TestBuildSucceededDeploysLinkedAutoDeployServices(t *testing.T) {
	const tenant, built = "tea-a", "zot/src@sha256:new"
	cr := func(name string) string { return store.CRName(tenant, name) }
	ds := newFakeStore()
	suspended := linkedService(cr("paused"), "srv-4", "bds-1", "zot/src@sha256:old", true)
	suspended.Spec.Suspended = true
	ds.links = []store.App{
		{ID: "srv-1", TenantID: tenant, Name: "web", BuildSourceID: "bds-1", BuildSourceImage: "zot/src@sha256:old"},
		{ID: "srv-2", TenantID: tenant, Name: "manual", BuildSourceID: "bds-1", BuildSourceImage: "zot/src@sha256:old"},
		{ID: "srv-3", TenantID: tenant, Name: "other", BuildSourceID: "bds-2", BuildSourceImage: "zot/b@sha256:old"},
		{ID: "srv-4", TenantID: tenant, Name: "paused", BuildSourceID: "bds-1", BuildSourceImage: "zot/src@sha256:old"},
		{ID: "srv-5", TenantID: tenant, Name: "current", BuildSourceID: "bds-1", BuildSourceImage: built},
		{ID: "srv-6", TenantID: tenant, Name: "first", BuildSourceID: "bds-1"},
	}
	svc, cl := newService(ds, builtCarrier("bds-1", built), builtCarrier("bds-2", "zot/b@sha256:x"),
		linkedService(cr("web"), "srv-1", "bds-1", "zot/src@sha256:old", true),
		linkedService(cr("manual"), "srv-2", "bds-1", "zot/src@sha256:old", false),
		linkedService(cr("other"), "srv-3", "bds-2", "zot/b@sha256:old", true),
		suspended,
		linkedService(cr("current"), "srv-5", "bds-1", built, true),
		linkedService(cr("first"), "srv-6", "bds-1", "", false))

	ds.mintIDs = true
	built1, err := ds.CreateDeploy(context.Background(), "bds-1", store.TriggerAPI, "", 1, store.CommitInfo{Hash: "abc1234", Message: "ship the monorepo"}, "")
	if err != nil {
		t.Fatal(err)
	}
	carrierDeploy := built1.ID
	svc.BuildSucceeded(context.Background(), store.BuiltSource{TenantID: tenant, CarrierID: "bds-1", DeployID: carrierDeploy, Commit: built1.CommitInfo(), Namespace: "default", Carrier: "bds-1", Image: built})

	for _, id := range []string{"srv-1", "srv-6"} {
		if n := len(ds.byApp[id]); n != 1 || ds.byApp[id][0].Trigger != store.TriggerNewCommit {
			t.Fatalf("%s deploys = %+v, want one new_commit deploy", id, ds.byApp[id])
		}
		if ds.builds[id] != built || ds.byApp[id][0].BuildID != BuildID(carrierDeploy) {
			t.Errorf("%s row build = %q, deploy buildId %q; want %s from %s", id, ds.builds[id], ds.byApp[id][0].BuildID, built, BuildID(carrierDeploy))
		}
		// w2/m262 t005: the deploy reports the build's commit, as Render's does.
		if d := ds.byApp[id][0]; d.Commit != "abc1234" || d.CommitMessage != "ship the monorepo" {
			t.Errorf("%s fan-out deploy commit = %q %q, want the build's", id, d.Commit, d.CommitMessage)
		}
	}
	if got := getApp(t, cl, cr("web")).Spec.BuildSource.Image; got != built {
		t.Errorf("web build = %q, want the new build", got)
	}
	for _, id := range []string{"srv-2", "srv-3", "srv-4", "srv-5"} {
		if len(ds.byApp[id]) != 0 {
			t.Errorf("%s was deployed: %+v", id, ds.byApp[id])
		}
	}
}

func codeOf(err error) string {
	var coded *core.CodedError
	if errors.As(err, &coded) {
		return coded.Code
	}
	return ""
}

// A deploy of a linked service records the build it ships (Render's
// deploy.buildId); buildId deploys an older succeeded build, and refuses an
// unknown or unsucceeded one with Render's codes.
func TestLinkedDeploysCarryAndAcceptABuildID(t *testing.T) {
	ds := newFakeStore()
	ds.mintIDs = true
	ctx := context.Background()
	old, _ := ds.CreateDeploy(ctx, "bds-1", store.TriggerAPI, "", 1, store.CommitInfo{}, "")
	failed, _ := ds.CreateDeploy(ctx, "bds-1", store.TriggerAPI, "", 2, store.CommitInfo{}, "")
	live, _ := ds.CreateDeploy(ctx, "bds-1", store.TriggerAPI, "", 3, store.CommitInfo{}, "")
	for i, row := range ds.byApp["bds-1"] {
		switch row.ID {
		case old.ID:
			row.Status, row.ResolvedImage = store.DeployDeactivated, "zot/src@sha256:old"
		case failed.ID:
			row.Status = store.DeployBuildFailed
		case live.ID:
			row.Status, row.ResolvedImage = store.DeployLive, "zot/src@sha256:new"
		}
		ds.byApp["bds-1"][i] = row
	}
	svc, cl := newService(ds, builtCarrier("bds-1", "zot/src@sha256:new"), linkedService("web", "srv-1", "bds-1", "", false), sampleApp("api", "srv-2"))

	d, err := svc.Trigger(ctx, "web", TriggerParams{})
	if err != nil || d.BuildID != BuildID(live.ID) || ds.byApp["srv-1"][0].BuildID != BuildID(live.ID) {
		t.Fatalf("deploy = %+v, %v; want buildId %s on the view and the row", d, err, BuildID(live.ID))
	}
	d, err = svc.Trigger(ctx, "web", TriggerParams{BuildID: BuildID(old.ID)})
	if err != nil || d.BuildID != BuildID(old.ID) || getApp(t, cl, "web").Spec.BuildSource.Image != "zot/src@sha256:old" {
		t.Fatalf("deploy of the older build = %+v, %v (image %s)", d, err, getApp(t, cl, "web").Spec.BuildSource.Image)
	}

	for _, tc := range []struct {
		name, service string
		p             TriggerParams
		code          string
		sentinel      error
	}{
		{"unknown build", "web", TriggerParams{BuildID: BuildID(ids.New(ids.Deploy))}, buildNotFoundCode, core.ErrNotFound},
		{"not a build id", "web", TriggerParams{BuildID: old.ID}, buildNotFoundCode, core.ErrNotFound},
		{"failed build", "web", TriggerParams{BuildID: BuildID(failed.ID)}, buildNotSucceededCode, core.ErrConflict},
		{"with commitId", "web", TriggerParams{BuildID: BuildID(old.ID), CommitID: "abc"}, "", core.ErrBadRequest},
		{"with deployMode", "web", TriggerParams{BuildID: BuildID(old.ID), DeployMode: "build_and_deploy"}, "", core.ErrBadRequest},
		{"unlinked service", "api", TriggerParams{BuildID: BuildID(old.ID)}, "", core.ErrBadRequest},
	} {
		_, err := svc.Trigger(ctx, tc.service, tc.p)
		if !errors.Is(err, tc.sentinel) || codeOf(err) != tc.code {
			t.Errorf("%s: err = %v (code %q), want %v with code %q", tc.name, err, codeOf(err), tc.sentinel, tc.code)
		}
	}
	if n := len(ds.byApp["srv-1"]); n != 2 {
		t.Errorf("web deploys = %d, want 2 (refusals open none)", n)
	}
}

// A rollback to a deploy that shipped a build reports that build too.
func TestRollbackCarriesTheRestoredBuildID(t *testing.T) {
	ds := newFakeStore()
	ctx := context.Background()
	first, _ := ds.CreateDeploy(ctx, "srv-1", "create", "web:v1", 1, store.CommitInfo{}, "")
	if won, err := ds.CloseDeploy(ctx, first.ID, store.DeployLive, "web:v1"); err != nil || !won {
		t.Fatalf("close first live: won=%v err=%v", won, err)
	}
	buildID := BuildID(ids.New(ids.Deploy))
	ds.byApp["srv-1"][0].BuildID = buildID
	bad, _ := ds.CreateDeploy(ctx, "srv-1", "api", "web:bad", 2, store.CommitInfo{}, "")
	if won, err := ds.CloseDeploy(ctx, bad.ID, store.DeployUpdateFailed, ""); err != nil || !won {
		t.Fatalf("close bad failed: won=%v err=%v", won, err)
	}
	app := sampleApp("web", "srv-1")
	app.Spec.Image = "web:bad"
	svc, _ := newService(ds, app)

	rolled, err := svc.Rollback(ctx, "web", first.ID)
	if err != nil || rolled.BuildID != buildID || ds.byApp["srv-1"][0].BuildID != buildID {
		t.Fatalf("rollback = %+v, %v; want buildId %s on the view and the row", rolled, err, buildID)
	}
}

// GraphQL triggerBuild/builds/build and the MCP build tools reach the same
// verbs as REST.
func TestBuildGraphQLAndMCP(t *testing.T) {
	ds := newFakeStore()
	ds.mintIDs = true
	svc, _ := newService(ds, sampleBuildSource("bds-git", "https://github.com/acme/web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `mutation { triggerBuild(buildSourceId:"bds-git") { id buildSourceId buildRun { id status runtime } image { imageUrl } } }`})
	if res.HasErrors() {
		t.Fatal(res.Errors)
	}
	built := res.Data.(map[string]any)["triggerBuild"].(map[string]any)
	run, _ := built["buildRun"].(map[string]any)
	if built["buildSourceId"] != "bds-git" || run["status"] != store.BuildRunCreated || run["runtime"] != "node" || built["image"] != nil {
		t.Fatalf("triggerBuild = %+v", built)
	}
	id := built["id"].(string)
	res = graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `{ builds(buildSourceId:"bds-git") { id cursor } build(buildSourceId:"bds-git", id:"` + id + `") { id } }`})
	if res.HasErrors() {
		t.Fatal(res.Errors)
	}
	data := res.Data.(map[string]any)
	builds, _ := data["builds"].([]any)
	if len(builds) != 1 || builds[0].(map[string]any)["cursor"] != run["id"] || data["build"].(map[string]any)["id"] != id {
		t.Fatalf("builds = %+v, build = %+v", builds, data["build"])
	}

	session := newMCPSession(t, svc)
	got, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_build_source_builds", Arguments: map[string]any{"buildSourceId": "bds-git"}})
	if err != nil || got.IsError {
		t.Fatalf("list_build_source_builds = %+v, %v", got, err)
	}
	encoded, _ := json.Marshal(got.StructuredContent)
	if !strings.Contains(string(encoded), id) {
		t.Fatalf("list_build_source_builds = %s, want %s", encoded, id)
	}
	got, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "trigger_build_source_build", Arguments: map[string]any{"buildSourceId": "web"}})
	if err != nil || !got.IsError {
		t.Fatalf("trigger_build_source_build on a non-source = %+v, %v; want a tool error", got, err)
	}
}

// w4/472: Render's buildRun carries commitUrl next to commitId.
func TestBuildViewCarriesTheCommitURL(t *testing.T) {
	carrier := sampleBuildSource("bds-git", "https://github.com/acme/web.git")
	got := buildView(carrier, "bds-git", DeployView{ID: "dep-1", CommitID: "abc123"})
	if got.BuildRun == nil || got.BuildRun.CommitURL != "https://github.com/acme/web/commit/abc123" {
		t.Fatalf("buildRun = %+v, want commitUrl https://github.com/acme/web/commit/abc123", got.BuildRun)
	}
	none := buildView(carrier, "bds-git", DeployView{ID: "dep-2"})
	if none.BuildRun.CommitURL != "" {
		t.Fatalf("a build with no commit has commitUrl %q", none.BuildRun.CommitURL)
	}
	image := buildView(sampleBuildSource("bds-img", ""), "bds-img", DeployView{ID: "dep-3", CommitID: "abc123"})
	if image.BuildRun != nil {
		t.Fatalf("an image-source build has a buildRun: %+v", image.BuildRun)
	}
}

// w9/m180: two fan-outs of one build (a resync overlapping a slow first run
// whose lease expired) race for the same services. The build is selected
// under each service's trigger lock, so between them every linked auto-deploy
// service gets exactly one deploy.
func TestConcurrentFanoutsOfOneBuildDeployEachServiceOnce(t *testing.T) {
	const tenant, built = "tea-a", "zot/src@sha256:new"
	cr := func(name string) string { return store.CRName(tenant, name) }
	ds := newFakeStore()
	ds.links = []store.App{
		{ID: "srv-1", TenantID: tenant, Name: "web", BuildSourceID: "bds-1", BuildSourceImage: "zot/src@sha256:old"},
		{ID: "srv-2", TenantID: tenant, Name: "api", BuildSourceID: "bds-1", BuildSourceImage: "zot/src@sha256:old"},
	}
	svc, _ := newService(ds, builtCarrier("bds-1", built),
		linkedService(cr("web"), "srv-1", "bds-1", "zot/src@sha256:old", true),
		linkedService(cr("api"), "srv-2", "bds-1", "zot/src@sha256:old", true))
	b := store.BuiltSource{TenantID: tenant, CarrierID: "bds-1", DeployID: "dep-carrier", Namespace: "default", Carrier: "bds-1", Image: built}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- svc.BuildSucceeded(context.Background(), b)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("fan-out: %v", err)
		}
	}
	for _, id := range []string{"srv-1", "srv-2"} {
		if n := len(ds.byApp[id]); n != 1 {
			t.Errorf("%s got %d deploys from two fan-outs of one build, want 1", id, n)
		}
	}
}

// w9/m180: a refusal a retry would meet again completes the fan-out instead
// of failing it (which the reconciler would re-run every lease period): a
// linked service whose CR is gone, and a workspace under billing enforcement.
func TestFanoutCompletesPastPermanentRefusals(t *testing.T) {
	const tenant, built = "tea-a", "zot/src@sha256:new"
	cr := func(name string) string { return store.CRName(tenant, name) }
	ds := newFakeStore()
	ds.links = []store.App{
		{ID: "srv-1", TenantID: tenant, Name: "web", BuildSourceID: "bds-1", BuildSourceImage: "zot/src@sha256:old"},
		{ID: "srv-9", TenantID: tenant, Name: "gone", BuildSourceID: "bds-1", BuildSourceImage: "zot/src@sha256:old"},
	}
	svc, _ := newService(ds, builtCarrier("bds-1", built), linkedService(cr("web"), "srv-1", "bds-1", "zot/src@sha256:old", true))
	b := store.BuiltSource{TenantID: tenant, CarrierID: "bds-1", DeployID: "dep-carrier", Namespace: "default", Carrier: "bds-1", Image: built}
	if err := svc.BuildSucceeded(context.Background(), b); err != nil {
		t.Fatalf("a missing linked service failed the fan-out: %v", err)
	}
	if len(ds.byApp["srv-1"]) != 1 {
		t.Fatalf("web deploys = %d, want 1 beside the missing service", len(ds.byApp["srv-1"]))
	}
	if !permanentFanoutError(core.NewBillingEnforcedError()) || permanentFanoutError(errors.New("connection reset")) {
		t.Fatal("billing enforcement must complete the fan-out and a transport error must retry it")
	}
}
