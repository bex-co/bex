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

// Per-workspace sandbox snapshot pull credentials (w3/m42 t002,
// docs/ADR042-sandbox-cluster-substrate.md § D5). The patched OpenSandbox
// controller resumes a hibernated sandbox by recreating its pod from a rootfs
// snapshot image pushed to Zot under a namespace-nested repository
// ("snapshots/<ws>-sandbox/<sandbox>-<container>") and injects one fixed
// Secret name (--resume-pull-secret) into the pod's imagePullSecrets. Secret
// references are namespace-local, so each tenant sandbox namespace needs its
// own copy — and a shared credential would let any tenant pull other tenants'
// snapshot rootfs (which may embed workspace secrets). Each sandbox namespace
// therefore gets its own read-only Zot user ("snap-<namespace>") whose ACL
// covers only that namespace's snapshot repositories.
//
// Custody: the operator (this package) already owns the Zot htpasswd/config
// Secrets and the per-App credential lifecycle (w7/m36); the sandbox-namespace
// variant mirrors that machinery, keyed on the namespace instead of the App.

package registry

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// SnapshotPullSecretName is the fixed Secret name the OpenSandbox controller's
// --resume-pull-secret flag references. One Secret of this name exists per
// provisioned sandbox namespace, each carrying that namespace's own user.
const SnapshotPullSecretName = "bex-snapshot-pull"

// SnapshotZotUsername returns the Zot htpasswd username scoped to one sandbox
// namespace (the namespace is the workspace boundary under ADR043).
func SnapshotZotUsername(namespace string) string { return "snap-" + namespace }

// SnapshotRepoGlob returns the Zot ACL repository pattern covering exactly the
// namespace's snapshot repositories, matching the patched controller's
// namespace-nested layout under the "snapshots" registry prefix.
func SnapshotRepoGlob(namespace string) string { return "snapshots/" + namespace + "/**" }

// EnsureSnapshotCreds idempotently provisions the per-namespace resume-pull
// credential:
//  1. a kubernetes.io/dockerconfigjson Secret "bex-snapshot-pull" in the
//     sandbox namespace (kubelet uses it to pull the snapshot image on resume);
//  2. a "snap-<namespace>" entry in the zot-htpasswd Secret;
//  3. a read-only ACL entry for "snapshots/<namespace>/**" in zot-config.
//
// New htpasswd entries take effect only after a zot restart (the same
// no-reload behavior the per-App path works around with activation probes);
// resumes are rare and per-App reconciles bounce zot regularly, so this path
// settles for the shared rate-limited bounce instead of its own probe loop.
func (c *Creds) EnsureSnapshotCreds(ctx context.Context, namespace string) error {
	log := logf.FromContext(ctx)
	zotUser := SnapshotZotUsername(namespace)

	var pullSec corev1.Secret
	err := c.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: SnapshotPullSecretName}, &pullSec)
	var password string
	switch {
	case apierrors.IsNotFound(err):
		password, err = generatePassword()
		if err != nil {
			return fmt.Errorf("generate password: %w", err)
		}
		pullSec = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      SnapshotPullSecretName,
				Namespace: namespace,
				Labels:    map[string]string{"app.bex.co/component": "snapshot-pull"},
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				corev1.DockerConfigJsonKey: buildDockerConfig(zotUser, password, c.Registry, c.KpackRegistry),
			},
		}
		if err := c.Client.Create(ctx, &pullSec); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create snapshot pull secret: %w", err)
		}
		if err := c.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: SnapshotPullSecretName}, &pullSec); err != nil {
			return fmt.Errorf("read snapshot pull secret after create: %w", err)
		}
		log.Info("created snapshot pull secret", "namespace", namespace, "secret", SnapshotPullSecretName)
	case err != nil:
		return fmt.Errorf("get snapshot pull secret: %w", err)
	}

	if password == "" {
		password, err = extractPassword(pullSec.Data[corev1.DockerConfigJsonKey], c.Registry, zotUser)
		if err != nil {
			return fmt.Errorf("extract password from snapshot pull secret: %w", err)
		}
	}

	added, err := c.ensureHTPasswdEntry(ctx, zotUser, password)
	if err != nil {
		return fmt.Errorf("htpasswd: %w", err)
	}
	if err := c.ensureSnapshotZotConfigEntry(ctx, namespace, zotUser); err != nil {
		return fmt.Errorf("zot config: %w", err)
	}
	if added {
		// Activate the fresh credential within the bounce cooldown (see the
		// function comment); without this the first resume 401s until an
		// unrelated per-App activation bounces zot.
		c.tryBounce(ctx)
	}
	return nil
}

// RevokeSnapshotCreds removes the namespace's snapshot credential from the Zot
// htpasswd and config. The in-namespace pull Secret needs no explicit delete —
// it is removed with the namespace itself. Idempotent: revoking a namespace
// that never had credentials is a no-op.
func (c *Creds) RevokeSnapshotCreds(ctx context.Context, namespace string) error {
	if err := c.removeHTPasswdEntry(ctx, SnapshotZotUsername(namespace)); err != nil {
		return fmt.Errorf("htpasswd revoke: %w", err)
	}
	removed, err := c.removeSnapshotZotConfigEntry(ctx, namespace)
	if err != nil {
		return fmt.Errorf("zot config revoke: %w", err)
	}
	if removed {
		c.tryBounce(ctx)
	}
	return nil
}

// ensureSnapshotZotConfigEntry adds the read-only glob ACL for the namespace's
// snapshot repositories, bootstrapping zot-config from the base config when it
// does not exist yet (the same shape as ensureZotConfigEntry, without the
// per-App storage/builder maintenance that path owns).
func (c *Creds) ensureSnapshotZotConfigEntry(ctx context.Context, namespace, zotUser string) error {
	repo := SnapshotRepoGlob(namespace)
	for range 3 {
		var sec corev1.Secret
		err := c.Client.Get(ctx, client.ObjectKey{
			Namespace: c.ZotNamespace, Name: c.ConfigName,
		}, &sec)
		if apierrors.IsNotFound(err) {
			cfg, err := addZotReadOnlyACLEntry(c.baseZotConfig(), repo, zotUser)
			if err != nil {
				return err
			}
			newSec := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      c.ConfigName,
					Namespace: c.ZotNamespace,
					Labels:    map[string]string{"app.bex.co/managed-by": "bex-operator"},
				},
				Data: map[string][]byte{"config.json": cfg},
			}
			if err := c.Client.Create(ctx, &newSec); err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}
			return nil
		} else if err != nil {
			return err
		}

		existing := sec.Data["config.json"]
		if zotConfigHasRepoReadOnlyPolicy(existing, repo, zotUser) {
			return nil
		}
		updated, err := addZotReadOnlyACLEntry(existing, repo, zotUser)
		if err != nil {
			return err
		}
		patch := sec.DeepCopy()
		patch.Data["config.json"] = updated
		if err := c.Client.Patch(ctx, patch, client.MergeFrom(&sec)); err != nil {
			if apierrors.IsConflict(err) {
				continue
			}
			return err
		}
		return nil
	}
	return ErrConflictRequeue
}

// removeSnapshotZotConfigEntry deletes the namespace's snapshot ACL entry,
// reporting whether an entry was actually removed.
func (c *Creds) removeSnapshotZotConfigEntry(ctx context.Context, namespace string) (bool, error) {
	repo := SnapshotRepoGlob(namespace)
	for range 3 {
		var sec corev1.Secret
		if err := c.Client.Get(ctx, client.ObjectKey{
			Namespace: c.ZotNamespace, Name: c.ConfigName,
		}, &sec); apierrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}

		existing := sec.Data["config.json"]
		if !zotConfigHasRepo(existing, repo) {
			return false, nil
		}
		updated, err := removeZotACLEntry(existing, repo)
		if err != nil {
			return false, err
		}
		patch := sec.DeepCopy()
		patch.Data["config.json"] = updated
		if err := c.Client.Patch(ctx, patch, client.MergeFrom(&sec)); err != nil {
			if apierrors.IsConflict(err) {
				continue
			}
			return false, err
		}
		return true, nil
	}
	return false, ErrConflictRequeue
}

// addZotReadOnlyACLEntry adds a read-only per-repo policy for zotUser. Unlike
// addZotACLEntry (the per-App read/write grant), snapshot consumers only ever
// pull: the commit Job pushes as bex-builder through the global adminPolicy.
func addZotReadOnlyACLEntry(configJSON []byte, repo, zotUser string) ([]byte, error) {
	var data map[string]any
	if err := json.Unmarshal(configJSON, &data); err != nil {
		return nil, err
	}
	if repo == platformBuilderRepository {
		return ensureZotPlatformBuilderReadPolicy(configJSON)
	}
	repos := zotReposMap(data)
	repos[repo] = map[string]any{
		"policies": []any{
			map[string]any{
				"users":   []any{zotUser},
				"actions": []any{"read"},
			},
		},
	}
	return json.Marshal(data)
}

// zotConfigHasRepoReadOnlyPolicy reports whether repo grants zotUser exactly
// read access (and nothing more).
func zotConfigHasRepoReadOnlyPolicy(configJSON []byte, repo, zotUser string) bool {
	raw, ok := zotRepos(configJSON)[repo].(map[string]any)
	if !ok {
		return false
	}
	policies, _ := raw["policies"].([]any)
	if len(policies) != 1 {
		return false
	}
	policy, _ := policies[0].(map[string]any)
	users, _ := policy["users"].([]any)
	actions, _ := policy["actions"].([]any)
	return containsString(users, zotUser) && len(actions) == 1 && actions[0] == "read"
}
