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
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// TenantQuotaName is the per-workspace ResourceQuota carrying the plan's
// count/<resource> caps (store.NamespaceReconciler applies it into `<ws>`).
const TenantQuotaName = "tenant-quota"

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
	return quotaCapExceeded(digits[:end], noun), true
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
	return quotaCapExceeded(strconv.FormatInt(limit, 10), c.Noun)
}

func quotaCapExceeded(limit, noun string) error {
	return fmt.Errorf("%w: workspace is limited to %s %ss; delete an existing %s to create another", ErrBadRequest, limit, noun, noun)
}
