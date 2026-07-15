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

// Package resourcemeta holds the neutral, cross-feature contract for the
// Render metadata projected by REST adapters. It deliberately contains no
// REST wire structs: apps, postgres, and keyvalue remain responsible for their
// own JSON shapes.
package resourcemeta

import (
	"context"
	"net/url"
	"path"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UpdatedAtAnnotation records the last successful bex-api resource mutation.
// Kubernetes managedFields supply reconciliation/update times for changes made
// by other writers; this annotation makes the API's own write response
// immediately authoritative without waiting for a controller round trip.
const UpdatedAtAnnotation = "app.bex.co/updated-at"

// Owner is the neutral workspace identity shared by resource adapters.
// Email is optional; Type is currently "team" for every bex workspace.
type Owner struct {
	ID    string
	Name  string
	Email string
	Type  string
}

// Available reports whether the minimum truthful Render owner identity can be
// emitted. Adapters share this omission rule while retaining package-local
// wire structs.
func (o Owner) Available() bool { return o.ID != "" && o.Name != "" }

// OwnerResolver resolves a batch of workspace ids visible to the caller. A
// missing key means the owner is unavailable or not visible and must be
// omitted, never replaced with an id-as-name placeholder.
type OwnerResolver interface {
	ResolveResourceOwners(context.Context, []string) map[string]Owner
}

// ResolveOwners de-duplicates ids before invoking the shared resolver. It is a
// no-op when the control-plane store is unavailable or every resource is a
// legacy unlabeled object.
func ResolveOwners(ctx context.Context, resolver OwnerResolver, ownerIDs []string) map[string]Owner {
	if resolver == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(ownerIDs))
	ids := make([]string, 0, len(ownerIDs))
	for _, id := range ownerIDs {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}
	return resolver.ResolveResourceOwners(ctx, ids)
}

// Config is the explicit platform metadata source. Region is BEX_REGION, not
// an application/storage setting. DashboardBaseURL is BEX_DASHBOARD_URL.
type Config struct {
	Region           string
	DashboardBaseURL string
}

// PlatformRegion returns the configured region after treating whitespace-only
// configuration as unavailable.
func (c Config) PlatformRegion() string { return strings.TrimSpace(c.Region) }

// DashboardURL joins a known dashboard route and resource id onto the trusted
// dashboard origin. Invalid/relative configuration and empty ids are omitted.
func (c Config) DashboardURL(route, id string) string {
	base := strings.TrimSpace(c.DashboardBaseURL)
	if base == "" || id == "" {
		return ""
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	u.Path = path.Join(u.Path, route, id)
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// Touch records a real API mutation on obj. Callers invoke it immediately
// before the Kubernetes write so a failed write never reports a new time.
func Touch(obj metav1.Object, now time.Time) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[UpdatedAtAnnotation] = now.UTC().Format(time.RFC3339Nano)
	obj.SetAnnotations(annotations)
}

// UpdatedAt returns the newest authoritative mutation/reconciliation time.
// Legacy objects with neither the bex annotation nor managed-field timestamps
// return empty instead of copying creationTimestamp into a fake update time.
func UpdatedAt(obj metav1.Object) string {
	var latest time.Time
	if raw := obj.GetAnnotations()[UpdatedAtAnnotation]; raw != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			latest = parsed
		}
	}
	for _, field := range obj.GetManagedFields() {
		if field.Time != nil && field.Time.After(latest) {
			latest = field.Time.Time
		}
	}
	if latest.IsZero() {
		return ""
	}
	return latest.UTC().Format(time.RFC3339Nano)
}
