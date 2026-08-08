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

// Package registry manages per-App Zot repository credentials (w7/m36,
// docs/ADR022-tenant-isolation.md). Each App that builds and pushes an image
// to the in-cluster Zot registry receives its own htpasswd user ("app-<name>")
// and a per-repo Zot ACL entry that restricts that user to its own image
// repository. A kubernetes.io/dockerconfigjson Secret
// ("reg-pull-<name>") is created in the App namespace and referenced as an
// imagePullSecret on all tenant workloads.
//
// The operator manages two Secrets in the Zot namespace:
//   - zot-htpasswd: htpasswd file (bcrypt) with bex-builder + per-App entries.
//   - zot-config:   full Zot config.json with a global builder admin policy and
//     per-App repo ACL entries.
//
// Both are mounted as external Secrets by the Zot Helm chart
// (deploy/gitops/base/zot.yaml). The running Zot never re-reads them — writes
// only take effect after a zot pod restart (w9/m43; the full account lives in
// verify.go and docs/ADR022-tenant-isolation.md § Credential activation), so
// EnsureActive (verify.go) verifies activation and bounces the pod when needed.
package registry

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"golang.org/x/crypto/bcrypt"
)

// Creds manages per-App Zot pull credentials. A nil *Creds disables the
// feature: the operator falls back to the shared RegistryPullSecret (w7/m8).
type Creds struct {
	Client         client.Client
	ZotNamespace   string // namespace that holds the zot Secrets (e.g. "bex-registry")
	HTPasswdName   string // Secret name for the Zot htpasswd (e.g. "zot-htpasswd")
	ConfigName     string // Secret name for the Zot config JSON (e.g. "zot-config")
	Registry       string // canonical registry host (e.g. "zot.bex-registry.svc:5000")
	KpackRegistry  string // optional alias used by containerd on nodes (e.g. "zot.local:5000")
	RetentionCount int    // mostRecentlyPushedCount in baseZotConfig; 0 defaults to 5

	// Credential activation (w9/m43, verify.go). Zero values mean the package
	// defaults; tests shrink them.
	HTTPClient      *http.Client  // probe client; nil => shared 5s-timeout default
	ActivationGrace time.Duration // rejected-probe tolerance before bouncing zot (default 30s)
	BounceCooldown  time.Duration // minimum time between zot bounces, across all Apps (default 2m)

	baseOnce    sync.Once
	baseJSON    []byte
	baseStorage map[string]any

	mu         sync.Mutex
	apps       map[string]activation
	lastBounce time.Time
}

// zotHTPasswdPath and zotHTTPPort are contract values that the Zot Helm chart
// mounts and exposes. A drift guard in TestBaseZotConfigContractValues pins them.
const (
	zotHTPasswdPath = "/secret/htpasswd"
	zotHTTPPort     = "5000"
	// platformBuilderRepository is the shared kpack ClusterBuilder image. Every
	// authenticated App build must be able to pull it, but only bex-builder may
	// create, update, or delete it (through the global adminPolicy).
	platformBuilderRepository = "bex-cnb-builder"
)

// PullSecretName returns the deterministic name of the per-App pull-credential
// Secret in the App namespace.
func PullSecretName(appName string) string { return "reg-pull-" + appName }

// ZotUsername returns the Zot htpasswd username for an App.
func ZotUsername(appName string) string { return "app-" + appName }

// EnsureCreds idempotently creates the per-App repository credential:
//  1. Creates (or reads) a kubernetes.io/dockerconfigjson Secret "reg-pull-<appName>"
//     in appNS with credentials for the per-App Zot user "app-<appName>".
//  2. Adds "app-<appName>:bcrypt(password)" to the zot-htpasswd Secret.
//  3. Adds a per-repo ACL entry for "<appName>" in the zot-config Secret so
//     that "app-<appName>" can read/write only the "<appName>" repository.
func (c *Creds) EnsureCreds(ctx context.Context, appName, appNS string) error {
	zotUser := ZotUsername(appName)
	password, err := c.ensurePullSecret(ctx, appNS, PullSecretName(appName), zotUser, map[string]string{
		"app.bex.co/component": "registry-pull",
		"app.bex.co/app":       appName,
	})
	if err != nil {
		return err
	}

	wroteUser, err := c.ensureHTPasswdEntry(ctx, zotUser, password)
	if err != nil {
		return fmt.Errorf("htpasswd: %w", err)
	}
	wroteACL, err := c.ensureZotConfigEntry(ctx, appName, zotUser, zotReadWriteActions)
	if err != nil {
		return fmt.Errorf("zot config: %w", err)
	}
	if wroteUser || wroteACL {
		// The running zot cannot honor either write until it restarts, so the
		// activation clock starts here (verify.go).
		c.recordWrite(appName)
	}
	return nil
}

// RevokeCreds removes the per-App credentials from the Zot htpasswd and config.
// The per-App pull Secret must be explicitly deleted by the caller (cross-namespace
// objects cannot carry owner references, so owner-ref GC does not apply here).
// The app controller's revokeAppRegistryCredentials does this.
//
// The running zot keeps accepting the removed credential until it restarts
// (the same no-reload behavior activation works around), so revocation ends
// with a best-effort rate-limited bounce; if the cooldown skips it, the
// credential stays live until the next bounce or zot restart.
func (c *Creds) RevokeCreds(ctx context.Context, appName string) error {
	c.clearActivation(appName)
	removedUser, err := c.removeHTPasswdEntry(ctx, ZotUsername(appName))
	if err != nil {
		return fmt.Errorf("htpasswd revoke: %w", err)
	}
	removedACL, err := c.removeZotConfigEntry(ctx, appName)
	if err != nil {
		return fmt.Errorf("zot config revoke: %w", err)
	}
	if removedUser || removedACL {
		c.tryBounce(ctx)
	}
	return nil
}

// RotateCreds re-issues the per-App pull credential with a newly generated
// password: the pull Secret and the htpasswd hash both move to it. The ACL
// entry is unchanged — the Zot username stays the same, only the password
// rotates.
//
// Runbook: trigger rotation by setting the annotation
//
//	bex.co/rotate-registry-creds: "true"
//
// on the App CR. The reconciler detects this, calls RotateCreds, then clears
// the annotation. Workloads pick up the new pull Secret on the next kubelet
// credential refresh, but the running zot still holds the old hash until the
// activation loop (verify.go) re-proves the credential — expect a short
// RegistryCredsPending window (bounded by the activation grace + a bounce)
// after every rotation.
func (c *Creds) RotateCreds(ctx context.Context, appName, appNS string) error {
	log := logf.FromContext(ctx)
	zotUser := ZotUsername(appName)
	pullSecName := PullSecretName(appName)

	newPassword, err := generatePassword()
	if err != nil {
		return fmt.Errorf("generate new password: %w", err)
	}

	var pullSec corev1.Secret
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: appNS, Name: pullSecName}, &pullSec); err != nil {
		return fmt.Errorf("get pull secret for rotation: %w", err)
	}
	patch := pullSec.DeepCopy()
	patch.Data[corev1.DockerConfigJsonKey] = c.dockerConfig(zotUser, newPassword)
	if err := c.Client.Patch(ctx, patch, client.MergeFrom(&pullSec)); err != nil {
		return fmt.Errorf("patch pull secret: %w", err)
	}
	log.Info("rotated per-app pull secret", "app", appName, "secret", pullSecName)

	if err := c.replaceHTPasswdEntry(ctx, zotUser, newPassword); err != nil {
		return fmt.Errorf("htpasswd rotation: %w", err)
	}
	// The running zot deterministically rejects the new password until it
	// restarts: restart the activation clock and drop the stale acceptance.
	c.recordWrite(appName)
	return nil
}

// -- pull Secret --------------------------------------------------------------

// ensurePullSecret gets or creates a dockerconfigjson Secret carrying zotUser's
// registry credential, returning the password it now holds. When a concurrent
// reconcile wins the create race its password is the authoritative one, since
// that is what the htpasswd entry must hash.
func (c *Creds) ensurePullSecret(ctx context.Context, ns, name, zotUser string, labels map[string]string) (string, error) {
	key := client.ObjectKey{Namespace: ns, Name: name}
	var sec corev1.Secret
	switch err := c.Client.Get(ctx, key, &sec); {
	case apierrors.IsNotFound(err):
		password, err := generatePassword()
		if err != nil {
			return "", fmt.Errorf("generate password: %w", err)
		}
		sec = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: labels},
			Type:       corev1.SecretTypeDockerConfigJson,
			Data:       map[string][]byte{corev1.DockerConfigJsonKey: c.dockerConfig(zotUser, password)},
		}
		switch err := c.Client.Create(ctx, &sec); {
		case err == nil:
			logf.FromContext(ctx).Info("created registry pull secret", "namespace", ns, "secret", name)
			return password, nil
		case apierrors.IsAlreadyExists(err):
			if err := c.Client.Get(ctx, key, &sec); err != nil {
				return "", fmt.Errorf("read pull secret after create race: %w", err)
			}
		default:
			return "", fmt.Errorf("create pull secret: %w", err)
		}
	case err != nil:
		return "", fmt.Errorf("get pull secret: %w", err)
	}

	password, err := c.passwordFrom(sec.Data[corev1.DockerConfigJsonKey], zotUser)
	if err != nil {
		return "", fmt.Errorf("extract password from pull secret: %w", err)
	}
	return password, nil
}

func generatePassword() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// dockerConfig returns a docker-config JSON for the given user/password,
// covering the kpack registry alias too when it differs from the canonical one.
func (c *Creds) dockerConfig(username, password string) []byte {
	type authEntry struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Auth     string `json:"auth"`
	}
	entry := authEntry{
		Username: username,
		Password: password,
		Auth:     base64.StdEncoding.EncodeToString([]byte(username + ":" + password)),
	}
	auths := map[string]authEntry{c.Registry: entry}
	if c.KpackRegistry != "" && c.KpackRegistry != c.Registry {
		auths[c.KpackRegistry] = entry
	}
	data, _ := json.Marshal(map[string]any{"auths": auths})
	return data
}

// passwordFrom reads the plaintext password a docker-config holds for username
// on the canonical registry.
func (c *Creds) passwordFrom(dockerConfigJSON []byte, username string) (string, error) {
	user, password, ok := BasicAuthFromDockerConfig(dockerConfigJSON, c.Registry)
	if !ok {
		return "", fmt.Errorf("no auth entry for %q", c.Registry)
	}
	if user != username {
		return "", fmt.Errorf("auth entry for %q has user %q, want %q", c.Registry, user, username)
	}
	return password, nil
}

// -- htpasswd Secret ----------------------------------------------------------

// ensureHTPasswdEntry makes username's entry match password, reporting whether
// it had to write. A hash that no longer verifies is replaced: when a pull
// Secret is deleted and recreated a new password is generated, and the stale
// htpasswd hash must be re-synced to it.
func (c *Creds) ensureHTPasswdEntry(ctx context.Context, username, password string) (bool, error) {
	return c.mutateSecretKey(ctx, c.HTPasswdName, htpasswdKey, nil,
		func(current []byte) ([]byte, bool, error) {
			if hash, found := htpasswdUserHash(current, username); found &&
				bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil {
				return nil, false, nil
			}
			next, err := setHTPasswdLine(current, username, password)
			return next, err == nil, err
		})
}

// replaceHTPasswdEntry unconditionally rehashes username's entry (rotation:
// the new password must displace a hash that still verifies against the old).
func (c *Creds) replaceHTPasswdEntry(ctx context.Context, username, password string) error {
	_, err := c.mutateSecretKey(ctx, c.HTPasswdName, htpasswdKey, nil,
		func(current []byte) ([]byte, bool, error) {
			next, err := setHTPasswdLine(current, username, password)
			return next, err == nil, err
		})
	return err
}

// removeHTPasswdEntry drops username's entry, reporting whether one was there.
func (c *Creds) removeHTPasswdEntry(ctx context.Context, username string) (bool, error) {
	removed, err := c.mutateSecretKey(ctx, c.HTPasswdName, htpasswdKey, nil,
		func(current []byte) ([]byte, bool, error) {
			if _, found := htpasswdUserHash(current, username); !found {
				return nil, false, nil
			}
			return removeHTPasswdLine(current, username), true, nil
		})
	return removed, client.IgnoreNotFound(err)
}

// -- Zot config Secret --------------------------------------------------------

// ensureZotConfigEntry grants zotUser exactly actions on repo, and brings the
// operator-owned storage and platform-builder policies up to canonical while it
// holds the document. The builder migration matters for configs created before
// per-App ACLs: Zot applies only the longest repository match, so an exact
// per-App rule shadows the builder's ** rule. Bootstraps the Secret from the
// canonical base config when it does not exist.
func (c *Creds) ensureZotConfigEntry(ctx context.Context, repo, zotUser string, actions []string) (bool, error) {
	return c.mutateSecretKey(ctx, c.ConfigName, zotConfigKey, c.baseZotConfig,
		func(current []byte) ([]byte, bool, error) {
			data, err := decodeZotConfig(current)
			if err != nil {
				return nil, false, err
			}
			changed := setZotStorage(data, c.canonicalStorage())
			if !zotHasBuilderAdminPolicy(data) {
				setZotBuilderAdminPolicy(data)
				changed = true
			}
			if !zotHasPlatformBuilderPolicy(data) {
				setZotPlatformBuilderPolicy(data)
				changed = true
			}
			// The platform builder repository is never tenant-owned: an App
			// whose public name collides with it must not replace the shared
			// read-only rule with tenant write permission.
			if repo != platformBuilderRepository && !zotRepoGrants(data, repo, zotUser, actions) {
				setZotRepoPolicy(data, repo, zotUser, actions)
				changed = true
			}
			if !changed {
				return nil, false, nil
			}
			next, err := json.Marshal(data)
			return next, err == nil, err
		})
}

// removeZotConfigEntry drops repo's ACL entry, reporting whether one was there.
func (c *Creds) removeZotConfigEntry(ctx context.Context, repo string) (bool, error) {
	removed, err := c.mutateSecretKey(ctx, c.ConfigName, zotConfigKey, nil,
		func(current []byte) ([]byte, bool, error) {
			data, err := decodeZotConfig(current)
			if err != nil {
				return nil, false, err
			}
			// A colliding App's deletion must not strip the platform rule.
			if repo == platformBuilderRepository || !zotHasRepo(data, repo) {
				return nil, false, nil
			}
			delete(zotRepos(data), repo)
			next, err := json.Marshal(data)
			return next, err == nil, err
		})
	return removed, client.IgnoreNotFound(err)
}

// decodeZotConfig parses a stored config document. A malformed config is an
// error rather than an empty document: silently treating it as "no entries"
// would let a revoke report success while the credential stayed live. An
// absent document is empty, and converges on the next ensure.
func decodeZotConfig(configJSON []byte) (map[string]any, error) {
	if len(configJSON) == 0 {
		return map[string]any{}, nil
	}
	var data map[string]any
	if err := json.Unmarshal(configJSON, &data); err != nil {
		return nil, fmt.Errorf("parse zot config: %w", err)
	}
	if data == nil {
		data = map[string]any{}
	}
	return data, nil
}
