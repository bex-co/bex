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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w9/117: an API key with a recorded creator creates a Postgres login and gets
// its password once, as Render's key with "Create and delete database
// credentials" does, acting with no more than its creator's current role.

// creatorRoleChecker answers model.fga's two relations CreateUser asks from each
// subject's role in tea-1, cached and uncached alike. A key's own entry is its
// mirrored tuple.
type creatorRoleChecker map[string]string

func (r creatorRoleChecker) Check(_ context.Context, subject, relation, object string) (bool, error) {
	if object != core.WorkspaceObject("tea-1") {
		return false, nil
	}
	role := r[strings.TrimPrefix(subject, "user:")]
	switch relation {
	case core.RelCanView:
		return role != "", nil
	case core.RelCanCreate:
		return role == "developer" || role == "admin", nil
	}
	return false, nil
}

func (r creatorRoleChecker) CheckFresh(ctx context.Context, subject, relation, object string) (bool, error) {
	return r.Check(ctx, subject, relation, object)
}

func keyUserFixture(t *testing.T) (*Service, client.Client, creatorRoleChecker) {
	t.Helper()
	svc, cl := newService()
	db := seedDatabaseSpec(t, cl, "key-db", appv1alpha1.DatabaseSpec{Plan: "free"}, false)
	db.Labels = map[string]string{core.LabelTenant: "tea-1", core.LabelWorkspace: "tea-1"}
	if err := cl.Update(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	roles := creatorRoleChecker{
		"dev-1": "developer", "admin-1": "admin", "contrib-1": "contributor",
		"key-dev": "developer", "key-admin": "admin",
		// A stray developer tuple under a Contributor creator, and a key with
		// no created-by stamp, bound as the Developer it never leaves (I3).
		"key-contrib": "developer", "key-legacy": "developer",
		"operator-client": "developer",
	}
	members := coretest.Members{}
	for subject := range roles {
		members[subject] = []string{"tea-1"}
	}
	svc.Authz = roles
	svc.Workspace = members
	svc.KeyCreators = coretest.KeyCreators{
		"key-dev": "dev-1", "key-admin": "admin-1", "key-contrib": "contrib-1", "key-legacy": "",
	}
	svc.PlatformClients = coretest.PlatformClients{}
	return svc, cl, roles
}

func keyCallerCtx(clientID string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: clientID, ClientID: clientID, Method: core.MethodOAuth2})
}

func loginsOf(t *testing.T, cl client.Client) []string {
	t.Helper()
	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "key-db"}, &db); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, u := range db.Spec.Users {
		out = append(out, u.Name)
	}
	return out
}

func TestAKeyCreatesAPostgresUserAsItsCreator(t *testing.T) {
	svc, cl, _ := keyUserFixture(t)
	for _, key := range []string{"key-dev", "key-admin"} {
		role := strings.ReplaceAll(key, "-", "_")
		res, err := svc.CreateUser(keyCallerCtx(key), "key-db", role)
		if err != nil || res.Name != role || res.Password == "" {
			t.Fatalf("%s CreateUser = %+v, %v; want the login and its password", key, res, err)
		}
		var db appv1alpha1.Database
		if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "key-db"}, &db); err != nil {
			t.Fatal(err)
		}
		var sec corev1.Secret
		last := db.Spec.Users[len(db.Spec.Users)-1]
		if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: last.SecretName}, &sec); err != nil {
			t.Fatalf("%s: the login's Secret: %v", key, err)
		}
		if last.Name != role || string(sec.Data["password"]) != res.Password {
			t.Fatalf("%s: spec.users records %q, want %q with the returned password", key, last.Name, role)
		}
	}
}

// A key is refused the login whenever its creator is: a Contributor's key
// holding a stray developer tuple, and a Developer's key from the request
// after its creator is demoted, back once they are promoted again.
func TestAKeyCreatesNoPostgresUserItsCreatorMayNot(t *testing.T) {
	svc, cl, roles := keyUserFixture(t)
	if _, err := svc.CreateUser(keyCallerCtx("key-contrib"), "key-db", "contrib"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a Contributor's key CreateUser = %v, want ErrForbidden", err)
	}
	roles["dev-1"] = "contributor" // the key's own tuple still says developer
	if _, err := svc.CreateUser(keyCallerCtx("key-dev"), "key-db", "demoted"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a demoted Developer's key CreateUser = %v, want ErrForbidden", err)
	}
	delete(roles, "dev-1") // removed from the workspace
	if _, err := svc.CreateUser(keyCallerCtx("key-dev"), "key-db", "removed"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a removed Developer's key CreateUser = %v, want ErrForbidden", err)
	}
	if got := loginsOf(t, cl); len(got) != 0 {
		t.Fatalf("a refused key created logins %v", got)
	}
	roles["dev-1"] = "developer"
	if _, err := svc.CreateUser(keyCallerCtx("key-dev"), "key-db", "promoted"); err != nil {
		t.Fatalf("the re-promoted Developer's key CreateUser = %v", err)
	}
}

// The class gate, not the relation, refuses a caller that is no attributed
// key: each of these holds can_create.
func TestCreateUserStillRefusesAnUnattributedCaller(t *testing.T) {
	svc, cl, _ := keyUserFixture(t)
	for name, ctx := range map[string]context.Context{
		// I3 lets a key with no created-by stamp keep can_create.
		"a key with no recorded creator": keyCallerCtx("key-legacy"),
		// Not a bex API key: an operator-provisioned client_credentials client.
		"an operator-provisioned client": keyCallerCtx("operator-client"),
		// A human token from a client bex did not provision (round 24 H-1).
		"a delegated third-party token": core.WithIdentity(context.Background(), core.Identity{
			Subject: "dev-1", Method: core.MethodOAuth2, ClientID: "dcr-agent-7", Human: true, CanonicalScopes: core.ScopeWrite,
		}),
	} {
		if _, err := svc.CreateUser(ctx, "key-db", "intruder"); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("%s CreateUser = %v, want ErrForbidden", name, err)
		}
	}
	if got := loginsOf(t, cl); len(got) != 0 {
		t.Fatalf("a refused caller created logins %v", got)
	}
	// The person behind the token still may, from their own session.
	if _, err := svc.CreateUser(ctxAs("dev-1"), "key-db", "person"); err != nil {
		t.Fatalf("a Developer's session CreateUser = %v", err)
	}
}

// failingCreators is a key registry that cannot answer.
type failingCreators struct{}

func (failingCreators) APIKeyCreator(context.Context, string) (string, bool, error) {
	return "", false, errors.New("hydra: unavailable")
}

func TestCreateUserFailsClosedWhenTheKeyRegistryCannotAnswer(t *testing.T) {
	svc, cl, _ := keyUserFixture(t)
	svc.KeyCreators = failingCreators{}
	if _, err := svc.CreateUser(keyCallerCtx("key-dev"), "key-db", "unknown"); !errors.Is(err, core.ErrAuthzUnavailable) {
		t.Fatalf("CreateUser with no key registry = %v, want ErrAuthzUnavailable", err)
	}
	if got := loginsOf(t, cl); len(got) != 0 {
		t.Fatalf("created logins %v", got)
	}
}
