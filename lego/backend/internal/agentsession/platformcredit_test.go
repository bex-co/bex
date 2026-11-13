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

package agentsession

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// The monitor reads the OpenRouter account's credits and usage from
// GET /api/v1/credits with the platform key as Bearer, exports the remaining
// balance and the time of the read, and never puts the key in an error.
func TestPlatformCreditMonitorExportsBalance(t *testing.T) {
	var body string
	var sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		if body == "" {
			http.Error(w, "nope "+testPlatformKey, http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	reg := prometheus.NewRegistry()
	m := NewPlatformCreditMonitor(reg, testPlatformKey)
	m.URL, m.HTTP = srv.URL, srv.Client()

	for _, tc := range []struct {
		body    string
		balance float64
	}{
		// The live response shape (2026-11-12).
		{`{"data":{"total_credits":260,"total_usage":251.875}}`, 8.125},
		{`{"data":{"total_credits":0,"total_usage":0}}`, 0},
		// Usage can overrun the credits; the balance goes negative.
		{`{"data":{"total_credits":100,"total_usage":100.5}}`, -0.5},
	} {
		body = tc.body
		before := float64(time.Now().Unix())
		if err := m.Poll(context.Background()); err != nil {
			t.Fatalf("%s: %v", tc.body, err)
		}
		got := gatherCredit(t, reg)
		if got[platformCreditBalance] != tc.balance {
			t.Errorf("%s: balance = %v, want %v", tc.body, got[platformCreditBalance], tc.balance)
		}
		if ts := got[platformCreditLastSuccess]; ts < before || ts > float64(time.Now().Unix()+1) {
			t.Errorf("%s: last success = %v, want about now", tc.body, ts)
		}
	}
	if sawAuth != "Bearer "+testPlatformKey {
		t.Fatal("the key must be sent as a Bearer token")
	}
	last := gatherCredit(t, reg)[platformCreditLastSuccess]
	for _, bad := range []string{
		"", `{"data":{}}`, `not json`,
		`{"data":{"total_credits":260}}`, `{"data":{"total_usage":1}}`,
		`{"data":{"total_credits":-1,"total_usage":1}}`,
	} {
		body = bad
		err := m.Poll(context.Background())
		if err == nil || strings.Contains(err.Error(), testPlatformKey) {
			t.Fatalf("%q: err = %v, want a key-free error", bad, err)
		}
	}
	got := gatherCredit(t, reg)
	if got[platformCreditBalance] != -0.5 || got[platformCreditLastSuccess] != last {
		t.Fatalf("a failed poll changed the last balance or its timestamp: %v", got)
	}
}

// No fake zero: until a read succeeds the monitor exports only that it is
// configured, never a balance a threshold alert would read as an empty account
// (PlatformModelCreditStale covers a monitor that never reads it).
func TestPlatformCreditMonitorExportsNoBalanceBeforeFirstSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer srv.Close()
	reg := prometheus.NewRegistry()
	m := NewPlatformCreditMonitor(reg, testPlatformKey)
	m.URL, m.HTTP = srv.URL, srv.Client()
	if err := m.Poll(context.Background()); err == nil {
		t.Fatal("a 502 must fail the poll")
	}
	got := gatherCredit(t, reg)
	if _, ok := got[platformCreditBalance]; ok {
		t.Fatalf("balance exported before any successful read: %v", got)
	}
	if _, ok := got[platformCreditLastSuccess]; ok {
		t.Fatalf("last success exported before any successful read: %v", got)
	}
	if got["bex_agent_platform_model_credit_monitor_configured"] != 1 {
		t.Fatalf("configured = %v, want 1", got)
	}
}

const (
	platformCreditBalance     = "bex_agent_platform_model_credit_balance_usd"
	platformCreditLastSuccess = "bex_agent_platform_model_credit_last_success_timestamp_seconds"
)

// gatherCredit is the registry's samples by family name (every family here
// is one unlabelled gauge).
func gatherCredit(t *testing.T, reg *prometheus.Registry) map[string]float64 {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]float64{}
	for _, f := range families {
		out[f.GetName()] = f.GetMetric()[0].GetGauge().GetValue()
	}
	return out
}
