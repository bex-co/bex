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

package logs

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"text/template"
)

// appLevelCorpus is every severity shape the shipper's app_logs pipeline
// distinguishes, with the level it stores ("" = no level label).
var appLevelCorpus = []struct{ line, want string }{
	{`{"level":"error","msg":"boom"}`, "error"},
	{`{"level":"ERROR"}`, "error"},
	{`  {"level":"warn"}`, "warning"},
	{`{"level":"notice"}`, "info"},
	{`{"level":"trace"}`, "debug"},
	{`{"level":30,"msg":"pino info"}`, "info"},
	{`{"level":50}`, "error"},
	{`{"level":60}`, "error"},
	{`{"level":20}`, "debug"},
	{`{"level":35}`, "unknown"},
	{`{"level":30.5}`, "unknown"},
	{`{"severity":"WARNING","message":"gcp"}`, "warning"},
	{`{"level":"","severity":"error"}`, "error"},
	{`{"level":"info","severity":"error"}`, "info"},
	{`{"level":null,"severity":"debug"}`, "debug"},
	{`{"level":""}`, ""},
	{`{"severity":""}`, "unknown"},
	{`{"level":true}`, "unknown"},
	{`{"level":["error"]}`, "unknown"},
	{`{"Level":"error"}`, "unknown"},
	{`{"log":{"level":"error"}}`, "unknown"},
	{`{"msg":"no severity"}`, "unknown"},
	{`{"level":"error"} trailing`, "unknown"},
	{`{not json level=error`, "unknown"},
	{`time=2026-07-05T00:00:01Z level=WARN msg="disk low"`, "warning"},
	{`level=error msg=boom`, "error"},
	{`severity=crit`, "error"},
	{`level="info" msg=quoted`, "info"},
	{`level="error"`, "error"},
	{`level= msg=empty`, ""},
	{`level= severity=debug`, "debug"},
	{`level=info level=error`, "error"},
	{`msg="unterminated level=error`, "unknown"},
	{`a=b"c level=error`, "unknown"},
	{`level=info a=b"c`, "info"},
	{`=x level=error`, "unknown"},
	{`xlevel=error`, "unknown"},
	{"\tlevel=debug", "debug"},
	{"\f{\"level\":\"error\"}", "unknown"}, // \s, not JSON whitespace: neither stage reads it
	{`plain text with the word error in it`, "unknown"},
	{`ERROR: something failed`, "unknown"},
	{`[error] bracketed`, "unknown"},
	{``, "unknown"},
}

func TestAppLineLevelClassifiesLikeTheShipper(t *testing.T) {
	for _, tc := range appLevelCorpus {
		if got := appLineLevel(tc.line); got != tc.want {
			t.Errorf("appLineLevel(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
}

// shipperAppLogsBlock returns the lines of log-shipper.yaml's
// `loki.process "app_logs"` block: the pipeline that stamps an App line's level.
func shipperAppLogsBlock(t *testing.T) []string {
	t.Helper()
	path := findRepoFile(t, filepath.Join("deploy", "gitops", "base", "log-shipper.yaml"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	lines := strings.Split(string(data), "\n")
	start := slices.IndexFunc(lines, func(l string) bool { return strings.TrimSpace(l) == `loki.process "app_logs" {` })
	if start < 0 {
		t.Fatal(`log-shipper.yaml has no loki.process "app_logs" block`)
	}
	end := slices.IndexFunc(lines[start:], func(l string) bool { return strings.HasPrefix(strings.TrimSpace(l), "forward_to =") })
	if end < 0 {
		t.Fatal(`the app_logs block has no forward_to line`)
	}
	return lines[start : start+end]
}

// stageBody returns the lines inside the first `<stage> {` block at or after
// from — up to the `}` at the stage line's own indentation (a brace inside a
// selector string is not structure) — and the index after it.
func stageBody(t *testing.T, block []string, stage string, from int) ([]string, int) {
	t.Helper()
	for i := from; i < len(block); i++ {
		if strings.TrimSpace(block[i]) != stage+" {" {
			continue
		}
		closing := block[i][:len(block[i])-len(strings.TrimLeft(block[i], " "))] + "}"
		for j := i + 1; j < len(block); j++ {
			if strings.TrimRight(block[j], " ") == closing {
				return block[i+1 : j], j + 1
			}
		}
	}
	t.Fatalf("the app_logs block has no %s stage after line %d", stage, from)
	return nil, 0
}

var riverField = regexp.MustCompile(`^(\w+)\s*=\s*"((?:[^"\\]|\\.)*)",?$`)

func riverFields(body []string) map[string]string {
	fields := map[string]string{}
	for _, l := range body {
		if m := riverField.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			fields[m[1]] = m[2]
		}
	}
	return fields
}

// TestAppLineLevelMatchesShipper is the drift guard for applevel.go: it reads
// the deployed app_logs pipeline and fails when its severity fields, its
// logfmt gate, its default bucket or its normalization template stop agreeing
// with the Go copy — executing the real template, as Alloy does, over the
// fields each corpus line yields.
func TestAppLineLevelMatchesShipper(t *testing.T) {
	block := shipperAppLogsBlock(t)

	jsonBody, next := stageBody(t, block, "stage.json", 0)
	if got := riverFields(jsonBody); !maps.Equal(got, map[string]string{"level": "level", "severity": "severity"}) {
		t.Errorf("stage.json expressions = %v, want level and severity (jsonSeverity reads exactly these)", got)
	}

	matchBody, next := stageBody(t, block, "stage.match", next)
	var selector string
	for _, l := range matchBody {
		if l = strings.TrimSpace(l); strings.HasPrefix(l, "selector =") {
			selector = strings.Trim(strings.TrimSpace(strings.TrimPrefix(l, "selector =")), "`")
		}
	}
	lineFilters := regexp.MustCompile(`(!~|\|~) "((?:[^"\\]|\\.)*)"`).FindAllStringSubmatch(selector, -1)
	if len(lineFilters) != 2 || lineFilters[0][1] != "!~" || lineFilters[1][1] != "|~" {
		t.Fatalf("stage.match selector %q: want one !~ filter then one |~ filter", selector)
	}
	for i, re := range []*regexp.Regexp{shipperJSONLine, shipperLogfmtLine} {
		pattern, err := strconv.Unquote(`"` + lineFilters[i][2] + `"`)
		if err != nil || pattern != re.String() {
			t.Errorf("stage.match line filter %d = %q (%v), want %q", i, pattern, err, re.String())
		}
	}
	logfmtBody, _ := stageBody(t, matchBody, "stage.logfmt", 0)
	if len(logfmtBody) != 1 || strings.TrimSpace(logfmtBody[0]) != `mapping = { level = "", severity = "" }` {
		t.Errorf("stage.logfmt = %q, want the level and severity keys under their own names", logfmtBody)
	}

	templateBody, next := stageBody(t, block, "stage.template", next)
	if source := riverFields(templateBody)["source"]; source != "level" {
		t.Fatalf(`the app_logs block's stage.template source = %q, want "level"`, source)
	}
	var tmplText string
	for _, l := range templateBody {
		if l = strings.TrimSpace(l); strings.HasPrefix(l, "template =") {
			tmplText = l
		}
	}
	staticBody, _ := stageBody(t, block, "stage.static_labels", next)
	if got := riverFields(staticBody)["level"]; got != unknownLevel {
		t.Errorf("stage.static_labels level = %q, want %q (appLineLevel's default)", got, unknownLevel)
	}

	tmpl := parseShipperTemplate(t, tmplText)
	for _, tc := range appLevelCorpus {
		fields := map[string]string{}
		extract := func(k, v string) {
			if k == "level" || k == "severity" {
				fields[k] = v
			}
		}
		if shipperJSONLine.MatchString(tc.line) {
			jsonSeverity(tc.line, extract)
		} else if shipperLogfmtLine.MatchString(tc.line) {
			scanLogfmt(tc.line, extract)
		}
		if got, want := appLineLevel(tc.line), alloyLevel(tmpl, fields, tc.line); got != want {
			t.Errorf("line %q (fields %v): appLineLevel = %q, the shipper's template gives %q", tc.line, fields, got, want)
		}
	}
}

// parseShipperTemplate recovers the template Alloy runs from its River line:
// Helm's tpl pass renders the escape actions to literal braces and River
// unquotes `\"` (see shipperPipelineTemplate). ToLower is Alloy's.
func parseShipperTemplate(t *testing.T, line string) *template.Template {
	t.Helper()
	text := line[strings.IndexByte(line, '"')+1 : strings.LastIndexByte(line, '"')]
	text = strings.ReplaceAll(text, `{{ "{{" }}`, "{{")
	text = strings.ReplaceAll(text, `{{ "}}" }}`, "}}")
	text = strings.ReplaceAll(text, `\"`, `"`)
	tmpl, err := template.New("level").Funcs(template.FuncMap{"ToLower": strings.ToLower}).Parse(text)
	if err != nil {
		t.Fatalf("the level template does not parse (%q): %v", text, err)
	}
	return tmpl
}

// alloyLevel is the rest of the pipeline after extraction, as Alloy runs it:
// stage.template (`.Value` is the source field; a failed execution leaves the
// extracted map alone; an empty result deletes the field), then
// stage.static_labels' `unknown` default overwritten by stage.labels, whose
// empty value Alloy omits.
func alloyLevel(tmpl *template.Template, fields map[string]string, line string) string {
	data := map[string]any{"Entry": line}
	for k, v := range fields {
		data[k] = v
		if k == "level" {
			data["Value"] = v
		}
	}
	var out strings.Builder
	if err := tmpl.Execute(&out, data); err == nil {
		if out.Len() == 0 {
			delete(fields, "level")
		} else {
			fields["level"] = out.String()
		}
	}
	if level, ok := fields["level"]; ok {
		return level
	}
	return unknownLevel
}
