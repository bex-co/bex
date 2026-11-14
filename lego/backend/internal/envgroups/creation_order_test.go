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
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/rollout"
)

// Between two linked environment groups the most recently CREATED one wins, as
// Render documents (w4/m355): every link writer keeps a service's group refs
// oldest-created first, so relinking an older group never flips the value the
// service runs. Round 247 linked A, then B, unlinked A and relinked it, and the
// service ran A's values.

// twoGroups creates A, then B, so B is the newer.
func twoGroups(t *testing.T, svc *Service) (older, newer string) {
	t.Helper()
	ctx := testCtx()
	a, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "a"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "b"})
	if err != nil {
		t.Fatal(err)
	}
	return a.ID, b.ID
}

func wantCreationOrder(t *testing.T, env, files []string, older, newer string) {
	t.Helper()
	if want := []string{envSecretName(older), envSecretName(newer)}; !slices.Equal(env, want) {
		t.Fatalf("envFromSecrets = %v, want %v (newer group last)", env, want)
	}
	if want := []string{filesSecretName(older), filesSecretName(newer)}; !slices.Equal(files, want) {
		t.Fatalf("filesFromSecrets = %v, want %v (newer group last)", files, want)
	}
}

func TestLinkServiceKeepsGroupsInCreationOrder(t *testing.T) {
	ctx := testCtx()
	svc := newService(newFakeStore(), sampleApp("web"))
	older, newer := twoGroups(t, svc)
	link := func(gid string) {
		t.Helper()
		if err := svc.LinkService(ctx, gid, "web"); err != nil {
			t.Fatalf("link %s: %v", gid, err)
		}
	}

	link(newer)
	link(older) // linked last, created first: must not win
	a := getApp(t, svc.Client, "web")
	wantCreationOrder(t, a.Spec.EnvFromSecrets, a.Spec.FilesFromSecrets, older, newer)

	if err := svc.UnlinkService(ctx, older, "web"); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	link(older)
	a = getApp(t, svc.Client, "web")
	wantCreationOrder(t, a.Spec.EnvFromSecrets, a.Spec.FilesFromSecrets, older, newer)
}

// An Auto-Deploy-off service stages both links (w4/m239), and the release that
// ships them puts the newer group last.
func TestStagedLinksOnAGatedServiceShipInCreationOrder(t *testing.T) {
	ctx := testCtx()
	svc := newService(newFakeStore(), repoApp("off", false))
	older, newer := twoGroups(t, svc)
	for _, gid := range []string{newer, older} {
		if err := svc.LinkService(ctx, gid, "off"); err != nil {
			t.Fatalf("link %s: %v", gid, err)
		}
	}
	a := getApp(t, svc.Client, "off")
	if len(a.Spec.EnvFromSecrets) != 0 {
		t.Fatalf("a gated link wrote the spec: %v", a.Spec.EnvFromSecrets)
	}
	env, files := rollout.EffectiveGroupSecrets(a)
	wantCreationOrder(t, env, files, older, newer)
	rollout.PrepareGroupRefsForRelease(a)
	wantCreationOrder(t, a.Spec.EnvFromSecrets, a.Spec.FilesFromSecrets, older, newer)
}

// A Blueprint-created service links its groups before it exists; the order
// it names them in does not decide the winner either.
func TestInitialGroupsAreLinkedInCreationOrder(t *testing.T) {
	ctx := testCtx()
	svc := newService(newFakeStore())
	older, newer := twoGroups(t, svc)
	app := sampleApp("web")
	if err := svc.WithInitialEnvGroups(ctx, []string{"b", "a"}, app, func() error {
		return svc.Client.Create(context.Background(), app.DeepCopy())
	}, nil); err != nil {
		t.Fatalf("initial groups: %v", err)
	}
	wantCreationOrder(t, app.Spec.EnvFromSecrets, app.Spec.FilesFromSecrets, older, newer)
}

// The service's own env Secret is a separate field the operator always puts
// after every group, and its own files Secret keeps its slot: linking groups
// never moves either (w2/m94).
func TestLinkingLeavesTheServicesOwnSecretsAlone(t *testing.T) {
	ctx := testCtx()
	app := sampleApp("web")
	app.Spec.EnvFromSecret = "web-env"
	app.Spec.FilesFromSecrets = []string{"web-files"}
	svc := newService(newFakeStore(), app)
	older, newer := twoGroups(t, svc)
	for _, gid := range []string{newer, older} {
		if err := svc.LinkService(ctx, gid, "web"); err != nil {
			t.Fatalf("link %s: %v", gid, err)
		}
	}
	a := getApp(t, svc.Client, "web")
	if a.Spec.EnvFromSecret != "web-env" {
		t.Fatalf("own env Secret = %q", a.Spec.EnvFromSecret)
	}
	if want := []string{"web-files", filesSecretName(older), filesSecretName(newer)}; !slices.Equal(a.Spec.FilesFromSecrets, want) {
		t.Fatalf("filesFromSecrets = %v, want %v", a.Spec.FilesFromSecrets, want)
	}
}
