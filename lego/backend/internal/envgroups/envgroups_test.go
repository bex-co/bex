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

package envgroups

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- harness ------------------------------------------------------------------

func testScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = appv1alpha1.AddToScheme(s)
	return s
}

func fakeClient(objs ...client.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(objs...).Build()
}

func sampleApp(name string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: name + ":v1", Replicas: 1},
	}
}

func newService(store core.SecretKV, objs ...client.Object) *Service {
	return &Service{
		Base:  &core.Base{Client: fakeClient(objs...), Namespace: "default", Clock: func() time.Time { return time.Unix(1_000_000, 0).UTC() }},
		Store: store,
	}
}

func getApp(t *testing.T, cl client.Client, name string) *appv1alpha1.App {
	t.Helper()
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &a); err != nil {
		t.Fatalf("get app %s: %v", name, err)
	}
	return &a
}

// fakeStore is an in-memory core.SecretKV keyed by logical path.
type fakeStore struct{ m map[string]map[string]string }

func newFakeStore() *fakeStore { return &fakeStore{m: map[string]map[string]string{}} }

func (f *fakeStore) Get(_ context.Context, path string) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range f.m[path] {
		out[k] = v
	}
	return out, nil
}

func (f *fakeStore) Put(_ context.Context, path string, data map[string]string) error {
	cp := map[string]string{}
	for k, v := range data {
		cp[k] = v
	}
	f.m[path] = cp
	return nil
}

func (f *fakeStore) Delete(_ context.Context, path string) error { delete(f.m, path); return nil }

func (f *fakeStore) List(_ context.Context, path string) ([]string, error) {
	prefix := path + "/"
	seen := map[string]bool{}
	var out []string
	for k := range f.m {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		rest := strings.TrimPrefix(k, prefix)
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			rest = rest[:i]
		}
		if !seen[rest] {
			seen[rest] = true
			out = append(out, rest)
		}
	}
	return out, nil
}

type fakeChecker struct {
	allow        bool
	lastRelation string
}

func (c *fakeChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	c.lastRelation = relation
	return c.allow, nil
}

func secret(t *testing.T, cl client.Client, name string) *corev1.Secret {
	t.Helper()
	var s corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &s); err != nil {
		t.Fatalf("get secret %s: %v", name, err)
	}
	return &s
}

// --- lifecycle + contents -----------------------------------------------------

func TestEnvGroup_CreateVarsAndView(t *testing.T) {
	svc := newService(newFakeStore())
	ctx := context.Background()

	g, err := svc.CreateEnvGroup(ctx, "shared")
	if err != nil {
		t.Fatalf("CreateEnvGroup: %v", err)
	}
	if k, ok := id.KindOf(g.ID); !ok || k != id.EnvGroup {
		t.Fatalf("group id should be an evg- id: %q", g.ID)
	}
	// Create materializes the (empty) projection Secrets so a link is safe.
	secret(t, svc.Client, envSecretName(g.ID))
	secret(t, svc.Client, filesSecretName(g.ID))

	if _, err := svc.SetEnvGroupVars(ctx, g.ID, []EnvVarView{{Key: "SHARED_KEY", Value: "v1"}, {Key: "A", Value: "1"}}); err != nil {
		t.Fatalf("SetEnvGroupVars: %v", err)
	}
	// The group's env Secret holds the values.
	if string(secret(t, svc.Client, envSecretName(g.ID)).Data["SHARED_KEY"]) != "v1" {
		t.Fatal("group env Secret not materialized")
	}
	// View is keys-only (no values leaked); a reveal returns the value.
	got, err := svc.GetEnvGroup(ctx, g.ID)
	if err != nil || len(got.EnvVars) != 2 || got.EnvVars[0].Key != "A" || got.EnvVars[0].Value != "" {
		t.Fatalf("GetEnvGroup keys-only sorted: %+v err=%v", got, err)
	}
	rev, err := svc.GetEnvGroupVar(ctx, g.ID, "SHARED_KEY")
	if err != nil || rev.Value != "v1" {
		t.Fatalf("GetEnvGroupVar reveal: %+v err=%v", rev, err)
	}

	// List surfaces the group.
	all, err := svc.ListEnvGroups(ctx)
	if err != nil || len(all) != 1 || all[0].ID != g.ID || all[0].Name != "shared" {
		t.Fatalf("ListEnvGroups: %+v err=%v", all, err)
	}

	if _, err := svc.GetEnvGroup(ctx, "evg-doesnotexist00000000"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown group => ErrNotFound, got %v", err)
	}
}

func TestEnvGroup_RenamePreservesIdentityContentsAndLinks(t *testing.T) {
	store := newFakeStore()
	svc := newService(store, sampleApp("web"))
	ctx := context.Background()

	g, _ := svc.CreateEnvGroup(ctx, "shared")
	_, _ = svc.SetEnvGroupVar(ctx, g.ID, "TOKEN", "secret")
	_ = svc.LinkService(ctx, g.ID, "web")
	before := getApp(t, svc.Client, "web").Spec.RestartedAt

	renamed, err := svc.RenameEnvGroup(ctx, g.ID, "shared-production")
	if err != nil {
		t.Fatalf("RenameEnvGroup: %v", err)
	}
	if renamed.ID != g.ID || renamed.Name != "shared-production" || !slices.Contains(renamed.ServiceLinks, "web") {
		t.Fatalf("rename changed identity or links: %+v", renamed)
	}
	if got, _ := svc.GetEnvGroupVar(ctx, g.ID, "TOKEN"); got.Value != "secret" {
		t.Fatalf("rename lost contents: %+v", got)
	}
	if after := getApp(t, svc.Client, "web").Spec.RestartedAt; after != before {
		t.Fatalf("metadata-only rename should not roll service: before=%q after=%q", before, after)
	}
	if _, err := svc.RenameEnvGroup(ctx, g.ID, "  "); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("blank rename => ErrBadRequest, got %v", err)
	}
}

func TestEnvGroup_SetAndDeleteOneVarPreservesSiblings(t *testing.T) {
	svc := newService(newFakeStore(), sampleApp("web"))
	ctx := context.Background()
	g, _ := svc.CreateEnvGroup(ctx, "shared")
	_, _ = svc.SetEnvGroupVars(ctx, g.ID, []EnvVarView{{Key: "A", Value: "one"}, {Key: "B", Value: "two"}})
	_ = svc.LinkService(ctx, g.ID, "web")

	if _, err := svc.SetEnvGroupVar(ctx, g.ID, "A", "updated"); err != nil {
		t.Fatalf("SetEnvGroupVar: %v", err)
	}
	if got, _ := svc.GetEnvGroupVar(ctx, g.ID, "B"); got.Value != "two" {
		t.Fatalf("per-key set lost sibling: %+v", got)
	}
	if err := svc.DeleteEnvGroupVar(ctx, g.ID, "A"); err != nil {
		t.Fatalf("DeleteEnvGroupVar: %v", err)
	}
	if _, err := svc.GetEnvGroupVar(ctx, g.ID, "A"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("deleted var should be gone: %v", err)
	}
	if got, _ := svc.GetEnvGroupVar(ctx, g.ID, "B"); got.Value != "two" {
		t.Fatalf("per-key delete lost sibling: %+v", got)
	}
	if err := svc.DeleteEnvGroupVar(ctx, g.ID, "missing"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("missing delete => ErrNotFound, got %v", err)
	}
}

func TestMCP_EnvGroupEditingRoundTrip(t *testing.T) {
	svc := newService(newFakeStore(), sampleApp("web"))
	ctx := context.Background()
	srv := mcp.NewServer(&mcp.Implementation{Name: "bex", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	call := func(name string, args map[string]any, out any) {
		t.Helper()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("call %s: err=%v isErr=%v", name, err, res != nil && res.IsError)
		}
		if out != nil {
			b, _ := json.Marshal(res.StructuredContent)
			if err := json.Unmarshal(b, out); err != nil {
				t.Fatalf("decode %s: %v", name, err)
			}
		}
	}

	var group EnvGroupView
	call("create_env_group", map[string]any{"name": "shared"}, &group)
	call("set_env_group_var", map[string]any{"id": group.ID, "key": "TOKEN", "value": "secret"}, nil)
	var variable EnvVarView
	call("get_env_group_var", map[string]any{"id": group.ID, "key": "TOKEN"}, &variable)
	if variable.Value != "secret" {
		t.Fatalf("get_env_group_var: %+v", variable)
	}
	call("set_env_group_secret_file", map[string]any{"id": group.ID, "name": "cert.pem", "content": "pem"}, nil)
	var file SecretFileView
	call("get_env_group_secret_file", map[string]any{"id": group.ID, "name": "cert.pem"}, &file)
	if file.Content != "pem" {
		t.Fatalf("get_env_group_secret_file: %+v", file)
	}
	call("rename_env_group", map[string]any{"id": group.ID, "name": "Shared production"}, &group)
	if group.Name != "Shared production" {
		t.Fatalf("rename_env_group: %+v", group)
	}
	call("link_env_group", map[string]any{"id": group.ID, "serviceId": "web"}, nil)
	call("unlink_env_group", map[string]any{"id": group.ID, "serviceId": "web"}, nil)
	call("delete_env_group_var", map[string]any{"id": group.ID, "key": "TOKEN"}, nil)
	call("delete_env_group_secret_file", map[string]any{"id": group.ID, "name": "cert.pem"}, nil)
	call("delete_env_group", map[string]any{"id": group.ID}, nil)
}

func TestEnvGroup_LinkProjectsAndUnlink(t *testing.T) {
	svc := newService(newFakeStore(), sampleApp("web"), sampleApp("api"))
	ctx := context.Background()

	g, _ := svc.CreateEnvGroup(ctx, "shared")
	if _, err := svc.SetEnvGroupVars(ctx, g.ID, []EnvVarView{{Key: "DB_URL", Value: "postgres://x"}}); err != nil {
		t.Fatalf("SetEnvGroupVars: %v", err)
	}
	if _, err := svc.SetEnvGroupFile(ctx, g.ID, "ca.pem", "CERT"); err != nil {
		t.Fatalf("SetEnvGroupFile: %v", err)
	}

	// Link to web: the App spec gains the group's env + files Secret refs and rolls.
	if err := svc.LinkService(ctx, g.ID, "web"); err != nil {
		t.Fatalf("LinkService: %v", err)
	}
	web := getApp(t, svc.Client, "web")
	if !slices.Contains(web.Spec.EnvFromSecrets, envSecretName(g.ID)) {
		t.Fatalf("web should reference the group env Secret: %+v", web.Spec.EnvFromSecrets)
	}
	if !slices.Contains(web.Spec.FilesFromSecrets, filesSecretName(g.ID)) {
		t.Fatalf("web should reference the group files Secret: %+v", web.Spec.FilesFromSecrets)
	}
	if web.Spec.RestartedAt == "" {
		t.Fatal("link should roll the service")
	}
	// The group records the link.
	if got, _ := svc.GetEnvGroup(ctx, g.ID); !slices.Contains(got.ServiceLinks, "web") {
		t.Fatalf("group should record the link: %+v", got.ServiceLinks)
	}

	// Link is idempotent (no duplicate refs).
	if err := svc.LinkService(ctx, g.ID, "web"); err != nil {
		t.Fatalf("re-link: %v", err)
	}
	web = getApp(t, svc.Client, "web")
	if n := countString(web.Spec.EnvFromSecrets, envSecretName(g.ID)); n != 1 {
		t.Fatalf("re-link must not duplicate the ref, got %d", n)
	}

	// A group-var change rolls the linked service (new stamp).
	before := getApp(t, svc.Client, "web").Spec.RestartedAt
	svc.Base.Clock = func() time.Time { return time.Unix(2_000_000, 0).UTC() }
	if _, err := svc.SetEnvGroupVars(ctx, g.ID, []EnvVarView{{Key: "DB_URL", Value: "postgres://y"}}); err != nil {
		t.Fatalf("update group vars: %v", err)
	}
	if after := getApp(t, svc.Client, "web").Spec.RestartedAt; after == before {
		t.Error("updating group vars should roll the linked service")
	}

	// Unlink drops the refs and rolls.
	if err := svc.UnlinkService(ctx, g.ID, "web"); err != nil {
		t.Fatalf("UnlinkService: %v", err)
	}
	web = getApp(t, svc.Client, "web")
	if slices.Contains(web.Spec.EnvFromSecrets, envSecretName(g.ID)) || slices.Contains(web.Spec.FilesFromSecrets, filesSecretName(g.ID)) {
		t.Fatalf("unlink should drop the refs: env=%+v files=%+v", web.Spec.EnvFromSecrets, web.Spec.FilesFromSecrets)
	}
	if got, _ := svc.GetEnvGroup(ctx, g.ID); slices.Contains(got.ServiceLinks, "web") {
		t.Fatalf("group should forget the link: %+v", got.ServiceLinks)
	}
}

func TestEnvGroup_DeleteUnlinksAndCleansUp(t *testing.T) {
	store := newFakeStore()
	svc := newService(store, sampleApp("web"))
	ctx := context.Background()

	g, _ := svc.CreateEnvGroup(ctx, "shared")
	if err := svc.LinkService(ctx, g.ID, "web"); err != nil {
		t.Fatalf("LinkService: %v", err)
	}

	if err := svc.DeleteEnvGroup(ctx, g.ID); err != nil {
		t.Fatalf("DeleteEnvGroup: %v", err)
	}
	// Linked service detached first.
	web := getApp(t, svc.Client, "web")
	if slices.Contains(web.Spec.EnvFromSecrets, envSecretName(g.ID)) {
		t.Errorf("delete must detach linked services: %+v", web.Spec.EnvFromSecrets)
	}
	// Projection Secrets removed.
	var s corev1.Secret
	if err := svc.Client.Get(ctx, client.ObjectKey{Namespace: "default", Name: envSecretName(g.ID)}, &s); !apierrors.IsNotFound(err) {
		t.Errorf("group env Secret should be deleted, got %v", err)
	}
	// Store paths gone.
	if _, ok := store.m[metaPath(g.ID)]; ok {
		t.Error("group meta should be deleted from the store")
	}
	// Group no longer listed / gettable.
	if _, err := svc.GetEnvGroup(ctx, g.ID); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("deleted group => ErrNotFound, got %v", err)
	}
}

func TestEnvGroup_Validation(t *testing.T) {
	svc := newService(newFakeStore())
	ctx := context.Background()
	if _, err := svc.CreateEnvGroup(ctx, "  "); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("blank name => ErrBadRequest, got %v", err)
	}
	g, _ := svc.CreateEnvGroup(ctx, "shared")
	_, err := svc.SetEnvGroupVars(ctx, g.ID, []EnvVarView{{Key: "bad key", Value: "topsecret"}})
	if !errors.Is(err, core.ErrBadRequest) || strings.Contains(err.Error(), "topsecret") {
		t.Errorf("invalid key must be ErrBadRequest without the value: %v", err)
	}
}

func TestEnvGroup_Authorization(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "c1", Method: "oauth2"})
	chk := &fakeChecker{allow: false}
	svc := &Service{Base: &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: chk}, Store: newFakeStore()}

	if _, err := svc.ListEnvGroups(ctx); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("list deny: %v", err)
	}
	if chk.lastRelation != core.RelCanView {
		t.Errorf("list checked %s, want can_view", chk.lastRelation)
	}
	if _, err := svc.CreateEnvGroup(ctx, "x"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("create deny: %v", err)
	}
	if chk.lastRelation != core.RelCanCreate {
		t.Errorf("create checked %s, want can_create", chk.lastRelation)
	}
	if _, err := svc.GetEnvGroupVar(ctx, "evg-00000000000000000000", "K"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("reveal deny: %v", err)
	}
	if chk.lastRelation != core.RelCanViewSensitive {
		t.Errorf("reveal checked %s, want can_view_sensitive", chk.lastRelation)
	}
}

func TestEnvGroup_Unconfigured503(t *testing.T) {
	svc := newService(nil)
	if _, err := svc.ListEnvGroups(context.Background()); !errors.Is(err, core.ErrSecretsUnavailable) {
		t.Errorf("nil store => ErrSecretsUnavailable, got %v", err)
	}
}

func countString(list []string, s string) int {
	n := 0
	for _, v := range list {
		if v == s {
			n++
		}
	}
	return n
}
