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
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// redwoodjsShapedBlueprint is the static half of Render's render-examples/
// redwoodjs template, in its own words: the first rewrite ships a placeholder
// destination its author replaces "after first deploy to Render", and the
// next one is the single-page app's fallback (w33/007).
const redwoodjsShapedBlueprint = `
services:
  - name: redwood-web
    type: web
    env: static
    repo: https://github.com/acme/redwood
    buildCommand: yarn rw deploy render web
    staticPublishPath: ./web/dist
    routes:
      - type: rewrite
        source: /.redwood/functions/*
        destination: replace_with_api_url/*
      - type: rewrite
        source: /*
        destination: /index.html
`

var redwoodjsShapedRoutes = []appv1alpha1.StaticRoute{
	{Type: "rewrite", Source: "/.redwood/functions/*", Destination: "replace_with_api_url/*"},
	{Type: "rewrite", Source: "/*", Destination: "/index.html"},
}

// Render documents a destination as "either a path or a full, publicly
// accessible URL" and its own template deploys one that is neither rooted nor
// a URL, so bex accepts any RFC 3986 relative reference too (w33/007).
func TestValidateRoutesAcceptsAnUnrootedDestination(t *testing.T) {
	for _, r := range []StaticRouteView{
		{Type: "rewrite", Source: "/.redwood/functions/*", Destination: "replace_with_api_url/*"},
		{Type: "redirect", Source: "/.redwood/functions/*", Destination: "replace_with_api_url/*"},
		{Type: "rewrite", Source: "/*", Destination: "index.html"},
		{Type: "rewrite", Source: "/docs/*", Destination: "docs/:splat"},
		{Type: "redirect", Source: "/u/:user", Destination: "people/:user?from=old#top"},
		{Type: "redirect", Source: "/a", Destination: "v1/items:"},
		{Type: "redirect", Source: "/a", Destination: "v1/a:1"},
		{Type: "redirect", Source: "/a", Destination: "?q=1"},
		{Type: "redirect", Source: "/a", Destination: " placeholder "},
	} {
		if err := validateRoutes([]StaticRouteView{r}); err != nil {
			t.Errorf("%+v refused: %v", r, err)
		}
	}
}

// Every other destination refusal holds for an unrooted destination: another
// scheme, a network-path reference, a backslash or control byte (round-5
// finding 11), a malformed escape, the length budget and an undeclared :name.
func TestValidateRoutesKeepsEveryOtherDestinationRefusal(t *testing.T) {
	for _, tc := range []struct {
		dest, want string
	}{
		{"", "destination is required"},
		{"   ", "destination is required"},
		{"javascript:alert(1)", "must be a path or an http(s) URL"},
		{"JaVaScRiPt:alert(1)", "must be a path or an http(s) URL"},
		{"mailto:ops@example.com", "must be a path or an http(s) URL"},
		{"data:text/html,<p>x</p>", "must be a path or an http(s) URL"},
		{"ftp://host.example/x", "must be a path or an http(s) URL"},
		{"http:/host.example/x", "must be a path or an http(s) URL"},
		{"replace_with_api_url:8080/*", "must be a path or an http(s) URL"},
		{":splat", "must be a path or an http(s) URL"},
		{`\\evil.example/x`, "network-path reference"},
		{`api\x`, "network-path reference"},
		{"api\r\nx", "must be a valid local URL"},
		{"api\x7fx", "must be a valid local URL"},
		{"%2F%2Fevil.example/x", "network-path reference"},
		{"api%5cx", "network-path reference"},
		{"api%00x", "network-path reference"},
		{"api%zz", "must be a valid local URL"},
		{"api?x=%zz", "must be a valid local URL"},
		{strings.Repeat("a", maxStaticPathLen+1), "must not exceed 2048 bytes"},
		{"people/:nobody", "references :nobody"},
	} {
		err := validateRoutes([]StaticRouteView{{Type: "redirect", Source: "/u/:user", Destination: tc.dest}})
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "routes[0].destination") || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("destination %q => %v, want a 400 on routes[0].destination saying %q", tc.dest, err, tc.want)
		}
	}
}

// The redwoodjs shape validates, and a Blueprint sync lands both rules on the
// static site exactly as written.
func TestRedwoodjsShapedBlueprintValidatesAndApplies(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	v, err := svc.ValidateBlueprint(context.Background(), "", redwoodjsShapedBlueprint, "")
	if err != nil || !v.Valid {
		t.Fatalf("validate = %+v, %v; want valid", v, err)
	}

	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/acme/redwood", Branch: "main",
		Path: CanonicalBlueprintFilename, Manifest: redwoodjsShapedBlueprint, Status: "active", Name: "redwood",
	})
	cl := fakeClient()
	svc = &Service{
		Base:       &core.Base{Client: cl, Namespace: "default", Workspace: ws},
		Blueprints: fs,
		GitFetcher: fakeBlueprintFetcher{contents: redwoodjsShapedBlueprint},
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})
	if _, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", "", "", nil); err != nil {
		t.Fatalf("SyncBlueprint: %v", err)
	}
	a := getTenantApp(t, cl, "tea-a", "redwood-web")
	if a.Spec.Type != appv1alpha1.TypeStaticSite || !slices.Equal(a.Spec.Routes, redwoodjsShapedRoutes) {
		t.Fatalf("app %s routes = %+v, want the static site with %+v", a.Spec.Type, a.Spec.Routes, redwoodjsShapedRoutes)
	}
}

// REST set-routes and add-route store the placeholder as written, and
// add-route still refuses another scheme.
func TestRESTRoutesTakeAnUnrootedDestination(t *testing.T) {
	do := staticRulesREST(t)
	body, _ := json.Marshal(redwoodjsShapedRoutes)
	if rec := do(http.MethodPut, "/routes", string(body)); rec.Code != http.StatusOK {
		t.Fatalf("set-routes = %d %s, want 200", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodPost, "/routes", `{"type":"redirect","source":"/old/*","destination":"new/*","priority":0}`); rec.Code != http.StatusCreated {
		t.Fatalf("add-route = %d %s, want 201", rec.Code, rec.Body.String())
	}
	var got []string
	for _, r := range listedRoutes(t, do) {
		got = append(got, r.Type+" "+r.Source+" "+r.Destination)
	}
	want := []string{"redirect /old/* new/*", "rewrite /.redwood/functions/* replace_with_api_url/*", "rewrite /* /index.html"}
	if !slices.Equal(got, want) {
		t.Fatalf("routes = %q, want %q", got, want)
	}
	rec := do(http.MethodPost, "/routes", `{"type":"redirect","source":"/x","destination":"javascript:alert(1)"}`)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "destination must be a path or an http(s) URL") {
		t.Fatalf("add-route javascript: = %d %s, want a 400 naming the destination", rec.Code, rec.Body.String())
	}
}
