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
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

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
