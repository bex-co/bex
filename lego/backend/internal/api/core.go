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

// Package api is the bex control-plane seed: the product front door that turns
// user intent (restart / suspend / resume) into App CR spec patches. It is a
// thin policy layer — the operator does all the mechanism.
//
// Structure: ONE domain layer (Core) with THREE adapters over it (REST, GraphQL
// and MCP). Every adapter calls the same Core methods, so the APIs can never
// drift in behavior — they share the single implementation, exactly the pattern
// Render uses (public REST + dashboard GraphQL over one internal service layer),
// extended with an MCP surface consistent with Render's official MCP server.
package api

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ErrNotFound is returned when an App does not exist (adapters map it to 404 /
// a GraphQL null+error).
var ErrNotFound = errors.New("app not found")

// ErrLogsUnavailable is returned by Logs when no pod-log source is wired (e.g.
// the binary was started without a clientset). Adapters surface it as a plain
// error rather than a 404.
var ErrLogsUnavailable = errors.New("logs source not configured")

// ErrMetricsUnavailable is returned by the Metrics verb when the backend a metric
// needs isn't wired: resource metrics (cpu/memory) need a ResourceMetrics source
// (metrics-server), request metrics need a RequestMetrics source (Traefik via
// Prometheus). Adapters surface it as 503, like ErrLogsUnavailable — the App
// exists, the data source doesn't. Instance count needs neither (it counts pods).
var ErrMetricsUnavailable = errors.New("metrics source not configured")

// ErrAPIKeysUnavailable is returned by the api-key verbs when no store is
// wired (e.g. the binary was started without a Hydra admin URL).
var ErrAPIKeysUnavailable = errors.New("api-key store not configured")

// ErrBadRequest is returned for invalid caller input (adapters map it to 400).
var ErrBadRequest = errors.New("bad request")

// ErrForbidden is returned when the authenticated caller lacks the permission
// a verb requires (adapters map it to 403; distinct from the auth gate's 401).
var ErrForbidden = errors.New("forbidden")

// ErrAuthzUnavailable is returned when a wired authorization checker cannot be
// consulted — requests fail closed (503), never pass through.
var ErrAuthzUnavailable = errors.New("authorization service unavailable")

// podLabelApp is the label the controller stamps on an App's pods
// (internal/controller labelApp). Kept in sync by hand: the api package must not
// import the controller. Log selection keys on it.
const podLabelApp = "app.bex.co/app"

// appContainer is the single container name the controller gives each App pod.
const appContainer = "app"

// defaultLogTail caps log lines per instance when the caller doesn't specify.
const defaultLogTail = 100

// AppView is the neutral, bex-native projection of an App — spec intent +
// observed status. Core returns this; each adapter maps it to its own wire
// format (the REST/GraphQL adapters render it in Render's Service shape). Keeping
// Core wire-agnostic is what lets one domain layer feed many API dialects.
type AppView struct {
	Name      string   `json:"name"`
	Phase     string   `json:"phase"`
	URL       string   `json:"url"`
	URLs      []string `json:"urls"`
	Image     string   `json:"image"`
	Replicas  int32    `json:"replicas"`
	Suspended bool     `json:"suspended"`
	Revision  string   `json:"revision"`
	CreatedAt string   `json:"createdAt"`
}

// PodLogSource fetches the raw (timestamped) log stream for one pod container.
// Core depends on this narrow function instead of a full client-go clientset so
// the domain layer stays an apiserver-thin client and is trivial to fake in
// tests; production wires it via NewPodLogSource. nil => the Logs verb reports
// ErrLogsUnavailable.
type PodLogSource func(ctx context.Context, namespace, pod, container string, tail int64) (io.ReadCloser, error)

// LogEntry is one log line in Render's log shape: a timestamp, the message, and
// a label set. Render tags every entry with labels (instance/type/...); bex tags
// service/instance/container. Adapters render it verbatim.
type LogEntry struct {
	Timestamp string            `json:"timestamp,omitempty"`
	Message   string            `json:"message"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// Core is the shared domain layer. Every product action lives here exactly
// once; REST and GraphQL are presentation only.
type Core struct {
	Client    client.Client
	Namespace string
	// Now supplies the restart timestamp; injectable for tests. Defaults to
	// time.Now if nil.
	Now func() time.Time
	// PodLogs fetches pod logs for the Logs verb; nil => Logs reports
	// ErrLogsUnavailable. Injected so Core needs no typed clientset.
	PodLogs PodLogSource
	// PodLogsFollow streams live pod logs for FollowLogs (the SSE tail); nil =>
	// FollowLogs reports ErrLogsUnavailable. See logs.go.
	PodLogsFollow PodLogStream
	// ResourceMetrics reads current per-pod CPU/memory (metrics-server); nil =>
	// the cpu/memory metrics report ErrMetricsUnavailable. See metrics.go.
	ResourceMetrics ResourceMetricsSource
	// RequestMetrics reads request time-series (Traefik via Prometheus); nil =>
	// the http_requests/http_latency/bandwidth metrics report ErrMetricsUnavailable.
	RequestMetrics RequestMetricsSource
	// MonthToDateBandwidthSource reads cumulative HTTP egress since a given
	// time (Prometheus increase()); nil => MonthToDateBandwidth reports
	// ErrMetricsUnavailable. See metrics.go.
	MonthToDateBandwidthSource MonthToDateBandwidthSource
	// MetricsFilterValuesSource discovers a Prometheus label's observed values
	// (e.g. STATUS_CODE); nil => that filter's discovered values are empty
	// rather than erroring — RESOURCE/INSTANCE/HOST need no source (real data
	// already on hand). See metrics.go.
	MetricsFilterValuesSource MetricsFilterValuesSource
	// APIKeys manages machine credentials (OAuth2 clients in Hydra); nil =>
	// the api-key verbs report ErrAPIKeysUnavailable. See apikeys.go.
	APIKeys APIKeyStore
	// Authz decides what the authenticated caller may do (OpenFGA); nil =>
	// every verb allowed (pre-authorization behavior). See authz.go.
	Authz Checker
	// Store is the Postgres source of truth for store-managed Apps (those
	// carrying the bex.co/app-id label). Suspend/Resume write the row first —
	// the row owns spec.suspended, and the projection loop reverts CR patches
	// it didn't originate — then patch the CR as the fast-converge path. Nil
	// (tests, DB-less mode) falls back to CR-only patches, safe only for
	// hand-applied Apps. See internal/store.
	Store IntentStore
}

// IntentStore is the slice of the source of truth Core writes through — kept to
// the one method the lifecycle verbs need, so Core can't quietly grow into a
// second store client and tests fake a single method. *store.PGStore satisfies it.
type IntentStore interface {
	SetAppSuspended(ctx context.Context, id string, suspended bool) error
}

func (c *Core) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func view(a *appv1alpha1.App) AppView {
	created := ""
	if !a.CreationTimestamp.IsZero() {
		created = a.CreationTimestamp.UTC().Format(time.RFC3339)
	}
	return AppView{
		Name:      a.Name,
		Phase:     string(a.Status.Phase),
		URL:       a.Status.URL,
		URLs:      a.Status.URLs,
		Image:     a.Status.Image,
		Replicas:  a.Spec.Replicas,
		Suspended: a.Spec.Suspended,
		Revision:  a.Status.ActiveRevision,
		CreatedAt: created,
	}
}

// List returns every App in the namespace.
func (c *Core) List(ctx context.Context) ([]AppView, error) {
	if err := c.authorize(ctx, relCanView); err != nil {
		return nil, err
	}
	var list appv1alpha1.AppList
	if err := c.Client.List(ctx, &list, client.InNamespace(c.Namespace)); err != nil {
		return nil, err
	}
	out := make([]AppView, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, view(&list.Items[i]))
	}
	return out, nil
}

// Get returns one App, or ErrNotFound.
func (c *Core) Get(ctx context.Context, name string) (AppView, error) {
	if err := c.authorize(ctx, relCanView); err != nil {
		return AppView{}, err
	}
	a, err := c.fetch(ctx, name)
	if err != nil {
		return AppView{}, err
	}
	return view(a), nil
}

// Restart requests a rolling restart (spec.restartedAt = now). The operator
// stamps the pod template and Kubernetes rolls the pods with no downtime.
func (c *Core) Restart(ctx context.Context, name string) (AppView, error) {
	if err := c.authorize(ctx, relCanOperate); err != nil {
		return AppView{}, err
	}
	return c.patch(ctx, name, func(a *appv1alpha1.App) {
		a.Spec.RestartedAt = c.now().UTC().Format(time.RFC3339)
	})
}

// Suspend parks the App (spec.suspended = true): scaled to 0, host/certs kept.
func (c *Core) Suspend(ctx context.Context, name string) (AppView, error) {
	if err := c.authorize(ctx, relCanOperate); err != nil {
		return AppView{}, err
	}
	return c.setSuspended(ctx, name, true)
}

// Resume brings a suspended App back (spec.suspended = false); the operator
// restores spec.replicas.
func (c *Core) Resume(ctx context.Context, name string) (AppView, error) {
	if err := c.authorize(ctx, relCanOperate); err != nil {
		return AppView{}, err
	}
	return c.setSuspended(ctx, name, false)
}

// setSuspended flips suspension with the row as the single writer of intent.
// For store-managed Apps the apps row is updated first (the projection loop
// owns spec.suspended and would revert a bare CR patch on the next resync);
// the CR patch after it makes the change converge immediately — and if that
// patch fails, the row is already right, so the resync converges it anyway.
// Restart needs no row write: spec.restartedAt is not projection-owned (see
// applyOwnedSpec in internal/store).
func (c *Core) setSuspended(ctx context.Context, name string, suspended bool) (AppView, error) {
	if c.Store != nil {
		a, err := c.fetch(ctx, name)
		if err != nil {
			return AppView{}, err
		}
		if id := a.Labels[store.LabelAppID]; id != "" {
			if err := c.Store.SetAppSuspended(ctx, id, suspended); err != nil {
				return AppView{}, fmt.Errorf("update source of truth: %w", err)
			}
		}
	}
	return c.patch(ctx, name, func(a *appv1alpha1.App) { a.Spec.Suspended = suspended })
}

// Logs returns recent log lines for an App, aggregated across its pods
// (instances) and sorted by timestamp so instances interleave chronologically.
// tail caps lines per instance (<=0 => defaultLogTail). It fails with
// ErrNotFound for an unknown App (same as Get) and ErrLogsUnavailable when no
// pod-log source is wired. This is the read path the MCP list_logs tool uses.
func (c *Core) Logs(ctx context.Context, name string, tail int64) ([]LogEntry, error) {
	if err := c.authorize(ctx, relCanViewLogs); err != nil {
		return nil, err
	}
	if c.PodLogs == nil {
		return nil, ErrLogsUnavailable
	}
	if _, err := c.fetch(ctx, name); err != nil {
		return nil, err // ErrNotFound for unknown apps, exactly like Get
	}
	if tail <= 0 {
		tail = defaultLogTail
	}
	return c.collectPodLogs(ctx, name, tail)
}

// collectPodLogs reads up to tail lines from every replica of an App, tagged and
// timestamp-sorted. Shared by Logs (MCP) and QueryLogs (REST/GraphQL) so all
// surfaces read pod logs the same way.
func (c *Core) collectPodLogs(ctx context.Context, name string, tail int64) ([]LogEntry, error) {
	var pods corev1.PodList
	if err := c.Client.List(ctx, &pods,
		client.InNamespace(c.Namespace),
		client.MatchingLabels{podLabelApp: name}); err != nil {
		return nil, err
	}
	var out []LogEntry
	for i := range pods.Items {
		entries, err := c.readPodLogs(ctx, name, pods.Items[i].Name, tail)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Timestamp < out[j].Timestamp })
	return out, nil
}

func (c *Core) readPodLogs(ctx context.Context, service, pod string, tail int64) ([]LogEntry, error) {
	rc, err := c.PodLogs(ctx, c.Namespace, pod, appContainer, tail)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	var entries []LogEntry
	sc := bufio.NewScanner(rc)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024) // allow long lines
	for sc.Scan() {
		entries = append(entries, parseLogLine(service, pod, sc.Text()))
	}
	return entries, sc.Err()
}

// parseLogLine splits kubelet's "Timestamps: true" prefix (an RFC3339Nano stamp,
// a space, then the message) off a log line, tagging it with Render-shaped
// labels. A line without a parseable stamp is kept whole as the message.
func parseLogLine(service, pod, line string) LogEntry {
	ts, msg := "", line
	if i := strings.IndexByte(line, ' '); i > 0 {
		if t, err := time.Parse(time.RFC3339Nano, line[:i]); err == nil {
			ts = t.UTC().Format(time.RFC3339Nano)
			msg = line[i+1:]
		}
	}
	return LogEntry{
		Timestamp: ts,
		Message:   msg,
		Labels: map[string]string{
			"service":   service,
			"instance":  pod,
			"container": appContainer,
		},
	}
}

func (c *Core) fetch(ctx context.Context, name string) (*appv1alpha1.App, error) {
	var a appv1alpha1.App
	err := c.Client.Get(ctx, client.ObjectKey{Namespace: c.Namespace, Name: name}, &a)
	if apierrors.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// patch fetches the App, applies mutate to its spec, and merge-patches — only
// spec fields change; the operator reconciles the rest. This is the single
// write path both adapters share.
func (c *Core) patch(ctx context.Context, name string, mutate func(*appv1alpha1.App)) (AppView, error) {
	a, err := c.fetch(ctx, name)
	if err != nil {
		return AppView{}, err
	}
	base := client.MergeFrom(a.DeepCopy())
	mutate(a)
	if err := c.Client.Patch(ctx, a, base); err != nil {
		return AppView{}, err
	}
	return view(a), nil
}
