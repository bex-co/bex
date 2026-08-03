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
	"reflect"
	"strings"
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

	mu            sync.Mutex
	activated     map[string]bool      // app name -> probe accepted (invalidated on rotate/revoke)
	writtenAt     map[string]time.Time // app name -> last htpasswd write by this process
	firstRejected map[string]time.Time // app name -> first rejected probe (grace fallback anchor)
	lastBounce    time.Time
}

// ErrConflictRequeue is returned when a Kubernetes write conflict persists
// beyond the short in-function retry budget. The controller should requeue
// the reconcile rather than fail permanently.
var ErrConflictRequeue = fmt.Errorf("persistent write conflict: requeue required")

// zotHTPasswdPath and zotHTTPPort are contract values that the Zot Helm chart
// mounts and exposes. A drift guard in TestBaseZotConfigContractValues pins them.
const (
	zotHTPasswdPath = "/secret/htpasswd"
	zotHTTPPort     = "5000"
	zotActionRead   = "read"
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
	log := logf.FromContext(ctx)
	zotUser := ZotUsername(appName)
	pullSecName := PullSecretName(appName)

	// 1. Get or create the per-App docker-config pull Secret.
	var pullSec corev1.Secret
	err := c.Client.Get(ctx, client.ObjectKey{Namespace: appNS, Name: pullSecName}, &pullSec)
	var password string
	switch {
	case apierrors.IsNotFound(err):
		password, err = generatePassword()
		if err != nil {
			return fmt.Errorf("generate password: %w", err)
		}
		pullSec = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pullSecName,
				Namespace: appNS,
				Labels:    map[string]string{"app.bex.co/component": "registry-pull", "app.bex.co/app": appName},
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				corev1.DockerConfigJsonKey: buildDockerConfig(zotUser, password, c.Registry, c.KpackRegistry),
			},
		}
		if err := c.Client.Create(ctx, &pullSec); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create pull secret: %w", err)
		}
		// Re-read in case we raced with another reconcile that already created it.
		if err := c.Client.Get(ctx, client.ObjectKey{Namespace: appNS, Name: pullSecName}, &pullSec); err != nil {
			return fmt.Errorf("read pull secret after create: %w", err)
		}
		log.Info("created per-app pull secret", "app", appName, "secret", pullSecName)
	case err != nil:
		return fmt.Errorf("get pull secret: %w", err)
	}

	// Extract plaintext password from the pull Secret so we can re-derive the bcrypt hash.
	if password == "" {
		password, err = extractPassword(pullSec.Data[corev1.DockerConfigJsonKey], c.Registry, zotUser)
		if err != nil {
			return fmt.Errorf("extract password from pull secret: %w", err)
		}
	}

	// 2. Ensure htpasswd entry. A genuinely new entry starts the activation
	// clock (verify.go): the running zot won't accept it until restarted.
	added, err := c.ensureHTPasswdEntry(ctx, zotUser, password)
	if err != nil {
		return fmt.Errorf("htpasswd: %w", err)
	}
	if added {
		c.recordWrite(appName)
	}

	// 3. Ensure Zot config per-repo ACL.
	if err := c.ensureZotConfigEntry(ctx, appName, zotUser); err != nil {
		return fmt.Errorf("zot config: %w", err)
	}

	return nil
}

// RevokeCreds removes the per-App credentials from the Zot htpasswd and config.
// The per-App pull Secret must be explicitly deleted by the caller (cross-namespace
// objects cannot carry owner references, so owner-ref GC does not apply here).
// The app controller's handleAppDeletion does this at app_controller.go:2306-2311.
//
// The running zot keeps accepting the removed credential until it restarts
// (the same no-reload behavior activation works around), so revocation ends
// with a best-effort rate-limited bounce; if the cooldown skips it, the
// credential stays live until the next bounce or zot restart.
func (c *Creds) RevokeCreds(ctx context.Context, appName string) error {
	c.clearActivation(appName)
	zotUser := ZotUsername(appName)
	if err := c.removeHTPasswdEntry(ctx, zotUser); err != nil {
		return fmt.Errorf("htpasswd revoke: %w", err)
	}
	if err := c.removeZotConfigEntry(ctx, appName); err != nil {
		return fmt.Errorf("zot config revoke: %w", err)
	}
	c.tryBounce(ctx)
	return nil
}

// RotateCreds re-issues the per-App pull credential with a newly generated
// password. It atomically:
//  1. Generates a fresh random password.
//  2. Updates the pull Secret in the App namespace.
//  3. Replaces the htpasswd entry (old hash → new hash). The ACL entry is
//     unchanged: the Zot username stays the same, only the password rotates.
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

	// 1. Generate new password.
	newPassword, err := generatePassword()
	if err != nil {
		return fmt.Errorf("generate new password: %w", err)
	}

	// 2. Update the pull Secret with the new password.
	var pullSec corev1.Secret
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: appNS, Name: pullSecName}, &pullSec); err != nil {
		return fmt.Errorf("get pull secret for rotation: %w", err)
	}
	patch := pullSec.DeepCopy()
	patch.Data[corev1.DockerConfigJsonKey] = buildDockerConfig(zotUser, newPassword, c.Registry, c.KpackRegistry)
	if err := c.Client.Patch(ctx, patch, client.MergeFrom(&pullSec)); err != nil {
		return fmt.Errorf("patch pull secret: %w", err)
	}
	log.Info("rotated per-app pull secret", "app", appName, "secret", pullSecName)

	// 3. Replace the htpasswd entry (forces bcrypt rehash with new password).
	if err := c.replaceHTPasswdEntry(ctx, zotUser, newPassword); err != nil {
		return fmt.Errorf("htpasswd rotation: %w", err)
	}
	// The running zot deterministically rejects the new password until it
	// restarts: restart the activation clock and drop the stale acceptance.
	c.recordWrite(appName)
	return nil
}

// -- htpasswd helpers ---------------------------------------------------------

// ensureHTPasswdEntry adds or refreshes the user entry in the zot-htpasswd
// Secret, reporting whether it actually added one. Uses optimistic locking
// (ResourceVersion) to handle concurrent reconciles; retries up to 3 times on
// conflict before returning ErrConflictRequeue.
func (c *Creds) ensureHTPasswdEntry(ctx context.Context, username, password string) (added bool, err error) {
	for range 3 {
		var sec corev1.Secret
		if err := c.Client.Get(ctx, client.ObjectKey{
			Namespace: c.ZotNamespace, Name: c.HTPasswdName,
		}, &sec); err != nil {
			return false, err
		}

		existing := sec.Data["htpasswd"]
		if htpasswdHasUser(existing, username) {
			// Verify the existing hash still matches the current password. If the
			// pull Secret was deleted and recreated, a new password was generated
			// but the htpasswd still has the old hash — detect and re-sync it.
			if bcrypt.CompareHashAndPassword(htpasswdUserHash(existing, username), []byte(password)) == nil {
				return false, nil
			}
			// Hash mismatch: fall through to replace the entry.
		}

		updated, err := addHTPasswdLine(existing, username, password)
		if err != nil {
			return false, err
		}
		patch := sec.DeepCopy()
		if patch.Data == nil {
			patch.Data = map[string][]byte{}
		}
		patch.Data["htpasswd"] = updated
		if err := c.Client.Patch(ctx, patch, client.MergeFromWithOptions(&sec, client.MergeFromWithOptimisticLock{})); err != nil {
			if apierrors.IsConflict(err) {
				continue
			}
			return false, err
		}
		return true, nil
	}
	return false, ErrConflictRequeue
}

// replaceHTPasswdEntry always replaces the user entry in the zot-htpasswd
// Secret with a new bcrypt hash (used by RotateCreds — no idempotency check).
// Retries up to 3 times on conflict before returning ErrConflictRequeue.
func (c *Creds) replaceHTPasswdEntry(ctx context.Context, username, password string) error {
	for range 3 {
		var sec corev1.Secret
		if err := c.Client.Get(ctx, client.ObjectKey{
			Namespace: c.ZotNamespace, Name: c.HTPasswdName,
		}, &sec); err != nil {
			return err
		}

		updated, err := addHTPasswdLine(sec.Data["htpasswd"], username, password)
		if err != nil {
			return err
		}
		patch := sec.DeepCopy()
		if patch.Data == nil {
			patch.Data = map[string][]byte{}
		}
		patch.Data["htpasswd"] = updated
		if err := c.Client.Patch(ctx, patch, client.MergeFromWithOptions(&sec, client.MergeFromWithOptimisticLock{})); err != nil {
			if apierrors.IsConflict(err) {
				continue
			}
			return err
		}
		return nil
	}
	return ErrConflictRequeue
}

// removeHTPasswdEntry removes the user entry from the zot-htpasswd Secret.
func (c *Creds) removeHTPasswdEntry(ctx context.Context, username string) error {
	for range 3 {
		var sec corev1.Secret
		if err := c.Client.Get(ctx, client.ObjectKey{
			Namespace: c.ZotNamespace, Name: c.HTPasswdName,
		}, &sec); apierrors.IsNotFound(err) {
			return nil
		} else if err != nil {
			return err
		}

		existing := sec.Data["htpasswd"]
		if !htpasswdHasUser(existing, username) {
			return nil
		}

		patch := sec.DeepCopy()
		patch.Data["htpasswd"] = removeHTPasswdLine(existing, username)
		if err := c.Client.Patch(ctx, patch, client.MergeFromWithOptions(&sec, client.MergeFromWithOptimisticLock{})); err != nil {
			if apierrors.IsConflict(err) {
				continue
			}
			return err
		}
		return nil
	}
	return ErrConflictRequeue
}

// -- Zot config helpers -------------------------------------------------------

// ensureZotConfigEntry adds a per-repo ACL entry for the App to the zot-config
// Secret and ensures the canonical storage + platform builder policies are
// present. The storage migration lets operational settings such as GC cadence
// reach existing registries without replacing their per-App ACLs. The admin
// policy migration is required for configs created before per-App ACLs: Zot
// applies only the longest repository match, so an exact per-App rule shadows
// the builder's ** rule. Creates the Secret with the base config if it does not
// exist.
func (c *Creds) ensureZotConfigEntry(ctx context.Context, appName, zotUser string) error {
	for range 3 {
		var sec corev1.Secret
		err := c.Client.Get(ctx, client.ObjectKey{
			Namespace: c.ZotNamespace, Name: c.ConfigName,
		}, &sec)
		if apierrors.IsNotFound(err) {
			// Bootstrap: create with base config + this entry.
			cfg, err := addZotACLEntry(c.baseZotConfig(), appName, zotUser)
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
		updated, storageChanged, err := ensureZotStorageConfig(existing, c.baseZotConfig())
		if err != nil {
			return err
		}
		repoReady := zotConfigHasRepoWritePolicy(existing, appName, zotUser)
		builderReady := zotConfigHasBuilderAdminPolicy(existing)
		platformBuilderReady := zotConfigHasPlatformBuilderReadPolicy(existing)
		if !storageChanged && repoReady && builderReady && platformBuilderReady {
			return nil
		}

		if !builderReady {
			updated, err = ensureZotBuilderAdminPolicy(updated)
			if err != nil {
				return err
			}
		}
		if !platformBuilderReady {
			updated, err = ensureZotPlatformBuilderReadPolicy(updated)
			if err != nil {
				return err
			}
		}
		if !repoReady {
			updated, err = addZotACLEntry(updated, appName, zotUser)
			if err != nil {
				return err
			}
		}
		patch := sec.DeepCopy()
		patch.Data["config.json"] = updated
		if err := c.Client.Patch(ctx, patch, client.MergeFromWithOptions(&sec, client.MergeFromWithOptimisticLock{})); err != nil {
			if apierrors.IsConflict(err) {
				continue
			}
			return err
		}
		return nil
	}
	return ErrConflictRequeue
}

// removeZotConfigEntry removes the per-App ACL entry from the zot-config Secret.
func (c *Creds) removeZotConfigEntry(ctx context.Context, appName string) error {
	for range 3 {
		var sec corev1.Secret
		if err := c.Client.Get(ctx, client.ObjectKey{
			Namespace: c.ZotNamespace, Name: c.ConfigName,
		}, &sec); apierrors.IsNotFound(err) {
			return nil
		} else if err != nil {
			return err
		}

		existing := sec.Data["config.json"]
		if !zotConfigHasRepo(existing, appName) {
			return nil
		}

		updated, err := removeZotACLEntry(existing, appName)
		if err != nil {
			return err
		}
		patch := sec.DeepCopy()
		patch.Data["config.json"] = updated
		if err := c.Client.Patch(ctx, patch, client.MergeFromWithOptions(&sec, client.MergeFromWithOptimisticLock{})); err != nil {
			if apierrors.IsConflict(err) {
				continue
			}
			return err
		}
		return nil
	}
	return ErrConflictRequeue
}

// -- low-level helpers --------------------------------------------------------

func generatePassword() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// buildDockerConfig returns a docker-config JSON for the given user/password.
// If kpackRegistry differs from registry, both are included.
func buildDockerConfig(username, password, registry, kpackRegistry string) []byte {
	type authEntry struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Auth     string `json:"auth"`
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	auths := map[string]authEntry{
		registry: {Username: username, Password: password, Auth: encoded},
	}
	if kpackRegistry != "" && kpackRegistry != registry {
		auths[kpackRegistry] = authEntry{Username: username, Password: password, Auth: encoded}
	}
	data, _ := json.Marshal(map[string]any{"auths": auths})
	return data
}

// extractPassword reads the plaintext password for username from a docker-config JSON.
func extractPassword(dockerConfigJSON []byte, registry, username string) (string, error) {
	var cfg struct {
		Auths map[string]struct {
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(dockerConfigJSON, &cfg); err != nil {
		return "", err
	}
	a, ok := cfg.Auths[registry]
	if !ok {
		return "", fmt.Errorf("no auth entry for %q", registry)
	}
	if a.Username != username {
		return "", fmt.Errorf("auth entry for %q has user %q, want %q", registry, a.Username, username)
	}
	return a.Password, nil
}

// htpasswdHasUser reports whether username appears in an htpasswd byte slice.
func htpasswdHasUser(htpasswd []byte, username string) bool {
	prefix := username + ":"
	for line := range strings.SplitSeq(string(htpasswd), "\n") {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

// htpasswdUserHash returns the bcrypt hash stored for username, or nil if not found.
func htpasswdUserHash(htpasswd []byte, username string) []byte {
	prefix := username + ":"
	for line := range strings.SplitSeq(string(htpasswd), "\n") {
		if hash, ok := strings.CutPrefix(line, prefix); ok {
			return []byte(hash)
		}
	}
	return nil
}

// addHTPasswdLine appends "username:bcrypt(password)" to htpasswd content.
// If the username already exists the entry is replaced.
func addHTPasswdLine(htpasswd []byte, username, password string) ([]byte, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	newLine := username + ":" + string(hash)
	prefix := username + ":"

	var lines []string
	for l := range strings.SplitSeq(strings.TrimRight(string(htpasswd), "\n"), "\n") {
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, prefix) {
			continue // replaced below
		}
		lines = append(lines, l)
	}
	lines = append(lines, newLine)
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

// removeHTPasswdLine removes the entry for username from htpasswd content.
func removeHTPasswdLine(htpasswd []byte, username string) []byte {
	prefix := username + ":"
	var lines []string
	for l := range strings.SplitSeq(strings.TrimRight(string(htpasswd), "\n"), "\n") {
		if l == "" || strings.HasPrefix(l, prefix) {
			continue
		}
		lines = append(lines, l)
	}
	if len(lines) == 0 {
		return nil
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

// zotConfigHasRepo reports whether the Zot config JSON already has an ACL entry
// for the named repository.
func zotConfigHasRepo(configJSON []byte, repo string) bool {
	repos := zotRepos(configJSON)
	_, ok := repos[repo]
	return ok
}

func zotConfigHasRepoWritePolicy(configJSON []byte, repo, user string) bool {
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
	if len(users) != 1 || users[0] != user {
		return false
	}
	for _, action := range []string{zotActionRead, "create", "update", "delete"} {
		if !containsString(actions, action) {
			return false
		}
	}
	return true
}

// zotConfigHasBuilderAdminPolicy reports whether bex-builder has the global
// actions required by buildkitd and cosign. A ** repository policy is not
// sufficient because Zot gives a longer exact per-repository rule precedence.
func zotConfigHasBuilderAdminPolicy(configJSON []byte) bool {
	var data map[string]any
	if err := json.Unmarshal(configJSON, &data); err != nil {
		return false
	}
	httpBlock, _ := data["http"].(map[string]any)
	accessControl, _ := httpBlock["accessControl"].(map[string]any)
	adminPolicy, _ := accessControl["adminPolicy"].(map[string]any)
	users, _ := adminPolicy["users"].([]any)
	actions, _ := adminPolicy["actions"].([]any)

	if !containsString(users, "bex-builder") {
		return false
	}
	for _, action := range []string{zotActionRead, "create", "update", "delete"} {
		if !containsString(actions, action) {
			return false
		}
	}
	return true
}

// zotConfigHasPlatformBuilderReadPolicy reports whether authenticated users
// can pull the shared kpack builder without gaining any write action. App
// builds authenticate as app-<name>; their repository-scoped credential must
// therefore cover this one platform input as well as their own output repo.
func zotConfigHasPlatformBuilderReadPolicy(configJSON []byte) bool {
	raw, ok := zotRepos(configJSON)[platformBuilderRepository].(map[string]any)
	if !ok {
		return false
	}
	actions, _ := raw["defaultPolicy"].([]any)
	return len(actions) == 1 && actions[0] == zotActionRead
}

// ensureZotBuilderAdminPolicy migrates an existing Zot config to the global
// builder policy. zot-config is operator-managed, so this policy is canonical.
func ensureZotBuilderAdminPolicy(configJSON []byte) ([]byte, error) {
	if zotConfigHasBuilderAdminPolicy(configJSON) {
		return configJSON, nil
	}

	var data map[string]any
	if err := json.Unmarshal(configJSON, &data); err != nil {
		return nil, err
	}
	httpBlock, _ := data["http"].(map[string]any)
	if httpBlock == nil {
		httpBlock = map[string]any{}
		data["http"] = httpBlock
	}
	accessControl, _ := httpBlock["accessControl"].(map[string]any)
	if accessControl == nil {
		accessControl = map[string]any{}
		httpBlock["accessControl"] = accessControl
	}
	accessControl["adminPolicy"] = map[string]any{
		"users":   []any{"bex-builder"},
		"actions": []any{zotActionRead, "create", "update", "delete"},
	}
	return json.Marshal(data)
}

// ensureZotPlatformBuilderReadPolicy installs the exact, read-only policy for
// the shared kpack builder repository. The global bex-builder adminPolicy still
// overrides this exact match for platform publishing; no tenant identity can
// create, update, or delete the builder image.
func ensureZotPlatformBuilderReadPolicy(configJSON []byte) ([]byte, error) {
	if zotConfigHasPlatformBuilderReadPolicy(configJSON) {
		return configJSON, nil
	}
	var data map[string]any
	if err := json.Unmarshal(configJSON, &data); err != nil {
		return nil, err
	}
	repos := zotReposMap(data)
	repos[platformBuilderRepository] = map[string]any{
		"defaultPolicy": []any{zotActionRead},
	}
	return json.Marshal(data)
}

// ensureZotStorageConfig replaces only the operator-owned storage block with
// the current canonical policy. Existing auth, per-App ACLs, and extensions are
// preserved. The changed result prevents no-op Secret writes and lets the
// caller distinguish a migration from an already-current config.
func ensureZotStorageConfig(configJSON, canonicalJSON []byte) ([]byte, bool, error) {
	var data, canonical map[string]any
	if err := json.Unmarshal(configJSON, &data); err != nil {
		return nil, false, err
	}
	if err := json.Unmarshal(canonicalJSON, &canonical); err != nil {
		return nil, false, err
	}
	desired, ok := canonical["storage"].(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf("canonical zot config has no storage block")
	}
	if current, _ := data["storage"].(map[string]any); reflect.DeepEqual(current, desired) {
		return configJSON, false, nil
	}
	data["storage"] = desired
	updated, err := json.Marshal(data)
	if err != nil {
		return nil, false, err
	}
	return updated, true, nil
}

func containsString(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// addZotACLEntry adds a per-repo policy for zotUser in the Zot config JSON.
func addZotACLEntry(configJSON []byte, repo, zotUser string) ([]byte, error) {
	var data map[string]any
	if err := json.Unmarshal(configJSON, &data); err != nil {
		return nil, err
	}
	repos := zotReposMap(data)
	if repo == platformBuilderRepository {
		// This repository is platform-owned. Never let an App with a colliding
		// public name replace its read-only rule with tenant write permission.
		return ensureZotPlatformBuilderReadPolicy(configJSON)
	}
	repos[repo] = map[string]any{
		"policies": []any{
			map[string]any{
				"users":   []any{zotUser},
				"actions": []any{zotActionRead, "create", "update", "delete"},
			},
		},
	}
	return json.Marshal(data)
}

// removeZotACLEntry removes the per-repo ACL entry for repo from the Zot config JSON.
func removeZotACLEntry(configJSON []byte, repo string) ([]byte, error) {
	var data map[string]any
	if err := json.Unmarshal(configJSON, &data); err != nil {
		return nil, err
	}
	repos := zotReposMap(data)
	if repo == platformBuilderRepository {
		return ensureZotPlatformBuilderReadPolicy(configJSON)
	}
	delete(repos, repo)
	return json.Marshal(data)
}

// zotRepos parses and returns the accessControl.repositories map (read-only copy).
func zotRepos(configJSON []byte) map[string]any {
	var data map[string]any
	_ = json.Unmarshal(configJSON, &data)
	return zotReposMap(data)
}

// zotReposMap returns a reference to data["http"]["accessControl"]["repositories"],
// creating intermediate maps as needed. Mutations to the returned map mutate data.
func zotReposMap(data map[string]any) map[string]any {
	if data == nil {
		return map[string]any{}
	}
	httpBlock, _ := data["http"].(map[string]any)
	if httpBlock == nil {
		httpBlock = map[string]any{}
		data["http"] = httpBlock
	}
	ac, _ := httpBlock["accessControl"].(map[string]any)
	if ac == nil {
		ac = map[string]any{}
		httpBlock["accessControl"] = ac
	}
	repos, _ := ac["repositories"].(map[string]any)
	if repos == nil {
		repos = map[string]any{}
		ac["repositories"] = repos
	}
	return repos
}

// -- base config --------------------------------------------------------------

// baseZotConfig returns the operator-canonical Zot config JSON. It replaces the
// Helm-embedded configFiles stanza (removed in w7/m36). The bex-puller shared
// user is NOT present: per-App users ("app-<name>") are added dynamically.
// The retention count defaults to 5 if c.RetentionCount is 0; override via
// BEX_ZOT_RETENTION_COUNT.
func (c *Creds) baseZotConfig() []byte {
	retentionCount := c.RetentionCount
	if retentionCount == 0 {
		retentionCount = 5
	}
	type keepTag struct {
		Patterns                []string `json:"patterns"`
		MostRecentlyPushedCount int      `json:"mostRecentlyPushedCount"`
	}
	type retentionPolicy struct {
		Repositories   []string  `json:"repositories"`
		DeleteUntagged bool      `json:"deleteUntagged"`
		KeepTags       []keepTag `json:"keepTags"`
	}
	type retention struct {
		DryRun   bool              `json:"dryRun"`
		Policies []retentionPolicy `json:"policies"`
	}
	type storage struct {
		RootDirectory string    `json:"rootDirectory"`
		Dedupe        bool      `json:"dedupe"`
		GC            bool      `json:"gc"`
		GCDelay       string    `json:"gcDelay"`
		GCInterval    string    `json:"gcInterval"`
		Retention     retention `json:"retention"`
	}
	type htpasswdAuth struct {
		Path string `json:"path"`
	}
	type auth struct {
		HTPasswd htpasswdAuth `json:"htpasswd"`
	}
	type policy struct {
		Users         []string `json:"users"`
		Actions       []string `json:"actions"`
		DefaultPolicy []string `json:"defaultPolicy,omitempty"`
	}
	type repoACL struct {
		Policies      []policy `json:"policies"`
		DefaultPolicy []string `json:"defaultPolicy,omitempty"`
	}
	type accessControl struct {
		Repositories map[string]repoACL `json:"repositories"`
		AdminPolicy  policy             `json:"adminPolicy"`
	}
	type httpBlock struct {
		Address       string        `json:"address"`
		Port          string        `json:"port"`
		Compat        []string      `json:"compat"`
		ReadTimeout   string        `json:"readTimeout"`
		WriteTimeout  string        `json:"writeTimeout"`
		Auth          auth          `json:"auth"`
		AccessControl accessControl `json:"accessControl"`
	}
	type logBlock struct {
		Level string `json:"level"`
	}
	type config struct {
		DistSpecVersion string    `json:"distSpecVersion"`
		Storage         storage   `json:"storage"`
		HTTP            httpBlock `json:"http"`
		Log             logBlock  `json:"log"`
	}

	cfg := config{
		DistSpecVersion: "1.1.0",
		Storage: storage{
			RootDirectory: "/var/lib/registry",
			Dedupe:        true,
			GC:            true,
			GCDelay:       "1h",
			GCInterval:    "1h",
			Retention: retention{
				DryRun: false,
				Policies: []retentionPolicy{
					{
						Repositories:   []string{"**"},
						DeleteUntagged: true,
						KeepTags: []keepTag{
							{
								Patterns:                []string{".*"},
								MostRecentlyPushedCount: retentionCount,
							},
						},
					},
				},
			},
		},
		HTTP: httpBlock{
			Address:      "0.0.0.0",
			Port:         zotHTTPPort,
			Compat:       []string{"docker2s2"},
			ReadTimeout:  "60s",
			WriteTimeout: "60s",
			Auth: auth{
				HTPasswd: htpasswdAuth{Path: zotHTPasswdPath},
			},
			AccessControl: accessControl{
				Repositories: map[string]repoACL{
					"**": {
						Policies: []policy{
							{
								Users:   []string{"bex-builder"},
								Actions: []string{zotActionRead, "create", "update", "delete"},
							},
						},
						DefaultPolicy: []string{},
					},
					platformBuilderRepository: {
						DefaultPolicy: []string{zotActionRead},
					},
				},
				AdminPolicy: policy{
					Users:   []string{"bex-builder"},
					Actions: []string{zotActionRead, "create", "update", "delete"},
				},
			},
		},
		Log: logBlock{Level: "info"},
	}

	data, _ := json.Marshal(cfg)
	return data
}
