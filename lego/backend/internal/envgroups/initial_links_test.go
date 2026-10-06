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
	"reflect"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestInitialGroupsExistOnFirstAppWrite(t *testing.T) {
	for _, kind := range []string{appv1alpha1.TypeWebService, appv1alpha1.TypeStaticSite, appv1alpha1.TypeCronJob, appv1alpha1.TypeBackgroundWorker, appv1alpha1.TypePrivateService} {
		for _, auto := range []bool{false, true} {
			name := kind
			if auto {
				name += "/auto"
			}
			t.Run(name, func(t *testing.T) {
				ctx := context.Background()
				svc := newService(newFakeStore())
				rec := &recordingDeploys{}
				svc.Rollout = &rollout.Tracker{Store: rec}
				var ids []string
				for _, name := range []string{"first", "last"} {
					group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: name})
					if err != nil {
						t.Fatal(err)
					}
					ids = append(ids, group.ID)
				}
				app := managedApp("web")
				app.Spec.Type = kind
				app.Spec.Repo = "https://github.com/bex-co/bex"
				app.Spec.Image = ""
				app.Spec.AutoDeploy = auto
				app.Spec.EnvFromSecret = "service-owned-env"
				app.Spec.FilesFromSecrets = []string{"service-owned-files"}
				app.Spec.Env = []appv1alpha1.EnvVar{{Name: "OWNED", Value: "literal"}}
				expectedEnv := []string{envSecretName(ids[0]), envSecretName(ids[1])}
				expectedFiles := []string{"service-owned-files", filesSecretName(ids[0]), filesSecretName(ids[1])}
				created, completed := false, false
				err := svc.WithInitialEnvGroups(ctx, []string{"first", "last"}, app, func() error {
					if !reflect.DeepEqual(app.Spec.EnvFromSecrets, expectedEnv) || !reflect.DeepEqual(app.Spec.FilesFromSecrets, expectedFiles) {
						t.Fatalf("first observable App lacks ordered full refs: %+v", app.Spec)
					}
					for _, gid := range ids {
						m, err := svc.readMeta(ctx, gid)
						if err != nil || !reflect.DeepEqual(m.links, []string{core.AppPublicID(app)}) {
							t.Fatalf("membership not reserved before create: %+v err=%v", m, err)
						}
					}
					created = true
					return svc.Client.Create(ctx, app)
				}, func() error {
					if !created {
						t.Fatal("completion preceded App create")
					}
					completed = true
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				if !completed {
					t.Fatal("creation never completed")
				}
				firstVersion := app.ResourceVersion
				for _, name := range []string{"first", "last"} {
					if err := svc.LinkEnvGroup(ctx, name, "web"); err != nil {
						t.Fatal(err)
					}
				}
				got := getApp(t, svc.Client, "web")
				if got.ResourceVersion != firstVersion || got.Spec.RestartedAt != "" || len(rec.rows) != 0 {
					t.Fatalf("initial attachment dispatched replacement: version=%s -> %s restart=%q rows=%+v", firstVersion, got.ResourceVersion, got.Spec.RestartedAt, rec.rows)
				}
				if got.Spec.EnvFromSecret != "service-owned-env" || got.Spec.Env[0].Value != "literal" {
					t.Fatal("service-owned env changed")
				}
			})
		}
	}
}
