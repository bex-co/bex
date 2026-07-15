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

// Package sshkeys manages identity-scoped SSH public keys. It never accepts or
// stores private material; the separately deployed SSH gateway resolves a
// presented key's canonical SHA256 fingerprint through the same store.
package sshkeys

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	maxKeyNameBytes   = 120
	maxPublicKeyBytes = 16 << 10
)

// Store is the public management service's narrow persistence seam.
// The separately deployed gateway owns its own lookup-and-audit interface so
// management adapters cannot accidentally grow gateway authority.
// *store.PGStore satisfies both interfaces structurally.
type Store interface {
	CreateSSHKey(context.Context, store.SSHKey) (store.SSHKey, error)
	ListSSHKeys(context.Context, string) ([]store.SSHKey, error)
	DeleteSSHKey(context.Context, string, string) error
}

type Service struct {
	*core.Base
	Store Store
}

func callerSubject(ctx context.Context) (string, error) {
	id, ok := core.IdentityFrom(ctx)
	if !ok || strings.TrimSpace(id.Subject) == "" {
		return "", core.ErrForbidden
	}
	return id.Subject, nil
}

// NormalizePublicKey parses exactly one authorized_keys record and returns its
// canonical key text and OpenSSH SHA256 fingerprint. Comments are deliberately
// discarded so equivalent key material cannot evade duplicate detection.
func NormalizePublicKey(raw string) (canonical, fingerprint string, err error) {
	if len(raw) > maxPublicKeyBytes {
		return "", "", fmt.Errorf("%w: invalid SSH public key", core.ErrBadRequest)
	}
	publicKey, _, options, rest, parseErr := ssh.ParseAuthorizedKey([]byte(strings.TrimSpace(raw)))
	if parseErr != nil || len(options) != 0 || len(strings.TrimSpace(string(rest))) != 0 {
		return "", "", fmt.Errorf("%w: invalid SSH public key", core.ErrBadRequest)
	}
	switch publicKey.Type() {
	case ssh.KeyAlgoED25519, ssh.KeyAlgoRSA,
		ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521,
		ssh.KeyAlgoSKED25519, ssh.KeyAlgoSKECDSA256:
	default:
		return "", "", fmt.Errorf("%w: unsupported SSH public key type %q", core.ErrBadRequest, publicKey.Type())
	}
	if publicKey.Type() == ssh.KeyAlgoRSA {
		cryptoKey, ok := publicKey.(ssh.CryptoPublicKey)
		if !ok {
			return "", "", fmt.Errorf("%w: invalid SSH public key", core.ErrBadRequest)
		}
		rsaKey, ok := cryptoKey.CryptoPublicKey().(*rsa.PublicKey)
		if !ok || rsaKey.N.BitLen() < 2048 {
			return "", "", fmt.Errorf("%w: RSA SSH public keys must be at least 2048 bits", core.ErrBadRequest)
		}
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(publicKey))), ssh.FingerprintSHA256(publicKey), nil
}

func (s *Service) Create(ctx context.Context, name, rawPublicKey string) (store.SSHKey, error) {
	if err := s.Authorize(ctx, core.RelCanManageSSHKeys); err != nil {
		return store.SSHKey{}, err
	}
	if s.Store == nil {
		return store.SSHKey{}, core.ErrSSHKeysUnavailable
	}
	subject, err := callerSubject(ctx)
	if err != nil {
		return store.SSHKey{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxKeyNameBytes {
		return store.SSHKey{}, fmt.Errorf("%w: SSH key name must be 1-%d bytes", core.ErrBadRequest, maxKeyNameBytes)
	}
	canonical, fingerprint, err := NormalizePublicKey(rawPublicKey)
	if err != nil {
		return store.SSHKey{}, err
	}
	key, err := s.Store.CreateSSHKey(ctx, store.SSHKey{
		ID: ids.New(ids.SSHKey), Subject: subject, Name: name,
		PublicKey: canonical, Fingerprint: fingerprint,
	})
	if errors.Is(err, store.ErrConflict) {
		return store.SSHKey{}, fmt.Errorf("%w: SSH public key is already registered", core.ErrConflict)
	}
	return key, err
}

func (s *Service) List(ctx context.Context) ([]store.SSHKey, error) {
	if err := s.Authorize(ctx, core.RelCanManageSSHKeys); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrSSHKeysUnavailable
	}
	subject, err := callerSubject(ctx)
	if err != nil {
		return nil, err
	}
	return s.Store.ListSSHKeys(ctx, subject)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.Authorize(ctx, core.RelCanManageSSHKeys); err != nil {
		return err
	}
	if s.Store == nil {
		return core.ErrSSHKeysUnavailable
	}
	if kind, ok := ids.KindOf(id); !ok || kind != ids.SSHKey {
		return core.ErrNotFound
	}
	subject, err := callerSubject(ctx)
	if err != nil {
		return err
	}
	if err := s.Store.DeleteSSHKey(ctx, subject, id); errors.Is(err, store.ErrNotFound) {
		return core.ErrNotFound
	} else {
		return err
	}
}
