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
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

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
	limit := digits[:end]
	return fmt.Errorf("%w: workspace is limited to %s %ss; delete an existing %s to create another", ErrBadRequest, limit, noun, noun), true
}
