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
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// pgRoleName matches a valid unquoted PostgreSQL role name: a lowercase letter
// or underscore, then lowercase letters/digits/underscores. Enforced so a role
// never needs double-quoting and can't be an injection vector.
var pgRoleName = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// --- IP allowlist ---

// GetIPAllowList returns the CIDR allowlist gating the external endpoint (empty
// => open to all source IPs). The internal -rw path is never gated.
func (s *Service) GetIPAllowList(ctx context.Context, name string) ([]string, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	d, err := s.fetchDatabase(ctx, core.RelCanView, name)
	if err != nil {
		return nil, err
	}
	return d.Spec.IPAllowList, nil
}

// SetIPAllowList replaces the external-endpoint CIDR allowlist. Every entry must
// be a valid CIDR (a bad one is a 400 before any write); an empty list opens the
// endpoint to all source IPs. The operator maps it to a Traefik ipAllowList
// middleware on the SNI route.
func (s *Service) SetIPAllowList(ctx context.Context, name string, cidrs []string) (PostgresView, error) {
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return PostgresView{}, err
	}
	if err := core.ValidateCIDRs(cidrs); err != nil {
		return PostgresView{}, err
	}
	return s.patchDatabase(ctx, core.RelCanOperate, name, func(d *appv1alpha1.Database) {
		if len(cidrs) == 0 {
			d.Spec.IPAllowList = nil
		} else {
			d.Spec.IPAllowList = cidrs
		}
	})
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
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	d, err := s.fetchDatabase(ctx, core.RelCanView, name)
	if err != nil {
		return nil, err
	}
	out := make([]PostgresUserView, 0, len(d.Spec.Users))
	for _, u := range d.Spec.Users {
		out = append(out, PostgresUserView{Name: u.Name})
	}
	return out, nil
}

// CreateUser adds a managed login role: it generates a password into a
// per-user basic-auth Secret ("<db>-user-<role>") and records the role on the
// Database (spec.users), which the operator projects to CNPG's managed roles.
// The password is returned once and never logged.
func (s *Service) CreateUser(ctx context.Context, name, role string) (CreateUserResult, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return CreateUserResult{}, err
	}
	if !pgRoleName.MatchString(role) {
		return CreateUserResult{}, fmt.Errorf("%w: role must match %s", core.ErrBadRequest, pgRoleName.String())
	}
	d, err := s.fetchDatabase(ctx, core.RelCanCreate, name)
	if err != nil {
		return CreateUserResult{}, err
	}
	orig := d.DeepCopy()
	for _, u := range d.Spec.Users {
		if u.Name == role {
			return CreateUserResult{}, fmt.Errorf("%w: user %q already exists", core.ErrBadRequest, role)
		}
	}
	pw, err := generatePassword()
	if err != nil {
		return CreateUserResult{}, err
	}
	secretName := fmt.Sprintf("%s-user-%s", name, role)
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: s.Namespace},
		Type:       corev1.SecretTypeBasicAuth, // CNPG passwordSecret expects username/password
		Data: map[string][]byte{
			"username": []byte(role),
			"password": []byte(pw),
		},
	}
	if err := s.Client.Create(ctx, sec); err != nil && !apierrors.IsAlreadyExists(err) {
		return CreateUserResult{}, err
	}
	d.Spec.Users = append(d.Spec.Users, appv1alpha1.DatabaseUser{Name: role, SecretName: secretName})
	if err := s.Client.Patch(ctx, d, client.MergeFrom(orig)); err != nil {
		return CreateUserResult{}, err
	}
	return CreateUserResult{Name: role, Password: pw}, nil
}

// DeleteUser removes a managed login role from the Database's spec.users, so the
// operator stops managing it, and deletes its Secret. (CNPG stops reconciling the
// role; dropping the role from Postgres outright would need an ensure:absent
// tombstone, out of scope for basic CRUD.)
func (s *Service) DeleteUser(ctx context.Context, name, role string) error {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return err
	}
	d, err := s.fetchDatabase(ctx, core.RelCanCreate, name)
	if err != nil {
		return err
	}
	orig := d.DeepCopy()
	idx := -1
	for i, u := range d.Spec.Users {
		if u.Name == role {
			idx = i
			break
		}
	}
	if idx < 0 {
		return core.ErrNotFound
	}
	secretName := d.Spec.Users[idx].SecretName
	d.Spec.Users = append(d.Spec.Users[:idx], d.Spec.Users[idx+1:]...)
	if err := s.Client.Patch(ctx, d, client.MergeFrom(orig)); err != nil {
		return err
	}
	if secretName != "" {
		sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: s.Namespace}}
		if err := s.Client.Delete(ctx, sec); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// generatePassword returns a URL-safe 24-byte random password (same scheme the
// operator uses for KeyValue credentials).
func generatePassword() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
