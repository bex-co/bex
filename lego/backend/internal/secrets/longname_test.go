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
	"maps"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w4/m168: an over-long env key or secret-file name wedged a service — the
// store kept what Kubernetes refused to project, and two such names made
// every later single-key delete fail. These tests drive the real service
// against a client that refuses Secrets the way the API server does.

// refusingClient rejects any Secret whose data carries a key the API server
// would refuse (over 253 characters) or a key named poison, so a valid name can
// still exercise a projection failure.
func refusingClient(objs ...client.Object) client.Client {
	refuse := func(obj client.Object) error {
		sec, ok := obj.(*corev1.Secret)
		if !ok {
			return nil
		}
		for k := range sec.Data {
			if len(k) > core.MaxConfigKeyLength || strings.HasPrefix(k, "poison") || strings.HasPrefix(k, "POISON") {
				return apierrors.NewInvalid(schema.GroupKind{Kind: "Secret"}, sec.Name,
					field.ErrorList{field.Invalid(field.NewPath("data").Key("…"), "…", "must be no more than 253 characters")})
			}
		}
		return nil
	}
	return fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(objs...).WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if err := refuse(obj); err != nil {
				return err
			}
			return c.Create(ctx, obj, opts...)
		},
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if err := refuse(obj); err != nil {
				return err
			}
			return c.Update(ctx, obj, opts...)
		},
	}).Build()
}

func refusingService(store core.SecretKV) *Service {
	return &Service{Base: &core.Base{Client: refusingClient(sampleApp("web")), Namespace: "default", Clock: fixedNow}, Store: store}
}

var long254 = strings.Repeat("x", 254)

func TestOverLongNamesAreRefusedBeforeAnyWrite(t *testing.T) {
	store := newFakeSecretStore()
	svc := refusingService(store)
	ctx := context.Background()
	if _, err := svc.SetSecretFile(ctx, "web", long254, "c"); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "253") {
		t.Fatalf("SetSecretFile(254) = %v, want a 400 naming the 253 limit", err)
	}
	if _, err := svc.SetEnvVar(ctx, "web", "K"+long254[1:], EnvVarWrite{Value: "v"}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("SetEnvVar(254) = %v, want 400", err)
	}
	if _, err := svc.PatchEnvironment(ctx, "web", EnvironmentPatch{SaveMode: SaveModeDeploy, SecretFiles: []SecretFilePatch{{Name: long254, Content: "c"}}}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("PatchEnvironment(254 file) = %v, want 400", err)
	}
	if len(store.m[filesPath("web")]) != 0 || len(store.m[envPath("web")]) != 0 {
		t.Fatalf("a refused name reached the store: %+v", store.m)
	}
	// The boundary itself is fine.
	if _, err := svc.SetSecretFile(ctx, "web", strings.Repeat("y", 253), "c"); err != nil {
		t.Fatalf("SetSecretFile(253) = %v, want success", err)
	}
}

func TestSingleKeyWritesRestoreTheStoreWhenProjectionIsRefused(t *testing.T) {
	ctx := context.Background()
	for name, prior := range map[string]map[string]string{"no prior": nil, "one prior": {"a.txt": "A"}} {
		t.Run("secret file, "+name, func(t *testing.T) {
			store := newFakeSecretStore()
			if prior != nil {
				store.m[filesPath("web")] = maps.Clone(prior)
			}
			_, err := refusingService(store).SetSecretFile(ctx, "web", "poison.txt", "c")
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("refused projection = %v, want 400", err)
			}
			if got := store.m[filesPath("web")]; !maps.Equal(got, prior) && !(len(got) == 0 && len(prior) == 0) {
				t.Fatalf("store after refused write = %+v, want %+v", got, prior)
			}
			if store.deletes != 0 {
				t.Fatalf("restore used a KV delete (%d); an empty prior is written as data", store.deletes)
			}
		})
	}
	t.Run("env var", func(t *testing.T) {
		store := newFakeSecretStore()
		store.m[envPath("web")] = map[string]string{"KEEP": "1"}
		if _, err := refusingService(store).SetEnvVar(ctx, "web", "POISON", EnvVarWrite{Value: "v"}); !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("refused env projection = %v, want 400", err)
		}
		if got := store.m[envPath("web")]; !maps.Equal(got, map[string]string{"KEEP": "1"}) {
			t.Fatalf("env store after refused write = %+v", got)
		}
	})
}

func TestEmptyPriorPatchFailureLeavesNoFileListed(t *testing.T) {
	store := newFakeSecretStore()
	svc := refusingService(store)
	ctx := context.Background()
	if _, err := svc.PatchEnvironment(ctx, "web", EnvironmentPatch{SaveMode: SaveModeDeploy, SecretFiles: []SecretFilePatch{{Name: "poison.txt", Content: "c"}}}); err == nil {
		t.Fatal("a refused projection must fail the patch")
	}
	files, err := svc.ListSecretFiles(ctx, "web")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("listing after a failed patch with no prior files = %+v, want empty", files)
	}
}

func TestTwoStoredUnprojectableNamesCanEachBeDeleted(t *testing.T) {
	store := newFakeSecretStore()
	a, b := "a"+long254[1:], "b"+long254[1:]
	store.m[filesPath("web")] = map[string]string{a: "1", b: "2"}
	store.m[envPath("web")] = map[string]string{"K" + long254[1:]: "v"}
	svc := refusingService(store)
	ctx := context.Background()

	for _, name := range []string{a, b} {
		if err := svc.DeleteSecretFile(ctx, "web", name); err != nil {
			t.Fatalf("DeleteSecretFile(stored %d-char name) = %v, want success", len(name), err)
		}
	}
	if err := svc.DeleteEnvVar(ctx, "web", "K"+long254[1:]); err != nil {
		t.Fatalf("DeleteEnvVar(stored 254-char key) = %v, want success", err)
	}
	if _, err := svc.SetSecretFile(ctx, "web", "ok.txt", "fine"); err != nil {
		t.Fatalf("SetSecretFile(ok.txt) after recovery = %v", err)
	}
	stored := store.m[filesPath("web")]
	projected := map[string]string{}
	for k, v := range getSecret(t, svc.Client, "web-files").Data {
		projected[k] = string(v)
	}
	if !maps.Equal(stored, projected) || !maps.Equal(stored, map[string]string{"ok.txt": "fine"}) {
		t.Fatalf("store %+v must equal the projected Secret %+v", stored, projected)
	}
	if len(store.m[envPath("web")]) != 0 {
		t.Fatalf("env store still holds the over-long key: %+v", store.m[envPath("web")])
	}
}

func TestPatchCanDeleteOrRenameAStoredOverLongName(t *testing.T) {
	store := newFakeSecretStore()
	store.m[filesPath("web")] = map[string]string{long254: "1", "b" + long254[1:]: "2"}
	svc := refusingService(store)
	ctx := context.Background()
	_, err := svc.PatchEnvironment(ctx, "web", EnvironmentPatch{SaveMode: SaveModeDeploy, SecretFiles: []SecretFilePatch{
		{Name: long254, Delete: true},
		{Name: "renamed.txt", FromName: "b" + long254[1:]},
	}})
	if err != nil {
		t.Fatalf("deleting/renaming stored over-long names = %v, want success", err)
	}
	if got := store.m[filesPath("web")]; !maps.Equal(got, map[string]string{"renamed.txt": "2"}) {
		t.Fatalf("store = %+v, want only renamed.txt", got)
	}
}
