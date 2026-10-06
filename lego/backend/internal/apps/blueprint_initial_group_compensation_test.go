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
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type failingInitialGroups struct{ *envgroups.Service }

func (g failingInitialGroups) WithInitialEnvGroups(ctx context.Context, names []string, a *appv1alpha1.App, create, complete func() error) error {
	return g.Service.WithInitialEnvGroups(ctx, names, a, create, func() error {
		return errors.New("injected creation completion failure")
	})
}

type initialAppCaptureClient struct {
	client.Client
	first *appv1alpha1.App
}

func (c *initialAppCaptureClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if a, ok := obj.(*appv1alpha1.App); ok {
		c.first = a.DeepCopy()
	}
	return c.Client.Create(ctx, obj, opts...)
}

func TestInitialGroupCompletionFailureCompensatesAppAndOwnSecrets(t *testing.T) {
	ctx := context.Background()
	svc, cl := newService(nil)
	capture := &initialAppCaptureClient{Client: cl}
	svc.Client = capture
	kv := newMemKV()
	groups := &envgroups.Service{Base: svc.Base, Store: kv}
	group, err := groups.CreateEnvGroup(ctx, envgroups.CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	svc.EnvGroups = failingInitialGroups{Service: groups}
	svc.CreateSecrets = secrets.NewCreateSecretsSeeder(&secrets.Service{Base: svc.Base, Store: kv})
	a := sampleApp("new-service")
	req := CreateRequest{Name: a.Name, initialEnvGroups: []string{"shared"}}
	err = svc.writeInitialApp(ctx, req, a, createSeed{files: []core.SecretFile{{Name: "token", Content: "fixture"}}, env: map[string]string{"OWNED": "fixture"}}, "", func(cause error) (error, bool) {
		// This callback releases the reserved service name. Seed cleanup must
		// already be complete before a replacement can take that name.
		for _, name := range []string{a.Name + "-env", a.Name + "-files"} {
			if err := cl.Get(ctx, client.ObjectKey{Namespace: a.Namespace, Name: name}, &corev1.Secret{}); !apierrors.IsNotFound(err) {
				t.Fatalf("seed %s still exists when row is released: %v", name, err)
			}
		}
		return cause, true
	})
	if err == nil {
		t.Fatal("failed completion returned success")
	}
	if capture.first == nil || len(capture.first.Spec.EnvFromSecrets) != 1 || len(capture.first.Spec.FilesFromSecrets) != 2 || capture.first.Spec.EnvFromSecret == "" {
		t.Fatalf("first App did not compose groups with own env/files: %+v", capture.first)
	}
	var persisted appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKeyFromObject(a), &persisted); !apierrors.IsNotFound(err) {
		t.Fatalf("App survived failed composition: %v", err)
	}
	got, err := groups.GetEnvGroup(ctx, group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ServiceLinks) != 0 {
		t.Fatalf("failed creation retained memberships: %+v", got.ServiceLinks)
	}
	var projections corev1.SecretList
	if err := cl.List(ctx, &projections); err != nil {
		t.Fatal(err)
	}
	for _, secret := range projections.Items {
		if secret.Name == a.Name+"-files" || secret.Name == a.Name+"-env" {
			t.Fatalf("owned projection survived rollback: %s", secret.Name)
		}
	}
	// Rollback must not erase the reusable group or its projections.
	if len(projections.Items) != 2 {
		t.Fatalf("group projections changed during rollback: %+v", projections.Items)
	}
}

func TestInitialGroupFailureRetainsCompleteAppWhenRowRollbackFails(t *testing.T) {
	ctx := context.Background()
	svc, cl := newService(nil)
	groups := &envgroups.Service{Base: svc.Base, Store: newMemKV()}
	if _, err := groups.CreateEnvGroup(ctx, envgroups.CreateEnvGroupRequest{Name: "shared"}); err != nil {
		t.Fatal(err)
	}
	svc.EnvGroups = failingInitialGroups{Service: groups}
	a := sampleApp("new-service")
	req := CreateRequest{Name: a.Name, initialEnvGroups: []string{"shared"}}
	rollbackCalled := false
	err := svc.writeInitialApp(ctx, req, a, createSeed{}, "", func(cause error) (error, bool) {
		rollbackCalled = true
		return errors.Join(cause, errors.New("store removal unavailable")), false
	})
	if err == nil || !rollbackCalled {
		t.Fatal("creation rollback failure was not reported")
	}
	preserved := getApp(t, cl, a.Name)
	if len(preserved.Spec.EnvFromSecrets) != 1 || len(preserved.Spec.FilesFromSecrets) != 1 {
		t.Fatalf("failed row removal lost complete App configuration: %+v", preserved.Spec)
	}
}

func TestInitialGroupFailureCompensatesGeneratedOperationalSecrets(t *testing.T) {
	for _, tc := range []struct{ afterCreate, replacement bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		t.Run(fmt.Sprintf("after-create-%t/replacement-%t", tc.afterCreate, tc.replacement), func(t *testing.T) {
			ctx := context.Background()
			svc, cl := newService(nil)
			groups := &envgroups.Service{Base: svc.Base, Store: newMemKV()}
			if _, err := groups.CreateEnvGroup(ctx, envgroups.CreateEnvGroupRequest{Name: "shared"}); err != nil {
				t.Fatal(err)
			}
			svc.EnvGroups = failingInitialGroups{Service: groups}
			svc.Store = &recordingStore{}
			a := sampleApp("new-service")
			a.Spec.Repo = "https://github.com/example/private"
			svc.GitHub = &fakeCloneTokens{token: "fixture", ok: true}
			var err error
			a.Spec.CloneSecret, err = svc.ensureCloneSecret(ctx, a)
			if err != nil {
				t.Fatal(err)
			}
			pullName := appv1alpha1.ExternalRegistryPullSecretName(a.Name)
			svc.RegistryCreds = &fakePullSecrets{name: pullName, ok: true}
			a.Spec.ExternalRegistryPullSecret, err = svc.ensureExternalRegistryPullSecret(ctx, a)
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{pullName, "unrelated-credentials"} {
				if err := cl.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: a.Namespace}}); err != nil {
					t.Fatal(err)
				}
			}
			groupName := "missing"
			if tc.afterCreate {
				groupName = "shared"
			}
			err = svc.writeInitialApp(ctx, CreateRequest{Name: a.Name, initialEnvGroups: []string{groupName}}, a, createSeed{}, "owned-fresh-row", func(cause error) (error, bool) {
				if tc.replacement {
					for _, name := range []string{a.Spec.CloneSecret, pullName} {
						secret := &corev1.Secret{}
						if err := cl.Get(ctx, client.ObjectKey{Namespace: a.Namespace, Name: name}, secret); err != nil {
							t.Fatal(err)
						}
						secret.Data = map[string][]byte{"replacement": []byte("new-request")}
						if err := cl.Update(ctx, secret); err != nil {
							t.Fatal(err)
						}
					}
				}
				return cause, true
			})
			if err == nil {
				t.Fatal("expected preparation/completion failure")
			}
			for _, name := range []string{a.Spec.CloneSecret, pullName} {
				secret := &corev1.Secret{}
				err := cl.Get(ctx, client.ObjectKey{Namespace: a.Namespace, Name: name}, secret)
				if tc.replacement {
					if err != nil || string(secret.Data["replacement"]) != "new-request" {
						t.Fatalf("replacement secret %s removed/changed: %v", name, err)
					}
				} else if !apierrors.IsNotFound(err) {
					t.Fatalf("generated secret %s survived: %v", name, err)
				}
			}
			if err := cl.Get(ctx, client.ObjectKey{Namespace: a.Namespace, Name: "unrelated-credentials"}, &corev1.Secret{}); err != nil {
				t.Fatal(err)
			}
			var remaining corev1.SecretList
			if err := cl.List(ctx, &remaining); err != nil {
				t.Fatal(err)
			}
			want := 3
			if tc.replacement {
				want += 2
			}
			if len(remaining.Items) != want {
				t.Fatalf("shared group projections/unrelated credentials changed: %d secrets", len(remaining.Items))
			}
		})
	}
}
