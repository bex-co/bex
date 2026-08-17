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
	"strings"
	"testing"

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
		if appServiceName(&apps.Items[i]) == name {
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

	// The takeover confirmation transfers ownership to B and proceeds.
	b, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/b", Branch: "main", Confirm: phrase})
	if err != nil {
		t.Fatalf("takeover create: %v", err)
	}
	if owner := appOwner(t, svc, "web"); owner != b.ID {
		t.Fatalf("owner after takeover = %q, want %q", owner, b.ID)
	}

	// A's own re-sync now conflicts the other way (B owns it).
	if _, err := svc.SyncBlueprint(ctx, a.ID, "tea-a", "", ""); err == nil {
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
	p, err := svc.PreviewBlueprint(ctx, "tea-a", "https://github.com/acme/b", "main", "")
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if p.Validation == nil || p.Validation.Valid || len(p.Validation.Errors) == 0 ||
		p.Validation.Errors[0].Code != "BLUEPRINT_RESOURCE_CONFLICT" {
		t.Fatalf("preview validation = %+v, want BLUEPRINT_RESOURCE_CONFLICT entry", p.Validation)
	}

	// The owning blueprint's own preview does not conflict with itself.
	p, err = svc.PreviewBlueprint(ctx, "tea-a", "https://github.com/acme/a", "main", "")
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
