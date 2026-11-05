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
	"fmt"
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
// rule forbids without a recorded, signed-off ADR018 row.
const allowlistSize = 108

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

// adr018AllowlistRows reads the first cells of the table under ADR018's
// "Allowlisted response-schema divergences" heading.
func adr018AllowlistRows(t *testing.T) map[string]bool {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "..", "..", "docs", "ADR018-render-parity.md"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	rows := map[string]bool{}
	inSection := false
	scanner := bufio.NewScanner(f)
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
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal(`ADR018 has no "Allowlisted response-schema divergences" table`)
	}
	return rows
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
