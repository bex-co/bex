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

package staticserver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// CachedResolver serves the host→Site map by periodically listing static_site
// Apps and atomically swapping an immutable snapshot. Polling (rather than an
// informer) keeps the standalone binary simple; static-site config changes are
// infrequent and a few seconds of edit propagation is acceptable (Render is not
// instant either). Only Apps that are static_site, not suspended, and already
// published (Status.ActiveRevision set) are served.
type CachedResolver struct {
	reader     client.Reader
	namespace  string // "" => all namespaces
	baseDomain string

	mu    sync.RWMutex
	sites map[string]Site
}

// NewCachedResolver builds a resolver reading Apps in namespace (empty = all)
// from reader; baseDomain expands an App's Expose flag to "<name>.<baseDomain>".
func NewCachedResolver(reader client.Reader, namespace, baseDomain string) *CachedResolver {
	return &CachedResolver{
		reader:     reader,
		namespace:  namespace,
		baseDomain: baseDomain,
		sites:      map[string]Site{},
	}
}

// Resolve returns the Site serving host, or ok=false when none does.
func (c *CachedResolver) Resolve(host string) (Site, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.sites[host]
	return s, ok
}

// Refresh rebuilds the host→Site snapshot from the current static_site Apps.
func (c *CachedResolver) Refresh(ctx context.Context) error {
	var list appv1alpha1.AppList
	opts := []client.ListOption{}
	if c.namespace != "" {
		opts = append(opts, client.InNamespace(c.namespace))
	}
	if err := c.reader.List(ctx, &list, opts...); err != nil {
		return fmt.Errorf("staticserver: list apps: %w", err)
	}
	next := map[string]Site{}
	for i := range list.Items {
		app := &list.Items[i]
		if app.Spec.Type != appv1alpha1.TypeStaticSite || app.Spec.Suspended {
			continue
		}
		if app.Status.ActiveRevision == "" {
			continue // not yet published — nothing to serve
		}
		site := Site{
			AppID:    app.Name,
			Revision: app.Status.ActiveRevision,
			Routes:   app.Spec.Routes,
			Headers:  app.Spec.Headers,
		}
		for _, h := range effectiveHosts(app, c.baseDomain) {
			next[h] = site
		}
	}
	c.mu.Lock()
	c.sites = next
	c.mu.Unlock()
	return nil
}

// Run refreshes the snapshot every interval until ctx is cancelled, after an
// immediate first refresh. A refresh error is non-fatal: the previous snapshot
// keeps serving until the next successful poll.
func (c *CachedResolver) Run(ctx context.Context, interval time.Duration, onErr func(error)) {
	if err := c.Refresh(ctx); err != nil && onErr != nil {
		onErr(err)
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := c.Refresh(ctx); err != nil && onErr != nil {
				onErr(err)
			}
		}
	}
}

// effectiveHosts mirrors the controller's host resolution for static sites so
// the resolver and the Ingress agree on which hosts route here: spec.host, the
// computed platform hostname when Expose is set, then spec.hosts (deduped,
// order preserved). The platform hostname prefers spec.subdomain — the
// control plane's globally-unique slug (w4/m19) — falling back to app.Name
// when unset (pre-migration or hand-applied Apps), same as
// controller.effectiveHosts.
func effectiveHosts(app *appv1alpha1.App, baseDomain string) []string {
	var hosts []string
	seen := map[string]bool{}
	add := func(h string) {
		if h != "" && !seen[h] {
			seen[h] = true
			hosts = append(hosts, h)
		}
	}
	add(app.Spec.Host)
	if app.Spec.Expose && baseDomain != "" && app.Spec.SubdomainPolicy != appv1alpha1.SubdomainPolicyDisabled {
		add(fmt.Sprintf("%s.%s", app.Spec.PlatformSubdomain(app.Name), baseDomain))
	}
	for _, h := range app.Spec.Hosts {
		add(h)
	}
	return hosts
}
