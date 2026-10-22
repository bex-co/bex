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
	"net/http"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type fakeProtectionStore struct {
	statuses map[string]string
	calls    int
}

func (s *fakeProtectionStore) GetEnvironmentProtectedStatus(_ context.Context, environmentID string) (string, error) {
	s.calls++
	if status := s.statuses[environmentID]; status != "" {
		return status, nil
	}
	return core.ProtectedStatusUnprotected, nil
}

func databaseForProtection(id, name string, protected bool) *appv1alpha1.Database {
	labels := map[string]string{}
	if protected {
		labels[core.LabelEnvironment] = "env-production"
	}
	return &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: id, Namespace: "default", Labels: labels},
		Spec:       appv1alpha1.DatabaseSpec{Name: name, Plan: "free"},
	}
}

// queryableProtectedService is protectedPostgresService plus the connection
// secret a query resolves, so a write by a non-admin can be observed completing.
func queryableProtectedService(db *appv1alpha1.Database) (*Service, []string) {
	db.Status.Phase = appv1alpha1.DBPhaseReady
	db.Status.SecretName = db.Name + "-app"
	svc, _, _ := protectedPostgresService(db, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: db.Name + "-app", Namespace: "default"},
		Data:       map[string][]byte{"uri": []byte("postgres://u:p@localhost:5432/db?sslmode=disable")},
	})
	ran := []string{}
	svc.queryExecutor = func(_ context.Context, _, sql string, _ queryLimits, _ bool) (QueryResult, error) {
		ran = append(ran, sql)
		return QueryResult{}, nil
	}
	return svc, ran
}

func protectedPostgresService(objects ...client.Object) (*Service, client.Client, *fakeProtectionStore) {
	svc, cl := newService(objects...)
	store := &fakeProtectionStore{statuses: map[string]string{"env-production": core.ProtectedStatusProtected}}
	svc.Protection = store
	return svc, cl, store
}

// denyWorkspaceManage allows every relation except the admin can_manage on a
// workspace — a Developer in the role ladder. Other objects' checks (the
// ordinary per-verb authorization) are untouched.
type denyWorkspaceManage struct{}

func (denyWorkspaceManage) Check(_ context.Context, _, relation, object string) (bool, error) {
	return !(relation == core.RelCanManage && strings.HasPrefix(object, "workspace:")), nil
}

type allowAllRelations struct{}

func (allowAllRelations) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

// ctxAsCaller is the identity every Authz-checked call needs.
func ctxAsCaller() context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "session"})
}

// The rule is Render's (w8/187): in a protected environment only a workspace
// Admin may delete, suspend or resume a database, with no typed phrase;
// everything else is open to anyone who may do it elsewhere.
func TestProtectedDatabaseDeleteSuspendResumeNeedAdmin(t *testing.T) {
	t.Run("a non-admin is refused and nothing moves", func(t *testing.T) {
		db := databaseForProtection("dpg-orders", "orders", true)
		svc, cl, _ := protectedPostgresService(db)
		svc.Authz = denyWorkspaceManage{}
		if err := svc.DeletePostgres(ctxAsCaller(), db.Name); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("DeletePostgres by a non-admin = %v, want ErrForbidden", err)
		}
		if _, err := svc.Suspend(ctxAsCaller(), db.Name); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("Suspend by a non-admin = %v, want ErrForbidden", err)
		}
		var got appv1alpha1.Database
		if err := cl.Get(context.Background(), client.ObjectKeyFromObject(db), &got); err != nil || got.Spec.Suspended {
			t.Fatalf("a refused verb changed the database: suspended=%v err=%v", got.Spec.Suspended, err)
		}
		// The old phrase substitutes for nothing.
		ctx := core.WithConfirm(ctxAsCaller(), "sudo delete database orders")
		if err := svc.DeletePostgres(ctx, db.Name); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("DeletePostgres with the old phrase = %v, want ErrForbidden", err)
		}
	})

	t.Run("an admin needs no phrase for delete, suspend or resume", func(t *testing.T) {
		db := databaseForProtection("dpg-orders", "orders", true)
		svc, cl, _ := protectedPostgresService(db)
		svc.Authz = allowAllRelations{}
		if _, err := svc.Suspend(ctxAsCaller(), db.Name); err != nil {
			t.Fatalf("Suspend by an admin: %v", err)
		}
		var got appv1alpha1.Database
		if err := cl.Get(context.Background(), client.ObjectKeyFromObject(db), &got); err != nil || !got.Spec.Suspended {
			t.Fatalf("Database not suspended: suspended=%v err=%v", got.Spec.Suspended, err)
		}
		if _, err := svc.Resume(ctxAsCaller(), db.Name); err != nil {
			t.Fatalf("Resume by an admin: %v", err)
		}
		if err := svc.DeletePostgres(ctxAsCaller(), db.Name); err != nil {
			t.Fatalf("DeletePostgres by an admin: %v", err)
		}
	})

	t.Run("a non-admin cannot resume a protected database", func(t *testing.T) {
		db := databaseForProtection("dpg-orders", "orders", true)
		db.Spec.Suspended = true
		svc, _, _ := protectedPostgresService(db)
		svc.Authz = denyWorkspaceManage{}
		if _, err := svc.Resume(ctxAsCaller(), db.Name); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("Resume by a non-admin = %v, want ErrForbidden", err)
		}
	})

	t.Run("environment-less resource skips store", func(t *testing.T) {
		db := databaseForProtection("dpg-dev", "dev", false)
		svc, _, store := protectedPostgresService(db)
		svc.Authz = denyWorkspaceManage{}
		if _, err := svc.Suspend(ctxAsCaller(), db.Name); err != nil {
			t.Fatalf("environment-less Suspend: %v", err)
		}
		if store.calls != 0 {
			t.Fatalf("protection store calls = %d, want 0", store.calls)
		}
	})

	t.Run("unwired protection store is a no-op", func(t *testing.T) {
		db := databaseForProtection("dpg-hand-applied", "hand-applied", true)
		svc, _ := newService(db)
		svc.Authz = denyWorkspaceManage{}
		if _, err := svc.Suspend(ctxAsCaller(), db.Name); err != nil {
			t.Fatalf("DB-less Suspend: %v", err)
		}
	})

	t.Run("unprotected environment needs no admin", func(t *testing.T) {
		db := databaseForProtection("dpg-staging", "staging", true)
		svc, _, store := protectedPostgresService(db)
		store.statuses["env-production"] = core.ProtectedStatusUnprotected
		svc.Authz = denyWorkspaceManage{}
		if _, err := svc.Suspend(ctxAsCaller(), db.Name); err != nil {
			t.Fatalf("unprotected Suspend: %v", err)
		}
	})

	t.Run("a protection lookup that fails refuses the verb", func(t *testing.T) {
		db := databaseForProtection("dpg-orders", "orders", true)
		svc, _, _ := protectedPostgresService(db)
		svc.Protection = failingProtection{}
		if _, err := svc.Suspend(ctxAsCaller(), db.Name); err == nil {
			t.Fatal("Suspend with an unreadable protection status succeeded")
		}
	})
}

type failingProtection struct{}

func (failingProtection) GetEnvironmentProtectedStatus(context.Context, string) (string, error) {
	return "", errors.New("store down")
}

// Over REST the Admin role is the whole rule: an admin suspends with no ?confirm=.
func TestProtectedDatabaseOverREST(t *testing.T) {
	db := databaseForProtection("dpg-rest", "rest-db", true)
	svc, _, _ := protectedPostgresService(db)
	svc.Authz = allowAllRelations{}
	if got := serveREST(svc, http.MethodPost, "/v1/postgres/dpg-rest/suspend", ""); got.Code != http.StatusAccepted {
		t.Fatalf("admin suspend status = %d: %s", got.Code, got.Body.String())
	}
}

// The console, failover, renames and version upgrades are not among the actions
// Render restricts in a protected environment (w4/m127 had gated them behind
// phrases): a Developer runs them with no phrase.
func TestProtectedDatabaseSettingsAndDataPlaneAreOpenToDevelopers(t *testing.T) {
	t.Run("writable console", func(t *testing.T) {
		db := databaseForProtection("dpg-orders", "orders", true)
		svc, _ := queryableProtectedService(db)
		svc.Authz = denyWorkspaceManage{}
		var ran []string
		svc.queryExecutor = func(_ context.Context, _, sql string, _ queryLimits, _ bool) (QueryResult, error) {
			ran = append(ran, sql)
			return QueryResult{}, nil
		}
		if _, err := svc.ExecuteQuery(ctxAsCaller(), db.Name, "DROP TABLE qa_probe", true); err != nil {
			t.Fatalf("writable query by a non-admin on a protected database: %v", err)
		}
		if len(ran) != 1 {
			t.Fatalf("the write did not execute: %v", ran)
		}
	})

	t.Run("failover", func(t *testing.T) {
		db := databaseForProtection("dpg-orders", "orders", true)
		svc, _, _ := protectedPostgresService(db)
		svc.Authz = denyWorkspaceManage{}
		if err := svc.Failover(ctxAsCaller(), db.Name); err != nil {
			t.Fatalf("Failover by a non-admin on a protected database: %v", err)
		}
	})

	t.Run("rename and version upgrade", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			patch PostgresPatch
		}{
			{"rename", PostgresPatch{Name: ptrTo("renamed")}},
			{"version upgrade", PostgresPatch{Version: ptrTo("17")}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				db := databaseForProtection("dpg-orders", "orders", true)
				svc, _, _ := protectedPostgresService(db)
				svc.Authz = denyWorkspaceManage{}
				_, err := svc.UpdatePostgres(ctxAsCaller(), db.Name, tc.patch)
				if errors.Is(err, core.ErrForbidden) || errors.Is(err, core.ErrBadRequest) && strings.Contains(err.Error(), "confirm=") {
					t.Fatalf("%s by a non-admin on a protected database was refused for protection: %v", tc.name, err)
				}
			})
		}
	})
}

func ptrTo[T any](v T) *T { return &v }
