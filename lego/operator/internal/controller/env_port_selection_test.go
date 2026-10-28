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

package controller

import (
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestEnvPortUsesSelectedInlineBeforeAndAfterMaterialization(t *testing.T) {
	for _, target := range []struct {
		name     string
		env      []appv1alpha1.EnvVar
		wantPort int
		wantEnv  int32
	}{
		{"no PORT", nil, 10000, 0},
		{"historical PORT", []appv1alpha1.EnvVar{{Name: "PORT", Value: "8080"}}, 8080, 8080},
	} {
		t.Run(target.name, func(t *testing.T) {
			ctx := context.Background()
			r, app := selectedReleaseFixture(t)
			app.Spec.Env = []appv1alpha1.EnvVar{{Name: "PORT", Value: "9999"}}
			if err := r.Update(ctx, app); err != nil {
				t.Fatal(err)
			}
			saved := app.Spec.DeepCopy()
			record, err := r.readRuntimeConfigRecord(ctx, app, 1)
			if err != nil {
				t.Fatal(err)
			}
			record.spec.Env = target.env
			if err := r.persistRuntimeConfigRecord(ctx, app, 1, record); err != nil {
				t.Fatal(err)
			}
			assertPort := func() {
				t.Helper()
				port, err := r.envPort(ctx, app, 10000)
				if err != nil || port != target.wantPort || app.Status.EnvPort != target.wantEnv {
					t.Fatalf("port=%d envPort=%d err=%v, want %d/%d", port, app.Status.EnvPort, err, target.wantPort, target.wantEnv)
				}
				if !reflect.DeepEqual(app.Spec, *saved) {
					t.Fatal("port selection changed the saved spec")
				}
			}
			assertPort()
			if _, err := r.readRuntimeConfigRecord(ctx, app, 3); !apierrors.IsNotFound(err) {
				t.Fatalf("PORT read materialized a release: %v", err)
			}
			if err := r.ensureReleaseConfigSnapshot(ctx, app); err != nil {
				t.Fatal(err)
			}
			assertPort()
			// The new record and snapshots own the selection after materialization.
			for _, name := range []string{appv1alpha1.ReleaseRecordName(app.Name, 1), "api-env-r1", "api-evg-shared-env-r1"} {
				if err := r.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: app.Namespace, Name: name}}); err != nil {
					t.Fatal(err)
				}
			}
			assertPort()
			persisted := &appv1alpha1.App{}
			if err := r.Get(ctx, client.ObjectKeyFromObject(app), persisted); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(persisted.Spec, *saved) {
				t.Fatal("historical selection rewrote the persisted saved spec")
			}
		})
	}
}

func TestEnvPortSelectedGroupsFollowRollbackAndRestartPolicy(t *testing.T) {
	for _, tc := range []struct {
		name              string
		preserve, deleted bool
		want              int
	}{
		{"rollback reads current group", false, false, 7000},
		{"restart reads historical group", true, false, 6000},
		{"rollback drops deleted group", false, true, 10000},
		{"restart retains deleted group", true, true, 6000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			r, app := selectedReleaseFixture(t)
			app.Spec.ReleaseConfig.PreserveGroupValues = tc.preserve
			if err := r.Update(ctx, app); err != nil {
				t.Fatal(err)
			}
			setPort := func(name, value string) {
				t.Helper()
				secret := &corev1.Secret{}
				if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: name}, secret); err != nil {
					t.Fatal(err)
				}
				secret.Data["PORT"] = []byte(value)
				if err := r.Update(ctx, secret); err != nil {
					t.Fatal(err)
				}
			}
			setPort("evg-shared-env", "7000")
			setPort("api-evg-shared-env-r1", "6000")
			if tc.deleted {
				if err := r.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: app.Namespace, Name: "evg-shared-env"}}); err != nil {
					t.Fatal(err)
				}
			}
			assertPort := func() {
				t.Helper()
				port, err := r.envPort(ctx, app, 10000)
				if err != nil || port != tc.want {
					t.Fatalf("selected group PORT=%d err=%v, want %d", port, err, tc.want)
				}
			}
			assertPort()
			if err := r.ensureReleaseConfigSnapshot(ctx, app); err != nil {
				t.Fatal(err)
			}
			if !tc.deleted {
				setPort("evg-shared-env", "8000")
			}
			// PORT must use the new selected snapshots, not an older status pointer
			// or a mutable group, even before the caller persists its next status.
			app.Status.ConfigSnapshotGeneration = 2
			assertPort()
		})
	}
}

func TestEnvPortNormalSavedInlineDoesNotRequireReleaseRecord(t *testing.T) {
	app := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Env: []appv1alpha1.EnvVar{{Name: "PORT", Value: "9090"}}}}
	port, err := (&AppReconciler{}).envPort(context.Background(), app, 10000)
	if err != nil || port != 9090 || app.Status.EnvPort != 9090 {
		t.Fatalf("saved inline PORT=%d/%d err=%v", port, app.Status.EnvPort, err)
	}
}
