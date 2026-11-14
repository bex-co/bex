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

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	boundedhttp "github.com/bex-co/bex/lego/operator/internal/httpclient"
	"github.com/bex-co/bex/lego/operator/internal/identity"
)

// Credential activation (w9/m43, reworked by w4/m347). The pinned zot
// (v2.1.22) hot-reloads both /secret/htpasswd and the accessControl block of
// /etc/zot/config.json: each file is fingerprint-polled every second, which
// catches the kubelet's atomic ..data symlink swap that v2.1.18 missed. So a
// credential write activates without a restart once the kubelet has projected
// the new Secret contents into the zot pod. The kubelet does that on its
// periodic pod sync (about a minute) or at once when the pod object changes,
// so every credential write ends with a reload nudge: a fresh annotation on the
// zot pod (nudgeZot). The registry keeps serving throughout.
//
// EnsureActive still proves activation with the App's own credential before a
// build pushes or a workload pulls. While it is rejected it re-nudges (at most
// every nudgeInterval). Restarting zot is a last resort: only a credential
// still rejected after ActivationGrace (far past the kubelet's own sync period,
// so hot reload itself is broken) bounces the pod, logs an error and counts
// bex_registry_activation_restarts_total, which
// RegistryCredentialActivationRestarted alerts on. A restart fails every
// tenant's in-flight push (w4/m347 round 190), so it must stay exceptional.
// Full record: docs/ADR022-tenant-isolation.md § Credential activation.
const (
	defaultActivationGrace = 3 * time.Minute
	defaultBounceCooldown  = 2 * time.Minute
	// nudgeInterval bounds re-nudges while a credential stays rejected: one
	// covers a nudge the kubelet processed before its Secret cache saw the write.
	nudgeInterval = 5 * time.Second
)

// ZotPodLabelKey/Value select the registry pods to nudge or bounce: the
// upstream Zot Helm chart's stable pod label (verified on the deployed chart
// 0.1.121). Exported so tests outside this package build fixtures from the
// same contract.
const (
	ZotPodLabelKey   = "app.kubernetes.io/name"
	ZotPodLabelValue = "zot"
)

// ZotReloadAnnotation is the pod annotation nudgeZot stamps with the write
// time. Its only purpose is to change the pod object, which makes the kubelet
// re-project the pod's Secret volumes now instead of at its next periodic sync.
const ZotReloadAnnotation = "bex.co/registry-credentials-written"

// zotPodName is the chart's single StatefulSet replica, the only pod the
// operator's RBAC lets it patch (resourceNames in operator-registry-rbac.yaml).
// Patching it by name saves the list a label lookup would cost per write.
const zotPodName = "zot-0"

// activationRestartsTotal counts last-resort zot restarts (EnsureActive found a
// credential still rejected past ActivationGrace). Any increase means hot
// reload is not working; RegistryCredentialActivationRestarted alerts on it.
var activationRestartsTotal = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "bex_registry_activation_restarts_total",
	Help: "Zot restarts the operator made because a per-App registry credential stayed rejected past the activation grace despite hot-reload nudges.",
})

func init() {
	ctrlmetrics.Registry.MustRegister(activationRestartsTotal)
}

// defaultHTTPClient is shared by every probe when Creds.HTTPClient is nil.
var defaultHTTPClient = boundedhttp.Shared

// activation is what this process knows about one App's credential. anchor is
// the grace clock: the moment this process wrote the credential when it did,
// else the first rejected probe. Zero once accepted.
type activation struct {
	accepted bool
	anchor   time.Time
}

// track applies mutate to appName's activation state under the lock.
func (c *Creds) track(appName string, mutate func(*activation)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.apps == nil {
		c.apps = map[string]activation{}
	}
	state := c.apps[appName]
	mutate(&state)
	c.apps[appName] = state
}

// EnsureActive reports whether the running Zot accepts the App's per-App pull
// credential. It returns (false, nil) while activation is pending — the caller
// should requeue and try again — and a non-nil error when the probe could not
// be performed (missing pull Secret, zot unreachable) or a needed bounce
// failed; both are retry-later conditions the caller should surface.
//
// Once a credential probes as accepted it is memoized and this returns true
// with no I/O until RotateCreds or RevokeCreds invalidates it: acceptance only
// changes when the operator itself rewrites the credential.
//
// While the credential is rejected the zot pod is nudged to reload (at most
// every nudgeInterval). Only past ActivationGrace (measured from the htpasswd
// write when this process performed it, else from the first rejected probe)
// is the zot pod deleted as a last resort, rate-limited by BounceCooldown
// across all Apps.
func (c *Creds) EnsureActive(ctx context.Context, appName, appNS string) (bool, error) {
	return c.EnsureActiveFor(ctx, identity.ForApp(appName, ""), appNS)
}

// EnsureActiveFor is EnsureActive for a (possibly workspace-scoped) identity.
func (c *Creds) EnsureActiveFor(ctx context.Context, id identity.Identity, appNS string) (bool, error) {
	log := logf.FromContext(ctx)
	key := id.Key()

	c.mu.Lock()
	accepted := c.apps[key].accepted
	c.mu.Unlock()
	if accepted {
		return true, nil
	}

	password, err := c.readPasswordFor(ctx, id, appNS)
	if err != nil {
		return false, fmt.Errorf("read credential for probe: %w", err)
	}

	ok, err := c.probeFor(ctx, id, password)
	if err != nil {
		return false, fmt.Errorf("probe registry: %w", err)
	}
	if ok {
		c.track(key, func(a *activation) { *a = activation{accepted: true} })
		return true, nil
	}

	now := time.Now()
	rejectedFor, bounce := c.recordRejection(key, now)
	if bounce {
		if err := c.bounceZot(ctx); err != nil {
			return false, fmt.Errorf("bounce zot for credential activation: %w", err)
		}
		activationRestartsTotal.Inc()
		log.Error(nil, "registry hot reload did not activate a per-app credential; restarted zot as a last resort",
			"app", id.Name, "user", id.ZotUsername(), "rejectedFor", rejectedFor.String())
		return false, nil
	}
	if c.claimNudge(now) {
		c.nudgeZot(ctx)
	}
	return false, nil
}

func (c *Creds) readPasswordFor(ctx context.Context, id identity.Identity, appNS string) (string, error) {
	var pullSec corev1.Secret
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: appNS, Name: id.PullSecretName()}, &pullSec); err != nil {
		return "", fmt.Errorf("get pull secret: %w", err)
	}
	return c.passwordFrom(pullSec.Data[corev1.DockerConfigJsonKey], id.ZotUsername())
}

// recordRejection tracks a rejected probe and decides whether this caller
// should bounce zot: past the activation grace and outside the global bounce
// cooldown.
func (c *Creds) recordRejection(appName string, now time.Time) (rejectedFor time.Duration, bounce bool) {
	grace := c.ActivationGrace
	if grace == 0 {
		grace = defaultActivationGrace
	}
	cooldown := c.BounceCooldown
	if cooldown == 0 {
		cooldown = defaultBounceCooldown
	}

	c.track(appName, func(a *activation) {
		if a.anchor.IsZero() {
			a.anchor = now
		}
		rejectedFor = now.Sub(a.anchor)
		bounce = rejectedFor >= grace && now.Sub(c.lastBounce) >= cooldown
		if bounce {
			c.lastBounce = now
		}
	})
	return rejectedFor, bounce
}

// recordWrite stamps the moment this process (re)wrote the App's credential,
// drops any stale acceptance (activation must be re-proven against the running
// zot), and nudges zot to reload so the first probe usually finds it active.
func (c *Creds) recordWrite(ctx context.Context, appName string) {
	c.track(appName, func(a *activation) { *a = activation{anchor: time.Now()} })
	c.nudgeZot(ctx)
}

// clearActivation forgets all per-App activation state (App deleted).
func (c *Creds) clearActivation(appName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.apps, appName)
}

// probeFor asks the running zot whether it accepts the App's credential for
// the App's own repository: GET /v2/<repo>/tags/list exercises both the
// htpasswd user and (unlike a bare /v2/ ping) the per-repo ACL entry, which
// live in two independently propagated Secrets. 2xx and 404 (authenticated +
// authorized, repo just not pushed yet — zot may also answer 404 for a denied
// repo to avoid existence leaks, so 404 is not a perfect ACL proof but 401/403
// are definitive rejections) count as accepted.
func (c *Creds) probeFor(ctx context.Context, id identity.Identity, password string) (bool, error) {
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v2/"+id.Repo()+"/tags/list", nil)
	if err != nil {
		return false, err
	}
	req.SetBasicAuth(id.ZotUsername(), password)
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

// nudgeZot makes the kubelet re-project the zot pod's Secret volumes now, so
// zot's own file poll picks up a credential write within seconds. It is best
// effort: when it fails (or the RBAC grant is missing) the kubelet's periodic
// sync still delivers the write, only later, and EnsureActive keeps probing.
func (c *Creds) nudgeZot(ctx context.Context) {
	now := time.Now()
	c.mu.Lock()
	c.lastNudge = now // the probe right after a write must not nudge again
	c.mu.Unlock()
	stamp := fmt.Appendf(nil, `{"metadata":{"annotations":{%q:%q}}}`,
		ZotReloadAnnotation, now.UTC().Format(time.RFC3339Nano))
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: zotPodName, Namespace: c.ZotNamespace}}
	if err := c.Client.Patch(ctx, pod, client.RawPatch(types.MergePatchType, stamp)); err != nil {
		logf.FromContext(ctx).Error(err, "nudging zot to reload registry credentials failed; they activate at the next kubelet sync")
	}
}

// claimNudge reports whether a rejected probe at now may re-nudge zot.
func (c *Creds) claimNudge(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if now.Sub(c.lastNudge) < nudgeInterval {
		return false
	}
	c.lastNudge = now
	return true
}

// claimBounce takes the one bounce the cooldown allows at now, reporting
// whether the caller may restart zot.
func (c *Creds) claimBounce(now time.Time) bool {
	cooldown := c.BounceCooldown
	if cooldown == 0 {
		cooldown = defaultBounceCooldown
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if now.Sub(c.lastBounce) < cooldown {
		return false
	}
	c.lastBounce = now
	return true
}

// zotPods lists the registry pods by the chart's pod label.
func (c *Creds) zotPods(ctx context.Context) ([]corev1.Pod, error) {
	var pods corev1.PodList
	if err := c.Client.List(ctx, &pods,
		client.InNamespace(c.ZotNamespace),
		client.MatchingLabels{ZotPodLabelKey: ZotPodLabelValue}); err != nil {
		return nil, fmt.Errorf("list zot pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no zot pods matching %s=%s in %s", ZotPodLabelKey, ZotPodLabelValue, c.ZotNamespace)
	}
	return pods.Items, nil
}

// bounceZot deletes the registry pods (StatefulSet-managed, so they restart
// and re-read htpasswd + config). Callers rate-limit via BounceCooldown. A
// restart fails every tenant's in-flight push and pull, so only the
// last-resort activation path and a storage-block change use it.
// Deliberately list+delete rather than DeleteAllOf so the RBAC grant stays
// "delete", never "deletecollection" (deploy/gitops/base/operator-registry-rbac.yaml).
func (c *Creds) bounceZot(ctx context.Context) error {
	pods, err := c.zotPods(ctx)
	if err != nil {
		return err
	}
	for i := range pods {
		if err := c.Client.Delete(ctx, &pods[i]); err != nil {
			return fmt.Errorf("delete zot pod %s: %w", pods[i].Name, err)
		}
	}
	return nil
}
