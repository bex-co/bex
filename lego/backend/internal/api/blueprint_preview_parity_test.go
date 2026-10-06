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

package api

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// takenNames is the service store's view a stack create's plan reads: names
// already used by a service (a display name included) and hosts some service
// already claims, verified or pending. Every other IntentStore method is the
// nil interface's, so a preview that reached a write would panic.
type takenNames struct {
	apps.IntentStore
	names, hosts []string
}

func (s takenNames) ServiceNameTaken(_ context.Context, _, _, name string) (bool, error) {
	return slices.Contains(s.names, name), nil
}

func (s takenNames) DomainHostsClaimed(_ context.Context, hosts []string) ([]string, error) {
	var claimed []string
	for _, host := range hosts {
		if slices.Contains(s.hosts, host) {
			claimed = append(claimed, host)
		}
	}
	return claimed, nil
}

// mismatchedCredential is a workspace whose one stored registry credential,
// "ghcr", is for a host the declared image does not pull from.
type mismatchedCredential struct{ apps.PullSecretSource }

func (mismatchedCredential) FindCredentialIDByName(_ context.Context, _, name string) (string, bool, error) {
	return "rgc-ghcr", name == "ghcr", nil
}

// ValidatePullSecret refuses only the resolved credential, so a plan that
// skipped resolving the manifest's name to it would pass where the apply fails.
func (mismatchedCredential) ValidatePullSecret(_ context.Context, _, _ string, credentialID *string) error {
	if credentialID == nil || *credentialID != "rgc-ghcr" {
		return nil
	}
	return core.NewBadRequestError("REGISTRY_CREDENTIAL_HOST_MISMATCH", "registry credential rgc-ghcr is for ghcr.io, not docker.io", nil)
}

// TestBlueprintPreviewAnswersExactlyTheApply is w5/m126's matrix: a refusal a
// Blueprint apply meets at a resource's create, its preview reports at that
// resource, writing nothing. The apply runs second, so the preview cannot lean
// on anything it wrote.
func TestBlueprintPreviewAnswersExactlyTheApply(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})
	cases := []struct {
		name     string
		atCap    bool
		store    takenNames
		creds    apps.PullSecretSource
		path     string // where preview locates the refusal
		manifest string
	}{
		{"a new service at the count cap", true, takenNames{}, nil, "services[0]", `services:
  - type: web
    name: capped
    runtime: image
    image:
      url: nginx:1
`},
		{"a new service with a CRD-invalid field", false, takenNames{}, nil, "services[0]", `services:
  - type: web
    name: invalid
    runtime: image
    image:
      url: nginx:1
    startCommand: crd-invalid
`},
		{"a new service named as another service is displayed", false, takenNames{names: []string{"shop"}}, nil, "services[0].name", `services:
  - type: web
    name: shop
    runtime: image
    image:
      url: nginx:1
`},
		{"a new service declaring a host another service claims", false, takenNames{hosts: []string{"shop.example.com"}}, nil, "services[0].domains[0]", `services:
  - type: web
    name: storefront
    runtime: image
    image:
      url: nginx:1
    domains:
      - shop.example.com
`},
		{"a new Postgres at the count cap", true, takenNames{}, nil, "databases[0]", `databases:
  - name: capped-db
    plan: free
`},
		{"a new Postgres with a CRD-invalid field", false, takenNames{}, nil, "databases[0].name", `databases:
  - name: crd-invalid
    plan: free
`},
		{"a new Key Value at the count cap", true, takenNames{}, nil, "services[0]", `services:
  - type: keyvalue
    name: capped-cache
    plan: free
    ipAllowList: []
`},
		{"a new Key Value with a CRD-invalid field", false, takenNames{}, nil, "services[0].name", `services:
  - type: keyvalue
    name: crd-invalid
    plan: free
    ipAllowList: []
`},
		{"a new service with a registry credential that cannot pull its image", false, takenNames{}, mismatchedCredential{}, "services[0]", `services:
  - type: web
    name: private
    runtime: image
    image:
      url: docker.io/acme/private:1
      creds:
        fromRegistryCreds:
          name: ghcr
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newParityFixture(&fakeChecker{allow: true}, false)
			f.adm.atCap = tc.atCap
			if tc.store.names != nil || tc.store.hosts != nil {
				f.apps.Store = tc.store
			}
			f.apps.RegistryCreds = tc.creds
			preview, err := f.apps.ValidateBlueprint(ctx, "tea-a", tc.manifest, "")
			if err != nil {
				t.Fatalf("preview: %v", err)
			}
			if f.adm.persisted != 0 {
				t.Fatalf("preview persisted %d writes", f.adm.persisted)
			}
			_, applyErr := f.apps.DeployStack(ctx, apps.DeployRequest{OwnerID: "tea-a", Manifest: tc.manifest})
			if applyErr == nil {
				t.Fatal("the apply succeeded; the case does not exercise a refusal")
			}
			if preview.Valid || len(preview.Errors) != 1 {
				t.Fatalf("preview = valid %v, errors %+v; want the apply's refusal: %v", preview.Valid, preview.Errors, applyErr)
			}
			got := preview.Errors[0]
			want := applyErr.Error()
			for _, sentinel := range []error{core.ErrBadRequest, core.ErrConflict} {
				want = strings.TrimPrefix(want, sentinel.Error()+": ")
			}
			var coded *core.CodedError
			wantCode := ""
			if errors.As(applyErr, &coded) {
				wantCode = coded.Code
			}
			if got.Error != want || got.Code != wantCode {
				t.Fatalf("preview refusal = %q (code %q)\napply refusal   = %q (code %q)", got.Error, got.Code, want, wantCode)
			}
			if got.Path == nil || *got.Path != tc.path {
				t.Fatalf("preview refusal path = %v, want %s", got.Path, tc.path)
			}
		})
	}
}

// TestBlueprintPreviewAnswersTheStackWideCountCap covers the gate that judges a
// Blueprint as a whole (w5/m126): the count cap across the resources it
// creates, which no single create's dry-run can see.
func TestBlueprintPreviewAnswersTheStackWideCountCap(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})
	f := newParityFixture(&fakeChecker{allow: true}, false)
	// web already counts against a cap of 2, so one more fits.
	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: core.TenantQuotaName, Namespace: "tea-a"},
		Status: corev1.ResourceQuotaStatus{
			Hard: corev1.ResourceList{store.AppsQuotaCountKey: resource.MustParse("2")},
			Used: corev1.ResourceList{store.AppsQuotaCountKey: resource.MustParse("1")},
		},
	}
	if err := f.apps.Client.Create(ctx, quota); err != nil {
		t.Fatal(err)
	}
	f.adm.persisted = 0
	f.adm.limits = map[string]int{store.AppsQuotaCountKey: 2}
	f.adm.used = map[string]int{store.AppsQuotaCountKey: 1}
	three := `services:
  - type: web
    name: first
    runtime: image
    image:
      url: nginx:1
  - type: web
    name: second
    runtime: image
    image:
      url: nginx:1
  - type: web
    name: third
    runtime: image
    image:
      url: nginx:1
`
	got, err := f.apps.ValidateBlueprint(ctx, "tea-a", three, "")
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if f.adm.persisted != 0 {
		t.Fatalf("preview persisted %d writes", f.adm.persisted)
	}
	_, applyErr := f.apps.DeployStack(ctx, apps.DeployRequest{OwnerID: "tea-a", Manifest: three})
	if applyErr == nil || f.adm.persisted != 1 {
		t.Fatalf("apply = %v after %d creates; want the second create refused", applyErr, f.adm.persisted)
	}
	if got.Valid || len(got.Errors) != 1 {
		t.Fatalf("preview = valid %v, errors %+v; want the apply's refusal: %v", got.Valid, got.Errors, applyErr)
	}
	if want := strings.TrimPrefix(applyErr.Error(), core.ErrBadRequest.Error()+": "); got.Errors[0].Error != want {
		t.Fatalf("preview refusal = %q\napply refusal   = %q", got.Errors[0].Error, want)
	}
	if path := got.Errors[0].Path; path == nil || *path != "services[1]" {
		t.Fatalf("preview refusal path = %v, want services[1], the first service past the cap", path)
	}
}

// TestBlueprintPreviewLeavesBillingToTheApply: a stack the payment gate would
// refuse previews clean (w5/m126). Its refusal is resolved by the payment
// onboarding the apply's 402 opens, as the protected-environment confirmation
// is by the dialog its refusal opens; marked invalid, the dashboard would
// disable the very Deploy and Sync whose refusal starts that flow.
func TestBlueprintPreviewLeavesBillingToTheApply(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})
	f := newParityFixture(&fakeChecker{allow: true}, false)
	paid := `services:
  - type: web
    name: paid
    runtime: image
    image:
      url: nginx:1
    plan: standard
`
	got, err := f.apps.ValidateBlueprint(ctx, "tea-a", paid, "")
	if err != nil || !got.Valid {
		t.Fatalf("preview = %+v, %v; want valid", got.Errors, err)
	}
	if _, err := f.apps.DeployStack(ctx, apps.DeployRequest{OwnerID: "tea-a", Manifest: paid}); !errors.Is(err, core.ErrPaymentRequired) {
		t.Fatalf("apply = %v, want the payment gate's 402", err)
	}
}

// TestBlueprintPreviewThatPassesWritesNothing: a Blueprint every create plan
// accepts previews valid, with its create plan, and persists nothing — the
// half of "stopped before its first write" the refusals above cannot show.
func TestBlueprintPreviewThatPassesWritesNothing(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})
	f := newParityFixture(&fakeChecker{allow: true}, false)
	manifest := `databases:
  - name: fresh-db
    plan: free
services:
  - type: web
    name: fresh
    runtime: image
    image:
      url: nginx:1
  - type: keyvalue
    name: fresh-cache
    plan: free
    ipAllowList: []
`
	got, err := f.apps.ValidateBlueprint(ctx, "tea-a", manifest, "")
	if err != nil || !got.Valid {
		t.Fatalf("preview = %+v, %v; want valid", got.Errors, err)
	}
	if f.adm.persisted != 0 {
		t.Fatalf("preview persisted %d writes", f.adm.persisted)
	}
	creates := 0
	for _, action := range got.Plan.Actions {
		if action.Operation == apps.BlueprintPlanCreate {
			creates++
		}
	}
	if creates != 3 {
		t.Fatalf("plan = %+v, want the three creates", got.Plan.Actions)
	}
}

// TestBlueprintDatastoreCreateEnsuresTheWorkspaceNamespace: a Blueprint's
// Postgres and Key Value creates take the interactive creates' path (w5/m126),
// so a workspace whose namespace the reconciler has not converged yet gets it
// before the CR lands there instead of a namespace NotFound 500 (w2/026).
// Datastores apply before services, so a new workspace's first Blueprint met
// it on its first database.
func TestBlueprintDatastoreCreateEnsuresTheWorkspaceNamespace(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})
	f := newParityFixture(&fakeChecker{allow: true}, false)
	var ensured []string
	f.apps.EnsureNamespaces = func(_ context.Context, workspace string) error {
		ensured = append(ensured, workspace)
		return nil
	}
	manifest := `databases:
  - name: first-db
    plan: free
services:
  - type: keyvalue
    name: first-cache
    plan: free
    ipAllowList: []
`
	if _, err := f.apps.DeployStack(ctx, apps.DeployRequest{OwnerID: "tea-a", Manifest: manifest}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !slices.Equal(ensured, []string{"tea-a", "tea-a"}) {
		t.Fatalf("namespaces ensured = %v, want tea-a before each datastore create", ensured)
	}
}
