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
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/webhooks"
)

// w2/m100 t001. TestRenderRouteIntersectionInventory walks Render's templates
// against bex's mux. Nothing walked the other way, which is how
// `GET /v1/webhooks/event-types` stayed registered, authz-classified,
// documented — and unreachable — for as long as it did: the strict validator
// matched it to Render's `GET /webhooks/{webhookId}`, the literal failed the
// `webhookId` id pattern, and the request died with 400 before bex's handler.
//
// This is the reverse guard. It fails on any bex literal route that a Render
// path parameter would swallow and that the pinned spec does not itself
// document, naming the offenders.

// muxRegistration matches a literal route registration:
//
//	mux.Handle("GET /v1/webhooks/event-types", …)
//	mux.HandleFunc("POST /v1/services", …)
//
// A pattern assembled from constants or fmt.Sprintf is invisible here. That is
// a known limit, not a silent one: such a route is also invisible to a reader
// grepping for it, and the composed-server tests remain the backstop.
var muxRegistration = regexp.MustCompile(`\.Handle(?:Func)?\(\s*"((?:[A-Z]+ )?/v1/[^"]*)"`)

func registeredBexRoutes(t *testing.T) []string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve internal/: %v", err)
	}
	seen := map[string]struct{}{}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range muxRegistration.FindAllStringSubmatch(string(source), -1) {
			seen[match[1]] = struct{}{}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal/: %v", err)
	}
	out := make([]string, 0, len(seen))
	for pattern := range seen {
		out = append(out, pattern)
	}
	sort.Strings(out)
	return out
}

func TestRegisteredRoutesAreNotShadowedByRenderParameters(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	routes := registeredBexRoutes(t)
	// The scan is the test's own foundation: if the regex stops matching, every
	// assertion below passes vacuously.
	if len(routes) < 100 {
		t.Fatalf("found only %d registered routes; the registration scan is broken", len(routes))
	}

	var shadowed []string
	for _, pattern := range routes {
		if isBexNativeLiteralRoute(contract, pattern) {
			shadowed = append(shadowed, pattern)
		}
	}
	// isBexNativeLiteralRoute is exactly the validator's own pass-through rule,
	// so a route it reports here is one the validator now lets through. The
	// assertion is that we know about every such route: each is a deliberate
	// bex-native extension wearing a Render-shaped path, not an accident.
	sort.Strings(shadowed)
	expected := []string{"GET /v1/webhooks/event-types"}
	if got, want := strings.Join(shadowed, ","), strings.Join(expected, ","); got != want {
		t.Fatalf("bex literal routes shadowed by a Render path parameter changed.\n got: %s\nwant: %s\n"+
			"A new entry means a newly-registered bex route collides with a Render template and is only\n"+
			"reachable because the validator's literal pass-through covers it — confirm that is intended,\n"+
			"then add it here.", got, want)
	}
}

// The pass-through must not weaken any genuine intersection: a bex pattern that
// is itself parameterised where Render is parameterised stays validated.
func TestLiteralPassThroughRuleIsNarrow(t *testing.T) {
	for _, tc := range []struct {
		name           string
		renderTemplate string
		bexPath        string
		want           bool
	}{
		{"the m100 case", "/webhooks/{webhookId}", "/webhooks/event-types", true},
		{"both parameterised is a real intersection", "/webhooks/{webhookId}", "/webhooks/{id}", false},
		{"identical literals are not a shadow", "/webhooks/event-types", "/webhooks/event-types", false},
		{"different literals do not match at all", "/webhooks/deliveries", "/webhooks/event-types", false},
		{"bex more general than Render is not a literal shadow", "/services/scale", "/services/{id}", false},
		{"arity must match", "/webhooks/{webhookId}", "/webhooks/event-types/extra", false},
		{"a literal after a shared parameter still counts", "/services/{id}/{action}", "/services/{id}/restart", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := shadowsBexLiteral(tc.renderTemplate, tc.bexPath); got != tc.want {
				t.Errorf("shadowsBexLiteral(%q, %q) = %v, want %v", tc.renderTemplate, tc.bexPath, got, tc.want)
			}
		})
	}
}

func TestBexPatternPath(t *testing.T) {
	for _, tc := range []struct {
		pattern string
		want    string
		ok      bool
	}{
		{"GET /v1/webhooks/event-types", "/webhooks/event-types", true},
		{"/v1/webhooks/event-types", "/webhooks/event-types", true},
		{"POST /v1/services", "/services", true},
		{"GET /healthz", "", false},
		{"", "", false},
	} {
		_, got, ok := bexPatternPath(tc.pattern)
		if got != tc.want || ok != tc.ok {
			t.Errorf("bexPatternPath(%q) = %q,%v; want %q,%v", tc.pattern, got, ok, tc.want, tc.ok)
		}
	}
}

// The behavioural half of w2/m100 t001: through the strict validator,
// `GET /v1/webhooks/event-types` must now reach its handler, while every real
// `{webhookId}` intersection stays validated exactly as before.
func TestEventTypesRouteSurvivesTheRenderValidator(t *testing.T) {
	mux := http.NewServeMux()
	(&webhooks.Service{Base: &core.Base{}}).RegisterREST(mux)
	h, err := newRenderRequestValidator(mux)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("the bex-native literal reaches its handler", func(t *testing.T) {
		for _, target := range []string{
			"/v1/webhooks/event-types",
			"/v1/webhooks/event-types?ownerId=tea-d98210cbbpdc73dcrkvg",
		} {
			w := requestOpenAPITest(t, h, http.MethodGet, target, "", "")
			if w.Code != http.StatusOK {
				t.Fatalf("%s status=%d, want 200: %s", target, w.Code, w.Body.String())
			}
			var got []string
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("%s body is not the event-type vocabulary: %v (%s)", target, err, w.Body.String())
			}
			if len(got) != len(webhooks.EventTypes) {
				t.Fatalf("%s returned %d event types, want %d", target, len(got), len(webhooks.EventTypes))
			}
		}
	})

	// The regression that matters most: loosening the validator must not stop
	// it validating the operation it was loosened around.
	t.Run("a well-formed webhook id is still validated and handled", func(t *testing.T) {
		w := requestOpenAPITest(t, h, http.MethodGet, "/v1/webhooks/whk-dajvu9ogsm7s73f64gc0", "", "")
		if w.Code == http.StatusBadRequest {
			t.Fatalf("a well-formed id was rejected by the validator: %s", w.Body.String())
		}
	})

	t.Run("a malformed webhook id is still a validator 400", func(t *testing.T) {
		w := requestOpenAPITest(t, h, http.MethodGet, "/v1/webhooks/not-an-id", "", "")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("malformed id status=%d, want the validator's 400: %s", w.Code, w.Body.String())
		}
	})
}
