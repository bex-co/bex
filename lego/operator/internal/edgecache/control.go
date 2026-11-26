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

package edgecache

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The pod annotations a running proxy follows (w3/m486). The operator patches
// them on each live pod, never on its template, and the kubelet refreshes the
// downward-API file that carries them, so a purge or a profile switch reaches
// the running proxy without restarting the instance, as on Render's CDN.
const (
	// PurgedAtAnnotation is the App's last manual purge; a new value empties
	// the cache. Aliases the v1alpha1 canonical, which bex-api stamps and the
	// controller propagates here, so the stamp and the purge can never drift
	// apart.
	PurgedAtAnnotation = appv1alpha1.CachePurgedAtAnnotation
	// RevisionAnnotation is the deploy whose success last emptied the cache.
	// The operator sets it on each pod of a service's cache tier (w3/m489),
	// which outlives the app's instances, so a deploy no longer starts it
	// empty by replacing it; a new value empties the cache, as Render purges
	// on each successful deploy and never on a failed one.
	RevisionAnnotation = "app.bex.co/cache-revision"
	// ProfileAnnotation is the profile the service caches by now; absent, the
	// proxy keeps the one its pod started with.
	ProfileAnnotation = "app.bex.co/cache-profile"
)

// Controller applies a pod's control file, its downward-API metadata.annotations
// file, to a Handler.
type Controller struct {
	handler *Handler
	path    string
	// Logf reports each purge, profile switch and read failure; nil is silent.
	Logf func(format string, args ...any)

	read bool
	// stamps is the value of each purge annotation as last read.
	stamps map[string]string
}

// purgeAnnotations are the annotations a new value of which empties the cache.
var purgeAnnotations = []string{PurgedAtAnnotation, RevisionAnnotation}

// NewController follows the file at path for h.
func NewController(h *Handler, path string) *Controller {
	return &Controller{handler: h, path: path, stamps: map[string]string{}}
}

// Sync reads the control file once. The first read only records the purge
// annotations (the manual purge stamp and the cache revision), since a new
// proxy starts empty; a later read in which either differs purges, once. The
// profile annotation becomes the handler's profile, and without one the
// profile the proxy started with (Config.Profile) holds.
func (c *Controller) Sync() error {
	raw, err := os.ReadFile(c.path)
	if err != nil {
		return err
	}
	annotations := parseAnnotations(raw)
	var causes []string
	for _, name := range purgeAnnotations {
		if stamp := annotations[name]; stamp != c.stamps[name] {
			causes = append(causes, name+"="+stamp)
			c.stamps[name] = stamp
		}
	}
	if c.read && len(causes) > 0 {
		c.handler.Purge()
		c.logf("edge-cache: purged (%s)", strings.Join(causes, ", "))
	}
	profile, ok := annotations[ProfileAnnotation]
	if !ok {
		profile = c.handler.cfg.Profile
	}
	if before := c.handler.Profile(); c.handler.SetProfile(profile) {
		c.logf("edge-cache: profile %q -> %q", before, profile)
	}
	c.read = true
	return nil
}

// Run syncs at once, then every interval until ctx ends, reporting a read
// failure once until it changes.
func (c *Controller) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	lastErr := ""
	for {
		msg := ""
		if err := c.Sync(); err != nil {
			msg = err.Error()
		}
		if msg != lastErr && msg != "" {
			c.logf("edge-cache: read %s: %s", c.path, msg)
		}
		lastErr = msg
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (c *Controller) logf(format string, args ...any) {
	if c.Logf != nil {
		c.Logf(format, args...)
	}
}

// parseAnnotations reads the kubelet's downward-API rendering of a map: one
// key="value" line per entry, the value quoted as Go's %q quotes it
// (k8s.io/kubernetes/pkg/fieldpath.FormatMap). A line that does not parse is
// skipped.
func parseAnnotations(raw []byte) map[string]string {
	annotations := map[string]string{}
	for line := range strings.SplitSeq(string(raw), "\n") {
		key, quoted, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if value, err := strconv.Unquote(quoted); err == nil {
			annotations[key] = value
		}
	}
	return annotations
}
