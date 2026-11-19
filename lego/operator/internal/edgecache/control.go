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
)

// The pod annotations a running proxy follows (w3/m486). The operator patches
// them on each live pod, never on its template, and the kubelet refreshes the
// downward-API file that carries them, so a purge or a profile switch reaches
// the running proxy without restarting the instance, as on Render's CDN.
const (
	// PurgedAtAnnotation is the App's last manual purge
	// (appv1alpha1.CachePurgedAtAnnotation); a new value empties the cache.
	PurgedAtAnnotation = "app.bex.co/cache-purged-at"
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

	read     bool
	purgedAt string
}

// NewController follows the file at path for h.
func NewController(h *Handler, path string) *Controller {
	return &Controller{handler: h, path: path}
}

// Sync reads the control file once. The first read only records the purge
// stamp, since a new proxy starts empty; a later read whose stamp differs
// purges. The profile annotation becomes the handler's profile, and without
// one the profile the proxy started with (Config.Profile) holds.
func (c *Controller) Sync() error {
	raw, err := os.ReadFile(c.path)
	if err != nil {
		return err
	}
	annotations := parseAnnotations(raw)
	if stamp := annotations[PurgedAtAnnotation]; stamp != c.purgedAt {
		if c.read {
			c.handler.Purge()
			c.logf("edge-cache: purged (%s=%s)", PurgedAtAnnotation, stamp)
		}
		c.purgedAt = stamp
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
