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

// access.go is the managed-Postgres access & connection surface: the external
// endpoint's IP allowlist (Render's ipAllowList) and additional Postgres login
// roles (Render's /postgres/{id}/users). Both write intent to the Database CR
// (spec.ipAllowList / spec.users) the operator projects onto the Traefik SNI
// route and CNPG's managed roles; a user's generated password lands in a
// per-user Secret and is revealed once, never logged.
package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/types/credpw"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// --- IP allowlist ---

// GetIPAllowList returns the external endpoint's effective rules: [] when
// external access is off, the unrestricted pair for a public database with no
// saved rules (core.EffectiveAllowList, w4/m315). The internal -rw path is
// never gated.
func (s *Service) GetIPAllowList(ctx context.Context, name string) ([]core.IPAllowListEntry, error) {
	d, err := s.fetchDatabaseForRead(ctx, core.RelCanView, name)
	if err != nil {
		return nil, err
	}
	return core.EffectiveAllowList(d.Spec.Public, d.Spec.IPAllowList), nil
}

// SetIPAllowList replaces the external-endpoint allowlist — full replace, so
// entries written without descriptions clear any stored ones. Every entry's
// CIDR must be valid (a bad one is a 400 before any write). A nonempty list
// enables external access for its sources; an empty list disables external
// access. The internal path is unaffected.
func (s *Service) SetIPAllowList(ctx context.Context, name string, entries []core.IPAllowListEntry) (PostgresView, error) {
	return s.UpdatePostgres(ctx, name, PostgresPatch{IPAllowList: &entries})
}

// --- Postgres users ---

// PostgresUserView is one additional managed login role. The password is never
// included here — it is returned once from CreateUser, then only via the role's
// Secret.
type PostgresUserView struct {
	Name string `json:"name"`
}

// CreateUserResult is CreateUser's one-time response: the role plus its freshly
// generated password (revealed once, to the authenticated creator).
type CreateUserResult struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

// ListUsers returns the database's additional managed login roles (not the
// owner role, which is CNPG-managed). Passwords are never surfaced.
func (s *Service) ListUsers(ctx context.Context, name string) ([]PostgresUserView, error) {
	d, err := s.fetchDatabaseForRead(ctx, core.RelCanView, name)
	if err != nil {
		return nil, err
	}
	out := make([]PostgresUserView, 0, len(d.Spec.Users))
	for _, u := range d.Spec.Users {
		out = append(out, PostgresUserView{Name: u.Name})
	}
	return out, nil
}

// PostgresCredentialView is Render's credential identity. Connection counts
// and creation timestamps are omitted because CNPG does not report them here.
type PostgresCredentialView struct {
	Username string `json:"username"`
	Default  bool   `json:"default"`
}

// ListCredentials includes the CNPG owner (unless its login was retired) and
// the additional managed roles, marking whichever one is the default. The native ListUsers contract intentionally keeps
// only additional roles for its existing editing clients.
func (s *Service) ListCredentials(ctx context.Context, name string) ([]PostgresCredentialView, error) {
	d, err := s.fetchDatabaseForRead(ctx, core.RelCanView, name)
	if err != nil {
		return nil, err
	}
	return credentialViews(d), nil
}

func credentialViews(d *appv1alpha1.Database) []PostgresCredentialView {
	def := d.Spec.DefaultCredential(d.Name)
	out := make([]PostgresCredentialView, 0, len(d.Spec.Users)+1)
	// A retired original owner no longer logs in, so it is not a credential.
	if owner := d.Spec.EffectiveDatabaseUser(d.Name); !slices.Contains(d.Spec.DeletedUsers, owner) {
		out = append(out, PostgresCredentialView{Username: owner, Default: owner == def})
	}
	for _, user := range d.Spec.Users {
		out = append(out, PostgresCredentialView{Username: user.Name, Default: user.Name == def})
	}
	return out
}

// CreateUser adds a managed login role: it generates a password into a
// generation-specific per-user basic-auth Secret and records the role on the
// Database (spec.users), which the operator projects to CNPG's managed roles.
// The password is returned once and never logged.
func (s *Service) CreateUser(ctx context.Context, name, role string) (CreateUserResult, error) {
	// The password is returned here, so minting it is credential issuance.
	if err := s.AuthorizeMintClass(ctx); err != nil {
		return CreateUserResult{}, err
	}
	res, _, err := s.createUser(ctx, name, "name", role, false)
	return res, err
}

// CreateCredential is Render's create-postgres-user (w5/m265): the new login
// becomes the database's default credential, so connection info, Blueprint
// fromDatabase references (on their next sync), the pooler and exports move
// to it while earlier credentials keep working until they are deleted. It is
// a member of the owner role, so it can use everything the owner created.
// Like Render, nothing is echoed: the password is read back through the
// connection info, which is gated as sensitive, so creating a credential is an
// ordinary write any Render API key can make. It returns the resulting
// credentials.
func (s *Service) CreateCredential(ctx context.Context, name, username string) ([]PostgresCredentialView, error) {
	_, d, err := s.createUser(ctx, name, "username", username, true)
	if err != nil {
		return nil, err
	}
	return credentialViews(d), nil
}

func (s *Service) createUser(ctx context.Context, name, field, role string, asDefault bool) (CreateUserResult, *appv1alpha1.Database, error) {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	d, err := s.fetchDatabase(ctx, core.RelCanCreate, name)
	if err != nil {
		return CreateUserResult{}, nil, err
	}
	// codex round-8 #8: minting a login (and returning its one-time password)
	// is durable-credential issuance — re-assert can_create uncached so a member
	// revoked within the last PositiveTTL cannot mint one last role.
	if err := s.AuthorizeDatabaseFresh(ctx, core.RelCanCreate, d); err != nil {
		return CreateUserResult{}, nil, err
	}
	if err := ValidateRoleIdentifier(field, role); err != nil {
		return CreateUserResult{}, nil, err
	}
	orig := d.DeepCopy()
	if role == d.Spec.EffectiveDatabaseUser(d.Name) {
		return CreateUserResult{}, nil, fmt.Errorf("%w: user %q already exists", core.ErrBadRequest, role)
	}
	for _, u := range d.Spec.Users {
		if u.Name == role {
			return CreateUserResult{}, nil, fmt.Errorf("%w: user %q already exists", core.ErrBadRequest, role)
		}
	}
	pw, err := credpw.Generate()
	if err != nil {
		return CreateUserResult{}, nil, err
	}

	// Earlier releases used a deterministic <db>-user-<role> name and silently
	// adopted it when a failed delete left the Secret behind. Remove that legacy
	// credential before recreating the role. Roles containing underscores could
	// never have produced a valid legacy Kubernetes Secret name, so there is
	// nothing to clean up for those names.
	legacySecretName := fmt.Sprintf("%s-user-%s", name, role)
	if len(validation.IsDNS1123Subdomain(legacySecretName)) == 0 {
		legacy := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: legacySecretName, Namespace: d.Namespace}}
		if err := s.Client.Delete(ctx, legacy); err != nil && !apierrors.IsNotFound(err) {
			return CreateUserResult{}, nil, err
		}
	}

	// A distinct Secret per issuance keeps concurrent creators and abandoned
	// attempts from overwriting the credential referenced by the successful CR
	// patch. The digest exposes no useful password material: pw has 192 random
	// bits and only a 128-bit prefix is used as a Kubernetes-safe generation id.
	secretName := postgresUserSecretName(name, pw)
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: d.Namespace},
		Type:       corev1.SecretTypeBasicAuth, // CNPG passwordSecret expects username/password
		Data: map[string][]byte{
			"username": []byte(role),
			"password": []byte(pw),
		},
	}
	if err := s.Client.Create(ctx, sec); err != nil {
		return CreateUserResult{}, nil, err
	}
	d.Spec.Users = append(d.Spec.Users, appv1alpha1.DatabaseUser{Name: role, SecretName: secretName, OwnerPrivileges: asDefault})
	if asDefault {
		d.Spec.DefaultUser = role
	}
	d.Spec.DeletedUsers = removePostgresUserTombstone(d.Spec.DeletedUsers, role)
	resourcemeta.Touch(d, s.Now())
	// The resource-version precondition makes exactly one concurrent creator the
	// winner; a loser removes only its generation-specific Secret below.
	patch := client.MergeFromWithOptions(orig, client.MergeFromWithOptimisticLock{})
	if err := s.Client.Patch(ctx, d, patch); err != nil {
		if cleanupErr := s.Client.Delete(ctx, sec); cleanupErr != nil && !apierrors.IsNotFound(cleanupErr) {
			return CreateUserResult{}, nil, errors.Join(err, fmt.Errorf("clean up unreferenced postgres user Secret: %w", cleanupErr))
		}
		return CreateUserResult{}, nil, err
	}
	s.RecordDatabaseEffect(ctx, d, core.DatabaseCredentialsCreated)
	return CreateUserResult{Name: role, Password: pw}, d, nil
}

// DeleteUser removes a managed login role from the Database's spec.users and
// records it in spec.deletedUsers so the operator retires the PostgreSQL role
// (NOLOGIN, NULL password) rather than dropping it (codex #8, w4/m179). It also
// deletes the password Secret. Success is accepted revocation intent: the operator applies it on the
// next reconcile. Without the tombstone the operator would simply stop listing
// the role, leaving it valid in Postgres after the API reports deletion.
func (s *Service) DeleteUser(ctx context.Context, name, role string) error {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	d, err := s.fetchDatabase(ctx, core.RelCanCreate, name)
	if err != nil {
		return err
	}
	return s.deleteUser(ctx, d, role)
}

func (s *Service) deleteUser(ctx context.Context, d *appv1alpha1.Database, role string) error {
	orig := d.DeepCopy()
	idx := slices.IndexFunc(d.Spec.Users, func(u appv1alpha1.DatabaseUser) bool { return u.Name == role })
	if idx < 0 {
		// Name the missing child like the service children (w4/154); a missing
		// or unauthorized parent database keeps the neutral answer above, so
		// this is never an existence oracle (w4/371).
		return core.NotFound("user")
	}
	if role == d.Spec.DefaultUser {
		return errDefaultCredential(role)
	}
	secretName := d.Spec.Users[idx].SecretName
	// Delete the credential first so a transient Secret deletion failure leaves
	// spec.users intact and the whole revocation can be retried. If the later CR
	// patch fails, retrying tolerates the already-absent Secret and completes the
	// tombstone write.
	if secretName != "" {
		sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: d.Namespace}}
		if err := s.Client.Delete(ctx, sec); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	d.Spec.Users = append(d.Spec.Users[:idx], d.Spec.Users[idx+1:]...)
	d.Spec.DeletedUsers = append(d.Spec.DeletedUsers, role)
	return s.patchCredentials(ctx, orig, d)
}

// patchCredentials writes a credential change under an optimistic lock, so a
// concurrent user/tombstone update is never erased by this full-list patch.
func (s *Service) patchCredentials(ctx context.Context, orig, d *appv1alpha1.Database) error {
	resourcemeta.Touch(d, s.Now())
	if err := s.Client.Patch(ctx, d, client.MergeFromWithOptions(orig, client.MergeFromWithOptimisticLock{})); err != nil {
		return err
	}
	s.RecordDatabaseEffect(ctx, d, core.DatabaseCredentialsDeleted)
	return nil
}

// DeleteCredential is Render's delete-postgres-user (w5/m265). The current
// default cannot be deleted (add a new credential first, as on Render). The
// create-time owner is never dropped: its login is revoked and it keeps owning
// the database's objects, which is what Render does to the original user.
// Any other credential is retired exactly like DeleteUser.
func (s *Service) DeleteCredential(ctx context.Context, name, username string) ([]PostgresCredentialView, error) {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	d, err := s.fetchDatabase(ctx, core.RelCanCreate, name)
	if err != nil {
		return nil, err
	}
	if owner := d.Spec.EffectiveDatabaseUser(d.Name); username == owner {
		err = s.retireOwner(ctx, d, owner)
	} else {
		err = s.deleteUser(ctx, d, username)
	}
	if err != nil {
		return nil, err
	}
	return credentialViews(d), nil
}

func (s *Service) retireOwner(ctx context.Context, d *appv1alpha1.Database, owner string) error {
	if slices.Contains(d.Spec.DeletedUsers, owner) {
		return core.NotFound("user")
	}
	if owner == d.Spec.DefaultCredential(d.Name) {
		return errDefaultCredential(owner)
	}
	// Consumers still read the owner until the operator has written the new
	// default's connection Secret; retiring it now would strand them.
	if d.CurrentConnectionSecretName() != d.ConnectionSecretName() {
		return fmt.Errorf("%w: the new default credential is still being applied; retry in a few seconds", core.ErrConflict)
	}
	orig := d.DeepCopy()
	d.Spec.DeletedUsers = append(d.Spec.DeletedUsers, owner)
	return s.patchCredentials(ctx, orig, d)
}

func errDefaultCredential(role string) error {
	return fmt.Errorf("%w: user %q is the database's default credential; add a new credential before deleting it", core.ErrBadRequest, role)
}

func postgresUserSecretName(databaseName, password string) string {
	digest := sha256.Sum256([]byte(password))
	return fmt.Sprintf("%s-user-%x", databaseName, digest[:16])
}

func removePostgresUserTombstone(tombstones []string, role string) []string {
	kept := tombstones[:0]
	for _, tombstone := range tombstones {
		if tombstone != role {
			kept = append(kept, tombstone)
		}
	}
	return kept
}
