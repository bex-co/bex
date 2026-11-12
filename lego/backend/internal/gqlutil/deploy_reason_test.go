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

package gqlutil

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/566: a known sentence resolves to its code and parameters; any other,
// including none, to null.
func TestDeployReasonFieldNamesKnownSentences(t *testing.T) {
	field := DeployReasonField(func(s string) string { return s })
	resolve := func(text string) any {
		v, err := field.Resolve(graphql.ResolveParams{Source: text})
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	want := deployReason{code: appv1alpha1.DeployReasonHTTPPortDetected, params: []appv1alpha1.DeployReasonParam{{Name: "port", Value: "80"}}}
	if got := resolve("Detected HTTP on port 80; serving it"); !reflect.DeepEqual(got, want) {
		t.Fatalf("detected = %#v, want %#v", got, want)
	}
	if got := resolve("Detecting the container's HTTP port"); !reflect.DeepEqual(got, deployReason{code: appv1alpha1.DeployReasonHTTPPortDetecting, params: []appv1alpha1.DeployReasonParam{}}) {
		t.Fatalf("detecting = %#v", got)
	}
	for _, text := range []string{"", "something the operator never words"} {
		if got := resolve(text); got != nil {
			t.Fatalf("%q = %#v, want null", text, got)
		}
	}
}

var templateParam = regexp.MustCompile(`\{([a-z]+)\}`)

// Every template formats and parses back to its code and parameters, so the
// dashboard can word any of them; a sentence none matches stays English.
func TestDeployReasonsRoundTrip(t *testing.T) {
	// By name, the order the parsed parameters are sorted in.
	params := []appv1alpha1.DeployReasonParam{
		{Name: "cause", Value: "Waiting for build capacity: 4 of 4 builds are running"},
		{Name: "code", Value: "137"}, {Name: "deploy", Value: "dep-d1a2b3c4d5e6f7g8h9i0"},
		{Name: "hint", Value: " No open port was detected on the instance."},
		{Name: "host", Value: "ghcr.io"}, {Name: "image", Value: "ghcr.io/acme/web:v1"},
		{Name: "kind", Value: "readiness"}, {Name: "minutes", Value: "10"}, {Name: "path", Value: "/healthz"},
		{Name: "port", Value: "8080"}, {Name: "restarts", Value: "3"}, {Name: "status", Value: "403"},
		{Name: "wait", Value: "3h0m0s"},
	}
	for code, tmpl := range appv1alpha1.DeployReasonTemplates() {
		var want []appv1alpha1.DeployReasonParam
		for _, p := range params {
			if slices.Contains(templateParam.FindAllString(tmpl, -1), "{"+p.Name+"}") {
				want = append(want, p)
			}
		}
		text := appv1alpha1.FormatDeployReason(code, params...)
		got, gotParams, ok := appv1alpha1.ParseDeployReason(text)
		slices.SortFunc(gotParams, func(a, b appv1alpha1.DeployReasonParam) int { return strings.Compare(a.Name, b.Name) })
		if !ok || got != code || !slices.Equal(gotParams, want) {
			t.Errorf("%s: %q parses as %s %v (ok=%v), want %v", code, text, got, gotParams, ok, want)
		}
	}
	if _, _, ok := appv1alpha1.ParseDeployReason("a sentence no template words"); ok {
		t.Error("an unknown sentence parsed")
	}
}

// w4/591: a sentence two templates match reads as the one that fixes more of
// it, never the one whose parameter swallows the difference.
func TestDeployReasonPrefersTheMoreSpecificTemplate(t *testing.T) {
	restarted := appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonProbeHTTPWaitingRestarted,
		appv1alpha1.DeployReasonParam{Name: "kind", Value: "startup"}, appv1alpha1.DeployReasonParam{Name: "path", Value: "/"},
		appv1alpha1.DeployReasonParam{Name: "port", Value: "80"}, appv1alpha1.DeployReasonParam{Name: "restarts", Value: "2"})
	for text, want := range map[string]string{
		"Superseded by a newer release":      appv1alpha1.DeployReasonSupersededByNewer,
		"Superseded by dep-abc":              appv1alpha1.DeployReasonSupersededBy,
		"Canceled along with a newer deploy": appv1alpha1.DeployReasonCanceledAlongNewer,
		"Canceled along with dep-abc":        appv1alpha1.DeployReasonCanceledAlongWith,
		restarted:                            appv1alpha1.DeployReasonProbeHTTPWaitingRestarted,
	} {
		if got, _, _ := appv1alpha1.ParseDeployReason(text); got != want {
			t.Errorf("%q parses as %s, want %s", text, got, want)
		}
	}
}

// The dashboard words each code in en exactly as its template does, so an en
// reader sees the sentence every API returns. It reads the dashboard's own
// catalog, so the two cannot drift.
func TestDashboardWordsEveryDeployReason(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var data []byte
	for {
		data, err = os.ReadFile(filepath.Join(dir, "dashboard", "src", "features", "deploys", "locales", "en.ts"))
		if err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("dashboard/src/features/deploys/locales/en.ts is not in this checkout")
		}
		dir = parent
	}
	for code, tmpl := range appv1alpha1.DeployReasonTemplates() {
		entry := regexp.MustCompile(`"deploys\.reason\.` + regexp.QuoteMeta(code) + `": \{\s*message:\s*"((?:[^"\\]|\\.)*)"`)
		m := entry.FindSubmatch(data)
		if m == nil {
			t.Errorf("the dashboard does not word %s", code)
			continue
		}
		if got := strings.ReplaceAll(string(m[1]), `\"`, `"`); got != tmpl {
			t.Errorf("%s: the dashboard says %q, the API %q", code, got, tmpl)
		}
	}
}
