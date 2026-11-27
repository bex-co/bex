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
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's Blueprint spec, § routes and § headers: "You can modify existing
// routing rules and add new ones. Render preserves any existing routing rules
// that are not included in the Blueprint file." A sync used to replace the
// whole list, so a rule added in the dashboard or over the API was deleted by
// the next sync of a file that declared any rule (w4/m697).

const staticRulesSite = "services:\n  - name: site\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/site\n    staticPublishPath: dist\n"

// syncStaticRules applies a static-site file to live as the sync does and
// reports what it changed.
func syncStaticRules(t *testing.T, live appv1alpha1.AppSpec, rules string) (appv1alpha1.AppSpec, bool) {
	t.Helper()
	svc := blueprintPlanService(t, staticRulesSite+rules)
	app := &appv1alpha1.App{Spec: live}
	dst := live.DeepCopy()
	changed, err := ApplyBlueprintServiceSpec(dst, blueprintPlanWant(t, svc, app), svc.fields)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	return *dst, changed
}

// staticRulesLive is the site staticRulesSite describes, as its sync leaves
// it, carrying the given rules: only its rules can differ from a file.
func staticRulesLive(t *testing.T, routes []appv1alpha1.StaticRoute, headers []appv1alpha1.StaticHeader) appv1alpha1.AppSpec {
	t.Helper()
	live := blueprintPlanWant(t, blueprintPlanService(t, staticRulesSite), &appv1alpha1.App{})
	live.Routes, live.Headers = routes, headers
	return live
}

func route(source, destination string) appv1alpha1.StaticRoute {
	return appv1alpha1.StaticRoute{Type: "redirect", Source: source, Destination: destination}
}

func header(path, name, value string) appv1alpha1.StaticHeader {
	return appv1alpha1.StaticHeader{Path: path, Name: name, Value: value}
}

func TestBlueprintRoutesKeepTheRulesAFileDoesNotName(t *testing.T) {
	t.Parallel()
	live := staticRulesLive(t, []appv1alpha1.StaticRoute{route("/a", "/1"), route("/b", "/2")}, nil)
	for _, tc := range []struct {
		name, rules string
		want        []appv1alpha1.StaticRoute
		changed     bool
	}{
		{"a changed rule stays where it is and a new one goes last",
			"    routes:\n      - {type: redirect, source: /b, destination: /20}\n      - {type: redirect, source: /c, destination: /3}\n",
			[]appv1alpha1.StaticRoute{route("/a", "/1"), route("/b", "/20"), route("/c", "/3")}, true},
		{"an empty list deletes nothing", "    routes: []\n", live.Routes, false},
		{"one unchanged rule of two is no change", "    routes:\n      - {type: redirect, source: /a, destination: /1}\n", live.Routes, false},
		// The static server answers with the first rule whose source matches,
		// so a second rule beside it would never run.
		{"a rule whose type changes is the same rule",
			"    routes:\n      - {type: rewrite, source: /a, destination: /index.html}\n",
			[]appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/a", Destination: "/index.html"}, route("/b", "/2")}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := syncStaticRules(t, live, tc.rules)
			if !reflect.DeepEqual(got.Routes, tc.want) || changed != tc.changed {
				t.Fatalf("routes = %+v changed %v, want %+v changed %v", got.Routes, changed, tc.want, tc.changed)
			}
			if _, again := syncStaticRules(t, got, tc.rules); again {
				t.Fatal("a second sync of the same file changed the routes again")
			}
		})
	}
}

func TestBlueprintHeadersKeepTheRulesAFileDoesNotName(t *testing.T) {
	t.Parallel()
	live := staticRulesLive(t, nil, []appv1alpha1.StaticHeader{
		header("/*", "X-Frame-Options", "DENY"),
		header("/blog/*", "Cache-Control", "no-cache"),
	})
	for _, tc := range []struct {
		name, rules string
		want        []appv1alpha1.StaticHeader
		changed     bool
	}{
		{"a changed rule stays where it is and a new one goes last",
			"    headers:\n      - {path: /blog/*, name: Cache-Control, value: must-revalidate}\n      - {path: /*, name: X-Robots-Tag, value: noindex}\n",
			[]appv1alpha1.StaticHeader{
				header("/*", "X-Frame-Options", "DENY"),
				header("/blog/*", "Cache-Control", "must-revalidate"),
				header("/*", "X-Robots-Tag", "noindex"),
			}, true},
		{"an empty list deletes nothing", "    headers: []\n", live.Headers, false},
		{"one unchanged rule of two is no change", "    headers:\n      - {path: /*, name: X-Frame-Options, value: DENY}\n", live.Headers, false},
		// The static server sets a header by its canonical name, so the two
		// spellings are one header.
		{"a header named in another case is the same rule",
			"    headers:\n      - {path: /*, name: x-frame-options, value: SAMEORIGIN}\n",
			[]appv1alpha1.StaticHeader{header("/*", "x-frame-options", "SAMEORIGIN"), header("/blog/*", "Cache-Control", "no-cache")}, true},
		{"the same header on another path is another rule",
			"    headers:\n      - {path: /assets/*, name: Cache-Control, value: immutable}\n",
			append(slices.Clone(live.Headers), header("/assets/*", "Cache-Control", "immutable")), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := syncStaticRules(t, live, tc.rules)
			if !reflect.DeepEqual(got.Headers, tc.want) || changed != tc.changed {
				t.Fatalf("headers = %+v changed %v, want %+v changed %v", got.Headers, changed, tc.want, tc.changed)
			}
			if _, again := syncStaticRules(t, got, tc.rules); again {
				t.Fatal("a second sync of the same file changed the headers again")
			}
		})
	}
}

// A file that names a rule twice takes the live rules with that key in order,
// so its second sync is no change rather than a rewrite of the first slot.
func TestMergeBlueprintRulesRepeatedKey(t *testing.T) {
	t.Parallel()
	live := []appv1alpha1.StaticRoute{route("/a", "/1"), route("/x", "/9"), route("/a", "/2")}
	declared := []appv1alpha1.StaticRoute{route("/a", "/10"), route("/a", "/20"), route("/a", "/30")}
	got := mergeBlueprintRules(live, declared, blueprintRouteKey)
	want := []appv1alpha1.StaticRoute{route("/a", "/10"), route("/x", "/9"), route("/a", "/20"), route("/a", "/30")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merge = %+v, want %+v", got, want)
	}
	if again := mergeBlueprintRules(got, declared, blueprintRouteKey); !reflect.DeepEqual(again, got) {
		t.Fatalf("second merge = %+v, want %+v", again, got)
	}
	if got := mergeBlueprintRules[appv1alpha1.StaticRoute](nil, []appv1alpha1.StaticRoute{}, blueprintRouteKey); got != nil {
		t.Fatalf("merge of nothing = %#v, want nil", got)
	}
}

// The rules a file does not name still count against the site's edge budget:
// a sync that would pass it is refused at the field before anything is
// written, and a file that only re-declares live rules still syncs.
func TestBlueprintRulesRespectTheEdgeBudget(t *testing.T) {
	t.Parallel()
	full := staticRulesLive(t, nil, nil)
	for i := range maxStaticRoutes {
		full.Routes = append(full.Routes, route(fmt.Sprintf("/r%d", i), "/"))
	}
	for i := range maxStaticHeaders {
		full.Headers = append(full.Headers, header(fmt.Sprintf("/h%d", i), "X-A", "b"))
	}
	for _, tc := range []struct{ field, rules string }{
		{"routes", "    routes:\n      - {type: redirect, source: /new, destination: /}\n"},
		{"headers", "    headers:\n      - {path: /new, name: X-A, value: b}\n"},
	} {
		t.Run(tc.field, func(t *testing.T) {
			svc := blueprintPlanService(t, staticRulesSite+tc.rules)
			dst := full.DeepCopy()
			_, err := ApplyBlueprintServiceSpec(dst, blueprintPlanWant(t, svc, &appv1alpha1.App{Spec: full}), svc.fields)
			var conflict *BlueprintFieldConflictError
			if !errors.As(err, &conflict) || conflict.Path != tc.field || !strings.Contains(conflict.Message, "more than the 100") {
				t.Fatalf("error = %v, want a %s conflict naming the budget", err, tc.field)
			}
			if !reflect.DeepEqual(*dst, full) {
				t.Fatal("a refused sync still changed the spec")
			}
		})
	}
	unchanged := "    routes:\n      - {type: redirect, source: /r0, destination: /}\n    headers:\n      - {path: /h0, name: X-A, value: b}\n"
	if _, changed := syncStaticRules(t, full, unchanged); changed {
		t.Fatal("re-declaring a live rule on a full site changed it")
	}
}

// Validate locates the budget refusal at the declared list, and the sync
// refuses it the same way, leaving the site's rules as they were.
func TestBlueprintRuleBudgetRefusalIsLocated(t *testing.T) {
	svc, cl := newService(nil)
	ctx := context.Background()
	manifest := staticRulesSite + "    routes:\n      - {type: redirect, source: /new, destination: /}\n"
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: staticRulesSite}); err != nil {
		t.Fatal(err)
	}
	var full []StaticRouteView
	for i := range maxStaticRoutes {
		full = append(full, StaticRouteView{Type: "redirect", Source: fmt.Sprintf("/r%d", i), Destination: "/"})
	}
	if _, err := svc.SetRoutes(ctx, "site", full); err != nil {
		t.Fatal(err)
	}
	validation, err := svc.ValidateBlueprint(ctx, "", manifest, "")
	if err != nil || validation.Valid || len(validation.Errors) != 1 {
		t.Fatalf("validation = %+v err %v, want one refusal", validation, err)
	}
	if refusal := validation.Errors[0]; refusal.Path == nil || *refusal.Path != "services[0].routes" || !strings.Contains(refusal.Error, "more than the 100") {
		t.Fatalf("refusal = %+v at %v, want the budget at services[0].routes", refusal, refusal.Path)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err == nil || !strings.Contains(err.Error(), "more than the 100") {
		t.Fatalf("sync err = %v, want the budget refusal", err)
	}
	if got := len(getApp(t, cl, "site").Spec.Routes); got != maxStaticRoutes {
		t.Fatalf("a refused sync left %d routes, want %d", got, maxStaticRoutes)
	}
}

// The plan every surface serves compares the merged lists: a file naming
// one of a site's three rules, unchanged, is no change on REST, GraphQL and
// MCP, and a file adding one rule names only the list it lands in. The sync
// then keeps the rules the file does not name.
func TestBlueprintStaticRulesPlanAndSyncAcrossSurfaces(t *testing.T) {
	manifest := staticRulesSite +
		"    routes:\n      - {type: redirect, source: /a, destination: /1}\n" +
		"    headers:\n      - {path: /*, name: X-Frame-Options, value: DENY}\n"
	svc, cl := newService(nil)
	ctx := context.Background()
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
		t.Fatal(err)
	}
	// Rules added in the dashboard or over the API.
	if _, err := svc.SetRoutes(ctx, "site", []StaticRouteView{
		{Type: "redirect", Source: "/a", Destination: "/1"},
		{Type: "redirect", Source: "/b", Destination: "/2"},
		{Type: "rewrite", Source: "/c/*", Destination: "/index.html"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddHeader(ctx, "site", StaticHeaderView{Path: "/blog/*", Name: "Cache-Control", Value: "no-cache"}); err != nil {
		t.Fatal(err)
	}
	svc.Client = fakeClient(serializedBlueprintApp(t, getApp(t, cl, "site")))

	for _, surface := range blueprintValidationSurfaces(t, svc) {
		t.Run(surface.name, func(t *testing.T) {
			for _, tc := range []struct {
				name, yaml, operation string
				paths                 []string
			}{
				{"one rule of three, unchanged", manifest, "noop", nil},
				{"empty lists", staticRulesSite + "    routes: []\n    headers: []\n", "noop", nil},
				{"a new route", strings.Replace(manifest, "destination: /1}\n", "destination: /1}\n      - {type: redirect, source: /d, destination: /4}\n", 1), "update", []string{"routes"}},
				{"a changed header", strings.Replace(manifest, "value: DENY", "value: SAMEORIGIN", 1), "update", []string{"headers"}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					out := surface.validate(t, tc.yaml)
					if out["valid"] != true {
						t.Fatalf("invalid: %+v", out)
					}
					actions, _ := out["plan"].(map[string]any)["actions"].([]any)
					if len(actions) != 1 {
						t.Fatalf("actions = %+v", actions)
					}
					action := actions[0].(map[string]any)
					if action["operation"] != tc.operation {
						t.Fatalf("action = %+v, want %s", action, tc.operation)
					}
					if paths := actionPaths(t, surface.name, action); !slices.Equal(paths, tc.paths) {
						t.Fatalf("changed paths = %v, want %v", paths, tc.paths)
					}
				})
			}
		})
	}

	synced := staticRulesSite +
		"    routes:\n      - {type: redirect, source: /a, destination: /10}\n      - {type: redirect, source: /d, destination: /4}\n" +
		"    headers:\n      - {path: /*, name: X-Frame-Options, value: SAMEORIGIN}\n"
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: synced}); err != nil {
		t.Fatal(err)
	}
	got := getApp(t, svc.Client, "site").Spec
	wantRoutes := []appv1alpha1.StaticRoute{
		route("/a", "/10"), route("/b", "/2"), {Type: "rewrite", Source: "/c/*", Destination: "/index.html"}, route("/d", "/4"),
	}
	wantHeaders := []appv1alpha1.StaticHeader{header("/*", "X-Frame-Options", "SAMEORIGIN"), header("/blog/*", "Cache-Control", "no-cache")}
	if !reflect.DeepEqual(got.Routes, wantRoutes) || !reflect.DeepEqual(got.Headers, wantHeaders) {
		t.Fatalf("after the sync routes = %+v headers = %+v, want %+v and %+v", got.Routes, got.Headers, wantRoutes, wantHeaders)
	}
}
