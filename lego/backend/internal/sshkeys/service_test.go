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

package sshkeys

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type memoryStore struct {
	keys map[string]store.SSHKey
}

type relationChecker struct {
	relations []string
}

func (c *relationChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	c.relations = append(c.relations, relation)
	return true, nil
}

func (m *memoryStore) CreateSSHKey(_ context.Context, key store.SSHKey) (store.SSHKey, error) {
	for _, existing := range m.keys {
		if existing.Fingerprint == key.Fingerprint {
			return store.SSHKey{}, store.ErrConflict
		}
	}
	key.CreatedAt = time.Now().UTC()
	m.keys[key.ID] = key
	return key, nil
}

func (m *memoryStore) ListSSHKeys(_ context.Context, subject string) ([]store.SSHKey, error) {
	var out []store.SSHKey
	for _, key := range m.keys {
		if key.Subject == subject {
			out = append(out, key)
		}
	}
	return out, nil
}

func (m *memoryStore) DeleteSSHKey(_ context.Context, subject, id string) error {
	key, ok := m.keys[id]
	if !ok || key.Subject != subject {
		return store.ErrNotFound
	}
	delete(m.keys, id)
	return nil
}

func (m *memoryStore) SSHKeyByFingerprint(_ context.Context, fingerprint string) (store.SSHKey, error) {
	for _, key := range m.keys {
		if key.Fingerprint == fingerprint {
			return key, nil
		}
	}
	return store.SSHKey{}, store.ErrNotFound
}

func publicKey(t *testing.T) string {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))) + " laptop comment"
}

func identity(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
}

func TestServiceOwnsCanonicalKeysAndRejectsAmbiguity(t *testing.T) {
	st := &memoryStore{keys: map[string]store.SSHKey{}}
	svc := &Service{Base: &core.Base{}, Store: st}
	raw := publicKey(t)

	created, err := svc.Create(identity("user-a"), "laptop", raw)
	if err != nil {
		t.Fatal(err)
	}
	if created.Subject != "user-a" || strings.Contains(created.PublicKey, "comment") || !strings.HasPrefix(created.Fingerprint, "SHA256:") {
		t.Fatalf("created key was not canonical and scoped: %+v", created)
	}
	if _, err := svc.Create(identity("user-b"), "same material", raw); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("duplicate across identities = %v, want conflict", err)
	}
	if got, err := svc.List(identity("user-b")); err != nil || len(got) != 0 {
		t.Fatalf("foreign list = %+v, %v", got, err)
	}
	if err := svc.Delete(identity("user-b"), created.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("foreign delete = %v, want constant-shape not found", err)
	}
	if err := svc.Delete(identity("user-a"), created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SSHKeyByFingerprint(context.Background(), created.Fingerprint); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted fingerprint still authenticates: %v", err)
	}
}

func TestEveryKeyManagementVerbUsesMemberSSHKeyPermission(t *testing.T) {
	checker := &relationChecker{}
	st := &memoryStore{keys: map[string]store.SSHKey{}}
	svc := &Service{Base: &core.Base{Authz: checker}, Store: st}
	ctx := identity("user-a")
	created, err := svc.Create(ctx, "laptop", publicKey(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.List(ctx); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if len(checker.relations) != 3 {
		t.Fatalf("authorization checks = %v, want three", checker.relations)
	}
	for _, relation := range checker.relations {
		if relation != core.RelCanManageSSHKeys {
			t.Fatalf("authorization relations = %v, want only %q", checker.relations, core.RelCanManageSSHKeys)
		}
	}
}

func TestNormalizePublicKeyRejectsPrivateMalformedAndTrailingRecords(t *testing.T) {
	valid := publicKey(t)
	for name, raw := range map[string]string{
		"private":                 "-----BEGIN OPENSSH PRIVATE KEY-----\nsecret\n-----END OPENSSH PRIVATE KEY-----",
		"malformed":               "ssh-ed25519 not-base64",
		"two keys":                valid + "\n" + publicKey(t),
		"authorized_keys options": `command="false",no-port-forwarding ` + valid,
		"oversized":               strings.Repeat("x", maxPublicKeyBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := NormalizePublicKey(raw); !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("NormalizePublicKey = %v, want bad request", err)
			}
		})
	}
}

func TestNormalizePublicKeySupportsDocumentedKeyFamilies(t *testing.T) {
	_, edPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ecdsaPrivate, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	rsaPrivate, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	for name, public := range map[string]any{
		"ed25519": edPrivate.Public(),
		"ecdsa":   ecdsaPrivate.Public(),
		"rsa":     rsaPrivate.Public(),
	} {
		t.Run(name, func(t *testing.T) {
			key, err := ssh.NewPublicKey(public)
			if err != nil {
				t.Fatal(err)
			}
			raw := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))) + " test-comment"
			canonical, fingerprint, err := NormalizePublicKey(raw)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(canonical, "test-comment") || fingerprint != ssh.FingerprintSHA256(key) {
				t.Fatalf("canonical/fingerprint = %q / %q", canonical, fingerprint)
			}
		})
	}

	// Public vectors generated by OpenSSH exercise the two FIDO/security-key
	// wire formats without requiring hardware in CI. Signatures remain the
	// client's authenticator's job; registration only parses public material.
	for name, raw := range map[string]string{
		"sk-ed25519": "sk-ssh-ed25519@openssh.com AAAAGnNrLXNzaC1lZDI1NTE5QG9wZW5zc2guY29tAAAAIJjzc2a20RjCvN/0ibH6UpGuN9F9hDvD7x182bOesNhHAAAABHNzaDo= test",
		"sk-ecdsa":   "sk-ecdsa-sha2-nistp256@openssh.com AAAAInNrLWVjZHNhLXNoYTItbmlzdHAyNTZAb3BlbnNzaC5jb20AAAAIbmlzdHAyNTYAAABBBGRNqlFgED/pf4zXz8IzqA6CALNwYcwgd4MQDmIS1GOtn1SySFObiuyJaOlpqkV5FeEifhxfIC2ejKKtNyO4CysAAAAEc3NoOg== test",
	} {
		t.Run(name, func(t *testing.T) {
			canonical, fingerprint, err := NormalizePublicKey(raw)
			if err != nil {
				t.Fatal(err)
			}
			if canonical == "" || fingerprint == "" || strings.Contains(canonical, " test") {
				t.Fatalf("canonical/fingerprint = %q / %q", canonical, fingerprint)
			}
		})
	}
}

func TestNormalizePublicKeyRejectsWeakRSA(t *testing.T) {
	private, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(&private.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	raw := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
	if _, _, err := NormalizePublicKey(raw); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("weak RSA key = %v, want bad request", err)
	}
}
