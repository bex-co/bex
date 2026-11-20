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
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A Restart pulls the serving release with the registry credential it went
// live with, and a rollback its target with the target's, whatever the service
// binds now (w8/m529). On production a failed switch to a bad Docker Hub
// credential left Restart pulling the served nginx digest with that
// credential, which Docker Hub refused with 401 (w8/353).
func TestReleaseSelectionCarriesTheRecordedRegistryBinding(t *testing.T) {
	const (
		credentialA, secretA = "rgc-a", "web-registry-pull-rgc-a"
		credentialB, secretB = "rgc-b", "web-registry-pull-rgc-b"
	)
	none := &appv1alpha1.ReleaseRegistryBinding{}
	withA := &appv1alpha1.ReleaseRegistryBinding{CredentialID: credentialA, PullSecret: secretA}
	for _, tc := range []struct {
		name string
		// restart selects the serving generation 2; else a rollback selects 1.
		restart  bool
		recorded *appv1alpha1.ReleaseRegistryBinding
		// adopted is the spec a record written before w8/m529 holds.
		adopted *appv1alpha1.AppSpec
		want    *appv1alpha1.ReleaseRegistryBinding
		// refreshed is the credential re-materialized from its stored token.
		refreshed []string
	}{
		{name: "restart of a release that pulled anonymously", restart: true, recorded: none, want: none},
		{name: "restart of a release that pulled with another credential", restart: true, recorded: withA, want: withA, refreshed: []string{credentialA}},
		{name: "restart of a release whose credential was deleted since", restart: true,
			recorded: &appv1alpha1.ReleaseRegistryBinding{CredentialID: "rgc-deleted", PullSecret: "web-registry-pull-rgc-deleted"},
			want:     none, refreshed: []string{"rgc-deleted"}},
		{name: "restart of a release adopted anonymously before w8/m529", restart: true,
			adopted: &appv1alpha1.AppSpec{Image: "web:v2"}, want: none},
		{name: "restart of a release adopted with a credential before w8/m529 moves to its own Secret", restart: true,
			adopted: &appv1alpha1.AppSpec{Image: "web:v2", RegistryCredentialID: new(credentialA), ExternalRegistryPullSecret: "web-registry-pull"},
			want:    withA, refreshed: []string{credentialA}},
		{name: "restart of a release recorded with neither keeps the saved binding", restart: true},
		{name: "rollback to a deploy that pulled anonymously", recorded: none, want: none},
		{name: "rollback to a deploy that pulled with another credential", recorded: withA, want: withA, refreshed: []string{credentialA}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc, cl, d1 := rollbackFixture(t, false)
			preparer := &fakePullSecretPreparer{credentials: map[string]string{credentialA: secretA, credentialB: secretB}}
			svc.PullSecrets = preparer
			app := getApp(t, cl, "web")
			app.Spec.RegistryCredentialID, app.Spec.ExternalRegistryPullSecret = new(credentialB), secretB
			if err := cl.Update(ctx, app); err != nil {
				t.Fatal(err)
			}
			generation := int64(1)
			if tc.restart {
				generation = 2
			}
			raw, _ := json.Marshal(appv1alpha1.ReleaseRecordSpec{StartCommand: "./server", Registry: tc.recorded})
			data := map[string][]byte{appv1alpha1.ReleaseRecordSpecKey: raw}
			if tc.adopted != nil {
				data[appv1alpha1.ReleaseRecordAdoptedSpecKey], _ = json.Marshal(tc.adopted)
			}
			if err := cl.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: appv1alpha1.ReleaseRecordName("web", generation)}, Data: data}); err != nil {
				t.Fatal(err)
			}
			var err error
			if tc.restart {
				_, err = svc.Restart(ctx, "web")
			} else {
				_, err = svc.Rollback(ctx, "web", d1.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			got := getApp(t, cl, "web")
			selected := got.Spec.ReleaseConfig
			if selected == nil || selected.SourceGeneration != generation || !equality.Semantic.DeepEqual(selected.Registry, tc.want) {
				t.Fatalf("selection = %+v, want generation %d pulling with %+v", selected, generation, tc.want)
			}
			if !slices.Equal(preparer.refreshed, tc.refreshed) {
				t.Errorf("refreshed credentials = %v, want %v", preparer.refreshed, tc.refreshed)
			}
			if got.Spec.RegistryCredentialID == nil || *got.Spec.RegistryCredentialID != credentialB || got.Spec.ExternalRegistryPullSecret != secretB {
				t.Fatalf("the selection rewrote the saved binding: %v %q", got.Spec.RegistryCredentialID, got.Spec.ExternalRegistryPullSecret)
			}
			pulls := secretB
			if tc.want != nil {
				pulls = tc.want.PullSecret
			}
			if got.ReleasePullSecret() != pulls {
				t.Fatalf("the release pulls with %q, want %q", got.ReleasePullSecret(), pulls)
			}
		})
	}
}

// A Restart of a Restart keeps the original release's binding: the operator
// records the selection's binding in the restarted release's own record, which
// the next Restart reads back (ensureSelectedReleaseConfig).
func TestRestartOfARestartKeepsTheOriginalBinding(t *testing.T) {
	ctx := context.Background()
	svc, cl, _ := rollbackFixture(t, false)
	svc.PullSecrets = &fakePullSecretPreparer{credentials: map[string]string{"rgc-b": "web-registry-pull-rgc-b"}}
	for _, gen := range []int64{2, 3} {
		raw, _ := json.Marshal(appv1alpha1.ReleaseRecordSpec{Registry: &appv1alpha1.ReleaseRegistryBinding{}})
		if err := cl.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: appv1alpha1.ReleaseRecordName("web", gen)},
			Data: map[string][]byte{appv1alpha1.ReleaseRecordSpecKey: raw}}); err != nil {
			t.Fatal(err)
		}
	}
	app := getApp(t, cl, "web")
	app.Spec.RegistryCredentialID, app.Spec.ExternalRegistryPullSecret = new("rgc-b"), "web-registry-pull-rgc-b"
	if err := cl.Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	first, err := svc.Restart(ctx, "web")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Store.(*fakeStore).CloseDeploy(ctx, first.ID, store.DeployLive, "web:v2"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Restart(ctx, "web"); err != nil {
		t.Fatal(err)
	}
	selected := getApp(t, cl, "web").Spec.ReleaseConfig
	if selected == nil || selected.SourceGeneration != 3 || selected.Registry == nil || selected.Registry.PullSecret != "" {
		t.Fatalf("second restart = %+v, want generation 3 pulling anonymously", selected)
	}
}
