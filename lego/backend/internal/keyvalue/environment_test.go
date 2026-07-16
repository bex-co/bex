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

package keyvalue

import (
	"context"
	"errors"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type fixedCreateEnvironment struct {
	assignment core.EnvironmentAssignment
	err        error
	calls      int
}

func (r *fixedCreateEnvironment) ResolveForCreate(_ context.Context, _, _ string) (core.EnvironmentAssignment, error) {
	r.calls++
	return r.assignment, r.err
}

func TestCreateKeyValueEnvironmentResolution(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "unknown", err: core.ErrNotFound},
		{name: "foreign", err: core.ErrForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService()
			svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
			svc.Environments = &fixedCreateEnvironment{err: tc.err}
			_, err := svc.CreateKeyValue(ctxAs("user-a"), CreateKeyValueRequest{Name: "cache", EnvironmentID: "env-x"})
			if !errors.Is(err, tc.err) {
				t.Fatalf("CreateKeyValue error = %v, want %v", err, tc.err)
			}
			var keyValues appv1alpha1.KeyValueList
			if listErr := cl.List(context.Background(), &keyValues); listErr != nil || len(keyValues.Items) != 0 {
				t.Fatalf("failed resolution wrote key values: %v, err=%v", len(keyValues.Items), listErr)
			}
		})
	}

	resolver := &fixedCreateEnvironment{assignment: core.EnvironmentAssignment{ID: "env-staging", ProjectID: "prj-platform", WorkspaceID: "tea-a"}}
	svc, cl := newService()
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
	svc.Environments = resolver
	view, err := svc.CreateKeyValue(ctxAs("user-a"), CreateKeyValueRequest{Name: "cache", EnvironmentID: "env-staging"})
	if err != nil {
		t.Fatal(err)
	}
	if !mintedKVID(view.ID) || view.Name != "cache" || view.ProjectID != "prj-platform" || view.EnvironmentID != "env-staging" || resolver.calls != 1 {
		t.Fatalf("view = %+v, resolver calls = %d", view, resolver.calls)
	}
	var keyValue appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: view.ID}, &keyValue); err != nil {
		t.Fatal(err)
	}
	if keyValue.Spec.Name != "cache" {
		t.Fatalf("spec.name = %q, want cache", keyValue.Spec.Name)
	}
	if keyValue.Labels[core.LabelProject] != "prj-platform" || keyValue.Labels[core.LabelEnvironment] != "env-staging" {
		t.Fatalf("labels = %v", keyValue.Labels)
	}
}

// environment_test.go covers w6/m20's SetEnvironmentID: the internal/
// environments feature's write path onto a KeyValue CR, mirroring
// postgres/environment_test.go and ownerid_test.go's coverage of
// SetProjectID's sibling label.

// TestSetEnvironmentID_WritesAndClearsLabel is w6/m20/t001's regression test:
// SetEnvironmentID stamps core.LabelEnvironment on the underlying KeyValue CR,
// and an empty environmentID clears it again.
func TestSetEnvironmentID_WritesAndClearsLabel(t *testing.T) {
	svc, cl := newService(sampleKeyValue("kv1"))
	svc.Authz = &fakeChecker{allow: true}

	if err := svc.SetEnvironmentID(ctxAs("user-a"), "kv1", "env-1"); err != nil {
		t.Fatalf("SetEnvironmentID: %v", err)
	}
	var kv appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "kv1"}, &kv); err != nil {
		t.Fatalf("get KeyValue: %v", err)
	}
	if kv.Labels[core.LabelEnvironment] != "env-1" {
		t.Fatalf("KeyValue labels = %+v, want LabelEnvironment=env-1", kv.Labels)
	}

	if err := svc.SetEnvironmentID(ctxAs("user-a"), "kv1", ""); err != nil {
		t.Fatalf("SetEnvironmentID (clear): %v", err)
	}
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "kv1"}, &kv); err != nil {
		t.Fatalf("get KeyValue: %v", err)
	}
	if _, ok := kv.Labels[core.LabelEnvironment]; ok {
		t.Fatalf("KeyValue labels = %+v, want LabelEnvironment cleared", kv.Labels)
	}
}

// TestListKeyValues_ReadsEnvironmentIDLabel proves ListKeyValues/kvView
// surface the label environments.Service's keyValueIDsForEnvironment read
// path (w6/m20/t004) needs to find member KeyValues.
func TestListKeyValues_ReadsEnvironmentIDLabel(t *testing.T) {
	kv := sampleKeyValue("kv1")
	kv.Labels = map[string]string{core.LabelEnvironment: "env-1"}
	svc, _ := newService(kv)

	v, err := svc.GetKeyValue(ctxAs("user-a"), "kv1")
	if err != nil || v.EnvironmentID != "env-1" {
		t.Fatalf("GetKeyValue = %+v, err=%v; want EnvironmentID=env-1", v, err)
	}

	list, err := svc.ListKeyValues(ctxAs("user-a"), "")
	if err != nil || len(list) != 1 || list[0].EnvironmentID != "env-1" {
		t.Fatalf("ListKeyValues = %+v, err=%v; want one instance with EnvironmentID=env-1", list, err)
	}
}
