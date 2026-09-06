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

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSpecProjectionDefaultsAndWithdrawnFields(t *testing.T) {
	object := &unstructured.Unstructured{Object: map[string]any{}}
	desired := map[string]any{
		"instances":    int64(2),
		"postgresql":   map[string]any{"parameters": map[string]any{"max_connections": "200"}},
		"plugins":      []any{map[string]any{"name": "barman", "parameters": map[string]any{"serverName": "source"}}},
		"managed":      map[string]any{"roles": []any{map[string]any{"name": "alice", "ensure": "present"}, map[string]any{"name": "bob", "ensure": "present"}}},
		"certificates": map[string]any{"serverAltDNSNames": []any{"old.example", "kept.example"}},
	}
	if err := projectUnstructuredSpec(object, desired); err != nil {
		t.Fatal(err)
	}
	// Admission supplies nested, top-level, and named-list-entry defaults.
	live := object.Object["spec"].(map[string]any)
	live["enablePDB"] = true
	live["postgresql"].(map[string]any)["parameters"].(map[string]any)["wal_level"] = "logical"
	live["managed"].(map[string]any)["roles"].([]any)[0].(map[string]any)["login"] = true
	before := object.DeepCopy()
	if err := projectUnstructuredSpec(object, desired); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Object, object.Object) {
		t.Fatal("unchanged desired spec lost admission defaults or changed JSON types")
	}
	next := runtime.DeepCopyJSONValue(desired).(map[string]any)
	delete(next, "plugins")
	delete(next["postgresql"].(map[string]any)["parameters"].(map[string]any), "max_connections")
	next["managed"].(map[string]any)["roles"] = []any{map[string]any{"name": "alice", "ensure": "absent"}}
	next["certificates"].(map[string]any)["serverAltDNSNames"] = []any{"kept.example"}
	next["instances"] = int64(1)
	if err := projectUnstructuredSpec(object, next); err != nil {
		t.Fatal(err)
	}
	live = object.Object["spec"].(map[string]any)
	if _, present := live["plugins"]; present {
		t.Fatal("disabled backup plugin lingered")
	}
	params := live["postgresql"].(map[string]any)["parameters"].(map[string]any)
	if _, present := params["max_connections"]; present {
		t.Fatal("withdrawn tenant parameter lingered")
	}
	if params["wal_level"] != "logical" || live["enablePDB"] != true || live["instances"] != int64(1) {
		t.Fatal("lost defaults or failed to apply changed intent")
	}
	roles := live["managed"].(map[string]any)["roles"].([]any)
	if len(roles) != 1 || roles[0].(map[string]any)["ensure"] != "absent" || roles[0].(map[string]any)["login"] != true {
		t.Fatal("role membership, intent, or defaults changed incorrectly")
	}
	names := live["certificates"].(map[string]any)["serverAltDNSNames"]
	if !reflect.DeepEqual(names, []any{"kept.example"}) {
		t.Fatal("removed certificate name lingered")
	}
	before = object.DeepCopy()
	if err := projectUnstructuredSpec(object, next); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Object, object.Object) {
		t.Fatal("changed projection did not converge")
	}
}

func TestSpecProjectionAdoptionAndInvalidHistory(t *testing.T) {
	object := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"obsolete": "value"}}}
	desired := map[string]any{"instances": int64(1)}
	if err := projectUnstructuredSpec(object, desired); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(object.Object["spec"], desired) {
		t.Fatal("legacy adoption failed to withdraw obsolete fields")
	}
	annotations := object.GetAnnotations()
	annotations[annotationProjectedSpec] = "invalid-json"
	object.SetAnnotations(annotations)
	before := object.DeepCopy()
	if err := projectUnstructuredSpec(object, desired); err == nil {
		t.Fatal("malformed ownership history must fail without changing spec")
	}
	if !reflect.DeepEqual(before.Object, object.Object) {
		t.Fatal("invalid history changed object")
	}
}

func TestUpsertOwnedPreservesDefaultedSpecs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		gvk      schema.GroupVersionKind
		spec     map[string]any
		defaults []string
		value    any
	}{
		{"pooler", cnpgPoolerGVK, poolerSpec("source"), []string{"pgbouncer", "parameters", "max_client_conn"}, "1000"},
		{"scheduled-backup", cnpgScheduledBackupGVK, scheduledBackupSpec("source"), []string{"suspend"}, false},
		{"backup", cnpgBackupGVK, onDemandBackupSpec("source"), []string{"target"}, "prefer-standby"},
		{"middleware", traefikHTTPMiddlewareGVK, cidrMiddlewareSpec([]string{"10.0.0.0/8"}), []string{"ipAllowList", "ipStrategy", "depth"}, int64(0)},
		{"certificate", certManagerCertificateGVK, map[string]any{"secretName": "tls", "dnsNames": []any{"example.test"}}, []string{"privateKey", "rotationPolicy"}, "Always"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			if err := appv1alpha1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			base := fake.NewClientBuilder().WithScheme(scheme).Build()
			rec := &writeRecorder{Client: base}
			owner := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "owner", Namespace: "test", UID: "owner-uid"}}
			run := func() error { return upsertOwned(ctx, rec, scheme, owner, tc.gvk, tc.name, tc.spec) }
			if err := run(); err != nil {
				t.Fatal(err)
			}
			object := &unstructured.Unstructured{}
			object.SetGroupVersionKind(tc.gvk)
			key := client.ObjectKey{Namespace: "test", Name: tc.name}
			if err := base.Get(ctx, key, object); err != nil {
				t.Fatal(err)
			}
			if err := unstructured.SetNestedField(object.Object, tc.value, append([]string{"spec"}, tc.defaults...)...); err != nil {
				t.Fatal(err)
			}
			if err := base.Update(ctx, object); err != nil {
				t.Fatal(err)
			}
			rec.reset()
			if err := run(); err != nil {
				t.Fatal(err)
			}
			if len(rec.writes) != 0 {
				t.Fatalf("redundant writes with admission defaults: %v", rec.writes)
			}
			// The no-write check must not mask drift in fields bex actually owns.
			if err := base.Get(ctx, key, object); err != nil {
				t.Fatal(err)
			}
			for key := range tc.spec {
				delete(object.Object["spec"].(map[string]any), key)
				break
			}
			if err := base.Update(ctx, object); err != nil {
				t.Fatal(err)
			}
			if err := run(); err != nil {
				t.Fatal(err)
			}
			if len(rec.writes) != 1 {
				t.Fatalf("owned spec drift should cause exactly one write: %v", rec.writes)
			}
		})
	}
}
