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
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// authmetrics.go measures what the 5 s token introspection window costs, so that
// narrowing it (w21/091, docs/ADR012-auth.md §8d) is decided from production
// numbers and not from an estimate. The window sets how often a live token
// misses the positive cache; every miss is a Hydra round trip and spends the
// per-credential admission budget (authadmission.go). Three families, all on the
// same registry as the origin telemetry and all kept by the bex-api scrape job
// (the bex_api_ prefix):
//
//   - bex_api_auth_introspection_misses_total{outcome}: every bearer request
//     that found no usable cache entry, by what became of it. The sum over the
//     outcomes is the miss rate (per replica; the admission budget is per
//     credential, so a replica's total is an upper bound on any one credential's
//     rate), active is the share a live token pays, and shed is the share the
//     admission budgets refused before Hydra was asked. It includes the
//     deliberate fresh introspections a durable-credential mint makes
//     (introspectFresh) and a cache hit the shared revocation marker evicted.
//   - bex_api_auth_admission_sheds_total{credential,cause,caller}: every 429
//     AUTH_OVERLOADED the gate answers, by the credential kind, the budget that
//     refused it and whether the credential was valid. A shed happens before
//     the upstream call, so the verdict is not known: caller=authenticated
//     means this replica resolved the very same credential to an identity
//     within authenticatedMemory (15 minutes). That is the number step 3 of
//     w21/091 reads, for credential=bearer (a Kratos session has no window).
//   - bex_api_auth_hydra_introspection_duration_seconds{outcome}: one
//     observation per Hydra introspection round trip (coalesced misses are one
//     call), ok or error. The 50 ms decision threshold is a bucket edge, so the
//     share under it reads exactly.
//
// Every label is a closed set, no credential, subject, client or workspace is
// ever a label, and every series is registered at zero so a replica that has
// not yet seen an event still exports it.
type authMetrics struct {
	misses *prometheus.CounterVec
	sheds  *prometheus.CounterVec
	hydra  *prometheus.HistogramVec
}

// Introspection miss outcomes.
const (
	// missActive: Hydra answered and the token resolved to an identity, now
	// cached for introspectionTTL.
	missActive = "active"
	// missRefused: Hydra answered inactive, or bex refused an active token
	// (audience, issuer, grant, revocation marker).
	missRefused = "refused"
	// missError: Hydra, or a dependency of the verdict, could not answer (503).
	missError = "error"
	// missShed: an admission budget refused the call before Hydra (429).
	missShed = "shed"
)

// Credential kinds a shed can land on, and whether it landed on a valid one.
const (
	credentialBearer  = "bearer"
	credentialSession = "session"

	callerAuthenticated = "authenticated"
	callerUnverified    = "unverified"
)

// Hydra round trip outcomes.
const (
	hydraOK    = "ok"
	hydraError = "error"
)

// hydraIntrospectionBuckets put the 50 ms decision threshold of w21/091 on a
// bucket edge and keep the milliseconds dense: Hydra answers an introspection
// in tens of milliseconds in-cluster, and the client's own timeout is 5 s.
var hydraIntrospectionBuckets = []float64{0.001, 0.0025, 0.005, 0.01, 0.02, 0.03, 0.04, 0.05, 0.075, 0.1, 0.25, 0.5, 1, 2.5, 5}

func newAuthMetrics(reg prometheus.Registerer) authMetrics {
	m := authMetrics{
		misses: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "api", Name: "auth_introspection_misses_total",
			Help: "Bearer requests that found no usable cached introspection, by outcome: active (Hydra resolved the token), refused (inactive or refused by bex), error (Hydra could not answer) or shed (an admission budget refused it before Hydra). Includes fresh introspections for durable-credential mints.",
		}, []string{"outcome"}),
		sheds: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "api", Name: "auth_admission_sheds_total",
			Help: "Authentication requests answered 429 AUTH_OVERLOADED, by credential kind, the budget that refused them (source, credential, credential_inflight, inflight) and whether this replica had resolved the same credential to an identity in the last 15 minutes (authenticated) or not (unverified).",
		}, []string{"credential", "cause", "caller"}),
		hydra: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "bex", Subsystem: "api", Name: "auth_hydra_introspection_duration_seconds",
			Help:    "Duration of one bex-api to Hydra token introspection round trip, by outcome (ok or error). Concurrent misses for one token share one call.",
			Buckets: hydraIntrospectionBuckets,
		}, []string{"outcome"}),
	}
	for _, outcome := range []string{missActive, missRefused, missError, missShed} {
		m.misses.WithLabelValues(outcome)
	}
	for _, credential := range []string{credentialBearer, credentialSession} {
		for _, cause := range []string{shedSource, shedCredential, shedCredentialInflight, shedInflight} {
			for _, caller := range []string{callerAuthenticated, callerUnverified} {
				m.sheds.WithLabelValues(credential, cause, caller)
			}
		}
	}
	for _, outcome := range []string{hydraOK, hydraError} {
		m.hydra.WithLabelValues(outcome)
	}
	reg.MustRegister(m.misses, m.sheds, m.hydra)
	return m
}

// observeIntrospectionMiss counts one bearer request that missed the cache. A
// nil receiver (metrics not wired: tests, stdio mode) is a no-op.
func (m *OriginMetrics) observeIntrospectionMiss(outcome string) {
	if m == nil {
		return
	}
	m.auth.misses.WithLabelValues(outcome).Inc()
}

// observeAuthShed counts one 429 AUTH_OVERLOADED. err is what admit returned: a
// *shedError names the budget, anything else cannot reach here.
func (m *OriginMetrics) observeAuthShed(credential string, err error, authenticated bool) {
	if m == nil {
		return
	}
	cause := shedInflight
	var shed *shedError
	if errors.As(err, &shed) {
		cause = shed.cause
	}
	caller := callerUnverified
	if authenticated {
		caller = callerAuthenticated
	}
	m.auth.sheds.WithLabelValues(credential, cause, caller).Inc()
}

// observeHydraIntrospection records one Hydra introspection round trip.
func (m *OriginMetrics) observeHydraIntrospection(outcome string, d time.Duration) {
	if m == nil {
		return
	}
	m.auth.hydra.WithLabelValues(outcome).Observe(d.Seconds())
}
