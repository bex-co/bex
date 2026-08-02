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

package apps

import (
	"errors"
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestMapServiceCapError pins the w3/m34 t004 wiring: a per-namespace
// ResourceQuota rejection of the App CR create (count/apps.app.bex.co, the
// cap that replaced the deleted BEX_MAX_SERVICES app-code check) comes back
// as the same Render-shaped cap message callers saw before
// (docs/ADR006-bex-api.md § Per-workspace resource caps), never a raw
// Kubernetes admission error. The real end-to-end admission behavior itself
// (a fake client never enforces ResourceQuota) is verified live per ADR043's
// existing production-verification methodology; this pins the translation
// this package owns.
func TestMapServiceCapError(t *testing.T) {
	quotaErr := apierrors.NewForbidden(
		schema.GroupResource{Group: "app.bex.co", Resource: "apps"}, "over-cap",
		fmt.Errorf("exceeded quota: tenant-quota, requested: count/apps.app.bex.co=1, used: count/apps.app.bex.co=25, limited: count/apps.app.bex.co=25"),
	)

	got := mapServiceCapError(quotaErr)
	if !errors.Is(got, core.ErrBadRequest) {
		t.Fatalf("mapServiceCapError(quota rejection) = %v, want it to wrap core.ErrBadRequest", got)
	}
	const wantMsg = "workspace is limited to 25 services; delete an existing service to create another"
	if got.Error() != fmt.Sprintf("%s: %s", core.ErrBadRequest, wantMsg) {
		t.Errorf("mapServiceCapError message = %q, want it to end with %q", got.Error(), wantMsg)
	}

	if got := mapServiceCapError(nil); got != nil {
		t.Errorf("mapServiceCapError(nil) = %v, want nil", got)
	}

	other := errors.New("some other create failure")
	if got := mapServiceCapError(other); got != other {
		t.Errorf("mapServiceCapError(non-quota error) = %v, want it passed through unchanged", got)
	}
}
