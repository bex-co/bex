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
	"net/http/httptest"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// buildSourceStore keeps the rows a build source create writes, under the
// bds- id the create minted, and reports their links and count.
type buildSourceStore struct {
	*recordingStore
	rows     map[string]store.App
	links    map[string][]store.App
	linked   []string
	carriers int
}

func newBuildSourceStore() *buildSourceStore {
	return &buildSourceStore{recordingStore: &recordingStore{}, rows: map[string]store.App{}, links: map[string][]store.App{}}
}

// CreateApp follows PGStore: a carrier keeps its minted id, any other row
// gets a fresh one.
func (b *buildSourceStore) CreateApp(ctx context.Context, a store.App) (store.App, error) {
	id := a.ID
	row, err := b.recordingStore.CreateApp(ctx, a)
	if err != nil {
		return row, err
	}
	if a.Type == store.TypeBuildSource {
		row.ID = id
	}
	b.rows[row.ID] = row
	return row, nil
}

func (b *buildSourceStore) GetApp(_ context.Context, id string) (store.App, error) {
	if row, ok := b.rows[id]; ok {
		return row, nil
	}
	return store.App{}, store.ErrNotFound
}

func (b *buildSourceStore) SetAppDisplayName(_ context.Context, id, name string) error {
	row := b.rows[id]
	row.DisplayName = name
	b.rows[id] = row
	return nil
}

func (b *buildSourceStore) ListBuildSourceLinks(_ context.Context, id string) ([]store.App, error) {
	return b.links[id], nil
}

func (b *buildSourceStore) CountBuildSourcesForTenant(context.Context, string) (int, error) {
	return b.carriers, nil
}

func (b *buildSourceStore) GetTenant(_ context.Context, id string) (store.Tenant, error) {
	return store.Tenant{ID: id, Plan: store.PlanHobby}, nil
}

func newBuildSourceService(t *testing.T, apps ...*appv1alpha1.App) (*Service, *buildSourceStore, context.Context) {
	t.Helper()
	st := newBuildSourceStore()
	svc, _ := newService(st, apps...)
	svc.Base.Workspace = fakeWorkspace{"user-a": "tea-a"}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})
	return svc, st, ctx
}

var monorepoSource = CreateBuildSourceRequest{OwnerID: "tea-a", Name: "Mono", Git: &BuildSourceGit{
	RepoURL: "https://github.com/acme/mono", Branch: "main", Runtime: "node", BuildCommand: "npm ci && npm run build",
}}

// w2/m254 t003: Render's create, get, list, patch and delete on a carrier
// that is never a service.
func TestBuildSourceLifecycle(t *testing.T) {
	svc, st, ctx := newBuildSourceService(t)
	created, err := svc.CreateBuildSource(ctx, monorepoSource)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.ID, "bds-") || created.Name != "Mono" || created.OwnerID != "tea-a" ||
		created.Git == nil || created.Git.RepoURL != monorepoSource.Git.RepoURL || created.Git.Runtime != "node" ||
		created.Image != nil || created.ServiceLinks == nil {
		t.Fatalf("created = %+v (git %+v)", created, created.Git)
	}
	row := st.rows[created.ID]
	if row.Type != store.TypeBuildSource || row.Name != created.ID {
		t.Fatalf("carrier row = %+v, want a build_source row named by its id", row)
	}
	var crs appv1alpha1.AppList
	if err := svc.Client.List(ctx, &crs); err != nil || len(crs.Items) != 1 || !core.IsBuildSource(&crs.Items[0]) || crs.Items[0].Spec.Expose {
		t.Fatalf("carrier CRs = %+v (err %v), want one unexposed build_source", crs.Items, err)
	}

	got, err := svc.GetBuildSource(ctx, created.ID)
	if err != nil || got.ID != created.ID {
		t.Fatalf("get = %+v err=%v", got, err)
	}
	services, err := svc.List(ctx, "tea-a")
	if err != nil || len(services) != 0 {
		t.Fatalf("services = %+v err=%v, want the carrier excluded", services, err)
	}
	listed, err := svc.ListBuildSources(ctx, "tea-a", BuildSourceFilter{})
	if err != nil || len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("list = %+v err=%v", listed, err)
	}
	if none, err := svc.ListBuildSources(ctx, "tea-a", BuildSourceFilter{Names: []string{"other"}}); err != nil || len(none) != 0 {
		t.Fatalf("name filter = %+v err=%v", none, err)
	}

	before := crs.Items[0]
	renamed, branch := "Monorepo", "release"
	updated, err := svc.UpdateBuildSource(ctx, created.ID, UpdateBuildSourceRequest{Name: &renamed,
		Git: &BuildSourceGit{Branch: branch, BuildCommand: "make", DockerfilePath: "./web/Dockerfile"}})
	if err != nil || updated.Name != renamed || updated.Git.Branch != branch || updated.Git.BuildCommand != "make" {
		t.Fatalf("update = %+v (git %+v) err=%v", updated, updated.Git, err)
	}
	// Render saves a build source edit without building: one untracked patch,
	// no restartedAt bump, the source marked pending until a build is
	// triggered.
	var after appv1alpha1.App
	if err := svc.Client.Get(ctx, client.ObjectKeyFromObject(&before), &after); err != nil {
		t.Fatal(err)
	}
	if after.Spec.RestartedAt != before.Spec.RestartedAt || after.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] == "" {
		t.Fatalf("edit rolled out: restartedAt %q→%q, pending source %q", before.Spec.RestartedAt, after.Spec.RestartedAt,
			after.Annotations[appv1alpha1.AnnotationPendingSourceGeneration])
	}
	if _, err := svc.UpdateBuildSource(ctx, created.ID, UpdateBuildSourceRequest{Git: &BuildSourceGit{Runtime: "static"}}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("bad runtime patch: err = %v, want bad request", err)
	}
	if again, _ := svc.GetBuildSource(ctx, created.ID); again.Git.Runtime != "node" {
		t.Fatalf("a refused patch changed the runtime to %q", again.Git.Runtime)
	}

	st.links[created.ID] = []store.App{{ID: "srv-web", Name: "web"}}
	if linked, _ := svc.GetBuildSource(ctx, created.ID); len(linked.ServiceLinks) != 1 || linked.ServiceLinks[0].ID != "srv-web" {
		t.Fatalf("serviceLinks = %+v", linked.ServiceLinks)
	}
	if err := svc.DeleteBuildSource(ctx, created.ID); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("delete with a linked service = %v, want a conflict", err)
	}
	delete(st.links, created.ID)
	if err := svc.DeleteBuildSource(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.GetBuildSource(ctx, created.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("get after delete = %v, want not found", err)
	}
}

// The build-source surface never reaches a service, and an image-backed
// source reads back as image-backed.
func TestBuildSourceSurfaceRefusesServicesAndBadInput(t *testing.T) {
	web := &appv1alpha1.App{}
	web.Name, web.Namespace = core.CRName("tea-a", "web"), "default"
	web.Labels = map[string]string{core.LabelTenant: "tea-a", core.LabelServiceName: "web", core.LabelAppID: "srv-web"}
	web.Spec = appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Image: "nginx:1"}
	svc, st, ctx := newBuildSourceService(t, web)
	for _, id := range []string{"srv-web", "web"} {
		if _, err := svc.GetBuildSource(ctx, id); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("GetBuildSource(%s) = %v, want not found", id, err)
		}
		if err := svc.DeleteBuildSource(ctx, id); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("DeleteBuildSource(%s) = %v, want not found", id, err)
		}
	}

	image, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{OwnerID: "tea-a", Name: "Base image",
		Image: &BuildSourceImage{ImageURL: "docker.io/library/nginx:1.27"}})
	if err != nil || image.Image == nil || image.Image.ImageURL != "docker.io/library/nginx:1.27" || image.Git != nil {
		t.Fatalf("image-backed = %+v err=%v", image, err)
	}

	both := monorepoSource
	both.Image = &BuildSourceImage{ImageURL: "nginx"}
	badRuntime := monorepoSource
	badRuntime.Git = &BuildSourceGit{RepoURL: "https://github.com/acme/mono", Runtime: "static"}
	withServices := monorepoSource
	withServices.EnvGroupIDs = []string{"evg-1"}
	for name, req := range map[string]CreateBuildSourceRequest{
		"no name":     {OwnerID: "tea-a", Git: monorepoSource.Git},
		"both":        both,
		"neither":     {OwnerID: "tea-a", Name: "x"},
		"bad runtime": badRuntime,
	} {
		if _, err := svc.CreateBuildSource(ctx, req); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("%s: err = %v, want bad request", name, err)
		}
	}
	var coded *core.CodedError
	if _, err := svc.CreateBuildSource(ctx, withServices); !errors.As(err, &coded) || coded.Code != "BUILD_SOURCE_FIELD_UNSUPPORTED" {
		t.Errorf("serviceIds on create: err = %v, want BUILD_SOURCE_FIELD_UNSUPPORTED", err)
	}
	st.carriers = int(store.QuotaCapsForPlan(store.PlanHobby).BuildSources)
	if _, err := svc.CreateBuildSource(ctx, monorepoSource); !errors.As(err, &coded) || coded.Code != BuildSourceLimitCode {
		t.Errorf("past the ceiling: err = %v, want %s", err, BuildSourceLimitCode)
	}
}

func (b *buildSourceStore) GetProject(_ context.Context, id string) (store.Project, error) {
	if id == "prj-a" {
		return store.Project{ID: id, TenantID: "tea-a"}, nil
	}
	return store.Project{ID: id, TenantID: "tea-other"}, nil
}

// A build source may be scoped to a project in its own workspace only.
func TestBuildSourceProjectScope(t *testing.T) {
	svc, st, ctx := newBuildSourceService(t)
	scoped := monorepoSource
	scoped.ProjectID = "prj-a"
	created, err := svc.CreateBuildSource(ctx, scoped)
	if err != nil {
		t.Fatal(err)
	}
	if st.rows[created.ID].ScopeProjectID != "prj-a" || created.ProjectID != "prj-a" {
		t.Fatalf("scope = row %q view %q, want prj-a", st.rows[created.ID].ScopeProjectID, created.ProjectID)
	}
	scoped.ProjectID = "prj-foreign"
	if _, err := svc.CreateBuildSource(ctx, scoped); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("foreign project: err = %v, want bad request", err)
	}
}

// t004: a create's envVars seed the build-time environment like a service's.
func TestBuildSourceCreateSeedsBuildTimeEnv(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	req := monorepoSource
	req.EnvVars = []envVarInput{{Key: "NODE_ENV", Value: "production", ValueSet: true}}
	created, err := svc.CreateBuildSource(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	var crs appv1alpha1.AppList
	if err := svc.Client.List(ctx, &crs); err != nil || len(crs.Items) != 1 {
		t.Fatalf("carriers = %d (err %v)", len(crs.Items), err)
	}
	found := false
	for _, e := range crs.Items[0].Spec.Env {
		found = found || (e.Name == "NODE_ENV" && e.Value == "production")
	}
	if !found {
		t.Fatalf("carrier %s env = %+v, want NODE_ENV=production", created.ID, crs.Items[0].Spec.Env)
	}
}

// t005: a service created with buildSourceId takes all source and build
// configuration from the build source; Render's refusals hold.
func TestServiceCreateLinksABuildSource(t *testing.T) {
	svc, st, ctx := newBuildSourceService(t)
	source, err := svc.CreateBuildSource(ctx, monorepoSource) // node: a native runtime
	if err != nil {
		t.Fatal(err)
	}
	image, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{OwnerID: "tea-a", Name: "Img", Image: &BuildSourceImage{ImageURL: "nginx:1"}})
	if err != nil {
		t.Fatal(err)
	}

	web, err := svc.Create(ctx, CreateRequest{OwnerID: "tea-a", Name: "web", Type: appv1alpha1.TypeWebService,
		BuildSourceID: source.ID, StartCommand: "node server.js"})
	if err != nil {
		t.Fatal(err)
	}
	if web.BuildSourceID != source.ID {
		t.Fatalf("view buildSourceId = %q, want %s", web.BuildSourceID, source.ID)
	}
	var crs appv1alpha1.AppList
	if err := svc.Client.List(ctx, &crs); err != nil {
		t.Fatal(err)
	}
	var linked *appv1alpha1.App
	for i := range crs.Items {
		if crs.Items[i].Spec.BuildSource != nil {
			linked = &crs.Items[i]
		}
	}
	if linked == nil || linked.Spec.Repo != "" || linked.Spec.Image != "" || linked.Spec.BuildSource.Name != core.CRName("tea-a", source.ID) ||
		linked.Spec.BuildSource.Image != "" {
		t.Fatalf("linked service spec = %+v, want no source of its own and a link with no build yet", linked)
	}
	var row store.App
	for _, r := range st.appCreates {
		if r.BuildSourceID == source.ID {
			row = r
		}
	}
	if row.BuildSourceID != source.ID {
		t.Fatalf("no service row records the link: %+v", st.appCreates)
	}

	for name, req := range map[string]CreateRequest{
		"repo alongside":            {Name: "a", Type: appv1alpha1.TypeWebService, BuildSourceID: source.ID, StartCommand: "x", Repo: "https://github.com/acme/web"},
		"runtime alongside":         {Name: "b", Type: appv1alpha1.TypeWebService, BuildSourceID: source.ID, StartCommand: "x", Runtime: "node"},
		"native without start":      {Name: "c", Type: appv1alpha1.TypeWebService, BuildSourceID: source.ID},
		"image source with start":   {Name: "d", Type: appv1alpha1.TypeWebService, BuildSourceID: image.ID, StartCommand: "x"},
		"unknown build source":      {Name: "e", Type: appv1alpha1.TypeWebService, BuildSourceID: "bds-missing", StartCommand: "x"},
		"a service as build source": {Name: "f", Type: appv1alpha1.TypeWebService, BuildSourceID: web.ID, StartCommand: "x"},
	} {
		req.OwnerID = "tea-a"
		if _, err := svc.Create(ctx, req); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("%s: err = %v, want bad request", name, err)
		}
	}
	if _, err := svc.Create(ctx, CreateRequest{OwnerID: "tea-a", Name: "g", Type: appv1alpha1.TypeBackgroundWorker, BuildSourceID: image.ID}); err != nil {
		t.Errorf("image source, no start command: %v", err)
	}
}

func (b *buildSourceStore) SetAppBuildSource(_ context.Context, id, carrierID, image string) error {
	b.linked = append(b.linked, id+"→"+carrierID)
	return nil
}

// t005: PATCH buildSourceId links an existing service — only startCommand may
// ride along — and, like Render, is saved without deploying.
func TestServicePatchLinksABuildSource(t *testing.T) {
	svc, st, ctx := newBuildSourceService(t)
	source, err := svc.CreateBuildSource(ctx, monorepoSource)
	if err != nil {
		t.Fatal(err)
	}
	web, err := svc.Create(ctx, CreateRequest{OwnerID: "tea-a", Name: "web", Type: appv1alpha1.TypeWebService, Image: "nginx:1"})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	patch := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPatch, "/v1/services/"+web.ID, strings.NewReader(body)).WithContext(ctx)
		req.ContentLength = int64(len(body))
		validated(t, mux).ServeHTTP(rec, req)
		return rec
	}
	for name, body := range map[string]string{
		"with name":            `{"buildSourceId":"` + source.ID + `","name":"renamed"}`,
		"with plan":            `{"buildSourceId":"` + source.ID + `","serviceDetails":{"plan":"standard"}}`,
		"native without start": `{"buildSourceId":"` + source.ID + `"}`,
	} {
		if rec := patch(body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d %s, want 400", name, rec.Code, rec.Body.String())
		}
	}
	before := getTenantApp(t, svc.Client, "tea-a", "web")
	rec := patch(`{"buildSourceId":"` + source.ID + `","serviceDetails":{"envSpecificDetails":{"startCommand":"node server.js"}}}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"buildSourceId":"`+source.ID+`"`) {
		t.Fatalf("link = %d %s", rec.Code, rec.Body.String())
	}
	after := getTenantApp(t, svc.Client, "tea-a", "web")
	if after.Spec.Image != "" || after.Spec.BuildSource == nil || after.Spec.StartCommand != "node server.js" ||
		after.Spec.RestartedAt != before.Spec.RestartedAt || after.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] == "" {
		t.Fatalf("linked spec = %+v (annotations %v), want the link saved without deploying", after.Spec, after.Annotations)
	}
	if len(st.linked) != 1 {
		t.Fatalf("store links = %v, want one", st.linked)
	}
}

// Review fixes for t005: a static site cannot link; a dry run writes nothing;
// linking to an image source drops the service's old start command; giving a
// linked service its own image unlinks it.
func TestBuildSourceLinkEdges(t *testing.T) {
	svc, st, ctx := newBuildSourceService(t)
	image, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{OwnerID: "tea-a", Name: "Img", Image: &BuildSourceImage{ImageURL: "nginx:1"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, CreateRequest{OwnerID: "tea-a", Name: "site", Type: appv1alpha1.TypeStaticSite, BuildSourceID: image.ID}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("static site create with buildSourceId: err = %v, want bad request", err)
	}
	web, err := svc.Create(ctx, CreateRequest{OwnerID: "tea-a", Name: "web", Type: appv1alpha1.TypeWebService,
		Repo: "https://github.com/acme/web", Runtime: "node", BuildCommand: "npm ci", StartCommand: "node old.js"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.LinkBuildSource(ctx, web.ID, image.ID, nil, true); err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if got := getTenantApp(t, svc.Client, "tea-a", "web"); got.Spec.BuildSource != nil || len(st.linked) != 0 {
		t.Fatalf("a dry run linked the service: %+v / %v", got.Spec.BuildSource, st.linked)
	}
	linked, err := svc.LinkBuildSource(ctx, web.ID, image.ID, nil, false)
	if err != nil {
		t.Fatalf("link to an image source without naming startCommand: %v", err)
	}
	got := getTenantApp(t, svc.Client, "tea-a", "web")
	if linked.BuildSourceID != image.ID || got.Spec.StartCommand != "" || got.Spec.Repo != "" {
		t.Fatalf("linked spec = %+v, want the image source and no start command of its own", got.Spec)
	}
	ownImage := "nginx:2"
	if _, err := svc.SetSourceAndRegistryCredential(ctx, web.ID, sourcePatch{Image: &ownImage}); err != nil {
		t.Fatalf("own image on a linked service: %v", err)
	}
	if got := getTenantApp(t, svc.Client, "tea-a", "web"); got.Spec.BuildSource != nil || got.Spec.Image != ownImage {
		t.Fatalf("after its own image: %+v, want unlinked", got.Spec)
	}
}

// w2/m254 t016: a native git source needs a build command (the operator's
// native build requires one); create refuses it by name instead of accepting
// a source whose every build fails.
func TestNativeBuildSourceRequiresABuildCommand(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	req := monorepoSource
	git := *req.Git
	git.BuildCommand = ""
	req.Git = &git
	if _, err := svc.CreateBuildSource(ctx, req); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "buildCommand") {
		t.Fatalf("native source without buildCommand = %v, want a 400 naming buildCommand", err)
	}
	git.Runtime, git.DockerfilePath = "docker", "Dockerfile"
	if _, err := svc.CreateBuildSource(ctx, req); err != nil {
		t.Fatalf("a Docker source needs no buildCommand: %v", err)
	}
}

// w4/473: a build source whose delete is accepted reads as gone while its
// finalizer still runs, in Get and List alike, as a deleting service does.
func TestDeletingBuildSourceReadsAsGone(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	created, err := svc.CreateBuildSource(ctx, monorepoSource)
	if err != nil {
		t.Fatal(err)
	}
	var crs appv1alpha1.AppList
	if err := svc.Client.List(ctx, &crs); err != nil || len(crs.Items) != 1 {
		t.Fatalf("carrier CRs = %+v (err %v)", crs.Items, err)
	}
	carrier := crs.Items[0]
	carrier.Finalizers = []string{"test/hold"}
	if err := svc.Client.Update(ctx, &carrier); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteBuildSource(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var held appv1alpha1.App
	if err := svc.Client.Get(ctx, client.ObjectKeyFromObject(&carrier), &held); err != nil || held.DeletionTimestamp == nil {
		t.Fatalf("carrier = %+v (err %v), want it held by its finalizer", held.ObjectMeta, err)
	}
	if _, err := svc.GetBuildSource(ctx, created.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("get while deleting = %v, want not found", err)
	}
	if listed, err := svc.ListBuildSources(ctx, "tea-a", BuildSourceFilter{}); err != nil || len(listed) != 0 {
		t.Fatalf("list while deleting = %+v err=%v, want none", listed, err)
	}
}
