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

package registry

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	boundedhttp "github.com/bex-co/bex/lego/operator/internal/httpclient"
)

// Credential activation (w9/m43). Zot loads /secret/htpasswd and the
// accessControl config once at process start — it has no working hot reload
// under Kubernetes Secret mounts (verified empirically against v2.1.18: a
// kubelet-style atomic symlink swap of both mounts activates nothing, and
// SIGHUP shuts the server down rather than reloading; full record in
// docs/ADR022-tenant-isolation.md § Credential activation). So a freshly
// written per-App credential is not accepted by the running registry until its
// pod restarts, and kubelet pulls fail 401 forever. EnsureActive closes that
// gap: probe zot with the App's own credential against its own repository, and
// when it stays rejected past the write grace, bounce the zot pod
// (rate-limited) so the restart loads the new htpasswd user + ACL entry.
const (
	defaultActivationGrace = 30 * time.Second
	defaultBounceCooldown  = 2 * time.Minute
)

// ZotPodLabelKey/Value select the registry pods to bounce — the upstream Zot
// Helm chart's stable pod label (verified on the deployed chart 0.1.121).
// Exported so tests outside this package build fixtures from the same contract.
const (
	ZotPodLabelKey   = "app.kubernetes.io/name"
	ZotPodLabelValue = "zot"
)

// defaultHTTPClient is shared by every probe when Creds.HTTPClient is nil.
var defaultHTTPClient = boundedhttp.Shared

// EnsureActive reports whether the running Zot accepts the App's per-App pull
// credential. It returns (false, nil) while activation is pending — the caller
// should requeue and try again — and a non-nil error when the probe could not
// be performed (missing pull Secret, zot unreachable) or a needed bounce
// failed; both are retry-later conditions the caller should surface.
//
// Once a credential probes as accepted it is memoized and this returns true
// with no I/O until RotateCreds or RevokeCreds invalidates it: a zot restart
// re-reads the current Secrets, which still contain the entry, so acceptance
// only changes when the operator itself rewrites the credential.
//
// When the credential stays rejected past ActivationGrace — measured from the
// htpasswd write when this process performed it, else from the first rejected
// probe — the zot pod is deleted so the StatefulSet restarts it and it
// re-reads htpasswd + config. Bounces are rate-limited by BounceCooldown
// across all Apps.
func (c *Creds) EnsureActive(ctx context.Context, appName, appNS string) (bool, error) {
	log := logf.FromContext(ctx)

	c.mu.Lock()
	activated := c.activated[appName]
	c.mu.Unlock()
	if activated {
		return true, nil
	}

	password, err := c.readPassword(ctx, appName, appNS)
	if err != nil {
		return false, fmt.Errorf("read credential for probe: %w", err)
	}

	accepted, err := c.probe(ctx, appName, ZotUsername(appName), password)
	if err != nil {
		return false, fmt.Errorf("probe registry: %w", err)
	}
	if accepted {
		c.mu.Lock()
		if c.activated == nil {
			c.activated = map[string]bool{}
		}
		c.activated[appName] = true
		delete(c.firstRejected, appName)
		delete(c.writtenAt, appName)
		c.mu.Unlock()
		return true, nil
	}

	rejectedFor, bounce := c.recordRejection(appName, time.Now())
	if bounce {
		if err := c.bounceZot(ctx); err != nil {
			// Surface the failure: without a working bounce the App would sit
			// at RegistryCredsPending forever with a message blaming the
			// credential rather than the broken bounce.
			return false, fmt.Errorf("bounce zot for credential activation: %w", err)
		}
		log.Info("bounced zot to activate stale per-app registry credential",
			"app", appName, "user", ZotUsername(appName), "rejectedFor", rejectedFor.String())
	}
	return false, nil
}

// readPassword extracts the App's plaintext registry password from its pull
// Secret (shared by EnsureCreds and the activation probe).
func (c *Creds) readPassword(ctx context.Context, appName, appNS string) (string, error) {
	var pullSec corev1.Secret
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: appNS, Name: PullSecretName(appName)}, &pullSec); err != nil {
		return "", fmt.Errorf("get pull secret: %w", err)
	}
	return extractPassword(pullSec.Data[corev1.DockerConfigJsonKey], c.Registry, ZotUsername(appName))
}

// recordRejection tracks a rejected probe and decides whether this caller
// should bounce zot: past the activation grace (anchored to the credential
// write when known, else the first rejected probe) and outside the global
// bounce cooldown. All mu-guarded rejection state lives here.
func (c *Creds) recordRejection(appName string, now time.Time) (rejectedFor time.Duration, bounce bool) {
	grace := c.ActivationGrace
	if grace == 0 {
		grace = defaultActivationGrace
	}
	cooldown := c.BounceCooldown
	if cooldown == 0 {
		cooldown = defaultBounceCooldown
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	anchor, ok := c.writtenAt[appName]
	if !ok {
		anchor, ok = c.firstRejected[appName]
		if !ok {
			anchor = now
			if c.firstRejected == nil {
				c.firstRejected = map[string]time.Time{}
			}
			c.firstRejected[appName] = now
		}
	}
	rejectedFor = now.Sub(anchor)
	bounce = rejectedFor >= grace && now.Sub(c.lastBounce) >= cooldown
	if bounce {
		c.lastBounce = now
	}
	return rejectedFor, bounce
}

// recordWrite stamps the moment this process (re)wrote the App's htpasswd
// entry and drops any stale acceptance: the running zot cannot know the new
// credential until it restarts, so activation must be re-proven. The stamp
// anchors the activation grace to the write instead of the first probe — for a
// repo-backed App the build runs for minutes in between, so by probe time the
// Secret has long propagated and a rejected credential can be bounced without
// dead waiting.
func (c *Creds) recordWrite(appName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.writtenAt == nil {
		c.writtenAt = map[string]time.Time{}
	}
	c.writtenAt[appName] = time.Now()
	delete(c.activated, appName)
	delete(c.firstRejected, appName)
}

// clearActivation forgets all per-App activation state (App deleted).
func (c *Creds) clearActivation(appName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.activated, appName)
	delete(c.firstRejected, appName)
	delete(c.writtenAt, appName)
}

// probe asks the running zot whether it accepts the credential for the App's
// own repository: GET /v2/<repo>/tags/list exercises both the htpasswd user
// and (unlike a bare /v2/ ping) the per-repo ACL entry, which live in two
// independently propagated Secrets. 2xx and 404 (authenticated + authorized,
// repo just not pushed yet — zot may also answer 404 for a denied repo to
// avoid existence leaks, so 404 is not a perfect ACL proof but 401/403 are
// definitive rejections) count as accepted.
func (c *Creds) probe(ctx context.Context, repo, username, password string) (bool, error) {
	requestCtx, cancel := boundedhttp.WithTimeout(ctx)
	defer cancel()
	ctx = requestCtx
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = defaultHTTPClient
	}
	base := c.Registry
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v2/"+repo+"/tags/list", nil)
	if err != nil {
		return false, err
	}
	req.SetBasicAuth(username, password)
	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300, resp.StatusCode == http.StatusNotFound:
		return true, nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected registry probe status %d", resp.StatusCode)
	}
}

// tryBounce performs a rate-limited, best-effort bounce for callers with no
// requeue loop to retry in (RevokeCreds). Failures and cooldown skips are
// log-only: the removed entry then stays live until the next bounce or zot
// restart.
func (c *Creds) tryBounce(ctx context.Context) {
	cooldown := c.BounceCooldown
	if cooldown == 0 {
		cooldown = defaultBounceCooldown
	}
	now := time.Now()
	c.mu.Lock()
	allowed := now.Sub(c.lastBounce) >= cooldown
	if allowed {
		c.lastBounce = now
	}
	c.mu.Unlock()
	if !allowed {
		return
	}
	if err := c.bounceZot(ctx); err != nil {
		logf.FromContext(ctx).Error(err, "best-effort zot bounce after credential revocation")
	}
}

// bounceZot deletes the registry pods (StatefulSet-managed, so they restart
// and re-read htpasswd + config). Callers rate-limit via BounceCooldown.
// Accepted blast radius: a restart interrupts concurrent pulls (kubelet
// retries) and can fail a build push in flight; the cooldown bounds the churn.
// Deliberately list+delete rather than DeleteAllOf so the RBAC grant stays
// "delete", never "deletecollection" (deploy/gitops/base/operator-registry-rbac.yaml).
func (c *Creds) bounceZot(ctx context.Context) error {
	var pods corev1.PodList
	if err := c.Client.List(ctx, &pods,
		client.InNamespace(c.ZotNamespace),
		client.MatchingLabels{ZotPodLabelKey: ZotPodLabelValue}); err != nil {
		return fmt.Errorf("list zot pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no zot pods matching %s=%s in %s", ZotPodLabelKey, ZotPodLabelValue, c.ZotNamespace)
	}
	for i := range pods.Items {
		if err := c.Client.Delete(ctx, &pods.Items[i]); err != nil {
			return fmt.Errorf("delete zot pod %s: %w", pods.Items[i].Name, err)
		}
	}
	return nil
}
