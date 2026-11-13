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
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/299: a Blueprint's `fromDatabase` and Key Value `fromService` variables
// are stored on spec.Env as SecretKeyRefs. Every env read skipped them, so a
// service's DATABASE_URL was invisible while its process used it, and a PUT of
// that key answered 200, listed the new value and changed nothing: the
// container's `env` reference beats the store's `envFrom` projection.

func secretRef(name, secret, key string) appv1alpha1.EnvVar {
	return appv1alpha1.EnvVar{Name: name, ValueFrom: &appv1alpha1.EnvVarSource{
		SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: secret, Key: key},
	}}
}

// referencingApp is the shape `render-examples/postgrest` produces: two
// properties of one database, plus a literal. MISSING names a Secret that does
// not exist (a datastore deleted, or still provisioning).
func referencingApp(name string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Spec.Env = []appv1alpha1.EnvVar{
		secretRef("PGRST_DB_URI", "dpg-x-app", "uri"),
		secretRef("PGRST_DB_ANON_ROLE", "dpg-x-app", "username"),
		{Name: "PGRST_DB_SCHEMA", Value: "public"},
		secretRef("MISSING", "gone", "uri"),
	}
	return a
}

func databaseSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-x-app", Namespace: "default"},
		Data: map[string][]byte{
			"uri":      []byte("postgresql://app:pw@dpg-x-rw.default:5432/app"),
			"username": []byte("app"),
		},
	}
}

func TestManifestReferencesAreListedWithTheirSecretsValues(t *testing.T) {
	store := newFakeSecretStore()
	// A write that landed before the guard: the store holds a value the
	// process never sees. The read must report the reference's value.
	store.m[envPath("pg")] = map[string]string{"PGRST_DB_URI": "postgres://qa:qa@127.0.0.1:1/qa"}
	svc := newService(store, referencingApp("pg"), databaseSecret())
	ctx := context.Background()

	want := map[string]string{
		"PGRST_DB_URI":       "postgresql://app:pw@dpg-x-rw.default:5432/app",
		"PGRST_DB_ANON_ROLE": "app",
		"PGRST_DB_SCHEMA":    "public",
		"MISSING":            "",
	}
	list, err := svc.ListEnvVars(ctx, "pg")
	if err != nil {
		t.Fatalf("ListEnvVars: %v", err)
	}
	got := map[string]string{}
	for _, v := range list {
		got[v.Key] = v.Value
		if v.ManagedBy != core.ManagedByBlueprint {
			t.Errorf("%s = %+v, want it flagged blueprint", v.Key, v)
		}
	}
	if len(list) != len(got) || !maps.Equal(got, want) {
		t.Fatalf("list = %+v, want exactly %v", list, want)
	}

	one, err := svc.GetEnvVar(ctx, "pg", "PGRST_DB_URI")
	if err != nil || one.Value != want["PGRST_DB_URI"] || one.ManagedBy != core.ManagedByBlueprint {
		t.Errorf("GetEnvVar = %+v (err %v), want the Secret's uri flagged blueprint", one, err)
	}
	value, err := svc.EnvVarValue(ctx, "pg", "PGRST_DB_ANON_ROLE")
	if err != nil || value.Value != "app" || value.ManagedBy != core.ManagedByBlueprint {
		t.Errorf("EnvVarValue = %+v (err %v), want the Secret's username", value, err)
	}

	keys, err := svc.EnvVarKeys(ctx, "pg")
	if err != nil {
		t.Fatalf("EnvVarKeys: %v", err)
	}
	listed := map[string]bool{}
	for _, k := range keys {
		listed[k.Key] = true
		if k.ManagedBy != core.ManagedByBlueprint || k.Value != "" {
			t.Errorf("key-list row %+v, want flagged and without a value", k)
		}
	}
	if len(keys) != len(want) || len(listed) != len(want) {
		t.Errorf("key list = %+v, want each of %v once", keys, want)
	}
}

// The key list and the Blueprint generator's names seam show no value, so
// they must not read the referenced Secrets: an unreadable Secret fails only
// the reads that return values.
func TestKeysOnlyReadsDoNotReadReferencedSecrets(t *testing.T) {
	svc := newService(newFakeSecretStore(), referencingApp("pg"), databaseSecret())
	svc.Client = interceptor.NewClient(svc.Client.(client.WithWatch), interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.Secret); ok {
				return errors.New("secrets unreadable")
			}
			return c.Get(ctx, key, obj, opts...)
		},
	})
	ctx := context.Background()

	if _, err := svc.EnvVarKeys(ctx, "pg"); err != nil {
		t.Errorf("EnvVarKeys read a Secret: %v", err)
	}
	// GraphQL's envVars page, as the Environment tab queries it: no value selected.
	if _, _, err := svc.listEnvVarsPage(ctx, "pg", "", 0, false); err != nil {
		t.Errorf("a keys-only envVars page read a Secret: %v", err)
	}
	names, err := svc.ListEnvVarNames(ctx, "pg")
	if err != nil {
		t.Fatalf("ListEnvVarNames read a Secret: %v", err)
	}
	if !slices.Contains(names, "PGRST_DB_URI") {
		t.Errorf("names = %v, want the referenced key", names)
	}
	if _, err := svc.ListEnvVars(ctx, "pg"); !errors.Is(err, core.ErrSecretsStoreFailed) {
		t.Errorf("ListEnvVars with the referenced Secret unreadable = %v, want ErrSecretsStoreFailed", err)
	}
}

// w8/302: a Blueprint sync could write a variable with no name. No process
// reads it, so no env read lists it.
func TestANamelessManifestVariableIsNotListed(t *testing.T) {
	a := sampleApp("pg")
	a.Spec.Env = []appv1alpha1.EnvVar{{Name: "APP_ENV", Value: "prod"}, {}}
	svc := newService(newFakeSecretStore(), a)
	list, err := svc.ListEnvVars(context.Background(), "pg")
	if err != nil {
		t.Fatalf("ListEnvVars: %v", err)
	}
	if len(list) != 1 || list[0].Key != "APP_ENV" {
		t.Fatalf("list = %+v, want APP_ENV alone", list)
	}
}
