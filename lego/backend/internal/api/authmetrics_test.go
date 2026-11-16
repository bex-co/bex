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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v3"
)

// authmetrics_test.go pins the measurement w21/091 reads: what the 5 s token
// introspection window costs. Each test drives the real gate against a counting
// Hydra and Kratos and reads the series back, so a counter that is incremented
// at the wrong place, or under the wrong label, fails here.

// meteredGate is a gate wired the way Server.newAuthGate wires it: the origin
// metrics on a registry of their own and a clock the test steps across
// introspectionTTL.
type meteredGate struct {
	handler http.Handler
	gate    *oryAuth
	metrics *OriginMetrics
	reg     *prometheus.Registry
	clock   *stepClock
}

func newMeteredGate(t *testing.T, hydraAndKratosURL string, adm *AuthAdmission) *meteredGate {
	t.Helper()
	clock := &stepClock{t: time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)}
	reg := prometheus.NewRegistry()
	metrics := NewOriginMetrics(reg)
	gate := newOryAuth(hydraAndKratosURL, hydraAndKratosURL, "", "", "", false, adm, nil, nil, "")
	gate.metrics = metrics
	gate.now, gate.cache.Now = clock.Now, clock.Now
	if adm != nil {
		adm.now = clock.Now
	}
	return &meteredGate{handler: gate.middleware(echoIdentity), gate: gate, metrics: metrics, reg: reg, clock: clock}
}

func (g *meteredGate) do(t *testing.T, r *http.Request) int {
	t.Helper()
	w := httptest.NewRecorder()
	g.handler.ServeHTTP(w, r)
	return w.Code
}

func (g *meteredGate) misses(t *testing.T, outcome string) float64 {
	t.Helper()
	return counterValue(t, g.reg, "bex_api_auth_introspection_misses_total", map[string]string{"outcome": outcome})
}

func (g *meteredGate) sheds(t *testing.T, credential, cause, caller string) float64 {
	t.Helper()
	return counterValue(t, g.reg, "bex_api_auth_admission_sheds_total",
		map[string]string{"credential": credential, "cause": cause, "caller": caller})
}

func (g *meteredGate) hydraCalls(t *testing.T, outcome string) uint64 {
	t.Helper()
	families, err := g.reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() != "bex_api_auth_hydra_introspection_duration_seconds" {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, pair := range m.GetLabel() {
				if pair.GetName() == "outcome" && pair.GetValue() == outcome {
					return m.GetHistogram().GetSampleCount()
				}
			}
		}
	}
	t.Fatalf("no Hydra introspection series for outcome %q", outcome)
	return 0
}

func counterValue(t *testing.T, reg *prometheus.Registry, family string, labels map[string]string) float64 {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() != family {
			continue
		}
	next:
		for _, m := range f.GetMetric() {
			for _, pair := range m.GetLabel() {
				if want, ok := labels[pair.GetName()]; ok && want != pair.GetValue() {
					continue next
				}
			}
			return m.GetCounter().GetValue()
		}
	}
	t.Fatalf("no series %s%v", family, labels)
	return 0
}

func sessionRequest(session, sourceIP string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
	r.Header.Set("X-Session-Token", session)
	r.RemoteAddr = sourceIP + ":40000"
	return r
}

// The three outcomes a Hydra round trip can have, counted per request that
// missed the cache, and a cache hit counted not at all. The Hydra histogram sees
// one observation per round trip.
func TestIntrospectionMissesAreCountedByOutcome(t *testing.T) {
	p := newCountingIdentityProvider(t, "good-token", "")
	g := newMeteredGate(t, p.url, NewAuthAdmission(60, 60, 0, 0))

	if code := g.do(t, bearerRequest("good-token", "203.0.113.1")); code != http.StatusOK {
		t.Fatalf("first request = %d, want 200", code)
	}
	if code := g.do(t, bearerRequest("good-token", "203.0.113.1")); code != http.StatusOK {
		t.Fatalf("cached request = %d, want 200", code)
	}
	if got := g.misses(t, missActive); got != 1 {
		t.Errorf("active misses after one miss and one hit = %v, want 1 (a hit is not a miss)", got)
	}

	if code := g.do(t, bearerRequest("not-a-token", "203.0.113.1")); code != http.StatusUnauthorized {
		t.Fatalf("invalid token = %d, want 401", code)
	}
	if got := g.misses(t, missRefused); got != 1 {
		t.Errorf("refused misses = %v, want 1", got)
	}

	g.clock.Advance(introspectionTTL + time.Second)
	if code := g.do(t, bearerRequest("good-token", "203.0.113.1")); code != http.StatusOK {
		t.Fatalf("request after the window = %d, want 200", code)
	}
	if got := g.misses(t, missActive); got != 2 {
		t.Errorf("active misses after the window ended = %v, want 2: a live token pays one miss per window", got)
	}

	if got, want := g.hydraCalls(t, hydraOK), uint64(p.introspects.Load()); got != want || want != 3 {
		t.Errorf("Hydra round trips observed = %d, Hydra received %d, want both 3", got, want)
	}
	for _, outcome := range []string{missError, missShed} {
		if got := g.misses(t, outcome); got != 0 {
			t.Errorf("%s misses = %v, want 0", outcome, got)
		}
	}
}

func TestHydraFailureIsCountedAsAnErrorMissAndAnErrorRoundTrip(t *testing.T) {
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer broken.Close()
	g := newMeteredGate(t, broken.URL, NewAuthAdmission(60, 60, 0, 0))

	if code := g.do(t, bearerRequest("any-token", "203.0.113.2")); code != http.StatusServiceUnavailable {
		t.Fatalf("Hydra 500 = %d, want 503", code)
	}
	if got := g.misses(t, missError); got != 1 {
		t.Errorf("error misses = %v, want 1", got)
	}
	if got := g.hydraCalls(t, hydraError); got != 1 {
		t.Errorf("error round trips = %d, want 1", got)
	}
	if got := g.hydraCalls(t, hydraOK); got != 0 {
		t.Errorf("ok round trips = %d, want 0", got)
	}
}

// Sheds are counted by the budget that refused them and by whether the caller
// was valid. The numbers step 3 of w21/091 reads are the authenticated bearer
// ones.
func TestAdmissionShedsAreCountedByCredentialCauseAndCaller(t *testing.T) {
	t.Run("a live token that spends its own budget is an authenticated bearer shed", func(t *testing.T) {
		p := newCountingIdentityProvider(t, "good-token", "")
		// Two misses a credential, no refill within the test.
		g := newMeteredGate(t, p.url, NewAuthAdmission(1, 2, 0, 0))
		for i := range 2 {
			if code := g.do(t, bearerRequest("good-token", "203.0.113.3")); code != http.StatusOK {
				t.Fatalf("miss %d = %d, want 200", i, code)
			}
			g.clock.Advance(introspectionTTL + time.Second)
		}
		if code := g.do(t, bearerRequest("good-token", "203.0.113.3")); code != http.StatusTooManyRequests {
			t.Fatalf("third miss = %d, want 429", code)
		}
		if got := g.sheds(t, credentialBearer, shedCredential, callerAuthenticated); got != 1 {
			t.Errorf("authenticated bearer credential sheds = %v, want 1", got)
		}
		if got := g.misses(t, missShed); got != 1 {
			t.Errorf("shed misses = %v, want 1", got)
		}
		if got := p.introspects.Load(); got != 2 {
			t.Errorf("Hydra introspections = %d, want 2: a shed never reaches Hydra", got)
		}
	})

	t.Run("a flood from one address sheds a valid caller on the source budget", func(t *testing.T) {
		p := newCountingIdentityProvider(t, "good-token", "")
		g := newMeteredGate(t, p.url, NewAuthAdmission(1, 2, 0, 0))
		// The valid token is seen from another address first, so this replica
		// has resolved it, and its cache entry then ages out.
		if code := g.do(t, bearerRequest("good-token", "203.0.113.4")); code != http.StatusOK {
			t.Fatalf("warm request = %d, want 200", code)
		}
		g.clock.Advance(introspectionTTL + time.Second)
		for i := range 2 {
			g.do(t, bearerRequest(fmt.Sprintf("invalid-%d", i), "203.0.113.5"))
		}
		// The flooder's own next unknown token is shed and is not authenticated.
		if code := g.do(t, bearerRequest("invalid-more", "203.0.113.5")); code != http.StatusTooManyRequests {
			t.Fatalf("flooded source, unknown token = %d, want 429", code)
		}
		if got := g.sheds(t, credentialBearer, shedSource, callerUnverified); got != 1 {
			t.Errorf("unverified source sheds = %v, want 1", got)
		}
		// The valid token arrives from the flooded address.
		if code := g.do(t, bearerRequest("good-token", "203.0.113.5")); code != http.StatusTooManyRequests {
			t.Fatalf("flooded source, valid token = %d, want 429", code)
		}
		if got := g.sheds(t, credentialBearer, shedSource, callerAuthenticated); got != 1 {
			t.Errorf("authenticated source sheds = %v, want 1", got)
		}
	})

	t.Run("a session is its own credential kind", func(t *testing.T) {
		p := newCountingIdentityProvider(t, "", "good-session")
		g := newMeteredGate(t, p.url, NewAuthAdmission(1, 2, 0, 0))
		for i := range 2 {
			if code := g.do(t, sessionRequest("good-session", "203.0.113.6")); code != http.StatusOK {
				t.Fatalf("session request %d = %d, want 200", i, code)
			}
		}
		if code := g.do(t, sessionRequest("good-session", "203.0.113.6")); code != http.StatusTooManyRequests {
			t.Fatalf("third session request = %d, want 429", code)
		}
		if got := g.sheds(t, credentialSession, shedCredential, callerAuthenticated); got != 1 {
			t.Errorf("authenticated session sheds = %v, want 1", got)
		}
		if got := g.sheds(t, credentialBearer, shedCredential, callerAuthenticated); got != 0 {
			t.Errorf("bearer sheds = %v, want 0: a session shed is not a bearer shed", got)
		}
		if got := g.misses(t, missShed); got != 0 {
			t.Errorf("introspection misses = %v, want 0: a session never introspects", got)
		}
	})
}

// The memory behind caller=authenticated holds only credentials an upstream
// verdict found valid, and forgets them after Hydra's access-token lifetime.
func TestAuthenticatedMemoryHoldsOnlyValidCredentialsAndExpires(t *testing.T) {
	p := newCountingIdentityProvider(t, "good-token", "")
	adm := NewAuthAdmission(60, 60, 0, 0)
	g := newMeteredGate(t, p.url, adm)

	g.do(t, bearerRequest("not-a-token", "203.0.113.7"))
	if adm.wasAuthenticated(bearerRequest("not-a-token", "203.0.113.7")) {
		t.Error("an invalid token was remembered as authenticated")
	}
	g.do(t, bearerRequest("good-token", "203.0.113.7"))
	valid := bearerRequest("good-token", "203.0.113.8")
	if !adm.wasAuthenticated(valid) {
		t.Fatal("a valid token was not remembered as authenticated")
	}
	g.clock.Advance(authenticatedMemory - time.Second)
	if !adm.wasAuthenticated(valid) {
		t.Error("the memory ended before authenticatedMemory")
	}
	g.clock.Advance(2 * time.Second)
	if adm.wasAuthenticated(valid) {
		t.Error("the memory outlived authenticatedMemory")
	}
}

// admit names the budget that refused a call, for every one of the four, and
// each still satisfies errAuthOverloaded (the gate's 429 mapping).
func TestEveryAdmissionBudgetNamesItselfWhenItSheds(t *testing.T) {
	cause := func(t *testing.T, err error) string {
		t.Helper()
		if !errors.Is(err, errAuthOverloaded) {
			t.Fatalf("err = %v, want errAuthOverloaded", err)
		}
		var shed *shedError
		if !errors.As(err, &shed) {
			t.Fatalf("err = %v is not a *shedError", err)
		}
		return shed.cause
	}

	t.Run(shedSource, func(t *testing.T) {
		adm := NewAuthAdmission(1, 1, 0, 0)
		r := bearerRequest("x", "203.0.113.10")
		adm.penalize(r)
		_, err := adm.admit(r)
		if got := cause(t, err); got != shedSource {
			t.Errorf("cause = %q, want %q", got, shedSource)
		}
	})
	t.Run(shedCredential, func(t *testing.T) {
		adm := NewAuthAdmission(1, 1, 0, 0)
		r := bearerRequest("x", "203.0.113.11")
		release, err := adm.admit(r)
		if err != nil {
			t.Fatal(err)
		}
		release()
		_, err = adm.admit(r)
		if got := cause(t, err); got != shedCredential {
			t.Errorf("cause = %q, want %q", got, shedCredential)
		}
	})
	t.Run(shedCredentialInflight, func(t *testing.T) {
		adm := NewAuthAdmission(10_000, 10_000, 64, 1)
		r := bearerRequest("x", "203.0.113.12")
		release, err := adm.admit(r)
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		_, err = adm.admit(r)
		if got := cause(t, err); got != shedCredentialInflight {
			t.Errorf("cause = %q, want %q", got, shedCredentialInflight)
		}
	})
	t.Run(shedInflight, func(t *testing.T) {
		adm := NewAuthAdmission(10_000, 10_000, 1, 0)
		release, err := adm.admit(bearerRequest("one", "203.0.113.13"))
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		_, err = adm.admit(bearerRequest("two", "203.0.113.13"))
		if got := cause(t, err); got != shedInflight {
			t.Errorf("cause = %q, want %q", got, shedInflight)
		}
	})
}

// Every series exists at zero before its first event, so a replica that has seen
// no miss or shed still exports it (a rate() over an absent series is empty) and
// the label sets stay the closed ones the dashboard and the w21/091 queries name.
func TestAuthMetricSeriesAreRegisteredAtZero(t *testing.T) {
	reg := prometheus.NewRegistry()
	NewOriginMetrics(reg)
	samples := gatherSamples(t, reg)

	count := func(family string) int {
		n := 0
		for _, s := range samples {
			if s.family == family {
				n++
			}
		}
		return n
	}
	for family, want := range map[string]int{
		"bex_api_auth_introspection_misses_total":           4,
		"bex_api_auth_admission_sheds_total":                16,
		"bex_api_auth_hydra_introspection_duration_seconds": 2,
	} {
		if got := count(family); got != want {
			t.Errorf("%s has %d series at start, want %d", family, got, want)
		}
	}
	if got := labelValues(samples, "bex_api_auth_admission_sheds_total", "cause"); strings.Join(got, ",") != "credential,credential_inflight,inflight,source" {
		t.Errorf("shed causes = %v", got)
	}
}

// authFamilies are the families authmetrics.go registers.
func authFamilies(t *testing.T) []string {
	t.Helper()
	reg := prometheus.NewRegistry()
	NewOriginMetrics(reg)
	seen := map[string]bool{}
	var names []string
	for _, s := range gatherSamples(t, reg) {
		if strings.HasPrefix(s.family, "bex_api_auth_") && !seen[s.family] {
			seen[s.family] = true
			names = append(names, s.family)
		}
	}
	return names
}

// The bex-api scrape job drops every family its keep-list does not match
// (prometheus.yaml): w21/088's first sweep metric names were never stored for
// that reason. Read the live keep-list and fail for a name it would drop.
const (
	prometheusPath = "../../../../deploy/gitops/base/prometheus.yaml"
	grafanaPath    = "../../../../deploy/gitops/base/values/grafana.values.yaml"
)

func TestEveryAuthMetricIsKeptByTheScrapeJob(t *testing.T) {
	raw, err := os.ReadFile(prometheusPath)
	if err != nil {
		t.Fatal(err)
	}
	found := regexp.MustCompile(`(?m)^\s*regex:\s*(bex_\(billing\|\S+)\s*$`).FindSubmatch(raw)
	if found == nil {
		t.Fatalf("the bex-api keep-list (the regex that starts bex_(billing|) is not in %s: update this test with it", prometheusPath)
	}
	keep := regexp.MustCompile(`^(?:` + string(found[1]) + `)$`) // Prometheus anchors relabel regexes

	families := authFamilies(t)
	if len(families) != 3 {
		t.Fatalf("the auth gate registers %d families, want 3: %v", len(families), families)
	}
	for _, name := range families {
		if !keep.MatchString(name) {
			t.Errorf("%s is not matched by the bex-api scrape job's keep-list %q: Prometheus would drop it", name, found[1])
		}
	}
}

// A family nobody draws is a number nobody reads: the bex-api dashboard must
// query every one of them.
func TestBexAPIDashboardDrawsEveryAuthMetric(t *testing.T) {
	raw, err := os.ReadFile(grafanaPath)
	if err != nil {
		t.Fatal(err)
	}
	var values struct {
		Dashboards map[string]map[string]struct {
			JSON string `yaml:"json"`
		} `yaml:"dashboards"`
	}
	if err := yaml.Unmarshal(raw, &values); err != nil {
		t.Fatal(err)
	}
	dashboard, ok := values.Dashboards["platform"]["bex-api-gateways"]
	if !ok {
		t.Fatal("the bex-api-gateways dashboard is not in grafana.values.yaml")
	}
	var parsed struct {
		Panels []struct {
			Title   string `json:"title"`
			Targets []struct {
				Expr string `json:"expr"`
			} `json:"targets"`
		} `json:"panels"`
	}
	if err := json.Unmarshal([]byte(dashboard.JSON), &parsed); err != nil {
		t.Fatalf("the bex-api-gateways dashboard is not valid JSON: %v", err)
	}
	var exprs []string
	for _, panel := range parsed.Panels {
		for _, target := range panel.Targets {
			exprs = append(exprs, target.Expr)
		}
	}
	for _, name := range authFamilies(t) {
		drawn := false
		for _, expr := range exprs {
			// A histogram is queried through its _bucket and _count series.
			if strings.Contains(expr, name+" ") || strings.Contains(expr, name+"{") || strings.Contains(expr, name+"[") ||
				strings.Contains(expr, name+"_bucket") || strings.Contains(expr, name+"_count") || strings.Contains(expr, name+")") {
				drawn = true
				break
			}
		}
		if !drawn {
			t.Errorf("no panel of the bex-api-gateways dashboard queries %s", name)
		}
	}
}
