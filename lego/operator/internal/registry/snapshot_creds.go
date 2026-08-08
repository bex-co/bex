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
	"fmt"
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
// New entries take effect only after a zot restart (the same no-reload
// behavior the per-App path works around with activation probes); resumes are
// rare and per-App reconciles bounce zot regularly, so this path settles for
// the shared rate-limited bounce instead of its own probe loop.
func (c *Creds) EnsureSnapshotCreds(ctx context.Context, namespace string) error {
	zotUser := SnapshotZotUsername(namespace)
	password, err := c.ensurePullSecret(ctx, namespace, SnapshotPullSecretName, zotUser, map[string]string{
		"app.bex.co/component": "snapshot-pull",
	})
	if err != nil {
		return err
	}

	wroteUser, err := c.ensureHTPasswdEntry(ctx, zotUser, password)
	if err != nil {
		return fmt.Errorf("htpasswd: %w", err)
	}
	wroteACL, err := c.ensureZotConfigEntry(ctx, SnapshotRepoGlob(namespace), zotUser, zotReadOnlyActions)
	if err != nil {
		return fmt.Errorf("zot config: %w", err)
	}
	if wroteUser || wroteACL {
		// Activate the fresh credential within the bounce cooldown; without
		// this the first resume 401s until an unrelated per-App activation
		// bounces zot.
		c.tryBounce(ctx)
	}
	return nil
}

// RevokeSnapshotCreds removes the namespace's snapshot credential from the Zot
// htpasswd and config. The in-namespace pull Secret needs no explicit delete —
// it is removed with the namespace itself. Idempotent: revoking a namespace
// that never had credentials is a no-op.
func (c *Creds) RevokeSnapshotCreds(ctx context.Context, namespace string) error {
	removedUser, err := c.removeHTPasswdEntry(ctx, SnapshotZotUsername(namespace))
	if err != nil {
		return fmt.Errorf("htpasswd revoke: %w", err)
	}
	removedACL, err := c.removeZotConfigEntry(ctx, SnapshotRepoGlob(namespace))
	if err != nil {
		return fmt.Errorf("zot config revoke: %w", err)
	}
	if removedUser || removedACL {
		c.tryBounce(ctx)
	}
	return nil
}
