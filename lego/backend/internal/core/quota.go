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

package core

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// TenantQuotaName is the per-workspace ResourceQuota carrying the plan's
// count/<resource> caps, plus the storage, pod and ephemeral-storage axes,
// over every object in the namespace (store.NamespaceReconciler applies it
// into `<ws>`).
const TenantQuotaName = "tenant-quota"

// TenantComputeQuotaName is the per-workspace ResourceQuota carrying the plan's
// CPU/memory budget. It is scoped to Pods outside the datastore PriorityClass
// (appv1alpha1.DatastorePriorityClassName), so it bounds services, jobs and
// builds but never a Postgres or Key Value instance (w2/m247).
const TenantComputeQuotaName = "tenant-compute-quota"

// QuotaCapError translates a per-namespace ResourceQuota admission rejection
// (the count/<resource> caps that replaced the retired app-code
// BEX_MAX_SERVICES/_POSTGRES/_KEYVALUES checks, ADR043 D3, w3/m34) into the
// same Render-shaped cap error those checks used to return
// (docs/ADR006-bex-api.md § Per-workspace resource caps): "workspace is
// limited to N <noun>s; delete an existing <noun> to create another". ok is
// false for any error that isn't a matching quota-exceeded rejection —
// including an unrelated Forbidden — which the caller must return unwrapped
// rather than leak a raw Kubernetes admission message to the API surface.
//
// The admission plugin's message shape (k8s.io/apiserver/pkg/admission/plugin/
// resourcequota) is "... limited: <key1>=<n1>, <key2>=<n2>, ...", so this scans
// for "<countKey>=" anywhere after "limited:" rather than assuming countKey is
// the first (or only) dimension listed there.
func QuotaCapError(err error, countKey, noun string) (mapped error, ok bool) {
	if !apierrors.IsForbidden(err) {
		return nil, false
	}
	msg := err.Error()
	limited := strings.Index(msg, "limited:")
	if limited < 0 {
		return nil, false
	}
	needle := countKey + "="
	rest := msg[limited+len("limited:"):]
	at := strings.Index(rest, needle)
	if at < 0 {
		return nil, false
	}
	digits := rest[at+len(needle):]
	end := 0
	for end < len(digits) && digits[end] >= '0' && digits[end] <= '9' {
		end++
	}
	if end == 0 {
		return nil, false
	}
	limit, err := strconv.ParseInt(digits[:end], 10, 64)
	if err != nil {
		return nil, false
	}
	return quotaCapExceeded(limit, noun), true
}

// CountCap names one per-kind workspace cap: the ResourceQuota key that
// enforces it and the noun its refusal reads in.
type CountCap struct{ Key, Noun string }

// CreateError maps the API server's refusal of a tenant CR create onto the
// API's answer, the same for the real create and its dry-run (DryRunCreate):
// this cap's Render-shaped message, or a 400 naming the invalid fields. Any
// other error comes back unchanged.
func (c CountCap) CreateError(err error) error {
	if mapped, ok := QuotaCapError(err, c.Key, c.Noun); ok {
		return mapped
	}
	if mapped, ok := InvalidFieldsError(err); ok {
		return mapped
	}
	return err
}

// Room is how many more objects q admits under this cap: its enforced hard
// limit less what is used. ok is false when q enforces no such limit.
func (c CountCap) Room(q *corev1.ResourceQuota) (room, limit int64, ok bool) {
	hard, ok := q.Status.Hard[corev1.ResourceName(c.Key)]
	if !ok {
		return 0, 0, false
	}
	used := q.Status.Used[corev1.ResourceName(c.Key)]
	return hard.Value() - used.Value(), hard.Value(), true
}

// Exceeded is the refusal of a create past this cap's limit: what CreateError
// maps admission's quota refusal to.
func (c CountCap) Exceeded(limit int64) error {
	return quotaCapExceeded(limit, c.Noun)
}

// WorkspaceResourceLimitCode is a create refused past its workspace's count
// cap, so a client recognizes the cap by code, not by its wording (w5/m130).
const WorkspaceResourceLimitCode = "WORKSPACE_RESOURCE_LIMIT"

// FreeInstanceLimitCode is a free Postgres/Key Value create (or plan change
// to free) refused because the workspace already holds its one free instance
// of that kind — Render's free rule on every workspace plan (w4/m241). It is
// deliberately not WorkspaceResourceLimitCode: no workspace-plan upgrade
// lifts it.
const FreeInstanceLimitCode = "FREE_INSTANCE_LIMIT"

// FreeInstanceLimitError names the existing free instance holding the slot,
// and plan as the field to change, so a Blueprint locates it there (w4/416).
// A terminating holder still holds it until deletion finishes (w6/m129).
func FreeInstanceLimitError(noun, existingName string, terminating bool) error {
	if terminating {
		return NewBadRequestError(FreeInstanceLimitCode,
			fmt.Sprintf("this workspace's free %s (%s) is still being deleted; wait for its deletion to finish or choose a paid instance type", noun, existingName),
			map[string]any{"limit": 1, "field": "plan"})
	}
	return NewBadRequestError(FreeInstanceLimitCode,
		fmt.Sprintf("this workspace already has a free %s (%s); delete it or choose a paid instance type", noun, existingName),
		map[string]any{"limit": 1, "field": "plan"})
}

// TenantAdvisoryLocker is the optional cross-replica lock a workspace
// resolver may offer (production: a Postgres advisory lock, store/advisory.go).
type TenantAdvisoryLocker interface {
	WithTenantAdvisoryLock(ctx context.Context, tenantID string, fn func() error) error
}

// WithTenantAdvisoryLock runs fn under the resolver's tenant lock when it
// offers one; otherwise fn runs directly (store-off/test wiring, where the
// caller's post-write recheck remains the race guard).
func WithTenantAdvisoryLock(ctx context.Context, resolver WorkspaceResolver, tenantID string, fn func() error) error {
	if locker, ok := resolver.(TenantAdvisoryLocker); ok {
		return locker.WithTenantAdvisoryLock(ctx, tenantID, fn)
	}
	return fn()
}

// quotaCapExceeded reads the limit in the singular at 1 ("limited to 1
// Postgres database", w4/339); every cap's noun pluralizes with a trailing s.
func quotaCapExceeded(limit int64, noun string) error {
	limited := noun + "s"
	if limit == 1 {
		limited = noun
	}
	return NewBadRequestError(WorkspaceResourceLimitCode,
		fmt.Sprintf("workspace is limited to %d %s; delete an existing %s to create another", limit, limited, noun),
		map[string]any{"limit": limit})
}
