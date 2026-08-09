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

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestCreateKeyValueLandsInTheWorkspaceNamespace is the Key Value sibling of
// postgres.TestCreatePostgresLandsInTheWorkspaceNamespace — same ADR043 D8
// contract, same relationship-not-literal assertion.
func TestCreateKeyValueLandsInTheWorkspaceNamespace(t *testing.T) {
	svc, cl := newService()
	svc.Authz = &fakeChecker{allow: true}
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}

	view, err := svc.CreateKeyValue(ctxAs("user-a"), CreateKeyValueRequest{Name: "kv1"})
	if err != nil {
		t.Fatalf("CreateKeyValue: %v", err)
	}

	want := store.WorkspaceNamespace("tea-a")
	var kv appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: want, Name: view.ID}, &kv); err != nil {
		t.Fatalf("KeyValue not created in the workspace namespace %q: %v — a fromService secretKeyRef cannot resolve across a namespace boundary", want, err)
	}
	if kv.Labels[core.LabelTenant] != "tea-a" {
		t.Errorf("tenant label = %q, want tea-a", kv.Labels[core.LabelTenant])
	}
}

// TestCreateKeyValueWithoutAWorkspaceStaysInTheSharedNamespace pins the
// store-off / unbound-caller path as byte-identical.
func TestCreateKeyValueWithoutAWorkspaceStaysInTheSharedNamespace(t *testing.T) {
	// No Authz and no Workspace: the pre-authorization / store-off mode.
	svc, cl := newService()

	view, err := svc.CreateKeyValue(context.Background(), CreateKeyValueRequest{Name: "kv1"})
	if err != nil {
		t.Fatalf("CreateKeyValue: %v", err)
	}
	var kv appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: svc.Namespace, Name: view.ID}, &kv); err != nil {
		t.Fatalf("unbound create did not land in the shared namespace %q: %v", svc.Namespace, err)
	}
}
