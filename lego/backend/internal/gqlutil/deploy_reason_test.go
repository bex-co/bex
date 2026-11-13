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
	"encoding/json"
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
	detected := "Detected HTTP on port 80; serving it"
	want := deployReason{code: appv1alpha1.DeployReasonHTTPPortDetected, params: []appv1alpha1.DeployReasonParam{{Name: "port", Value: "80"}},
		parts: []deployReason{}, text: detected}
	if got := resolve(detected); !reflect.DeepEqual(got, want) {
		t.Fatalf("detected = %#v, want %#v", got, want)
	}
	detecting := "Detecting the container's HTTP port"
	if got := resolve(detecting); !reflect.DeepEqual(got, deployReason{code: appv1alpha1.DeployReasonHTTPPortDetecting,
		params: []appv1alpha1.DeployReasonParam{}, parts: []deployReason{}, text: detecting}) {
		t.Fatalf("detecting = %#v", got)
	}
	for _, text := range []string{"", "something the operator never words"} {
		if got := resolve(text); got != nil {
			t.Fatalf("%q = %#v, want null", text, got)
		}
	}
}

// w5/m356: GraphQL reads a stall's port hints and a never-started build's
// cause as parts, each with its own code and English, and returns the tail no
// template words as the remainder.
func TestDeployReasonFieldReadsATailAsParts(t *testing.T) {
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{"reason": {
			Type: DeployReasonType,
			Args: graphql.FieldConfigArgument{"text": ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return DeployReasonField(func(s string) string { return s }).Resolve(graphql.ResolveParams{Source: p.Args["text"]})
			},
		}},
	})})
	if err != nil {
		t.Fatal(err)
	}
	type part struct {
		Code   string `json:"code"`
		Params []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"params"`
		Text  string `json:"text"`
		Parts []any  `json:"parts"`
	}
	read := func(text string) (code string, parts []part, remainder string) {
		t.Helper()
		res := graphql.Do(graphql.Params{Schema: schema, VariableValues: map[string]any{"text": text},
			RequestString: `query($text: String!) { reason(text: $text) { code text remainder parts { code text params { name value } parts { code } remainder } } }`})
		if len(res.Errors) > 0 {
			t.Fatalf("%q: %v", text, res.Errors)
		}
		raw, _ := json.Marshal(res.Data)
		var data struct {
			Reason struct {
				Code, Text, Remainder string
				Parts                 []part
			}
		}
		if err := json.Unmarshal(raw, &data); err != nil {
			t.Fatal(err)
		}
		if data.Reason.Text != text {
			t.Errorf("text = %q, want %q", data.Reason.Text, text)
		}
		return data.Reason.Code, data.Reason.Parts, data.Reason.Remainder
	}

	stall := appv1alpha1.ProbeStallReason("startup", true, "", "10000", 0)
	noOpen := appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintNoOpenPort, appv1alpha1.DeployReasonParam{Name: "port", Value: "10000"})
	worker := appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintBackgroundWorker)
	code, parts, remainder := read(stall + noOpen + worker)
	if code != appv1alpha1.DeployReasonProbeTCPWaiting || remainder != "" || len(parts) != 2 ||
		parts[0].Code != appv1alpha1.DeployReasonHintNoOpenPort || parts[0].Text != noOpen ||
		len(parts[0].Params) != 1 || parts[0].Params[0].Value != "10000" || len(parts[0].Parts) != 0 ||
		parts[1].Code != appv1alpha1.DeployReasonHintBackgroundWorker || parts[1].Text != worker {
		t.Errorf("a stall with two hints = %s %+v remainder %q", code, parts, remainder)
	}

	// A sentence without a tail has no parts.
	if code, parts, remainder := read(stall); code != appv1alpha1.DeployReasonProbeTCPWaiting || len(parts) != 0 || remainder != "" {
		t.Errorf("a stall without a hint = %s %+v remainder %q", code, parts, remainder)
	}

	cause := "workspace has 2/2 concurrent builds active; waiting for a slot"
	if code, parts, remainder := read("the build never started: " + cause); code != appv1alpha1.DeployReasonBuildNeverStarted ||
		len(parts) != 1 || parts[0].Code != appv1alpha1.DeployReasonQueuedBuildLimit || parts[0].Text != cause || remainder != "" {
		t.Errorf("a queued cause = %s %+v remainder %q", code, parts, remainder)
	}

	// Kubernetes' own verdict in an older row's capacity wait stays English.
	verdict := "0/7 nodes are available: 7 Insufficient cpu"
	if _, parts, remainder := read("the build never started: waiting for build capacity: " + verdict); len(parts) != 1 ||
		parts[0].Code != appv1alpha1.DeployReasonQueuedCapacity || parts[0].Params[0].Value != verdict || remainder != "" {
		t.Errorf("a raw capacity verdict = %+v remainder %q", parts, remainder)
	}
	unknown := "a wait no template words"
	if _, parts, remainder := read("the build never started: " + unknown); len(parts) != 0 || remainder != unknown {
		t.Errorf("an unknown cause = %+v remainder %q", parts, remainder)
	}
}

var templateParam = regexp.MustCompile(`\{([a-z]+)\}`)

// Every template formats and parses back to its code and parameters, a part
// off a tail and never as a reason of its own, so the dashboard can word any
// of them; a sentence none matches stays English.
func TestDeployReasonsRoundTrip(t *testing.T) {
	// By name, the order the parsed parameters are sorted in.
	params := []appv1alpha1.DeployReasonParam{
		{Name: "active", Value: "2"},
		{Name: "cause", Value: "Waiting for build capacity: 4 of 4 builds are running"},
		{Name: "code", Value: "137"}, {Name: "database", Value: "dpg-d1a2b3c4d5e6f7g8h9i0"}, {Name: "deploy", Value: "dep-d1a2b3c4d5e6f7g8h9i0"},
		{Name: "detail", Value: "0/7 nodes are available: 7 Insufficient cpu"},
		{Name: "hint", Value: " No open port was detected on the instance."},
		{Name: "host", Value: "ghcr.io"}, {Name: "image", Value: "ghcr.io/acme/web:v1"},
		{Name: "kind", Value: "readiness"}, {Name: "limit", Value: "3"}, {Name: "minutes", Value: "10"},
		{Name: "noun", Value: "workspace"}, {Name: "path", Value: "/healthz"},
		{Name: "port", Value: "8080"}, {Name: "ports", Value: "80, 9000, 9009"}, {Name: "previous", Value: "3000"},
		{Name: "ref", Value: "g3-1a2b3c4d"}, {Name: "resources", Value: "cpu, ephemeral-storage"},
		{Name: "restarts", Value: "3"}, {Name: "status", Value: "403"},
		{Name: "wait", Value: "3h0m0s"},
	}
	parts := appv1alpha1.DeployReasonPartTemplates()
	for code, tmpl := range appv1alpha1.DeployReasonTemplates() {
		var want []appv1alpha1.DeployReasonParam
		for _, p := range params {
			if slices.Contains(templateParam.FindAllString(tmpl, -1), "{"+p.Name+"}") {
				want = append(want, p)
			}
		}
		text := appv1alpha1.FormatDeployReason(code, params...)
		var (
			got       string
			gotParams []appv1alpha1.DeployReasonParam
			ok        bool
		)
		if _, part := parts[code]; part {
			if whole, _, ok := appv1alpha1.ParseDeployReason(text); ok {
				t.Errorf("%s: %q parses as the reason %s", code, text, whole)
			}
			read, rest := appv1alpha1.ParseDeployReasonParts(text)
			if ok = len(read) == 1 && rest == "" && read[0].Text == text; ok {
				got, gotParams = read[0].Code, read[0].Params
			}
		} else {
			got, gotParams, ok = appv1alpha1.ParseDeployReason(text)
		}
		slices.SortFunc(gotParams, func(a, b appv1alpha1.DeployReasonParam) int { return strings.Compare(a.Name, b.Name) })
		if !ok || got != code || !slices.Equal(gotParams, want) {
			t.Errorf("%s: %q parses as %s %v (ok=%v), want %v", code, text, got, gotParams, ok, want)
		}
	}
	if _, _, ok := appv1alpha1.ParseDeployReason("a sentence no template words"); ok {
		t.Error("an unknown sentence parsed")
	}
}

// w5/m356: a tail reads as its sentences in order; from the first text no
// part template words, the rest is the remainder, unchanged.
func TestDeployReasonPartsReadATailInOrder(t *testing.T) {
	port := func(name, value string) appv1alpha1.DeployReasonParam {
		return appv1alpha1.DeployReasonParam{Name: name, Value: value}
	}
	declared := appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintImageDeclaredPorts, port("port", "8123"), port("ports", "8123, 9000"))
	worker := appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintBackgroundWorker)
	listening := appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintListening, port("port", "9000"))
	adopted := appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintListeningPrimary, port("port", "9000"))
	unknown := " A sentence bex never words. Nor this one."
	codes := func(parts []appv1alpha1.DeployReasonPart) []string {
		out := make([]string, len(parts))
		for i, p := range parts {
			out[i] = p.Code
		}
		return out
	}
	for name, tc := range map[string]struct {
		tail, remainder string
		want            []string
	}{
		"hints in order": {tail: declared + worker + listening, want: []string{
			appv1alpha1.DeployReasonHintImageDeclaredPorts, appv1alpha1.DeployReasonHintBackgroundWorker, appv1alpha1.DeployReasonHintListening,
		}},
		// " The process is listening on port 9000." is a prefix of the
		// adoption's sentence; the longer template wins.
		"a sentence another one starts": {tail: adopted + worker, want: []string{
			appv1alpha1.DeployReasonHintListeningPrimary, appv1alpha1.DeployReasonHintBackgroundWorker,
		}},
		"an unknown trailing sentence": {tail: listening + unknown, remainder: unknown, want: []string{appv1alpha1.DeployReasonHintListening}},
		"an unknown sentence first":    {tail: unknown + listening, remainder: unknown + listening, want: []string{}},
		// The fallback hint as bex worded it before w21/059.
		"an older wording": {tail: " No port answered HTTP during startup discovery; set PORT to the HTTP listener.",
			remainder: " No port answered HTTP during startup discovery; set PORT to the HTTP listener.", want: []string{}},
		"a mismatched repeat": {tail: " Nothing accepted a TCP connection on port 80, the default because the image declares no TCP ports." +
			" The server must bind host 0.0.0.0 on the port in PORT (81).",
			remainder: " Nothing accepted a TCP connection on port 80, the default because the image declares no TCP ports." +
				" The server must bind host 0.0.0.0 on the port in PORT (81).", want: []string{}},
		"a capacity wait with an unknown detail": {tail: "waiting for build capacity: no build node has room for it yet (something new)",
			remainder: " (something new)", want: []string{appv1alpha1.DeployReasonQueuedCapacityNoNode}},
		"nothing": {want: []string{}},
	} {
		parts, remainder := appv1alpha1.ParseDeployReasonParts(tc.tail)
		if got := codes(parts); !slices.Equal(got, tc.want) || remainder != tc.remainder {
			t.Errorf("%s: %q reads as %v + %q, want %v + %q", name, tc.tail, got, remainder, tc.want, tc.remainder)
		}
		var joined strings.Builder
		for _, p := range parts {
			joined.WriteString(p.Text)
		}
		if joined.String()+remainder != tc.tail {
			t.Errorf("%s: the parts and remainder read %q, not the tail %q", name, joined.String()+remainder, tc.tail)
		}
	}
}

// w4/591: a sentence two templates match reads as the one that fixes more of
// it, never the one whose parameter swallows the difference. w5/m356: a part
// never reads as a whole reason, so a stored sentence keeps its code.
func TestDeployReasonPrefersTheMoreSpecificTemplate(t *testing.T) {
	restarted := appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonProbeHTTPWaitingRestarted,
		appv1alpha1.DeployReasonParam{Name: "kind", Value: "startup"}, appv1alpha1.DeployReasonParam{Name: "path", Value: "/"},
		appv1alpha1.DeployReasonParam{Name: "port", Value: "80"}, appv1alpha1.DeployReasonParam{Name: "restarts", Value: "2"})
	tcp := appv1alpha1.ProbeStallReason("readiness", true, "", "10000", 0)
	hints := " No open port was detected on the instance: the server must bind host 0.0.0.0 on the port in PORT (10000)." +
		" If the service does not need to receive traffic on any port, create a background worker instead."
	for text, want := range map[string]string{
		"Superseded by a newer release":            appv1alpha1.DeployReasonSupersededByNewer,
		"Superseded by dep-abc":                    appv1alpha1.DeployReasonSupersededBy,
		"Canceled along with a newer deploy":       appv1alpha1.DeployReasonCanceledAlongNewer,
		"Canceled along with dep-abc":              appv1alpha1.DeployReasonCanceledAlongWith,
		restarted:                                  appv1alpha1.DeployReasonProbeHTTPWaitingRestarted,
		tcp + hints:                                appv1alpha1.DeployReasonProbeTCPWaiting,
		appv1alpha1.TerminalStallText(tcp + hints): appv1alpha1.DeployReasonProbeTCPFailed,
		"the build never started: workspace has 2/2 concurrent builds active; waiting for a slot": appv1alpha1.DeployReasonBuildNeverStarted,
		"the container is running but its readiness health check has not succeeded, so the rollout is waiting: the configured health check." +
			" Check that the path returns a 2xx or 3xx status on the service's own port.": appv1alpha1.DeployReasonProbeOtherWaiting,
		// A part alone is no reason.
		strings.TrimPrefix(hints, " "):                                   "",
		"workspace has 2/2 concurrent builds active; waiting for a slot": "",
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
