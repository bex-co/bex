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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// twinWrites counts the write verbs a datastore service issues, so a test can
// assert HOW a setter writes rather than only what it wrote.
type twinWrites struct{ patches, updates int }

func recordingClient(rec *twinWrites, objs ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(testScheme()).
		WithObjects(objs...).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, p client.Patch, opts ...client.PatchOption) error {
				rec.patches++
				return c.Patch(ctx, obj, p, opts...)
			},
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				rec.updates++
				return c.Update(ctx, obj, opts...)
			},
		}).
		Build()
}

func twinObjects(name string) (*appv1alpha1.Database, *appv1alpha1.KeyValue) {
	meta := metav1.ObjectMeta{Name: name, Namespace: "default", Labels: map[string]string{core.LabelTenant: "user-a"}}
	return &appv1alpha1.Database{ObjectMeta: meta}, &appv1alpha1.KeyValue{ObjectMeta: *meta.DeepCopy()}
}

// TestDatastoreTwinsShareWriteMechanics holds Postgres and Key Value to the same
// write semantics for the verbs the environments/projects fan-out calls on both.
// They are the same product shape implemented twice and had already drifted:
// Postgres merge-patched while Key Value sent a whole-object Update, so the
// identical operation carried last-writer-wins semantics on one resource and
// targeted-merge semantics on the other, against an operator that writes status
// concurrently.
//
// This lives at the composition root because it is a claim about the two
// features together — neither package can make it alone.
func TestDatastoreTwinsShareWriteMechanics(t *testing.T) {
	const name = "twin"
	cases := []struct {
		verb string
		pg   func(*postgres.Service) error
		kv   func(*keyvalue.Service) error
	}{
		{
			verb: "SetProjectID",
			pg:   func(s *postgres.Service) error { return s.SetProjectID(t.Context(), name, "prj-1") },
			kv:   func(s *keyvalue.Service) error { return s.SetProjectID(t.Context(), name, "prj-1") },
		},
		{
			verb: "SetProjectID (clearing)",
			pg:   func(s *postgres.Service) error { return s.SetProjectID(t.Context(), name, "") },
			kv:   func(s *keyvalue.Service) error { return s.SetProjectID(t.Context(), name, "") },
		},
		{
			verb: "SetEnvironmentID",
			pg:   func(s *postgres.Service) error { return s.SetEnvironmentID(t.Context(), name, "evm-1") },
			kv:   func(s *keyvalue.Service) error { return s.SetEnvironmentID(t.Context(), name, "evm-1") },
		},
		{
			verb: "SetEnvironmentIPAllowList",
			pg: func(s *postgres.Service) error {
				return s.SetEnvironmentIPAllowList(t.Context(), name, []string{"10.0.0.0/8"})
			},
			kv: func(s *keyvalue.Service) error {
				return s.SetEnvironmentIPAllowList(t.Context(), name, []string{"10.0.0.0/8"})
			},
		},
	}

	for _, c := range cases {
		t.Run(c.verb, func(t *testing.T) {
			db, kv := twinObjects(name)

			var pgWrites twinWrites
			pgClient := recordingClient(&pgWrites, db)
			if err := c.pg(&postgres.Service{Base: &core.Base{Client: pgClient, Namespace: "default"}}); err != nil {
				t.Fatalf("postgres %s: %v", c.verb, err)
			}

			var kvWrites twinWrites
			kvClient := recordingClient(&kvWrites, kv)
			if err := c.kv(&keyvalue.Service{Base: &core.Base{Client: kvClient, Namespace: "default"}}); err != nil {
				t.Fatalf("keyvalue %s: %v", c.verb, err)
			}

			for _, w := range []struct {
				resource string
				got      twinWrites
			}{{"postgres", pgWrites}, {"keyvalue", kvWrites}} {
				if w.got.updates != 0 {
					t.Errorf("%s %s issued %d whole-object Update(s); the fan-out verbs must merge-patch",
						w.resource, c.verb, w.got.updates)
				}
				if w.got.patches == 0 {
					t.Errorf("%s %s wrote nothing", w.resource, c.verb)
				}
			}
		})
	}
}

// TestDatastoreTwinsSkipUnchangedEnvironmentAllowList keeps the short-circuit
// that avoids resourceVersion churn when the projected layer is already
// correct, on both twins.
func TestDatastoreTwinsSkipUnchangedEnvironmentAllowList(t *testing.T) {
	const name = "same"
	cidrs := []string{"10.0.0.0/8"}
	db, kv := twinObjects(name)
	db.Spec.EnvironmentIPAllowList = cidrs
	kv.Spec.EnvironmentIPAllowList = cidrs

	var pgWrites twinWrites
	pgClient := recordingClient(&pgWrites, db)
	if err := (&postgres.Service{Base: &core.Base{Client: pgClient, Namespace: "default"}}).
		SetEnvironmentIPAllowList(t.Context(), name, cidrs); err != nil {
		t.Fatal(err)
	}

	var kvWrites twinWrites
	kvClient := recordingClient(&kvWrites, kv)
	if err := (&keyvalue.Service{Base: &core.Base{Client: kvClient, Namespace: "default"}}).
		SetEnvironmentIPAllowList(t.Context(), name, cidrs); err != nil {
		t.Fatal(err)
	}

	if pgWrites != (twinWrites{}) || kvWrites != (twinWrites{}) {
		t.Fatalf("an unchanged allowlist wrote: postgres%+v keyvalue%+v", pgWrites, kvWrites)
	}
}
