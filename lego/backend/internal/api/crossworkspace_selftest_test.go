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

package api

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// crossworkspace_selftest_test.go is w7/m55/t006: the anti-tautology proof. A
// matrix that passes on today's correct code but would ALSO pass on broken code
// guards nothing. These tests seed the regression classes the matrix claims to
// catch — as a permanent synthetic leaky service, not a transient hand-edit — and
// assert the matrix's own decision logic (callVerbResult / driveNonMember: a
// cross-workspace SUCCESS is the failure) fires. If the fixture ever stops
// denying, or the "success == leak" logic breaks, THESE go red first.
//
// The FGA-MODEL regression class (5) can't be seeded in-process (it needs a real
// OpenFGA model change), so it is evidenced by a one-time manual relaxation
// recorded in the milestone README; the E2E's carl-viewer-denied-write
// assertions (multiworkspace_e2e_test.go cases 4/6/7) are its standing guard.

// leakyService embeds *core.Base and exposes three verbs of the same shape a real
// resource verb has — one correct, two broken in exactly the ways the matrix
// exists to catch — all reading the tea-b "target" App the shared fixture seeds.
type leakyService struct{ *core.Base }

// CorrectGet authorizes against the RESOURCE's own workspace (the w6/m17 seam):
// tea-b, which the caller is not a member of ⇒ refused. The matrix passes it.
func (s *leakyService) CorrectGet(ctx context.Context, name string) (string, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, name)
	if err != nil {
		return "", err
	}
	return a.Labels[core.LabelTenant], nil
}

// ConfusedDeputyGet is regression class (2): it authorizes the CALLER'S OWN
// workspace (tea-a, allowed by the allow-own checker) and then raw-fetches the
// tea-b resource — leaking it. A deny-all checker would mask this; the matrix's
// allow-own checker surfaces it as a success.
func (s *leakyService) ConfusedDeputyGet(ctx context.Context, name string) (string, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return "", err
	}
	var a appv1alpha1.App
	if err := s.Client.Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: name}, &a); err != nil {
		return "", err
	}
	return a.Labels[core.LabelTenant], nil
}

// UnguardedGet is regression class (1): no Authorize call at all — it raw-fetches
// and returns the tea-b resource.
func (s *leakyService) UnguardedGet(ctx context.Context, name string) (string, error) {
	var a appv1alpha1.App
	if err := s.Client.Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: name}, &a); err != nil {
		return "", err
	}
	return a.Labels[core.LabelTenant], nil
}

// TestCrossWorkspaceMatrixCatchesVerbLeaks proves the service-verb sweep's core
// decision (callVerbResult returning nil ⇒ the verb leaked ⇒ flagged) is not
// tautological: a confused-deputy and an unguarded verb both return SUCCESS
// against the tea-b fixture (which is what TestCrossWorkspaceServiceVerbMatrix
// reports as a failure), while the correctly-guarded verb is denied.
func TestCrossWorkspaceMatrixCatchesVerbLeaks(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "attacker", Method: "oauth2"})
	svc := &leakyService{Base: crossWorkspaceBase()}
	cv := reflect.ValueOf(svc)
	call := func(method string) error {
		m, ok := cv.Type().MethodByName(method)
		if !ok {
			t.Fatalf("no method %q", method)
		}
		return callVerbResult(cv, m, ctx, crossWorkspaceTarget)
	}

	// (1) missing guard and (2) confused deputy both LEAK — success is exactly
	// what the matrix flags. If either returned an error here, the matrix would
	// be blind to that regression class.
	if err := call("UnguardedGet"); err != nil {
		t.Errorf("unguarded verb returned %v — the matrix relies on a missing-guard leak surfacing as SUCCESS", err)
	}
	if err := call("ConfusedDeputyGet"); err != nil {
		t.Errorf("confused-deputy verb returned %v — the matrix relies on it surfacing as SUCCESS "+
			"(and an allow-own checker, not deny-all, is what makes that possible)", err)
	}
	// The correctly-guarded verb is denied — proving the fixture actually blocks
	// cross-workspace access (so the two successes above are the leak, not the
	// fixture failing open).
	if err := call("CorrectGet"); err == nil {
		t.Error("correctly-guarded verb succeeded against the tea-b fixture — the fixture is not denying, so the whole matrix is vacuous")
	}
}

// TestCrossWorkspaceMatrixCatchesRESTLeaks is regression class (3): a REST route
// wired to a handler that skips the resource's authz path returns 2xx, which the
// REST matrix's driveNonMember + "never 2xx" assertion flags. A correctly-wired
// route refuses. This proves driveNonMember observes the leak as a 2xx and the
// refusal as a non-2xx.
func TestCrossWorkspaceMatrixCatchesRESTLeaks(t *testing.T) {
	svc := &leakyService{Base: crossWorkspaceBase()}
	handler := func(get func(context.Context, string) (string, error)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			owner, err := get(r.Context(), r.PathValue("name"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			_, _ = w.Write([]byte(owner))
		}
	}
	mux := http.NewServeMux()
	mux.Handle("GET /leak/{name}", handler(svc.ConfusedDeputyGet))
	mux.Handle("GET /safe/{name}", handler(svc.CorrectGet))

	if code := driveNonMember(mux, "GET", "/leak/"+crossWorkspaceTarget, ""); code < 200 || code >= 300 {
		t.Errorf("leaky route returned %d — the REST matrix relies on a route-level leak surfacing as 2xx", code)
	}
	if code := driveNonMember(mux, "GET", "/safe/"+crossWorkspaceTarget, ""); code >= 200 && code < 300 {
		t.Errorf("correctly-wired route returned %d (2xx) — driveNonMember must observe the refusal as non-2xx", code)
	}
}
