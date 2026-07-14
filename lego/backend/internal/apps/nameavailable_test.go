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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestNameAvailable_FreeNameNoSuggestion(t *testing.T) {
	svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"}, tenantApp("web", "tea-a"))
	ctx := ctxAs("identity-a")

	got, err := svc.NameAvailable(ctx, "api")
	if err != nil {
		t.Fatalf("NameAvailable: %v", err)
	}
	if !got.Available || got.Suggestion != "" {
		t.Errorf("free name = %+v, want available with no suggestion", got)
	}
}

func TestNameAvailable_TakenSuggestsFirstFreeSuffix(t *testing.T) {
	svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"}, tenantApp("beancount-cms-v2", "tea-a"))
	ctx := ctxAs("identity-a")

	got, err := svc.NameAvailable(ctx, "beancount-cms-v2")
	if err != nil {
		t.Fatalf("NameAvailable: %v", err)
	}
	if got.Available || got.Suggestion != "beancount-cms-v2-1" {
		t.Errorf("taken name = %+v, want unavailable with suggestion beancount-cms-v2-1", got)
	}
}

func TestNameAvailable_SuffixChainSkipsOccupied(t *testing.T) {
	svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"},
		tenantApp("web", "tea-a"), tenantApp("web-1", "tea-a"))
	ctx := ctxAs("identity-a")

	got, err := svc.NameAvailable(ctx, "web")
	if err != nil {
		t.Fatalf("NameAvailable: %v", err)
	}
	if got.Available || got.Suggestion != "web-2" {
		t.Errorf("chained suffix = %+v, want unavailable with suggestion web-2 (web-1 already taken)", got)
	}
}

func TestNameAvailable_TruncatesAtMaxLength(t *testing.T) {
	base := strings.Repeat("a", 30) // ValidAppName's cap
	svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"}, tenantApp(base, "tea-a"))
	ctx := ctxAs("identity-a")

	got, err := svc.NameAvailable(ctx, base)
	if err != nil {
		t.Fatalf("NameAvailable: %v", err)
	}
	if got.Available {
		t.Fatalf("max-length taken name reported available")
	}
	if len(got.Suggestion) > 30 {
		t.Errorf("suggestion %q is %d chars, want <=30", got.Suggestion, len(got.Suggestion))
	}
	if !store.ValidAppName(got.Suggestion) {
		t.Errorf("suggestion %q does not pass ValidAppName", got.Suggestion)
	}
}

func TestNameAvailable_InvalidNameIsBadRequest(t *testing.T) {
	svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
	ctx := ctxAs("identity-a")

	_, err := svc.NameAvailable(ctx, "Not_Valid!")
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("invalid name: got %v, want ErrBadRequest", err)
	}
}

// TestNameAvailable_NeverReflectsAnotherWorkspace is the check's whole point
// (w4/m19): tenant A's "web" must never leak into tenant B's availability
// check — cross-workspace existence leaks are exactly what the create path
// was fixed to stop (t003); the pre-check must not reopen the same leak.
func TestNameAvailable_NeverReflectsAnotherWorkspace(t *testing.T) {
	svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a", "identity-b": "tea-b"},
		tenantApp("web", "tea-a"))
	ctx := ctxAs("identity-b")

	got, err := svc.NameAvailable(ctx, "web")
	if err != nil {
		t.Fatalf("NameAvailable: %v", err)
	}
	if !got.Available {
		t.Errorf("tenant B's check reflected tenant A's name: %+v", got)
	}
}
