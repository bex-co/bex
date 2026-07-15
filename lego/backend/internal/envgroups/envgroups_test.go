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
	"github.com/graphql-go/graphql"
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

// ownedApp is sampleApp with an explicit owning workspace (core.LabelTenant) —
// w6/m24's cross-workspace link tests need a service whose workspace differs
// from (or matches) a group's.
func ownedApp(name, tenantID string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Labels = map[string]string{core.LabelTenant: tenantID}
	return a
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

	g, err := svc.CreateEnvGroup(ctx, "", "shared", "")
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
	all, err := svc.ListEnvGroups(ctx, "")
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

	g, _ := svc.CreateEnvGroup(ctx, "", "shared", "")
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
	g, _ := svc.CreateEnvGroup(ctx, "", "shared", "")
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
	svc.EnvironmentWorkspace = func(_ context.Context, environmentID string) (string, error) {
		if environmentID != "env-alpha" {
			return "", core.ErrNotFound
		}
		return "", nil
	}
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
	call("create_env_group", map[string]any{"name": "shared", "environmentId": "env-alpha"}, &group)
	if group.EnvironmentID != "env-alpha" {
		t.Fatalf("create_env_group environmentId = %q, want env-alpha", group.EnvironmentID)
	}
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

func TestGraphQL_CreateEnvGroupAcceptsEnvironmentID(t *testing.T) {
	svc := newService(newFakeStore())
	svc.EnvironmentWorkspace = func(_ context.Context, environmentID string) (string, error) {
		if environmentID != "env-alpha" {
			return "", core.ErrNotFound
		}
		return "", nil
	}
	field := svc.GraphQLMutation()["createEnvGroup"]
	out, err := field.Resolve(graphql.ResolveParams{
		Context: context.Background(),
		Args:    map[string]any{"name": "shared", "environmentId": "env-alpha"},
	})
	if err != nil {
		t.Fatalf("createEnvGroup: %v", err)
	}
	group := out.(EnvGroupView)
	if group.EnvironmentID != "env-alpha" {
		t.Fatalf("createEnvGroup environmentId = %q, want env-alpha", group.EnvironmentID)
	}
}

func TestEnvGroup_LinkProjectsAndUnlink(t *testing.T) {
	svc := newService(newFakeStore(), sampleApp("web"), sampleApp("api"))
	ctx := context.Background()

	g, _ := svc.CreateEnvGroup(ctx, "", "shared", "")
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

	g, _ := svc.CreateEnvGroup(ctx, "", "shared", "")
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
	if _, err := svc.CreateEnvGroup(ctx, "", "  ", ""); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("blank name => ErrBadRequest, got %v", err)
	}
	g, _ := svc.CreateEnvGroup(ctx, "", "shared", "")
	_, err := svc.SetEnvGroupVars(ctx, g.ID, []EnvVarView{{Key: "bad key", Value: "topsecret"}})
	if !errors.Is(err, core.ErrBadRequest) || strings.Contains(err.Error(), "topsecret") {
		t.Errorf("invalid key must be ErrBadRequest without the value: %v", err)
	}
}

func TestEnvGroup_Authorization(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "c1", Method: "oauth2"})
	chk := &fakeChecker{allow: false}
	svc := &Service{Base: &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: chk}, Store: newFakeStore()}

	if _, err := svc.ListEnvGroups(ctx, ""); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("list deny: %v", err)
	}
	if chk.lastRelation != core.RelCanView {
		t.Errorf("list checked %s, want can_view", chk.lastRelation)
	}
	if _, err := svc.CreateEnvGroup(ctx, "", "x", ""); !errors.Is(err, core.ErrForbidden) {
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
	if _, err := svc.ListEnvGroups(context.Background(), ""); !errors.Is(err, core.ErrSecretsUnavailable) {
		t.Errorf("nil store => ErrSecretsUnavailable, got %v", err)
	}
}

// --- w6/m24: workspace attribution + cross-tenant scoping ---------------------

// multiWorkspace is a core.WorkspaceResolver for callers who belong to MULTIPLE
// workspaces (mirrors apikeys' own multiWorkspace, w6/m18) — memberships[0] is
// the default (what Tenant returns absent an explicit core.WithWorkspace).
type multiWorkspace map[string][]string

func (f multiWorkspace) Tenant(_ context.Context, id core.Identity) (string, bool) {
	m := f[id.Subject]
	if len(m) == 0 {
		return "", false
	}
	return m[0], true
}

func (f multiWorkspace) IsMember(_ context.Context, id core.Identity, tenantID string) (bool, error) {
	for _, tid := range f[id.Subject] {
		if tid == tenantID {
			return true, nil
		}
	}
	return false, nil
}

func TestEnvGroup_CreateStampsOwnerAndListScopesToTargetWorkspace(t *testing.T) {
	// dana belongs to both tea-a (her default) and tea-b. Before this
	// milestone ListEnvGroups had no workspace filter at all and returned
	// every group in the shared store to any caller who could can_view their
	// own workspace.
	svc := &Service{
		Base:  &core.Base{Client: fakeClient(), Namespace: "default", Workspace: multiWorkspace{"dana": {"tea-a", "tea-b"}}},
		Store: newFakeStore(),
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})

	groupA, err := svc.CreateEnvGroup(ctx, "", "shared-a", "") // no ownerId => default, tea-a
	if err != nil {
		t.Fatalf("create shared-a: %v", err)
	}
	if groupA.OwnerID != "tea-a" || groupA.CreatedAt == "" || groupA.UpdatedAt == "" {
		t.Fatalf("shared-a shape: %+v", groupA)
	}
	groupB, err := svc.CreateEnvGroup(ctx, "tea-b", "shared-b", "")
	if err != nil {
		t.Fatalf("create shared-b: %v", err)
	}
	if groupB.OwnerID != "tea-b" {
		t.Fatalf("shared-b ownerId = %q, want tea-b", groupB.OwnerID)
	}

	listA, err := svc.ListEnvGroups(ctx, "")
	if err != nil || len(listA) != 1 || listA[0].ID != groupA.ID {
		t.Fatalf("list default (tea-a) = %+v err=%v, want exactly [%s]", listA, err, groupA.ID)
	}
	listB, err := svc.ListEnvGroups(ctx, "tea-b")
	if err != nil || len(listB) != 1 || listB[0].ID != groupB.ID {
		t.Fatalf("list tea-b = %+v err=%v, want exactly [%s]", listB, err, groupB.ID)
	}
}

func TestEnvGroup_EnvironmentMembershipValidatesWorkspaceAndPersists(t *testing.T) {
	resolver := multiWorkspace{"dana": {"tea-a", "tea-b"}}
	svc := &Service{
		Base:  &core.Base{Client: fakeClient(), Namespace: "default", Workspace: resolver},
		Store: newFakeStore(),
		EnvironmentWorkspace: func(_ context.Context, environmentID string) (string, error) {
			switch environmentID {
			case "env-alpha", "env-alpha-2":
				return "tea-a", nil
			case "env-bravo":
				return "tea-b", nil
			default:
				return "", core.ErrNotFound
			}
		},
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})

	g, err := svc.CreateEnvGroup(ctx, "tea-a", "shared", "env-alpha")
	if err != nil {
		t.Fatalf("create with environmentId: %v", err)
	}
	if g.EnvironmentID != "env-alpha" {
		t.Fatalf("created environmentId = %q, want env-alpha", g.EnvironmentID)
	}
	got, err := svc.GetEnvGroup(ctx, g.ID)
	if err != nil || got.EnvironmentID != "env-alpha" {
		t.Fatalf("read-back after create: %+v err=%v", got, err)
	}
	memberships, err := svc.ListEnvironmentMemberships(ctx, "tea-a")
	if err != nil || len(memberships) != 1 || memberships[0].ID != g.ID || memberships[0].EnvironmentID != "env-alpha" {
		t.Fatalf("membership projection after create: %+v err=%v", memberships, err)
	}

	if err := svc.SetEnvironmentID(ctx, g.ID, "env-alpha-2"); err != nil {
		t.Fatalf("move within workspace: %v", err)
	}
	got, _ = svc.GetEnvGroup(ctx, g.ID)
	if got.EnvironmentID != "env-alpha-2" {
		t.Fatalf("environmentId after move = %q, want env-alpha-2", got.EnvironmentID)
	}

	if err := svc.SetEnvironmentID(ctx, g.ID, "env-bravo"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("cross-workspace move: want ErrForbidden, got %v", err)
	}
	got, _ = svc.GetEnvGroup(ctx, g.ID)
	if got.EnvironmentID != "env-alpha-2" {
		t.Fatalf("refused move changed membership to %q", got.EnvironmentID)
	}

	if err := svc.SetEnvironmentID(ctx, g.ID, ""); err != nil {
		t.Fatalf("unassign: %v", err)
	}
	got, _ = svc.GetEnvGroup(ctx, g.ID)
	if got.EnvironmentID != "" {
		t.Fatalf("environmentId after unassign = %q, want empty", got.EnvironmentID)
	}
}

func TestEnvGroup_CreateWithEnvironmentRequiresEnvironmentService(t *testing.T) {
	svc := newService(newFakeStore())
	if _, err := svc.CreateEnvGroup(context.Background(), "", "shared", "env-alpha"); !errors.Is(err, core.ErrWorkspacesUnavailable) {
		t.Fatalf("create with environmentId and no environment service: want ErrWorkspacesUnavailable, got %v", err)
	}
}

func TestEnvGroup_GetAndRevealRefuseCrossWorkspace(t *testing.T) {
	store := newFakeStore()
	resolver := multiWorkspace{"dana": {"tea-a"}, "erin": {"tea-b"}}
	svcAs := func(subject string) (*Service, context.Context) {
		s := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: resolver}, Store: store}
		return s, core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
	}

	erinSvc, erinCtx := svcAs("erin")
	group, err := erinSvc.CreateEnvGroup(erinCtx, "", "bravo-secrets", "")
	if err != nil {
		t.Fatalf("erin create: %v", err)
	}
	if _, err := erinSvc.SetEnvGroupVar(erinCtx, group.ID, "TOKEN", "s3cret"); err != nil {
		t.Fatalf("erin seed var: %v", err)
	}

	danaSvc, danaCtx := svcAs("dana")
	if _, err := danaSvc.GetEnvGroup(danaCtx, group.ID); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("dana GetEnvGroup(bravo's group): want ErrForbidden, got %v", err)
	}
	if _, err := danaSvc.GetEnvGroupVar(danaCtx, group.ID, "TOKEN"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("dana GetEnvGroupVar(bravo's group): want ErrForbidden, got %v", err)
	}
	// Owner can still reach it.
	if _, err := erinSvc.GetEnvGroup(erinCtx, group.ID); err != nil {
		t.Errorf("erin GetEnvGroup(own group): %v", err)
	}
}

func TestEnvGroup_LinkRefusesForeignWorkspaceGroupEvenForDualMember(t *testing.T) {
	// dana administers BOTH tea-a and tea-b (a plausible real caller — an
	// owner with two workspaces). Linking tea-b's group into tea-a's service
	// must still be refused: membership+relation in both workspaces does not
	// license moving a workspace's secret values into another's Secrets.
	resolver := multiWorkspace{"dana": {"tea-a", "tea-b"}}
	svc := &Service{
		Base:  &core.Base{Client: fakeClient(ownedApp("web", "tea-a")), Namespace: "default", Workspace: resolver},
		Store: newFakeStore(),
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})

	groupB, err := svc.CreateEnvGroup(ctx, "tea-b", "bravo-secrets", "")
	if err != nil {
		t.Fatalf("create bravo group: %v", err)
	}
	if err := svc.LinkService(ctx, groupB.ID, "web"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("link tea-b's group into tea-a's service: want ErrForbidden, got %v", err)
	}
	if got, _ := svc.GetEnvGroup(ctx, groupB.ID); len(got.ServiceLinks) != 0 {
		t.Errorf("refused link must not record the service: %+v", got.ServiceLinks)
	}

	groupA, err := svc.CreateEnvGroup(ctx, "tea-a", "alpha-secrets", "")
	if err != nil {
		t.Fatalf("create alpha group: %v", err)
	}
	if err := svc.LinkService(ctx, groupA.ID, "web"); err != nil {
		t.Errorf("link tea-a's own group into tea-a's service: %v", err)
	}
}

func TestEnvGroup_MigratesLegacyOwnerlessGroupOnceStoreIsLive(t *testing.T) {
	store := newFakeStore()
	legacy := id.New(id.EnvGroup)
	// A group created before workspace attribution existed: meta with no
	// "workspace" key at all, written directly to bypass CreateEnvGroup.
	if err := store.Put(context.Background(), metaPath(legacy), map[string]string{
		"name": "legacy", "links": "",
	}); err != nil {
		t.Fatalf("seed legacy meta: %v", err)
	}

	// A caller in the platform's default (bootstrap) workspace can reach it —
	// the deterministic migration target — and the store now records it.
	defaultSvc := &Service{
		Base:  &core.Base{Client: fakeClient(), Namespace: "default", Workspace: multiWorkspace{"boot": {core.DefaultTenant}}},
		Store: store,
	}
	defaultCtx := core.WithIdentity(context.Background(), core.Identity{Subject: "boot", Method: "session"})
	got, err := defaultSvc.GetEnvGroup(defaultCtx, legacy)
	if err != nil {
		t.Fatalf("default-workspace caller should reach the migrated legacy group: %v", err)
	}
	if got.OwnerID != core.DefaultTenant {
		t.Errorf("legacy group ownerId = %q, want the migrated default tenant %q", got.OwnerID, core.DefaultTenant)
	}
	raw, _ := store.Get(context.Background(), metaPath(legacy))
	if raw["workspace"] != core.DefaultTenant {
		t.Errorf("migration should persist the assigned owner, got meta %+v", raw)
	}

	// A caller in an unrelated real workspace still can't reach it — the
	// migration assigns a real owner, it doesn't strand it open to everyone.
	otherSvc := &Service{
		Base:  &core.Base{Client: fakeClient(), Namespace: "default", Workspace: multiWorkspace{"outsider": {"tea-z"}}},
		Store: store,
	}
	otherCtx := core.WithIdentity(context.Background(), core.Identity{Subject: "outsider", Method: "session"})
	if _, err := otherSvc.GetEnvGroup(otherCtx, legacy); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("unrelated workspace caller: want ErrForbidden, got %v", err)
	}
}

func TestEnvGroup_StoreOffOmitsOwnerID(t *testing.T) {
	// No control-plane store (Workspace nil, the single-tenant/dev default):
	// groups aren't attributed to a distinct real workspace, so ownerId stays
	// unset — never faked — matching AppView.OwnerID's own convention.
	svc := newService(newFakeStore())
	g, err := svc.CreateEnvGroup(context.Background(), "", "shared", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if g.OwnerID != "" {
		t.Errorf("store-off ownerId = %q, want empty (never faked)", g.OwnerID)
	}
	all, err := svc.ListEnvGroups(context.Background(), "")
	if err != nil || len(all) != 1 {
		t.Fatalf("store-off list stays unfiltered: %+v err=%v", all, err)
	}
}

// TestEnvGroup_BlueprintSeamScopesToActingWorkspace locks in a real cross-tenant
// WRITE bug found while merging w6/m24 with the concurrently-landed w1/m35
// blueprint-apply seam (GroupNames/ApplyEnvGroup/LinkEnvGroup/findGroupByName):
// as first written, findGroupByName searched every workspace's groups for a
// NAME match with no scoping at all, so a workspace A blueprint apply naming a
// group "shared" would find, reuse, and silently overwrite workspace B's
// same-named group's secret values — and GroupNames' pre-flight would leak
// another workspace's group names into A's validation. All four now scope
// through boundWorkspace, matching ListEnvGroups.
func TestEnvGroup_BlueprintSeamScopesToActingWorkspace(t *testing.T) {
	resolver := multiWorkspace{"dana": {"tea-a"}, "erin": {"tea-b"}}
	store := newFakeStore()
	svcAs := func(subject string) (*Service, context.Context) {
		s := &Service{Base: &core.Base{Client: fakeClient(ownedApp("web-a", "tea-a")), Namespace: "default", Workspace: resolver}, Store: store}
		return s, core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
	}

	// erin (tea-b) pre-creates a group named "shared" with a secret value.
	erinSvc, erinCtx := svcAs("erin")
	bravoGroup, err := erinSvc.CreateEnvGroup(erinCtx, "", "shared", "")
	if err != nil {
		t.Fatalf("erin create shared: %v", err)
	}
	if err := erinSvc.ApplyEnvGroup(erinCtx, "shared", map[string]string{"DB_URL": "postgres://bravo"}, nil); err != nil {
		t.Fatalf("erin seed shared: %v", err)
	}

	// dana (tea-a) never sees tea-b's "shared" in a pre-flight name check.
	danaSvc, danaCtx := svcAs("dana")
	names, err := danaSvc.GroupNames(danaCtx)
	if err != nil {
		t.Fatalf("dana GroupNames: %v", err)
	}
	if slices.Contains(names, "shared") {
		t.Fatalf("dana's GroupNames leaked tea-b's group: %+v", names)
	}

	// dana's blueprint apply of a SAME-NAMED "shared" group must create her OWN
	// group, never touch/reuse tea-b's.
	if err := danaSvc.ApplyEnvGroup(danaCtx, "shared", map[string]string{"DB_URL": "postgres://alpha"}, nil); err != nil {
		t.Fatalf("dana apply shared: %v", err)
	}
	alphaGID, _, found, err := danaSvc.findGroupByName(danaCtx, "shared")
	if err != nil || !found {
		t.Fatalf("dana findGroupByName(shared): found=%v err=%v", found, err)
	}
	if alphaGID == bravoGroup.ID {
		t.Fatal("dana's apply reused tea-b's group id — cross-tenant collision")
	}
	bravoVal, err := erinSvc.GetEnvGroupVar(erinCtx, bravoGroup.ID, "DB_URL")
	if err != nil || bravoVal.Value != "postgres://bravo" {
		t.Fatalf("tea-b's DB_URL was overwritten by dana's apply: %+v err=%v", bravoVal, err)
	}

	// dana's LinkEnvGroup("shared", "web-a") links HER group (found via the
	// scoped search), not tea-b's, and a foreign-name search reports "unknown"
	// rather than matching cross-tenant.
	if err := danaSvc.LinkEnvGroup(danaCtx, "shared", "web-a"); err != nil {
		t.Fatalf("dana LinkEnvGroup(shared): %v", err)
	}
	got, err := danaSvc.GetEnvGroup(danaCtx, alphaGID)
	if err != nil || !slices.Contains(got.ServiceLinks, "web-a") {
		t.Fatalf("dana's own group should record the link: %+v err=%v", got, err)
	}
	if bravoGot, err := erinSvc.GetEnvGroup(erinCtx, bravoGroup.ID); err != nil || slices.Contains(bravoGot.ServiceLinks, "web-a") {
		t.Fatalf("tea-b's group must not record dana's link: %+v err=%v", bravoGot, err)
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
