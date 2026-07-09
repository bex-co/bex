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

package postgres

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ownerid_test.go covers w6/m2/t004 for managed Postgres: the ownerId field
// (always empty today — Database CRs aren't tenant-labeled, see
// PostgresView.OwnerID's doc comment) and the ownerId filter's honest no-op
// behavior — it authorizes the requested workspace but never fabricates
// scoped results out of unlabeled data.

// fakeChecker mirrors apps' test helper: allow every object except one denied.
type fakeChecker struct {
	allow bool
	deny  string
}

func (c *fakeChecker) Check(_ context.Context, _, _, object string) (bool, error) {
	if c.deny != "" {
		return object != c.deny, nil
	}
	return c.allow, nil
}

func ctxAs(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
}

func sampleDatabase(name string) *appv1alpha1.Database {
	return &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
}

func TestListPostgres_OwnerIDAlwaysEmptyToday(t *testing.T) {
	svc, _ := newService(sampleDatabase("db1"))
	list, err := svc.ListPostgres(ctxAs("user-a"), "")
	if err != nil || len(list) != 1 || list[0].OwnerID != "" {
		t.Fatalf("ListPostgres = %+v, err=%v; want one unlabeled instance", list, err)
	}
}

func TestListPostgres_OwnerIDFilterYieldsEmptyNotUnscoped(t *testing.T) {
	svc, _ := newService(sampleDatabase("db1"))
	svc.Authz = &fakeChecker{allow: true}

	// Authorized ownerId, but no Database carries that label yet: empty, not
	// db1 (never silently return unscoped data for a scoped request).
	list, err := svc.ListPostgres(ctxAs("user-a"), "tea-1")
	if err != nil || len(list) != 0 {
		t.Fatalf("ListPostgres(tea-1) = %+v, err=%v; want empty (documented gap, w6/001.md)", list, err)
	}
}

func TestListPostgres_OwnerIDFilterForbiddenWhenCallerCantAccess(t *testing.T) {
	svc, _ := newService(sampleDatabase("db1"))
	svc.Authz = &fakeChecker{deny: core.WorkspaceObject("tea-2")}

	if _, err := svc.ListPostgres(ctxAs("user-a"), "tea-2"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("want ErrForbidden for an inaccessible ownerId, got %v", err)
	}
}
