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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// staleAllowChecker models the codex round-8 #8 window: the cached path (Check)
// still answers a warm positive while the source of truth (CheckFresh) already
// says the membership is gone — a member revoked on another replica inside
// PositiveTTL.
type staleAllowChecker struct{}

func (staleAllowChecker) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

func (staleAllowChecker) CheckFresh(context.Context, string, string, string) (bool, error) {
	return false, nil
}

// codex round-8 #8: minting a login (and returning its one-time password) is
// durable-credential issuance — a revoked member riding a stale positive must
// not mint one last role, and no Secret may exist afterward.
func TestCreateUserFailsClosedOnFreshRevocation(t *testing.T) {
	svc, cl := newService()
	seedDatabaseSpec(t, cl, "fresh-db", appv1alpha1.DatabaseSpec{Plan: "free"}, false)
	svc.Authz = staleAllowChecker{}
	ctx := ctxAs("user-a")

	if _, err := svc.CreateUser(ctx, "fresh-db", "reporting"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("CreateUser on a stale positive: %v, want ErrForbidden", err)
	}
	var sec corev1.Secret
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "fresh-db-user-reporting"}, &sec); !apierrors.IsNotFound(err) {
		t.Fatalf("denied CreateUser must not create the user secret, got err=%v sec=%+v", err, sec.Data)
	}
	var db appv1alpha1.Database
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "fresh-db"}, &db); err != nil {
		t.Fatal(err)
	}
	if len(db.Spec.Users) != 0 {
		t.Fatalf("denied CreateUser must not record the role on spec.users: %+v", db.Spec.Users)
	}
}

// codex round-8 #8: the connection strings ARE the password — a stale positive
// must not surface one last credential.
func TestPostgresConnectionInfoFailsClosedOnFreshRevocation(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "fresh-ci")
	svc.Authz = staleAllowChecker{}
	ctx := ctxAs("user-a")

	_, err := svc.PostgresConnectionInfo(ctx, "fresh-ci")
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("PostgresConnectionInfo on a stale positive: %v, want ErrForbidden", err)
	}
	if strings.Contains(err.Error(), "s3cret") {
		t.Errorf("denial leaked the password: %v", err)
	}
}
