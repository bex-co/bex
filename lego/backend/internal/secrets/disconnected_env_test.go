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

package secrets

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/381: a service whose Blueprint was disconnected keeps the variables its
// last sync wrote, literals and references alike, as ordinary ones: Render's
// disconnect "does not _delete_ any services or other resources that were
// managed by the blueprint" (https://api-docs.render.com/reference/disconnect-blueprint),
// and nothing syncs them again. bex answered 409 ENV_VAR_MANIFEST_MANAGED to
// every write of them, forever.

// disconnectedApp is a service whose Blueprint declared a literal and two
// properties of one database. released says the disconnect recorded the
// release (apps.releaseBlueprintMarkers); otherwise the App is as a
// disconnect before w8/381 left it, the label gone and the last sync's record
// in place.
func disconnectedApp(name string, released bool) *appv1alpha1.App {
	a := sampleApp(name)
	declared := "DATABASE_URL,DB_USER,MESSAGE"
	if released {
		declared = ""
	}
	a.Annotations = map[string]string{core.AnnotationBlueprintEnv: declared}
	a.Spec.Env = []appv1alpha1.EnvVar{
		{Name: "MESSAGE", Value: "hello"},
		secretRef("DATABASE_URL", "dpg-x-app", "uri"),
		secretRef("DB_USER", "dpg-x-app", "username"),
	}
	return a
}

func envValues(t *testing.T, svc *Service, service string) map[string]string {
	t.Helper()
	list, err := svc.ListEnvVars(context.Background(), service)
	if err != nil {
		t.Fatalf("ListEnvVars: %v", err)
	}
	out := map[string]string{}
	for _, v := range list {
		if v.ManagedBy != "" {
			t.Errorf("%s reads managedBy %q on a service no Blueprint manages", v.Key, v.ManagedBy)
		}
		out[v.Key] = v.Value
	}
	return out
}

func TestADisconnectedServicesVariablesAreOrdinary(t *testing.T) {
	ctx := context.Background()
	uri := string(databaseSecret().Data["uri"])
	for _, tc := range []struct {
		name string
		act  func(svc *Service) error
		gone []string // off spec.Env, where they would shadow the store
		want map[string]string
	}{
		{"edit a literal", func(svc *Service) error {
			_, err := svc.SetEnvVar(ctx, "web", "MESSAGE", EnvVarWrite{Value: "edited"})
			return err
		}, []string{"MESSAGE"}, map[string]string{"MESSAGE": "edited", "DATABASE_URL": uri, "DB_USER": "app"}},
		{"edit a reference", func(svc *Service) error {
			_, err := svc.SetEnvVar(ctx, "web", "DATABASE_URL", EnvVarWrite{Value: "postgres://elsewhere"})
			return err
		}, []string{"DATABASE_URL"}, map[string]string{"MESSAGE": "hello", "DATABASE_URL": "postgres://elsewhere", "DB_USER": "app"}},
		{"delete a reference", func(svc *Service) error {
			return svc.DeleteEnvVar(ctx, "web", "DB_USER")
		}, []string{"DB_USER"}, map[string]string{"MESSAGE": "hello", "DATABASE_URL": uri}},
		{"bulk replace", func(svc *Service) error {
			_, err := svc.SetEnvVars(ctx, "web", []EnvVarView{{Key: "DATABASE_URL", Value: "postgres://bulk"}})
			return err
		}, []string{"MESSAGE", "DATABASE_URL", "DB_USER"}, map[string]string{"DATABASE_URL": "postgres://bulk"}},
		{"rename a reference", func(svc *Service) error {
			_, err := svc.PatchEnvironment(ctx, "web", EnvironmentPatch{
				SaveMode: SaveModeOnly,
				EnvVars:  []EnvVarPatch{{Key: "PGUSER", FromKey: "DB_USER"}},
			})
			return err
		}, []string{"DB_USER"}, map[string]string{"MESSAGE": "hello", "DATABASE_URL": uri, "PGUSER": "app"}},
	} {
		for _, released := range []bool{true, false} {
			name := tc.name
			if !released {
				name += " (disconnected before the release was recorded)"
			}
			t.Run(name, func(t *testing.T) {
				svc := newService(newFakeSecretStore(), disconnectedApp("web", released), databaseSecret())
				if got := envValues(t, svc, "web"); got["DATABASE_URL"] != uri || got["DB_USER"] != "app" || got["MESSAGE"] != "hello" {
					t.Fatalf("before the write: %v, want the literal and the Secret's values", got)
				}
				if err := tc.act(svc); err != nil {
					t.Fatalf("%s: %v", tc.name, err)
				}
				onSpec := specEnvNames(getApp(t, svc.Client, "web"))
				for _, name := range tc.gone {
					if _, still := onSpec[name]; still {
						t.Errorf("spec.Env still carries %s (%v)", name, onSpec)
					}
				}
				got := envValues(t, svc, "web")
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
}

// While the Blueprint is connected its references stay the manifest's, and one
// its latest sync no longer declares is retained like a literal (w21/m421).
func TestAConnectedServicesReferencesFollowItsManifest(t *testing.T) {
	ctx := context.Background()
	a := disconnectedApp("web", false)
	a.Labels = map[string]string{core.LabelBlueprint: "exs-connected"}
	a.Annotations[core.AnnotationBlueprintEnv] = "DATABASE_URL,MESSAGE"
	svc := newService(newFakeSecretStore(), a, databaseSecret())
	for _, key := range []string{"DATABASE_URL", "MESSAGE"} {
		_, err := svc.SetEnvVar(ctx, "web", key, EnvVarWrite{Value: "x"})
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != CodeEnvVarManifestManaged {
			t.Errorf("SetEnvVar(%s) on a declared variable = %v, want %s", key, err, CodeEnvVarManifestManaged)
		}
	}
	if err := svc.DeleteEnvVar(ctx, "web", "DB_USER"); err != nil {
		t.Fatalf("deleting the reference the manifest dropped: %v", err)
	}
	if _, still := specEnvNames(getApp(t, svc.Client, "web"))["DB_USER"]; still {
		t.Error("spec.Env still carries the deleted DB_USER")
	}
}
