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

package core

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The per-feature purgers (apps, postgres, keyvalue) assert their own
// tenant-scoping and idempotency against their own CR kind. What only the
// shared helper can be held to is that it stays inside the Base's namespace and
// works for any list kind — a purge that reached across namespaces would delete
// another cluster tenant's CRs, which no single-namespace feature test would
// catch.

func labeledApp(ns, name, tenant string) *appv1alpha1.App {
	return &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: ns, Labels: map[string]string{LabelTenant: tenant},
	}}
}

func labeledDB(ns, name, tenant string) *appv1alpha1.Database {
	return &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: ns, Labels: map[string]string{LabelTenant: tenant},
	}}
}

func TestPurgeByTenant_StaysInTheBaseNamespace(t *testing.T) {
	base := &Base{
		Client:    fakeAppClient(labeledApp("bex-apps", "mine", "tea-a"), labeledApp("other-ns", "elsewhere", "tea-a")),
		Namespace: "bex-apps",
	}

	if err := base.PurgeByTenant(context.Background(), &appv1alpha1.AppList{}, "tea-a"); err != nil {
		t.Fatalf("PurgeByTenant: %v", err)
	}

	var list appv1alpha1.AppList
	if err := base.Client.List(context.Background(), &list); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].Namespace != "other-ns" {
		t.Fatalf("apps after purge = %+v, want only other-ns's app left", list.Items)
	}
}

// One helper, any CR kind — the whole point of the dedup (w6/m15/t002): the
// three purgers differ only in the list they hand it.
func TestPurgeByTenant_WorksForAnyListKind(t *testing.T) {
	base := &Base{
		Client: fakeAppClient(
			labeledApp("bex-apps", "app-a", "tea-a"),
			labeledDB("bex-apps", "db-a", "tea-a"),
			labeledDB("bex-apps", "db-b", "tea-b"),
		),
		Namespace: "bex-apps",
	}
	ctx := context.Background()

	if err := base.PurgeByTenant(ctx, &appv1alpha1.AppList{}, "tea-a"); err != nil {
		t.Fatalf("purge Apps: %v", err)
	}
	if err := base.PurgeByTenant(ctx, &appv1alpha1.DatabaseList{}, "tea-a"); err != nil {
		t.Fatalf("purge Databases: %v", err)
	}

	var apps appv1alpha1.AppList
	if err := base.Client.List(ctx, &apps); err != nil {
		t.Fatalf("list apps: %v", err)
	}
	if len(apps.Items) != 0 {
		t.Fatalf("apps after purge = %+v, want none", apps.Items)
	}
	var dbs appv1alpha1.DatabaseList
	if err := base.Client.List(ctx, &dbs); err != nil {
		t.Fatalf("list databases: %v", err)
	}
	if len(dbs.Items) != 1 || dbs.Items[0].Name != "db-b" {
		t.Fatalf("databases after purge = %+v, want only tea-b's db-b left", dbs.Items)
	}
}

// WorkspacePurger's contract: a retried workspace delete must complete, so
// purging a tenant with nothing left is a no-op, not an error.
func TestPurgeByTenant_NothingToPurgeIsANoOp(t *testing.T) {
	base := &Base{Client: fakeAppClient(labeledApp("bex-apps", "other", "tea-b")), Namespace: "bex-apps"}

	if err := base.PurgeByTenant(context.Background(), &appv1alpha1.AppList{}, "tea-a"); err != nil {
		t.Fatalf("purge of a tenant with no CRs should be a no-op, got: %v", err)
	}
}
