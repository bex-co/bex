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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/381: after a Blueprint was disconnected, its services' render.yaml
// variables answered 409 ENV_VAR_MANIFEST_MANAGED to every edit, delete and
// bulk replace, and reads still said managedBy: blueprint. Disconnect only
// dropped the label, so the last sync's record still declared every variable.
// Render's disconnect "does not _delete_ any services or other resources that
// were managed by the blueprint"
// (https://api-docs.render.com/reference/disconnect-blueprint): they become
// ordinary resources, and nothing syncs them again.

// disconnectEnvService is ownershipService with the env verbs wired to the
// same cluster and a store.
func disconnectEnvService(t *testing.T) (*Service, *secrets.Service) {
	t.Helper()
	svc, _ := ownershipService(t)
	svc.GitFetcher = fakeBlueprintFetcher{contents: ownershipEnvManifest, sha: "abc1234"}
	return svc, &secrets.Service{Base: svc.Base, Store: newMemKV()}
}

func manifestManaged(err error) bool {
	var coded *core.CodedError
	return errors.As(err, &coded) && coded.Code == secrets.CodeEnvVarManifestManaged
}

func serviceApp(t *testing.T, svc *Service, name string) *appv1alpha1.App {
	t.Helper()
	var apps appv1alpha1.AppList
	if err := svc.Client.List(context.Background(), &apps); err != nil {
		t.Fatal(err)
	}
	for i := range apps.Items {
		if core.AppPublicName(&apps.Items[i]) == name {
			return &apps.Items[i]
		}
	}
	t.Fatalf("service %s not found", name)
	return nil
}

func TestADisconnectedBlueprintsVariablesAreOrdinary(t *testing.T) {
	for _, tc := range []struct {
		name string
		act  func(ctx context.Context, sec *secrets.Service) error
		// gone is the variable the verb leaves off spec.Env, where it would
		// shadow the store; want is what each listed variable reads after.
		gone string
		want map[string]string
	}{
		{"edit", func(ctx context.Context, sec *secrets.Service) error {
			_, err := sec.SetEnvVar(ctx, "web", "MESSAGE", secrets.EnvVarWrite{Value: "edited"})
			return err
		}, "MESSAGE", map[string]string{"MESSAGE": "edited", "KEEPME": "1"}},
		{"delete", func(ctx context.Context, sec *secrets.Service) error {
			return sec.DeleteEnvVar(ctx, "web", "MESSAGE")
		}, "MESSAGE", map[string]string{"KEEPME": "1"}},
		{"bulk replace", func(ctx context.Context, sec *secrets.Service) error {
			_, err := sec.SetEnvVars(ctx, "web", []secrets.EnvVarView{{Key: "MESSAGE", Value: "bulk"}})
			return err
		}, "KEEPME", map[string]string{"MESSAGE": "bulk"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, sec := disconnectEnvService(t)
			ctx := ownershipCtx()
			bp, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/a", Branch: "main"})
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			// Connected, the manifest owns its variables (w4/m120).
			if err := tc.act(ctx, sec); !manifestManaged(err) {
				t.Fatalf("%s while connected = %v, want %s", tc.name, err, secrets.CodeEnvVarManifestManaged)
			}

			if err := svc.DisconnectBlueprint(ctx, bp.ID, "tea-a"); err != nil {
				t.Fatalf("disconnect: %v", err)
			}
			app := serviceApp(t, svc, "web")
			if got, ok := app.Annotations[core.AnnotationBlueprintEnv]; app.Labels[core.LabelBlueprint] != "" || !ok || got != "" {
				t.Fatalf("after disconnect: label %q, declared %q (recorded %v); want no label and nothing declared",
					app.Labels[core.LabelBlueprint], got, ok)
			}
			list, err := sec.ListEnvVars(ctx, "web")
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			for _, v := range list {
				if v.ManagedBy != "" {
					t.Errorf("after disconnect %s reads managedBy %q, want none", v.Key, v.ManagedBy)
				}
			}

			if err := tc.act(ctx, sec); err != nil {
				t.Fatalf("%s after disconnect: %v", tc.name, err)
			}
			for _, item := range serviceApp(t, svc, "web").Spec.Env {
				if item.Name == tc.gone {
					t.Errorf("spec.Env still carries %s: it would shadow the store", tc.gone)
				}
			}
			list, err = sec.ListEnvVars(ctx, "web")
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			got := map[string]string{}
			for _, v := range list {
				got[v.Key] = v.Value
			}
			if len(got) != len(tc.want) {
				t.Errorf("after %s: %v, want %v", tc.name, got, tc.want)
			}
			for key, value := range tc.want {
				if got[key] != value {
					t.Errorf("after %s %s = %q, want %q", tc.name, key, got[key], value)
				}
			}
		})
	}
}

// A sync that stops declaring a service releases it the way a disconnect
// does (w4/m125): its variables are ordinary from then on.
func TestAServiceASyncStopsDeclaringHasOrdinaryVariables(t *testing.T) {
	svc, sec := disconnectEnvService(t)
	two := ownershipEnvManifest + `  - name: other
    type: web
    runtime: image
    image: {url: nginx:1}
    envVars:
      - key: OTHER
        value: o
`
	svc.GitFetcher = fakeBlueprintFetcher{contents: two, sha: "abc1234"}
	ctx := ownershipCtx()
	bp, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/a", Branch: "main"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := sec.DeleteEnvVar(ctx, "other", "OTHER"); !manifestManaged(err) {
		t.Fatalf("delete while declared = %v, want %s", err, secrets.CodeEnvVarManifestManaged)
	}
	if _, err := svc.SyncBlueprint(ctx, bp.ID, "tea-a", ownershipEnvManifest, "", nil); err != nil {
		t.Fatalf("sync without other: %v", err)
	}
	if err := sec.DeleteEnvVar(ctx, "other", "OTHER"); err != nil {
		t.Fatalf("delete after the sync released other: %v", err)
	}
	// web is still the Blueprint's.
	if err := sec.DeleteEnvVar(ctx, "web", "MESSAGE"); !manifestManaged(err) {
		t.Fatalf("delete on the still-managed web = %v, want %s", err, secrets.CodeEnvVarManifestManaged)
	}
}

// The release also ends the fromService port records: the propagator would
// otherwise keep rewriting those variables and put back one the owner moved
// or deleted. What the Blueprint stamped for other reasons stays.
func TestADisconnectDropsTheServicesReferenceRecords(t *testing.T) {
	svc, _ := ownershipService(t)
	ctx := ownershipCtx()
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
		Name: "web", Namespace: "tea-a",
		Labels: map[string]string{core.LabelTenant: "tea-a", core.LabelBlueprint: "blp-a", serviceRefsLabel: "true", core.LabelServiceName: "web"},
		Annotations: map[string]string{
			core.AnnotationBlueprintEnv: "PEER_PORT",
			serviceRefsAnnotation:       `[{"key":"PEER_PORT","target":"peer","property":"port"}]`,
			initialDeployHookAnnotation: "make migrate",
		},
	}}
	if err := svc.Client.Create(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := svc.clearBlueprintOwnership(ctx, "tea-a", "blp-a"); err != nil {
		t.Fatalf("clear: %v", err)
	}
	var got appv1alpha1.App
	if err := svc.Client.Get(ctx, client.ObjectKeyFromObject(app), &got); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{core.LabelBlueprint, serviceRefsLabel} {
		if _, ok := got.Labels[label]; ok {
			t.Errorf("label %s survived the release", label)
		}
	}
	if _, ok := got.Annotations[serviceRefsAnnotation]; ok {
		t.Errorf("the port references survived the release: %s", got.Annotations[serviceRefsAnnotation])
	}
	if declared, ok := got.Annotations[core.AnnotationBlueprintEnv]; !ok || declared != "" {
		t.Errorf("declared env after the release = %q (recorded %v), want recorded and empty", declared, ok)
	}
	if got.Labels[core.LabelTenant] != "tea-a" || got.Annotations[initialDeployHookAnnotation] != "make migrate" {
		t.Errorf("the release touched what it does not own: labels %v, annotations %v", got.Labels, got.Annotations)
	}
}
