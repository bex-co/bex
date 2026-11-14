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

// The sandbox storage meter's registry users (w22/m321, docs/ADR023 § Sandbox
// snapshot storage). bex-api sizes a suspended sandbox's retained image from its
// manifest and deletes a terminated sandbox's images, and until now it had no
// registry credential of its own: the only identity that could do either was
// bex-builder, which holds Zot's adminPolicy and must not be handed to bex-api.
// Two platform-wide users cover exactly that, each scoped to the snapshot prefix
// and nothing else:
//
//	bex-sandbox-meter   read          (sizing)
//	bex-sandbox-reaper  read, delete  (deleting a gone sandbox's images)
//
// They live in the registry namespace's Secret below, where the operator already
// manages Zot's own Secrets, so its Secret cache and RBAC stay as they are (the
// manager ClusterRole deliberately has no Secret access in bex-system). bex-api
// reads that one Secret by name through a namespaced Role.

package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// SandboxMeterSecretName is the Secret, in the registry namespace, that holds
	// the platform users' credentials for bex-api.
	SandboxMeterSecretName = "bex-sandbox-registry"

	SandboxMeterReaderUser = "bex-sandbox-meter"
	SandboxMeterReaperUser = "bex-sandbox-reaper"
	// SandboxImporterUser copies imported sandbox images into a workspace's
	// snapshot repositories (w2/m358).
	SandboxImporterUser = "bex-sandbox-importer"

	// The Secret's keys; bex-api reads the same names.
	SandboxMeterReaderUserKey     = "reader-username"
	SandboxMeterReaderPasswordKey = "reader-password"
	SandboxMeterReaperUserKey     = "reaper-username"
	SandboxMeterReaperPasswordKey = "reaper-password"
	SandboxImporterUserKey        = "importer-username"
	SandboxImporterPasswordKey    = "importer-password"

	// snapshotsRepoGlob covers every sandbox snapshot repository, which the
	// patched controller nests as snapshots/<ws>-sandbox/<sandbox>-<container>.
	snapshotsRepoGlob = "snapshots/**"
)

// zotReadDeleteActions is the reaper's grant: it reads a repository's tags and
// deletes them, and never writes an image.
var zotReadDeleteActions = []string{zotActionRead, zotActionDelete}

// zotImportActions is the importer's grant: it pushes an image and never
// deletes one (the reaper does).
var zotImportActions = []string{zotActionRead, zotActionCreate, zotActionUpdate}

// sandboxMeterUsers are the platform's users on the snapshot repositories: each
// one's Secret keys and grant.
var sandboxMeterUsers = []struct {
	user, userKey, passwordKey string
	actions                    []string
}{
	{SandboxMeterReaderUser, SandboxMeterReaderUserKey, SandboxMeterReaderPasswordKey, zotReadOnlyActions},
	{SandboxMeterReaperUser, SandboxMeterReaperUserKey, SandboxMeterReaperPasswordKey, zotReadDeleteActions},
	{SandboxImporterUser, SandboxImporterUserKey, SandboxImporterPasswordKey, zotImportActions},
}

// sandboxMeterGrants are the platform's grants on a snapshot pattern.
func sandboxMeterGrants() []zotGrant {
	grants := make([]zotGrant, 0, len(sandboxMeterUsers))
	for _, u := range sandboxMeterUsers {
		grants = append(grants, zotGrant{u.user, u.actions})
	}
	return grants
}

// isSnapshotNamespaceGlob reports whether repo is a workspace's own snapshot
// pattern ("snapshots/<ns>/**"), which Zot matches ahead of "snapshots/**".
func isSnapshotNamespaceGlob(repo string) bool {
	return repo != snapshotsRepoGlob && strings.HasPrefix(repo, "snapshots/") && strings.HasSuffix(repo, "/**")
}

// snapshotPlatformGrants is what a snapshot pattern carries besides its
// namespace's own read-only user: the meter's two users, once they are minted.
func (c *Creds) snapshotPlatformGrants(repo string) []zotGrant {
	if c.SandboxMeterUsers && isSnapshotNamespaceGlob(repo) {
		return sandboxMeterGrants()
	}
	return nil
}

// EnsureSandboxMeterCreds idempotently provisions the platform's users:
//  1. the Secret SandboxMeterSecretName in the registry namespace, with a
//     generated password for each (a missing or empty one is regenerated);
//  2. a htpasswd entry for each in the zot-htpasswd Secret;
//  3. an ACL entry for each on "snapshots/**" in zot-config: read for the
//     meter, read and delete for the reaper, read and push for the importer.
//
// A new entry takes effect once Zot reloads the projected Secrets, like every
// other credential here, so a write nudges the Zot pod (verify.go).
func (c *Creds) EnsureSandboxMeterCreds(ctx context.Context) error {
	passwords, err := c.ensureSandboxMeterSecret(ctx)
	if err != nil {
		return err
	}
	wrote := false
	for _, u := range sandboxMeterUsers {
		wroteUser, err := c.ensureHTPasswdEntry(ctx, u.user, passwords[u.user])
		if err != nil {
			return fmt.Errorf("htpasswd %s: %w", u.user, err)
		}
		// A per-user policy on the shared pattern: the users must not replace
		// each other's, and the per-namespace snap-* users are on other patterns.
		wroteACL, err := c.grantZotRepoUser(ctx, snapshotsRepoGlob, u.user, u.actions)
		if err != nil {
			return fmt.Errorf("zot config %s: %w", u.user, err)
		}
		wrote = wrote || wroteUser || wroteACL
	}
	// Zot applies only the longest matching pattern, so a workspace's own
	// "snapshots/<ns>/**" shadows "snapshots/**": without these grants the meter
	// can read, the reaper delete and the importer push only where a workspace
	// has no pattern of its own (production, 2026-11-11: 403 for every workspace
	// that has one).
	wroteNamespaces, err := c.grantMeterOnNamespacePatterns(ctx)
	if err != nil {
		return fmt.Errorf("zot config namespace patterns: %w", err)
	}
	if wrote || wroteNamespaces {
		c.nudgeZot(ctx)
	}
	return nil
}

// grantMeterOnNamespacePatterns adds the platform's users to every existing
// "snapshots/<ns>/**" pattern in one document hold, leaving the namespace's own
// user and every other policy where they are.
func (c *Creds) grantMeterOnNamespacePatterns(ctx context.Context) (bool, error) {
	return c.mutateSecretKey(ctx, c.ConfigName, zotConfigKey, c.baseZotConfig,
		func(current []byte) ([]byte, bool, error) {
			data, err := decodeZotConfig(current)
			if err != nil {
				return nil, false, err
			}
			var patterns []string
			for repo := range zotRepos(data) {
				if isSnapshotNamespaceGlob(repo) {
					patterns = append(patterns, repo)
				}
			}
			changed := false
			for _, repo := range patterns {
				for _, g := range sandboxMeterGrants() {
					if !zotRepoUserGrants(data, repo, g.user, g.actions) {
						addZotRepoUser(data, repo, g.user, g.actions)
						changed = true
					}
				}
			}
			if !changed {
				return nil, false, nil
			}
			next, err := json.Marshal(data)
			return next, err == nil, err
		})
}

// ensureSandboxMeterSecret gets or creates the credentials Secret, filling any
// password it lacks, and returns each user's password it now holds.
func (c *Creds) ensureSandboxMeterSecret(ctx context.Context) (map[string]string, error) {
	key := client.ObjectKey{Namespace: c.ZotNamespace, Name: SandboxMeterSecretName}
	var sec corev1.Secret
	create := false
	switch err := c.Client.Get(ctx, key, &sec); {
	case apierrors.IsNotFound(err):
		create = true
		sec = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: SandboxMeterSecretName, Namespace: c.ZotNamespace,
				Labels: map[string]string{"app.bex.co/component": "sandbox-meter-registry"},
			},
			Type: corev1.SecretTypeOpaque,
		}
	case err != nil:
		return nil, fmt.Errorf("get %s: %w", SandboxMeterSecretName, err)
	}
	changed := create
	if sec.Data == nil {
		sec.Data = map[string][]byte{}
	}
	for _, field := range sandboxMeterUsers {
		if string(sec.Data[field.userKey]) != field.user {
			sec.Data[field.userKey] = []byte(field.user)
			changed = true
		}
		if len(sec.Data[field.passwordKey]) == 0 {
			password, err := generatePassword()
			if err != nil {
				return nil, fmt.Errorf("generate password: %w", err)
			}
			sec.Data[field.passwordKey] = []byte(password)
			changed = true
		}
	}
	if changed {
		var err error
		if create {
			err = c.Client.Create(ctx, &sec)
		} else {
			err = c.Client.Update(ctx, &sec)
		}
		switch {
		case err == nil:
			logf.FromContext(ctx).Info("provisioned sandbox meter registry credentials", "secret", SandboxMeterSecretName)
		case create && apierrors.IsAlreadyExists(err):
			// A concurrent reconcile won the race; its passwords are the ones
			// the htpasswd entries must hash.
			return c.ensureSandboxMeterSecret(ctx)
		default:
			return nil, fmt.Errorf("write %s: %w", SandboxMeterSecretName, err)
		}
	}
	passwords := make(map[string]string, len(sandboxMeterUsers))
	for _, u := range sandboxMeterUsers {
		passwords[u.user] = string(sec.Data[u.passwordKey])
	}
	return passwords, nil
}
