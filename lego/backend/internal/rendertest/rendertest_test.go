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

package rendertest

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// allowlistSize pins the number of Allowlist entries. Lower it when an entry
// goes; raising it means accepting a new divergence, which this milestone's
// rule forbids without a recorded, signed-off ADR018 row. w3/m302 raised it
// once, from 106 to 112, without a new divergence: it split the two event
// details union entries, which excused any mismatch of any type, into one
// entry per type and missing field, each recorded in ADR018's events row.
// w10/m373 lowered it from 109 to 40: stamped managedFields and Wrap's
// metadata Sources retired the fixture-only metadata entries, and serviceDetails
// gained buildPlan and healthCheckPath; then to 34 (fixture plan, publishPath
// and disk id; ADR018 § Production-shaped fixtures), then to 32 (a Postgres
// create stores its default major, w4/m378 t013), then to 30 (a service
// linked to a build source reads the source's runtime, w3/070), then to 23 (an
// image service reads Render's docker-shaped envSpecificDetails, so only a
// runtime-less buildpack build lacks the block, w9/m473), then to 22
// (build_ended carries Render's required buildId, w4/710), then to 18
// (Render's 2026-11-23 OpenAPI dropped postgresDetail's phantom required
// project field, w4/m596).
const allowlistSize = 18

func TestAllowlistOnlyShrinks(t *testing.T) {
	n := 0
	for _, entries := range Allowlist {
		n += len(entries)
	}
	switch {
	case n > allowlistSize:
		t.Fatalf("Allowlist grew to %d entries (pinned %d). Fix the response instead; a divergence stands only with a signed-off ADR018 row", n, allowlistSize)
	case n < allowlistSize:
		t.Fatalf("Allowlist shrank to %d entries: lower allowlistSize from %d to %d so it cannot grow back", n, allowlistSize, n)
	}
}

// TestAllowlistCitesADR018Rows fails an entry whose citation is not the first
// cell of a row in ADR018's allowlisted-divergences table, or that names no
// suite to reproduce it, or that duplicates another entry of its operation.
func TestAllowlistCitesADR018Rows(t *testing.T) {
	rows := adr018AllowlistRows(t)
	for op, entries := range Allowlist {
		seen := map[string]bool{}
		for _, d := range entries {
			if !rows[d.ADR018] {
				t.Errorf("%s %q cites %q, which is not a row of ADR018's allowlisted-divergences table", op, d.Contains, d.ADR018)
			}
			if d.Suite != SuiteAPI && d.Suite != SuiteApps {
				t.Errorf("%s %q names no reproducing suite", op, d.Contains)
			}
			if seen[d.Contains] {
				t.Errorf("%s lists %q twice", op, d.Contains)
			}
			seen[d.Contains] = true
		}
	}
}

// adr018AllowlistFile holds ADR018's "Allowlisted response-schema
// divergences" table: the API contract part of the parity ledger, which moved
// out of docs/ADR018-render-parity.md in w10/m607. backend-test.yml's paths
// list it, so editing the table runs this suite.
var adr018AllowlistFile = filepath.Join("..", "..", "..", "..", "docs", "ADR018", "api-contract.md")

// adr018AllowlistRows reads the first cells of ADR018's allowlisted-divergences
// table from adr018AllowlistFile.
func adr018AllowlistRows(t *testing.T) map[string]bool {
	t.Helper()
	f, err := os.Open(adr018AllowlistFile)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	rows, err := allowlistTableRows(f)
	if err != nil {
		t.Fatalf("%s: %v", adr018AllowlistFile, err)
	}
	return rows
}

// allowlistTableRows reads the first cells of the table under the
// "Allowlisted response-schema divergences" heading, up to the next heading.
func allowlistTableRows(r io.Reader) (map[string]bool, error) {
	rows := map[string]bool{}
	inSection := false
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<24)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			inSection = strings.Contains(line, "Allowlisted response-schema divergences")
			continue
		}
		if !inSection || !strings.HasPrefix(line, "| ") {
			continue
		}
		cell := strings.TrimSpace(strings.Split(line, "|")[1])
		rows[strings.Trim(cell, "*")] = true
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New(`no "Allowlisted response-schema divergences" table`)
	}
	return rows, nil
}

// TestAllowlistTableRows: the reader takes the first cells of the table under
// the allowlisted-divergences heading and of no other table, and refuses a file
// without it, so a cited row that leaves the table, or a table that leaves
// adr018AllowlistFile, fails TestAllowlistCitesADR018Rows.
func TestAllowlistTableRows(t *testing.T) {
	doc := strings.Join([]string{
		"## API contract",
		"| Rate limits (429 + Retry-After) | ✅ |",
		"### Allowlisted response-schema divergences (w3/m272)",
		"| Allowlisted divergence | Operations | Record |",
		"| --- | --- | --- |",
		"| **Dry-run previews** | create-service | the bex dryRun extension |",
		"| Partial serviceDetails projection | list-services | fixture gap |",
		"## Platform events & integrations",
		"| Outbound event webhooks | ◐ |",
	}, "\n")
	rows, err := allowlistTableRows(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Dry-run previews", "Partial serviceDetails projection"} {
		if !rows[want] {
			t.Errorf("row %q not read", want)
		}
	}
	for _, other := range []string{"Rate limits (429 + Retry-After)", "Outbound event webhooks"} {
		if rows[other] {
			t.Errorf("row %q of another table read as an allowlisted divergence", other)
		}
	}
	if _, err := allowlistTableRows(strings.NewReader("## API contract\n| Rate limits | ✅ |\n")); err == nil {
		t.Error("a file without the allowlisted-divergences table read as one")
	}
}

// customDomains is a list-custom-domains body that conforms, with one field
// replaceable so each case below breaks exactly one thing.
func customDomains(field, value string) []byte {
	fields := map[string]string{
		"id": `"cdm-1"`, "name": `"app.example.com"`, "domainType": `"subdomain"`,
		"publicSuffix": `"com"`, "verificationStatus": `"verified"`,
		"createdAt": `"2026-01-02T00:00:00Z"`, "redirectForName": `""`,
	}
	if value == "" {
		delete(fields, field)
	} else {
		fields[field] = value
	}
	var parts []string
	for _, k := range slices.Sorted(maps.Keys(fields)) {
		parts = append(parts, fmt.Sprintf("%q:%s", k, fields[k]))
	}
	return []byte(`[{"customDomain":{` + strings.Join(parts, ",") + `},"cursor":"c"}]`)
}

var jsonHeader = http.Header{"Content-Type": {"application/json"}}

func TestValidate(t *testing.T) {
	c := Load(t)
	route := c.Operation("list-custom-domains")
	if route == nil {
		t.Fatal("pinned spec lost list-custom-domains")
	}
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, _ = zw.Write(customDomains("createdAt", `12`))
	_ = zw.Close()

	for _, tc := range []struct {
		name   string
		status int
		header http.Header
		body   []byte
		want   string // "" = conforms
	}{
		{"conforming body", 200, jsonHeader, customDomains("id", `"cdm-1"`), ""},
		{"wrong-typed field", 200, jsonHeader, customDomains("createdAt", `12`), "/0/customDomain/createdAt: "},
		{"bad date-time format", 200, jsonHeader, customDomains("createdAt", `"yesterday"`), "/0/customDomain/createdAt: "},
		{"value outside the enum", 200, jsonHeader, customDomains("domainType", `"wildcard"`), `/0/customDomain/domainType: "wildcard" is not in the documented enum`},
		{"missing required key", 200, jsonHeader, customDomains("publicSuffix", ""), `/0/customDomain/publicSuffix: property "publicSuffix" is missing`},
		{"undocumented success status", 201, jsonHeader, customDomains("id", `"cdm-1"`), "status 201: not a documented response"},
		{"undocumented error status with Render's error body", 409, jsonHeader, []byte(`{"id":"conflict","message":"taken"}`), ""},
		{"undocumented error status with a wrong-typed error body", 409, jsonHeader, []byte(`{"message":7}`), "/message: "},
		{"undocumented error status with no JSON", 409, jsonHeader, []byte(`taken`), "error body is not JSON"},
		{"documented error with a wrong-typed body", 404, jsonHeader, []byte(`{"message":["x"]}`), "/message: "},
		{"wrong content type", 200, http.Header{"Content-Type": {"text/plain"}}, customDomains("id", `"cdm-1"`), "Content-Type"},
		{"gzip body is decoded first", 200, http.Header{"Content-Type": {"application/json"}, "Content-Encoding": {"gzip"}}, gz.Bytes(), "/0/customDomain/createdAt: "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Validate(route, tc.status, tc.header, tc.body)
			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("violations = %q, want none", got)
				}
				return
			}
			if !slices.ContainsFunc(got, func(v string) bool { return strings.Contains(v, tc.want) }) {
				t.Fatalf("violations = %q, want one containing %q", got, tc.want)
			}
		})
	}
}

// TestValidateSelectsTheServiceDetailsMember pins the union refinement: a
// service's serviceDetails is checked against the member its type names, so
// a wrong field is reported at its own pointer, not as an opaque union miss.
func TestValidateSelectsTheServiceDetailsMember(t *testing.T) {
	route := Load(t).Operation("retrieve-service")
	service := func(kind, details string) []byte {
		return []byte(`{"id":"srv-x","name":"x","type":"` + kind + `","autoDeploy":"no","createdAt":"2026-01-02T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z","notifyOnFail":"default","ownerId":"tea-1","repo":"","rootDir":"","slug":"x","suspended":"not_suspended","suspenders":[],"dashboardUrl":"https://d","serviceDetails":` + details + `}`)
	}
	cron := `{"env":"image","runtime":"image","envSpecificDetails":{"buildCommand":"","startCommand":"date"},"plan":"starter","region":"frankfurt","buildPlan":"starter","schedule":%s}`

	got := Validate(route, 200, jsonHeader, service("cron_job", fmt.Sprintf(cron, `"*/5 * * * *"`)))
	if slices.ContainsFunc(got, func(v string) bool { return strings.Contains(v, "/serviceDetails") }) {
		t.Fatalf("a conforming cron's details reported %q", got)
	}
	got = Validate(route, 200, jsonHeader, service("cron_job", fmt.Sprintf(cron, `5`)))
	if !slices.ContainsFunc(got, func(v string) bool { return strings.HasPrefix(v, "/serviceDetails/schedule: ") }) {
		t.Fatalf("a wrong-typed cron schedule reported %q, want it at /serviceDetails/schedule", got)
	}
	// The same details under a web_service type miss web's required members.
	got = Validate(route, 200, jsonHeader, service("web_service", fmt.Sprintf(cron, `"*/5 * * * *"`)))
	if !slices.Contains(got, `/serviceDetails/openPorts: property "openPorts" is missing`) {
		t.Fatalf("cron details on a web service reported %q, want web's missing openPorts", got)
	}
	// Render's suspend documents a bodiless 202: a service bex sends beyond it
	// is an extension, not checked against a member.
	suspend := Load(t).Operation("suspend-service")
	if got := Validate(suspend, 202, jsonHeader, service("web_service", fmt.Sprintf(cron, `5`))); len(got) != 0 {
		t.Fatalf("a bodiless 202 reported %q", got)
	}
}

func TestValidateNoContent(t *testing.T) {
	route := Load(t).Operation("delete-build-source")
	if route == nil {
		t.Fatal("pinned spec lost delete-build-source")
	}
	for _, tc := range []struct {
		name   string
		header http.Header
		body   []byte
		want   []string
	}{
		{"empty body and no content-type conforms", nil, nil, nil},
		{"non-empty body fails", nil, []byte("null\n"), []string{"status 204: must not carry a body"}},
		{"content-type header fails", jsonHeader, nil, []string{"status 204: must not set Content-Type"}},
		{"both body and content-type fail", jsonHeader, []byte("null\n"), []string{"status 204: must not carry a body", "status 204: must not set Content-Type"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Validate(route, http.StatusNoContent, tc.header, tc.body)
			if len(tc.want) == 0 {
				if len(got) != 0 {
					t.Fatalf("violations = %q, want none", got)
				}
				return
			}
			for _, w := range tc.want {
				if !slices.ContainsFunc(got, func(v string) bool { return strings.Contains(v, w) }) {
					t.Errorf("violations = %q, want one containing %q", got, w)
				}
			}
		})
	}
}

// Each event's details are checked against the member its type names, with
// the type named last; a bex-named type is left to its enum violation, and
// empty details no longer fail as matching several members (w3/m302).
func TestValidateSelectsTheEventDetailsMember(t *testing.T) {
	route := Load(t).Operation("retrieve-event")
	event := func(kind, details string) []byte {
		return []byte(`{"id":"evt-0123456789abcdefghij","timestamp":"2026-11-10T09:00:00Z","serviceId":"srv-0123456789abcdefghij","type":"` + kind + `","details":` + details + `}`)
	}
	for _, tc := range []struct {
		kind, details string
		want          []string
	}{
		{kind: "instance_count_changed", details: `{"fromInstances":1,"toInstances":2}`},
		{kind: "service_resumed", details: `{}`},
		{kind: "instance_count_changed", details: `{"from":1,"to":2}`, want: []string{
			`/details/fromInstances: property "fromInstances" is missing (instance_count_changed)`,
			`/details/toInstances: property "toInstances" is missing (instance_count_changed)`,
		}},
		{kind: "build_ended", details: `{"buildId":"bld-1","buildStatus":"failed","reason":{},"status":"failed"}`, want: []string{
			`/details/status: value must be an integer (build_ended)`,
		}},
		{kind: "branch_changed", details: `{"from":"a","to":"b"}`, want: []string{
			`/type: "branch_changed" is not in the documented enum`,
		}},
	} {
		got := Validate(route, 200, jsonHeader, event(tc.kind, tc.details))
		slices.Sort(got)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s %s reported %q, want %q", tc.kind, tc.details, got, tc.want)
		}
	}
}

// recordingTB captures what Wrap reports instead of failing the real test.
type recordingTB struct {
	testing.TB
	errors []string
}

func (r *recordingTB) Helper() {}
func (r *recordingTB) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func TestWrap(t *testing.T) {
	c := Load(t)
	serve := func(status int, body string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		})
	}
	run := func(next http.Handler, path string) (*recordingTB, *httptest.ResponseRecorder) {
		rec := &recordingTB{TB: t}
		w := httptest.NewRecorder()
		Wrap(rec, next, c.Route).ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return rec, w
	}

	t.Run("a wrong field fails the calling test with operation, status and pointer", func(t *testing.T) {
		rec, w := run(serve(200, string(customDomains("createdAt", `12`))), "/v1/services/srv-x/custom-domains")
		if len(rec.errors) != 1 {
			t.Fatalf("errors = %q, want one", rec.errors)
		}
		for _, want := range []string{"operationId=list-custom-domains", "=> 200", "/0/customDomain/createdAt"} {
			if !strings.Contains(rec.errors[0], want) {
				t.Errorf("error %q lacks %q", rec.errors[0], want)
			}
		}
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"createdAt":12`) {
			t.Fatalf("the response did not pass through unchanged: %d %s", w.Code, w.Body)
		}
	})
	t.Run("a conforming response passes", func(t *testing.T) {
		if rec, _ := run(serve(200, string(customDomains("id", `"cdm-1"`))), "/v1/services/srv-x/custom-domains"); len(rec.errors) != 0 {
			t.Fatalf("errors = %q", rec.errors)
		}
	})
	t.Run("a route outside the contract is not validated", func(t *testing.T) {
		if rec, _ := run(serve(200, `not json`), "/graphql"); len(rec.errors) != 0 {
			t.Fatalf("errors = %q", rec.errors)
		}
	})
	t.Run("an allowlisted divergence does not fail", func(t *testing.T) {
		// An event type outside Render's enum is a recorded bex extension.
		body := `[{"event":{"id":"evt-1","serviceId":"srv-x","timestamp":"2026-01-02T00:00:00Z","type":"env_vars_changed","details":{}},"cursor":"c"}]`
		rec, _ := run(serve(200, body), "/v1/services/srv-x/events")
		if slices.ContainsFunc(rec.errors, func(e string) bool { return strings.Contains(e, "event/type") }) {
			t.Fatalf("errors = %q, want the allowlisted event type excused", rec.errors)
		}
	})
}

func TestValidateMetricDatapoint(t *testing.T) {
	route := Load(t).Operation("get-disk-capacity")
	if route == nil {
		t.Fatal("pinned spec lost get-disk-capacity")
	}
	// Render's pinned spec (round 9, w4/m596) requires only {timestamp, value}
	// on values.items (`unit` lives on the parent series).
	valid := []byte(`[{"unit":"bytes","labels":[{"field":"resource","value":"dpg-1"}],"values":[{"timestamp":"2026-06-01T12:00:00Z","value":1024}]}]`)
	if got := Validate(route, http.StatusOK, jsonHeader, valid); len(got) != 0 {
		t.Fatalf("conforming metric datapoint reported %q, want none", got)
	}
	// Real datapoint defects are still caught.
	missingVal := []byte(`[{"unit":"bytes","labels":[{"field":"resource","value":"dpg-1"}],"values":[{"timestamp":"2026-06-01T12:00:00Z"}]}]`)
	got := Validate(route, http.StatusOK, jsonHeader, missingVal)
	if !slices.ContainsFunc(got, func(v string) bool { return strings.Contains(v, `/0/values/0/value: property "value" is missing`) }) {
		t.Fatalf("missing datapoint value reported %q, want /0/values/0/value missing", got)
	}
}

func TestWrapRecordsValidatedStatuses(t *testing.T) {
	c := Load(t)
	serve := func(status int, body string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if status != http.StatusNoContent {
				w.Header().Set("Content-Type", "application/json")
			}
			w.WriteHeader(status)
			if body != "" {
				_, _ = w.Write([]byte(body))
			}
		})
	}
	// A failing validation is NOT recorded.
	failRec := &recordingTB{TB: t}
	Wrap(failRec, serve(http.StatusNoContent, "null\n"), c.Route).ServeHTTP(
		httptest.NewRecorder(), httptest.NewRequest(http.MethodDelete, "/v1/build-sources/bds-1", nil))
	if len(failRec.errors) == 0 {
		t.Fatal("204 with body did not fail Wrap")
	}
	if ValidatedStatuses()["delete-build-source"][http.StatusNoContent] {
		t.Fatal("failed validation was recorded in ValidatedStatuses")
	}

	// A passing validation IS recorded.
	passRec := &recordingTB{TB: t}
	Wrap(passRec, serve(http.StatusNoContent, ""), c.Route).ServeHTTP(
		httptest.NewRecorder(), httptest.NewRequest(http.MethodDelete, "/v1/build-sources/bds-1", nil))
	if len(passRec.errors) != 0 {
		t.Fatalf("clean 204 failed Wrap: %q", passRec.errors)
	}
	if !ValidatedStatuses()["delete-build-source"][http.StatusNoContent] {
		t.Fatal("passing validation was not recorded in ValidatedStatuses")
	}
}

func TestMuxMatcher(t *testing.T) {
	c := Load(t)
	mux := http.NewServeMux()
	noop := func(http.ResponseWriter, *http.Request) {}
	mux.HandleFunc("GET /v1/services/{id}", noop)
	mux.HandleFunc("GET /v1/webhooks/event-types", noop)
	mux.HandleFunc("PUT /v1/env-groups/{id}/env-vars", noop)
	match := c.MuxMatcher(mux)
	for _, tc := range []struct {
		method, path, want string
	}{
		{"GET", "/v1/services/srv-x", "retrieve-service"},
		// A literal bex segment where Render has a parameter is bex-native.
		{"GET", "/v1/webhooks/event-types", ""},
		// A shorter bex route under a longer Render template is bex-native.
		{"PUT", "/v1/env-groups/evg-x/env-vars", ""},
		// Unrouted by the mux, though Render documents it.
		{"GET", "/v1/postgres", ""},
	} {
		route := match(httptest.NewRequest(tc.method, tc.path, nil))
		got := ""
		if route != nil {
			got = route.Operation.OperationID
		}
		if got != tc.want {
			t.Errorf("%s %s matched %q, want %q", tc.method, tc.path, got, tc.want)
		}
	}
}
