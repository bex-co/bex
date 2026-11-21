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
	"fmt"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/367: a service renamed from "web" kept "web" reserved, so a create under
// it answered 409 "already in use" while no service showed that name. As on
// Render, the name is free: the newcomer is shown as "web" under a suffixed
// resource name, the renamed service keeps its own, and by name "web" is the
// newcomer.
func TestCreateUnderARenamedServicesOldNameTakesTheName(t *testing.T) {
	renamed := tenantApp("web", "tea-1")
	renamed.Spec.DisplayName = "web-old"
	svc := danaService(renamed)
	ctx := ctxAs("dana")

	if available, err := svc.NameAvailable(ctx, "web"); err != nil || !available.Available {
		t.Fatalf("NameAvailable(web) after its holder's rename = %+v, %v; want available", available, err)
	}
	for _, dryRun := range []bool{true, false} {
		view, err := svc.Create(ctx, CreateRequest{Name: "web", Image: "nginx:2", OwnerID: "tea-1", DryRun: dryRun})
		if err != nil {
			t.Fatalf("create web (dry run %v) after its holder's rename: %v", dryRun, err)
		}
		if renderServiceName(view) != "web" || !strings.HasPrefix(view.Name, "web-") || len(view.Name) != len("web-")+4 {
			t.Fatalf("create (dry run %v) = name %q, resource %q; want web shown under web-xxxx", dryRun, renderServiceName(view), view.Name)
		}
	}
	got, err := svc.Get(ctx, "web")
	if err != nil || got.Name == "web" || renderServiceName(got) != "web" {
		t.Fatalf("GET web = %q shown as %q, %v; want the newcomer", got.Name, renderServiceName(got), err)
	}
	if old, err := svc.Get(ctx, "web-old"); err != nil || old.Name != "web" {
		t.Fatalf("GET web-old = %q, %v; want the renamed service, its resource name unchanged", old.Name, err)
	}
	if _, err := svc.Create(ctx, CreateRequest{Name: "web", Image: "nginx:3", OwnerID: "tea-1"}); err == nil || err.Error() != errServiceNameInUse("web").Error() {
		t.Fatalf("a second create of web = %v, want %v", err, errServiceNameInUse("web"))
	}
	if available, err := svc.NameAvailable(ctx, "web-old"); err != nil || available.Available {
		t.Fatalf("NameAvailable(web-old) = %+v, %v; want taken", available, err)
	}
}

// The label a create writes after it is checked before anything is written: a
// name another service is shown as refuses the create, rather than leaving a
// service behind under its suffixed resource name.
func TestCreateRefusesALabelAnotherServiceShowsBeforeWriting(t *testing.T) {
	renamed := tenantApp("web", "tea-a")
	renamed.Spec.DisplayName = "web-old"
	rec := &recordingStore{takenDisplayNames: map[string]bool{"web": true}}
	svc := &Service{
		Base:  &core.Base{Client: fakeClient(renamed), Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Store: rec, DomainTraffic: allowDomainTraffic{},
	}
	ctx := ctxAs("id-a")
	for _, dryRun := range []bool{true, false} {
		_, err := svc.Create(ctx, CreateRequest{Name: "web", Image: "nginx:2", DryRun: dryRun})
		if err == nil || err.Error() != errServiceNameInUse("web").Error() {
			t.Fatalf("create web (dry run %v) while another service shows it = %v, want %v", dryRun, err, errServiceNameInUse("web"))
		}
	}
	if len(rec.appCreates) != 0 {
		t.Fatalf("rows written for a refused create: %v", rec.appCreates)
	}
	// Once nobody shows "web", the same create lands under a suffixed
	// resource name and is labelled "web".
	delete(rec.takenDisplayNames, "web")
	if _, err := svc.Create(ctx, CreateRequest{Name: "web", Image: "nginx:2"}); err != nil {
		t.Fatalf("create web once it is free: %v", err)
	}
	if len(rec.appCreates) != 1 || !strings.HasPrefix(rec.appCreates[0].Name, "web-") ||
		len(rec.displayNameCalls) != 1 || rec.displayNameCalls[0].displayName != "web" {
		t.Fatalf("rows = %+v, labels = %+v; want one web-xxxx row labelled web", rec.appCreates, rec.displayNameCalls)
	}
}

// Clearing a label shows the creation name again, which a newcomer may now
// be shown as: the refusal names that name, never "", on the patch preflight
// and on the store's own refusal alike.
func TestClearingALabelWhoseNameIsShownElsewhereNamesIt(t *testing.T) {
	blank, want := "", errServiceNameInUse("immutable-id").Error()
	rec := &recordingStore{takenDisplayNames: map[string]bool{"immutable-id": true}}
	svc, _ := newService(rec, manage(displayNameApp("immutable-id"), "srv-1"))
	if _, err := svc.ApplyServicePatch(context.Background(), "immutable-id", ServicePatch{DisplayName: &blank}); err == nil || err.Error() != want {
		t.Errorf("patch clearing the label = %v, want %s", err, want)
	}
	rec = &recordingStore{err: fmt.Errorf("service name %q: %w", "immutable-id", store.ErrConflict)}
	svc, _ = newService(rec, manage(displayNameApp("immutable-id"), "srv-1"))
	if _, err := svc.SetDisplayName(context.Background(), "immutable-id", blank); err == nil || err.Error() != want {
		t.Errorf("SetDisplayName clearing the label = %v, want %s", err, want)
	}
}

// The suffixed resource name is a valid resource name within the cap.
func TestSuffixedResourceNameFitsTheCap(t *testing.T) {
	for _, name := range []string{"web", strings.Repeat("a", appv1alpha1.MaxResourceNameLength)} {
		got := suffixedResourceName(name)
		if !appv1alpha1.ValidResourceName(got) || !strings.HasPrefix(got, name[:min(len(name), appv1alpha1.MaxResourceNameLength-5)]+"-") {
			t.Errorf("suffixedResourceName(%q) = %q, want a valid name keeping as much of it as fits", name, got)
		}
	}
}
