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

// blueprint_ownership_test.go covers w8/m23: ownership stamping on blueprint
// apply, the cross-blueprint conflict refusal + takeover transfer, preview
// surfacing, webhook non-takeover, and disconnect clearing.

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const ownershipManifest = `services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1}
`

func ownershipService(t *testing.T, blueprints ...store.Blueprint) (*Service, *fakeBlueprintStore) {
	t.Helper()
	fs := newFakeBlueprintStore(blueprints...)
	svc := &Service{
		Base:       &core.Base{Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Blueprints: fs,
		GitFetcher: fakeBlueprintFetcher{contents: ownershipManifest, sha: "abc1234"},
	}
	return svc, fs
}

func ownershipCtx() context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})
}

func appOwner(t *testing.T, svc *Service, name string) string {
	t.Helper()
	var apps appv1alpha1.AppList
	if err := svc.Client.List(context.Background(), &apps); err != nil {
		t.Fatalf("list apps: %v", err)
	}
	for i := range apps.Items {
		if core.AppPublicName(&apps.Items[i]) == name {
			return apps.Items[i].Labels[core.LabelBlueprint]
		}
	}
	t.Fatalf("app %s not found", name)
	return ""
}

func TestBlueprintOwnershipStampAndConflictAndTakeover(t *testing.T) {
	svc, fs := ownershipService(t)
	ctx := ownershipCtx()

	// Blueprint A creates the service — the resource is stamped with A.
	a, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/a", Branch: "main"})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	if owner := appOwner(t, svc, "web"); owner != a.ID {
		t.Fatalf("owner after A = %q, want %q", owner, a.ID)
	}

	// Blueprint B (different repo) naming the same service: refused pre-write
	// with the coded conflict naming A and the takeover phrase.
	_, err = svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/b", Branch: "main"})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "BLUEPRINT_RESOURCE_CONFLICT" {
		t.Fatalf("B create error = %v, want BLUEPRINT_RESOURCE_CONFLICT", err)
	}
	phrase := BlueprintTakeoverConfirmation(a.ID)
	if !strings.Contains(err.Error(), "retry with confirm=") || !strings.Contains(err.Error(), phrase) {
		t.Fatalf("conflict error must carry the takeover phrase: %v", err)
	}
	if owner := appOwner(t, svc, "web"); owner != a.ID {
		t.Fatalf("refused create must not change ownership, got %q", owner)
	}

	// The takeover confirmation transfers ownership to B and proceeds. The
	// refused attempt above left B's own row behind (admission precedes the
	// resource preflight), so this retry also exercises w4/m125's rule that
	// re-connecting the SAME path is not a replacement — otherwise a create
	// whose apply failed could never be retried, since one Confirm cannot
	// carry both a resource and a connection phrase.
	b, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/b", Branch: "main", Confirm: phrase})
	if err != nil {
		t.Fatalf("takeover create: %v", err)
	}
	if owner := appOwner(t, svc, "web"); owner != b.ID {
		t.Fatalf("owner after takeover = %q, want %q", owner, b.ID)
	}

	// A's own re-sync now conflicts the other way (B owns it).
	if _, err := svc.SyncBlueprint(ctx, a.ID, "tea-a", "", "", nil); err == nil {
		t.Fatal("A's sync after takeover must conflict")
	}
	_ = fs
}

func TestBlueprintOwnershipPreviewSurfacesConflict(t *testing.T) {
	svc, _ := ownershipService(t)
	ctx := ownershipCtx()
	a, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/a", Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}

	// Previewing a DIFFERENT repo that names the owned service reports the
	// conflict as a validation entry (no verb error).
	p, err := svc.PreviewBlueprint(ctx, "tea-a", "https://github.com/acme/b", "main", "", "")
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if p.Validation == nil || p.Validation.Valid || len(p.Validation.Errors) == 0 ||
		p.Validation.Errors[0].Code != "BLUEPRINT_RESOURCE_CONFLICT" {
		t.Fatalf("preview validation = %+v, want BLUEPRINT_RESOURCE_CONFLICT entry", p.Validation)
	}

	// The owning blueprint's own preview does not conflict with itself.
	p, err = svc.PreviewBlueprint(ctx, "tea-a", "https://github.com/acme/a", "main", "", a.ID)
	if err != nil || p.Validation == nil || !p.Validation.Valid {
		t.Fatalf("self preview must stay valid: %+v err=%v", p.Validation, err)
	}
	_ = a
}

func TestBlueprintOwnershipWebhookNeverTakesOver(t *testing.T) {
	svc, fs := ownershipService(t)
	ctx := ownershipCtx()
	a, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/a", Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/b", Branch: "main", Confirm: BlueprintTakeoverConfirmation(a.ID)})
	if err != nil {
		t.Fatal(err)
	}

	// A push-triggered auto-sync of A (no confirm) must record an error run,
	// not silently take the resource back.
	svc.triggerBlueprintSync(core.WithWorkspace(ctx, "tea-a"), "tea-a", "https://github.com/acme/a", "main")
	if owner := appOwner(t, svc, "web"); owner != b.ID {
		t.Fatalf("webhook sync must not take over, owner = %q", owner)
	}
	// The fake store doesn't persist sync runs; the recorded ERROR outcome is
	// visible on the blueprint's own status, which runSync updates.
	row, err := fs.GetBlueprint(context.Background(), a.ID, "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != store.BlueprintStatusError {
		t.Fatalf("webhook conflict blueprint status = %q, want %q", row.Status, store.BlueprintStatusError)
	}
}

func TestBlueprintOwnershipDisconnectClears(t *testing.T) {
	svc, _ := ownershipService(t)
	ctx := ownershipCtx()
	a, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/a", Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DisconnectBlueprint(ctx, a.ID, "tea-a"); err != nil {
		t.Fatal(err)
	}
	if owner := appOwner(t, svc, "web"); owner != "" {
		t.Fatalf("disconnect must clear ownership, got %q", owner)
	}
	// Unmanaged again: a new blueprint adopts freely.
	if _, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/c", Branch: "main"}); err != nil {
		t.Fatalf("adopt after disconnect: %v", err)
	}
}

func TestBlueprintResourceClaimRaceLoserCannotWrite(t *testing.T) {
	svc, fs := ownershipService(t)
	ctx := ownershipCtx()

	a, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/a", Branch: "main"})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	// Simulate a racing claim already held by A before B's write path runs.
	if err := fs.ClaimBlueprintResource(ctx, "tea-a", store.BlueprintClaimService, "web", a.ID, ""); err != nil {
		t.Fatalf("seed claim: %v", err)
	}

	ctxB := withDeployAuthority(ctx, DeployRequest{BlueprintID: "blp-b-race", Confirm: ""})
	if err := svc.claimBlueprintResourceName(ctxB, store.BlueprintClaimService, "web"); err == nil {
		t.Fatal("loser claim must conflict")
	} else {
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != "BLUEPRINT_RESOURCE_CONFLICT" {
			t.Fatalf("loser claim = %v, want BLUEPRINT_RESOURCE_CONFLICT", err)
		}
	}
	if owner := appOwner(t, svc, "web"); owner != a.ID {
		t.Fatalf("race must leave A as owner, got %q", owner)
	}
}

func TestBlueprintTakeoverRejectsInterveningOwner(t *testing.T) {
	svc, fs := ownershipService(t)
	ctx := ownershipCtx()
	a, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/a", Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{
		Repo: "https://github.com/acme/b", Branch: "main", Confirm: BlueprintTakeoverConfirmation(a.ID),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Stale confirmation naming A must not transfer from B to C.
	_, err = svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{
		Repo: "https://github.com/acme/c", Branch: "main", Confirm: BlueprintTakeoverConfirmation(a.ID),
	})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "BLUEPRINT_RESOURCE_CONFLICT" {
		t.Fatalf("stale takeover = %v, want BLUEPRINT_RESOURCE_CONFLICT", err)
	}
	if owner := appOwner(t, svc, "web"); owner != b.ID {
		t.Fatalf("intervening owner must stay B, got %q", owner)
	}
	_ = fs
}

// TestBlueprintResourceConflictNamesTheManifestKind (w5/098): a resource
// conflict's kind param is the manifest kind a client labels it by, as plan
// actions name it, while the message keeps the display wording. Every path that
// refuses agrees: the pre-write ownership scan, the durable claim, and the
// post-apply stamp, which used to name a Key Value "key_value".
func TestBlueprintResourceConflictNamesTheManifestKind(t *testing.T) {
	type want struct{ kind, wording string }
	expect := func(t *testing.T, err error, w want) {
		t.Helper()
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != "BLUEPRINT_RESOURCE_CONFLICT" || coded.Params["kind"] != w.kind ||
			!strings.HasPrefix(err.Error(), w.wording+" is managed by blueprint blp-a;") {
			t.Fatalf("conflict = %v (%+v), want kind %s and the wording %q", err, coded, w.kind, w.wording)
		}
	}
	postgres := want{"postgres", `database "orders"`}
	keyValue := want{"key_value", `key value "cache"`}

	t.Run("ownership scan", func(t *testing.T) {
		owned := map[string]string{core.LabelTenant: "tea-a", core.LabelBlueprint: "blp-a"}
		svc := &Service{Base: &core.Base{Client: fakeClient(
			&appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "dpg-1", Namespace: "default", Labels: owned}, Spec: appv1alpha1.DatabaseSpec{Name: "orders"}},
			&appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-1", Namespace: "default", Labels: owned}, Spec: appv1alpha1.KeyValueSpec{Name: "cache"}},
		), Namespace: "default"}}
		conflicts, err := svc.blueprintOwnershipConflicts(context.Background(), "tea-a", "blp-b", parsedStack{
			databases: []parsedDatabase{{name: "orders"}},
			keyValues: []parsedKeyValue{{name: "cache"}},
		}, nil, nil)
		if err != nil || len(conflicts) != 2 {
			t.Fatalf("conflicts = %+v, %v; want the Postgres and the Key Value", conflicts, err)
		}
		expect(t, blueprintOwnershipError(conflicts[0]), postgres)
		expect(t, blueprintOwnershipError(conflicts[1]), keyValue)
	})
	t.Run("durable claim", func(t *testing.T) {
		svc, fs := ownershipService(t)
		ctx := ownershipCtx()
		for kind, name := range map[store.BlueprintClaimKind]string{store.BlueprintClaimDatabase: "orders", store.BlueprintClaimKeyValue: "cache"} {
			if err := fs.ClaimBlueprintResource(ctx, "tea-a", kind, name, "blp-a", ""); err != nil {
				t.Fatal(err)
			}
		}
		ctxB := withDeployAuthority(ctx, DeployRequest{BlueprintID: "blp-b"})
		expect(t, svc.claimBlueprintResourceName(ctxB, store.BlueprintClaimDatabase, "orders"), postgres)
		expect(t, svc.claimBlueprintResourceName(ctxB, store.BlueprintClaimKeyValue, "cache"), keyValue)
	})
	t.Run("post-apply stamp", func(t *testing.T) {
		svc, fs := ownershipService(t)
		ctx := ownershipCtx()
		if err := fs.ClaimBlueprintResource(ctx, "tea-a", store.BlueprintClaimKeyValue, "cache", "blp-a", ""); err != nil {
			t.Fatal(err)
		}
		cache := &appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: "red-1", Namespace: "default", Labels: map[string]string{core.LabelTenant: "tea-a"}},
			Spec:       appv1alpha1.KeyValueSpec{Name: "cache"},
		}
		if err := svc.Client.Create(ctx, cache); err != nil {
			t.Fatal(err)
		}
		expect(t, svc.stampBlueprintOwnership(ctx, "blp-b", 0, "", parsedStack{keyValues: []parsedKeyValue{{name: "cache"}}}), keyValue)
	})
}

// datastoreListCounter counts the Database and KeyValue Lists a Service makes.
// An apply lists both kinds concurrently.
type datastoreListCounter struct {
	client.Client
	databases, keyValues atomic.Int32
}

func (c *datastoreListCounter) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	switch list.(type) {
	case *appv1alpha1.DatabaseList:
		c.databases.Add(1)
	case *appv1alpha1.KeyValueList:
		c.keyValues.Add(1)
	}
	return c.Client.List(ctx, list, opts...)
}

// TestABlueprintApplyAndReadEachReadTheClaimsOnce (w5/114): ownership cost one
// claim query per declared resource on every apply and Blueprint read, and
// deployParsedStack listed the workspace's datastores for its ownership
// preflight and again for itself. Each now reads the claims once, and a claim
// still outranks a stale CR label.
func TestABlueprintApplyAndReadEachReadTheClaimsOnce(t *testing.T) {
	const manifest = ownershipManifest + `  - name: api
    type: web
    runtime: image
    image: {url: nginx:1}
  - type: keyvalue
    name: cache
    plan: free
    ipAllowList: []
databases:
  - name: orders
    plan: free
`
	svc, fs := ownershipService(t)
	svc.GitFetcher = fakeBlueprintFetcher{contents: manifest, sha: "abc1234"}
	lists := &datastoreListCounter{Client: svc.Client}
	svc.Client = lists
	ctx := ownershipCtx()
	bp, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/a", Branch: "main"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	req := DeployRequest{Manifest: manifest, BlueprintID: bp.ID}
	st, _, err := compileStack(req)
	if err != nil {
		t.Fatal(err)
	}
	apply := func() error {
		fs.ownersReads, fs.ownerLookups = 0, 0
		lists.databases.Store(0)
		lists.keyValues.Store(0)
		_, err := svc.deployParsedStack(withDeployAuthority(ctx, req), req, st)
		return err
	}
	read := func() []BlueprintResource {
		fs.ownersReads, fs.ownerLookups = 0, 0
		view, err := svc.GetBlueprintByID(ctx, bp.ID, "tea-a")
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if fs.ownersReads != 1 || fs.ownerLookups != 0 {
			t.Errorf("the read read the workspace's claims %d times and looked up %d resources, want one read", fs.ownersReads, fs.ownerLookups)
		}
		return view.Resources
	}

	if err := apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if fs.ownersReads != 1 || fs.ownerLookups != 0 {
		t.Errorf("the apply read the workspace's claims %d times and looked up %d resources, want one read", fs.ownersReads, fs.ownerLookups)
	}
	// Once before the writes, and once for the post-write ownership stamp.
	if databases, keyValues := lists.databases.Load(), lists.keyValues.Load(); databases > 2 || keyValues > 2 {
		t.Errorf("the apply listed Databases %d and Key Values %d times, want at most twice each", databases, keyValues)
	}
	if resources := read(); len(resources) != 4 {
		t.Fatalf("read %d resources, want the two services, the Postgres and the Key Value", len(resources))
	}

	// Another Blueprint takes the Postgres over. Its CR label still names bp,
	// but the claim decides: the read no longer lists it, and the preflight
	// refuses the re-apply before any write-time claim has to.
	if err := fs.ClaimBlueprintResource(ctx, "tea-a", store.BlueprintClaimDatabase, "orders", "blp-other", bp.ID); err != nil {
		t.Fatal(err)
	}
	if resources := read(); len(resources) != 3 {
		t.Fatalf("read %d resources after the takeover, want 3", len(resources))
	}
	var coded *core.CodedError
	if err := apply(); !errors.As(err, &coded) || coded.Params["owningBlueprintId"] != "blp-other" || fs.ownerLookups != 0 {
		t.Fatalf("re-apply = %v after %d lookups, want the preflight's refusal naming blp-other", err, fs.ownerLookups)
	}
}

// TestAServicesOnlyApplyListsNoDatastores (w5/114): the post-write ownership
// stamp lists a datastore kind only when the manifest declares one.
func TestAServicesOnlyApplyListsNoDatastores(t *testing.T) {
	svc, _ := ownershipService(t)
	lists := &datastoreListCounter{Client: svc.Client}
	svc.Client = lists
	ctx := ownershipCtx()
	bp, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/a", Branch: "main"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	req := DeployRequest{Manifest: ownershipManifest, BlueprintID: bp.ID}
	st, _, err := compileStack(req)
	if err != nil {
		t.Fatal(err)
	}
	lists.databases.Store(0)
	lists.keyValues.Store(0)
	if _, err := svc.deployParsedStack(withDeployAuthority(ctx, req), req, st); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if databases, keyValues := lists.databases.Load(), lists.keyValues.Load(); databases != 0 || keyValues != 0 {
		t.Errorf("a services-only apply listed Databases %d and Key Values %d times, want none", databases, keyValues)
	}
}

// TestEveryClaimKindHasItsOwnManifestKind (w5/115): each claim kind maps to a
// distinct manifest kind, so a new kind cannot ship without its mapping.
func TestEveryClaimKindHasItsOwnManifestKind(t *testing.T) {
	seen := map[BlueprintResourceKind]store.BlueprintClaimKind{}
	for _, kind := range store.BlueprintClaimKinds() {
		manifest := blueprintClaimResourceKind(kind)
		if !slices.Contains([]BlueprintResourceKind{BlueprintResourceService, BlueprintResourcePostgres, BlueprintResourceKeyValue}, manifest) {
			t.Errorf("claim kind %q maps to %q, not a manifest kind", kind, manifest)
		}
		if other, dup := seen[manifest]; dup {
			t.Errorf("claim kinds %q and %q both map to %q", other, kind, manifest)
		}
		seen[manifest] = kind
	}
}
