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
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestCanceledPhaseReachesEverySurface pins w6/m52's cross-surface contract:
// the operator's new Canceled phase must be readable identically wherever a
// service's phase is exposed. REST serializes AppView, MCP returns it, and
// GraphQL projects AppView.Phase — one field, so this asserts the value is
// carried verbatim rather than folded back into "Failed" anywhere en route.
func TestCanceledPhaseReachesEverySurface(t *testing.T) {
	a := sampleApp("web")
	a.Status.Phase = appv1alpha1.PhaseCanceled
	svc, _ := newService(nil, a)

	v, err := svc.Get(context.Background(), "web")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v.Phase != string(appv1alpha1.PhaseCanceled) {
		t.Fatalf("AppView.Phase = %q, want %q — a canceled service must not read as failed anywhere",
			v.Phase, appv1alpha1.PhaseCanceled)
	}

	// REST and MCP both render this same struct, so the JSON tag is the shared
	// wire contract for both.
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Phase string `json:"phase"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.Phase != string(appv1alpha1.PhaseCanceled) {
		t.Fatalf("wire phase = %q, want %q", wire.Phase, appv1alpha1.PhaseCanceled)
	}
}

// TestDeletingServiceIsNotFoundByID pins the w3/m81 read contract: once an App's
// deletion is accepted, the by-id Get is absent (core.ErrNotFound) — it must not
// keep serving `phase: Deleting` plus a withdrawn URL the way the m81 fixture
// (srv-da7tf87krsvc73c3mcng) did for 2+ hours. The shared projection still labels
// a deleting App Deleting (superseding the last operator phase, Canceled here)
// and drops its dead URL, as defense-in-depth for any caller that reaches the
// pure view() without the verb-level gate.
func TestDeletingServiceIsNotFoundByID(t *testing.T) {
	a := sampleApp("web")
	a.Status.Phase = appv1alpha1.PhaseCanceled
	a.Status.URL = "https://web.onbex.co"
	now := metav1.NewTime(time.Unix(1_000_000, 0))
	a.DeletionTimestamp = &now
	a.Finalizers = []string{"app.bex.co/finalizer"}
	svc, _ := newService(nil, a)

	if _, err := svc.Get(context.Background(), "web"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("Get on a deleting service: got %v, want core.ErrNotFound (absent, like List and Render's GET 404)", err)
	}

	v := view(a)
	if v.Phase != "Deleting" {
		t.Fatalf("view phase = %q, want Deleting for an App under deletion", v.Phase)
	}
	if v.URL != "" {
		t.Fatalf("view URL = %q, want empty for a deleting App (route/cert withdrawn)", v.URL)
	}
}
