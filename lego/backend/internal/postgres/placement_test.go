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
package postgres

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func placementResource() *appv1alpha1.Database {
	obj := sampleDatabase("placement")
	obj.Labels = map[string]string{core.LabelProject: "prj-a", core.LabelEnvironment: "env-a", core.LabelTenant: "tea-a"}
	obj.Spec.IPAllowList = []appv1alpha1.IPAllowEntry{{CIDR: "203.0.113.0/24", Description: "resource-owned"}}
	obj.Spec.EnvironmentIPAllowList = []string{"198.51.100.0/24"}
	return obj
}

func readPlacement(t *testing.T, cl client.Client) *appv1alpha1.Database {
	t.Helper()
	var obj appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "placement"}, &obj); err != nil {
		t.Fatal(err)
	}
	return &obj
}

func TestPlacementProjectDepartureClearsEnvironment(t *testing.T) {
	for _, project := range []string{"prj-b", "", "prj-a"} {
		t.Run("project="+project, func(t *testing.T) {
			original := placementResource()
			svc, cl := newService(original)
			before := readPlacement(t, cl)
			if err := svc.SetProjectID(ctxAs("user-a"), original.Name, project); err != nil {
				t.Fatal(err)
			}
			got := readPlacement(t, cl)
			if got.Labels[core.LabelProject] != project {
				t.Errorf("project=%q, want %q", got.Labels[core.LabelProject], project)
			}
			if project == "prj-a" {
				if !reflect.DeepEqual(got, before) {
					t.Errorf("unchanged project rewrote resource: before=%+v after=%+v", before.ObjectMeta, got.ObjectMeta)
				}
			} else if got.Labels[core.LabelEnvironment] != "" || len(got.Spec.EnvironmentIPAllowList) != 0 {
				t.Errorf("project departure retained environment=%q inherited rules=%v", got.Labels[core.LabelEnvironment], got.Spec.EnvironmentIPAllowList)
			}
			if !slices.Equal(got.Spec.IPAllowList, original.Spec.IPAllowList) {
				t.Errorf("owned rules changed: %v", got.Spec.IPAllowList)
			}
			stable := got.DeepCopy()
			if err := svc.SetProjectID(ctxAs("user-a"), original.Name, project); err != nil {
				t.Fatal(err)
			}
			if got := readPlacement(t, cl); !reflect.DeepEqual(got, stable) {
				t.Error("repeated project assignment rewrote resource")
			}
		})
	}
}

func TestPlacementEnvironmentChangeClearsOnlyInheritedRules(t *testing.T) {
	for _, environment := range []string{"env-b", "", "env-a"} {
		t.Run("environment="+environment, func(t *testing.T) {
			original := placementResource()
			svc, cl := newService(original)
			before := readPlacement(t, cl)
			if err := svc.SetEnvironmentID(ctxAs("user-a"), original.Name, environment); err != nil {
				t.Fatal(err)
			}
			got := readPlacement(t, cl)
			if got.Labels[core.LabelEnvironment] != environment || got.Labels[core.LabelProject] != "prj-a" {
				t.Errorf("placement labels=%v", got.Labels)
			}
			if environment == "env-a" {
				if !reflect.DeepEqual(got, before) {
					t.Error("unchanged environment rewrote resource")
				}
			} else if len(got.Spec.EnvironmentIPAllowList) != 0 {
				t.Errorf("environment change retained old inherited rules=%v", got.Spec.EnvironmentIPAllowList)
			}
			if !slices.Equal(got.Spec.IPAllowList, original.Spec.IPAllowList) {
				t.Errorf("owned rules changed: %v", got.Spec.IPAllowList)
			}
		})
	}
}

func TestPlacementAssignmentAuthorizesBeforeNoOp(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Service) error
	}{
		{"same project", func(s *Service) error { return s.SetProjectID(ctxAs("user-a"), "placement", "prj-a") }},
		{"depart project", func(s *Service) error { return s.SetProjectID(ctxAs("user-a"), "placement", "prj-b") }},
		{"same environment", func(s *Service) error { return s.SetEnvironmentID(ctxAs("user-a"), "placement", "env-a") }},
		{"depart environment", func(s *Service) error { return s.SetEnvironmentID(ctxAs("user-a"), "placement", "") }},
		{"clear project", func(s *Service) error { return s.ClearProjectID(ctxAs("user-a"), "placement", "prj-a") }},
		{"stale project clear", func(s *Service) error { return s.ClearProjectID(ctxAs("user-a"), "placement", "prj-old") }},
		{"clear environment", func(s *Service) error { return s.ClearEnvironmentID(ctxAs("user-a"), "placement", "env-a") }},
		{"stale environment clear", func(s *Service) error { return s.ClearEnvironmentID(ctxAs("user-a"), "placement", "env-old") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService(placementResource())
			svc.Authz = &fakeChecker{allow: false}
			before := readPlacement(t, cl)
			if err := tc.change(svc); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("denied assignment error=%v, want forbidden", err)
			}
			if got := readPlacement(t, cl); !reflect.DeepEqual(got, before) {
				t.Error("refused assignment changed placement or rules")
			}
		})
	}
}

type placementPatchClient struct {
	client.Client
	patches     int
	beforePatch func()
	beforeEach  func()
	fail        error
}

func (c *placementPatchClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	c.patches++
	if c.beforeEach != nil {
		c.beforeEach()
	}
	if c.beforePatch != nil {
		before := c.beforePatch
		c.beforePatch = nil
		before()
	}
	if c.fail != nil {
		return c.fail
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func TestPlacementConditionalClear(t *testing.T) {
	for _, kind := range []string{"project", "environment"} {
		for _, stale := range []bool{false, true} {
			name := kind
			if stale {
				name += "/reassigned"
			}
			t.Run(name, func(t *testing.T) {
				svc, cl := newService(placementResource())
				tracking := &placementPatchClient{Client: cl}
				svc.Client = tracking
				before := readPlacement(t, cl)
				expected := "prj-a"
				clear := svc.ClearProjectID
				if kind == "environment" {
					expected = "env-a"
					clear = svc.ClearEnvironmentID
				}
				if stale {
					expected = "former-membership"
				}
				if err := clear(ctxAs("user-a"), "placement", expected); err != nil {
					t.Fatal(err)
				}
				got := readPlacement(t, cl)
				if stale {
					if tracking.patches != 0 || !reflect.DeepEqual(got, before) {
						t.Fatal("stale cleanup changed a reassigned resource")
					}
					return
				}
				if tracking.patches != 1 {
					t.Fatalf("cleanup used %d patches, want one atomic write", tracking.patches)
				}
				wantProject := "prj-a"
				if kind == "project" {
					wantProject = ""
				}
				if got.Labels[core.LabelProject] != wantProject || got.Labels[core.LabelEnvironment] != "" || len(got.Spec.EnvironmentIPAllowList) != 0 {
					t.Fatalf("cleanup left labels=%v inherited=%v", got.Labels, got.Spec.EnvironmentIPAllowList)
				}
				if !slices.Equal(got.Spec.IPAllowList, before.Spec.IPAllowList) {
					t.Fatal("cleanup changed owned rules")
				}
				if err := clear(ctxAs("user-a"), "placement", expected); err != nil {
					t.Fatal(err)
				}
				if tracking.patches != 1 || !reflect.DeepEqual(got, readPlacement(t, cl)) {
					t.Error("repeated cleanup rewrote resource")
				}
			})
		}
	}
}

func TestPlacementConditionalClearRetriesAfterWriteFailure(t *testing.T) {
	for _, kind := range []string{"project", "environment"} {
		t.Run(kind, func(t *testing.T) {
			svc, cl := newService(placementResource())
			failure := errors.New("interrupted write")
			tracking := &placementPatchClient{Client: cl, fail: failure}
			svc.Client = tracking
			before := readPlacement(t, cl)
			clear, expected := svc.ClearProjectID, "prj-a"
			if kind == "environment" {
				clear, expected = svc.ClearEnvironmentID, "env-a"
			}
			if err := clear(ctxAs("user-a"), "placement", expected); !errors.Is(err, failure) {
				t.Fatalf("error=%v", err)
			}
			if got := readPlacement(t, cl); !reflect.DeepEqual(got, before) {
				t.Fatal("failed cleanup partially changed the resource")
			}
			tracking.fail = nil
			if err := clear(ctxAs("user-a"), "placement", expected); err != nil {
				t.Fatal(err)
			}
			got := readPlacement(t, cl)
			if got.Labels[core.LabelEnvironment] != "" || len(got.Spec.EnvironmentIPAllowList) != 0 || !slices.Equal(got.Spec.IPAllowList, before.Spec.IPAllowList) {
				t.Fatalf("retry did not finish cleanup while preserving owned rules: %+v", got.Spec)
			}
		})
	}
}

func TestPlacementConcurrentReassignmentWinsOverStalePatch(t *testing.T) {
	for _, kind := range []string{"project", "environment"} {
		t.Run(kind, func(t *testing.T) {
			svc, cl := newService(placementResource())
			tracking := &placementPatchClient{Client: cl}
			tracking.beforePatch = func() {
				reassigned := readPlacement(t, cl)
				reassigned.Labels[core.LabelProject] = "prj-b"
				reassigned.Labels[core.LabelEnvironment] = "env-b"
				reassigned.Spec.EnvironmentIPAllowList = []string{"192.0.2.0/24"}
				if err := cl.Update(context.Background(), reassigned); err != nil {
					t.Fatal(err)
				}
			}
			svc.Client = tracking
			clear, expected := svc.ClearProjectID, "prj-a"
			if kind == "environment" {
				clear, expected = svc.ClearEnvironmentID, "env-a"
			}
			// The lost patch re-reads, sees the reassignment and leaves it alone
			// (w4/m214): a stale clear is a no-op, not an error.
			if err := clear(ctxAs("user-a"), "placement", expected); err != nil {
				t.Fatalf("stale cleanup error=%v, want a no-op after re-reading", err)
			}
			got := readPlacement(t, cl)
			if got.Labels[core.LabelProject] != "prj-b" || got.Labels[core.LabelEnvironment] != "env-b" || !slices.Equal(got.Spec.EnvironmentIPAllowList, []string{"192.0.2.0/24"}) {
				t.Fatalf("stale cleanup overwrote concurrent placement: labels=%v inherited=%v", got.Labels, got.Spec.EnvironmentIPAllowList)
			}
			if err := clear(ctxAs("user-a"), "placement", expected); err != nil {
				t.Fatal(err)
			}
			if tracking.patches != 1 || !reflect.DeepEqual(got, readPlacement(t, cl)) {
				t.Fatal("cleanup retry changed the reassignment")
			}
		})
	}
}

// TestPlacementConditionalClearRecoversFromLockConflicts is w4/m214's a698
// mechanism: an unrelated write between the clear's read and its patch (a
// status or annotation update) costs the patch its resourceVersion. The clear
// now re-reads and finishes in the same call, keeping that write; conflicts
// that never stop end in a public, explanatory conflict with placement intact;
// a resource replaced under the same name is never cleared; and a canceled
// request stops retrying with its own error.
func TestPlacementConditionalClearRecoversFromLockConflicts(t *testing.T) {
	unrelated := func(t *testing.T, cl client.Client) func() {
		return func() {
			updated := readPlacement(t, cl)
			if updated.Annotations == nil {
				updated.Annotations = map[string]string{}
			}
			updated.Annotations["unrelated-writes"] += "x"
			if err := cl.Update(context.Background(), updated); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, kind := range []string{"project", "environment"} {
		for _, scenario := range []string{"one unrelated update", "persistent updates", "replaced", "canceled"} {
			t.Run(kind+"/"+scenario, func(t *testing.T) {
				svc, cl := newService(placementResource())
				original := readPlacement(t, cl)
				original.UID = "uid-original"
				if err := cl.Delete(context.Background(), original); err != nil {
					t.Fatal(err)
				}
				original.ResourceVersion = ""
				if err := cl.Create(context.Background(), original); err != nil {
					t.Fatal(err)
				}
				before := readPlacement(t, cl)
				tracking := &placementPatchClient{Client: cl}
				svc.Client = tracking
				ctx, cancel := context.WithCancel(ctxAs("user-a"))
				defer cancel()
				switch scenario {
				case "one unrelated update":
					tracking.beforePatch = unrelated(t, cl)
				case "persistent updates":
					tracking.beforeEach = unrelated(t, cl)
				case "replaced":
					tracking.beforePatch = func() {
						replacement := readPlacement(t, cl)
						if err := cl.Delete(context.Background(), replacement); err != nil {
							t.Fatal(err)
						}
						replacement.UID, replacement.ResourceVersion = "uid-replacement", ""
						if err := cl.Create(context.Background(), replacement); err != nil {
							t.Fatal(err)
						}
						// A real apiserver's resourceVersions are global, so the
						// replacement never matches the stale read; the fake
						// restarts them per object, so move it on once.
						unrelated(t, cl)()
					}
				case "canceled":
					tracking.beforePatch = func() { unrelated(t, cl)(); cancel() }
				}
				clear, expected := svc.ClearProjectID, "prj-a"
				if kind == "environment" {
					clear, expected = svc.ClearEnvironmentID, "env-a"
				}
				err := clear(ctx, "placement", expected)
				got := readPlacement(t, cl)
				placementKept := got.Labels[core.LabelProject] == "prj-a" && got.Labels[core.LabelEnvironment] == "env-a" &&
					slices.Equal(got.Spec.EnvironmentIPAllowList, before.Spec.EnvironmentIPAllowList)
				var coded *core.CodedError
				switch scenario {
				case "one unrelated update":
					wantProject := "prj-a"
					if kind == "project" {
						wantProject = ""
					}
					if err != nil || tracking.patches != 2 || got.Annotations["unrelated-writes"] != "x" ||
						got.Labels[core.LabelProject] != wantProject || got.Labels[core.LabelEnvironment] != "" ||
						len(got.Spec.EnvironmentIPAllowList) != 0 || !slices.Equal(got.Spec.IPAllowList, before.Spec.IPAllowList) {
						t.Fatalf("err=%v patches=%d labels=%v annotations=%v spec=%+v, want one retried clear keeping the unrelated write",
							err, tracking.patches, got.Labels, got.Annotations, got.Spec)
					}
				case "persistent updates", "replaced":
					if !errors.Is(err, core.ErrConflict) || !errors.As(err, &coded) || coded.Code != core.CodePlacementCleanupConflict ||
						!strings.Contains(err.Error(), "reload the current placement") {
						t.Fatalf("err=%v, want the public placement-cleanup conflict", err)
					}
					if !placementKept {
						t.Fatalf("an unfinished clear changed placement: labels=%v", got.Labels)
					}
					if scenario == "replaced" && (tracking.patches != 1 || got.UID != "uid-replacement" || got.Annotations["unrelated-writes"] != "x") {
						t.Fatalf("patches=%d uid=%s, want the replacement untouched after one lost patch", tracking.patches, got.UID)
					}
				case "canceled":
					if !errors.Is(err, context.Canceled) || tracking.patches != 1 || !placementKept {
						t.Fatalf("err=%v patches=%d labels=%v, want the request's cancellation and no further attempt", err, tracking.patches, got.Labels)
					}
				}
			})
		}
	}
}

func TestPlacementInheritedRulesRequireExpectedEnvironment(t *testing.T) {
	for _, transition := range []string{"update", "unchanged", "project departure", "environment departure", "new environment"} {
		t.Run(transition, func(t *testing.T) {
			svc, cl := newService(placementResource())
			cidrs := []string{"192.0.2.0/24"}
			switch transition {
			case "unchanged":
				cidrs = []string{"198.51.100.0/24"}
			case "project departure":
				if err := svc.SetProjectID(ctxAs("user-a"), "placement", "prj-b"); err != nil {
					t.Fatal(err)
				}
			case "environment departure":
				if err := svc.SetEnvironmentID(ctxAs("user-a"), "placement", ""); err != nil {
					t.Fatal(err)
				}
			case "new environment":
				if err := svc.SetEnvironmentID(ctxAs("user-a"), "placement", "env-b"); err != nil {
					t.Fatal(err)
				}
				if err := svc.SetEnvironmentIPAllowList(ctxAs("user-a"), "placement", "env-b", []string{"203.0.113.1/32"}); err != nil {
					t.Fatal(err)
				}
			}
			before := readPlacement(t, cl)
			tracking := &placementPatchClient{Client: cl}
			svc.Client = tracking
			if err := svc.SetEnvironmentIPAllowList(ctxAs("user-a"), "placement", "env-a", cidrs); err != nil {
				t.Fatal(err)
			}
			got := readPlacement(t, cl)
			if transition == "update" {
				if tracking.patches != 1 || !slices.Equal(got.Spec.EnvironmentIPAllowList, cidrs) {
					t.Fatalf("matching fanout patches=%d inherited=%v", tracking.patches, got.Spec.EnvironmentIPAllowList)
				}
				// Only the inherited layer and update metadata may change.
				got.Spec.EnvironmentIPAllowList = before.Spec.EnvironmentIPAllowList
				if !reflect.DeepEqual(got.Spec, before.Spec) || !reflect.DeepEqual(got.Labels, before.Labels) {
					t.Fatal("fanout changed resource-owned configuration or membership")
				}
			} else if tracking.patches != 0 || !reflect.DeepEqual(got, before) {
				t.Fatalf("stale or unchanged fanout rewrote resource: patches=%d inherited=%v", tracking.patches, got.Spec.EnvironmentIPAllowList)
			}
		})
	}
}

func TestPlacementInheritedRulesAuthorizeBeforeNoOp(t *testing.T) {
	for _, expected := range []string{"env-a", "env-former"} {
		t.Run(expected, func(t *testing.T) {
			svc, cl := newService(placementResource())
			svc.Authz = &fakeChecker{allow: false}
			before := readPlacement(t, cl)
			if err := svc.SetEnvironmentIPAllowList(ctxAs("user-a"), "placement", expected, before.Spec.EnvironmentIPAllowList); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("denied fanout error=%v, want forbidden", err)
			}
			if !reflect.DeepEqual(readPlacement(t, cl), before) {
				t.Fatal("refused fanout changed resource")
			}
		})
	}
}

func TestPlacementInheritedRulesConflictWithConcurrentDeparture(t *testing.T) {
	for _, nextEnvironment := range []string{"", "env-b"} {
		t.Run("environment="+nextEnvironment, func(t *testing.T) {
			svc, cl := newService(placementResource())
			before := readPlacement(t, cl)
			tracking := &placementPatchClient{Client: cl}
			tracking.beforePatch = func() {
				reassigned := readPlacement(t, cl)
				reassigned.Labels[core.LabelProject] = "prj-b"
				delete(reassigned.Labels, core.LabelEnvironment)
				reassigned.Spec.EnvironmentIPAllowList = nil
				if nextEnvironment != "" {
					reassigned.Labels[core.LabelEnvironment] = nextEnvironment
					reassigned.Spec.EnvironmentIPAllowList = []string{"203.0.113.1/32"}
				}
				if err := cl.Update(context.Background(), reassigned); err != nil {
					t.Fatal(err)
				}
			}
			svc.Client = tracking
			oldLayer := []string{"192.0.2.0/24"}
			if err := svc.SetEnvironmentIPAllowList(ctxAs("user-a"), "placement", "env-a", oldLayer); !apierrors.IsConflict(err) {
				t.Fatalf("stale fanout error=%v, want resourceVersion conflict", err)
			}
			got := readPlacement(t, cl)
			var wantLayer []string
			if nextEnvironment != "" {
				wantLayer = []string{"203.0.113.1/32"}
			}
			if got.Labels[core.LabelProject] != "prj-b" || got.Labels[core.LabelEnvironment] != nextEnvironment || !slices.Equal(got.Spec.EnvironmentIPAllowList, wantLayer) || !slices.Equal(got.Spec.IPAllowList, before.Spec.IPAllowList) {
				t.Fatalf("stale fanout overwrote concurrent placement: labels=%v inherited=%v own=%v", got.Labels, got.Spec.EnvironmentIPAllowList, got.Spec.IPAllowList)
			}
			if err := svc.SetEnvironmentIPAllowList(ctxAs("user-a"), "placement", "env-a", oldLayer); err != nil {
				t.Fatal(err)
			}
			if tracking.patches != 1 || !reflect.DeepEqual(readPlacement(t, cl), got) {
				t.Fatal("stale fanout retry changed the reassignment")
			}
		})
	}
}
