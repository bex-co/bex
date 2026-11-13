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
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Platform credit backstop (ADR091 D5 "Global backstop"; user decision
// 2026-11-12). The platform OpenRouter key has no credit limit of its own:
// the OpenRouter account balance is the cap, and at zero OpenRouter refuses
// every platform-model request for every workspace. bex-api reads the
// account's purchased credits and usage from OpenRouter's GET /api/v1/credits
// every 10 minutes and exports the remaining balance, and
// PlatformModelCreditLow / PlatformModelCreditCritical fire below $25 / $5.
//
// No fake zero: the balance and last-success samples appear only after the
// first successful read, so a monitor that cannot read the account exports
// nothing a threshold alert could mistake for an empty account. Instead
// PlatformModelCreditStale fires when the last success is older than 30
// minutes, or when the monitor is configured (the configured gauge, exported
// from construction) yet has never read the account.
//
// The key never leaves the Authorization header: it is not logged, labelled
// or returned.

// openRouterCreditsURL is OpenRouter's account credits endpoint.
const openRouterCreditsURL = openRouterEndpoint + "/v1/credits"

const (
	// PlatformCreditPollInterval is how often the account balance is read.
	PlatformCreditPollInterval = 10 * time.Minute
	platformCreditFetchTimeout = 15 * time.Second
	maxPlatformCreditBody      = 1 << 20
)

var (
	platformCreditBalanceDesc = prometheus.NewDesc(
		"bex_agent_platform_model_credit_balance_usd",
		"OpenRouter account balance (USD) behind the platform model key: total_credits - total_usage, as of the last successful read. Negative when usage has overrun the credits; absent until the first successful read.",
		nil, nil)
	platformCreditLastSuccessDesc = prometheus.NewDesc(
		"bex_agent_platform_model_credit_last_success_timestamp_seconds",
		"Unix time of the last successful OpenRouter account balance read; absent until the first one.",
		nil, nil)
	platformCreditConfiguredDesc = prometheus.NewDesc(
		"bex_agent_platform_model_credit_monitor_configured",
		"1 when this bex-api runs the platform model route and polls the OpenRouter account balance.",
		nil, nil)
)

// PlatformCreditMonitor polls the OpenRouter account's balance. It is a
// prometheus.Collector, so it can leave its samples out until it has one.
type PlatformCreditMonitor struct {
	Key      string
	URL      string
	HTTP     *http.Client
	Interval time.Duration

	mu          sync.Mutex
	balance     float64
	lastSuccess time.Time // zero until the first successful read
}

// NewPlatformCreditMonitor registers the monitor on registerer:
// bex_agent_platform_model_credit_monitor_configured (1 from now on), and,
// once a read succeeds, bex_agent_platform_model_credit_balance_usd and
// bex_agent_platform_model_credit_last_success_timestamp_seconds.
func NewPlatformCreditMonitor(registerer prometheus.Registerer, key string) *PlatformCreditMonitor {
	m := &PlatformCreditMonitor{Key: key}
	registerer.MustRegister(m)
	return m
}

// Describe implements prometheus.Collector.
func (m *PlatformCreditMonitor) Describe(ch chan<- *prometheus.Desc) {
	ch <- platformCreditBalanceDesc
	ch <- platformCreditLastSuccessDesc
	ch <- platformCreditConfiguredDesc
}

// Collect implements prometheus.Collector.
func (m *PlatformCreditMonitor) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(platformCreditConfiguredDesc, prometheus.GaugeValue, 1)
	m.mu.Lock()
	balance, last := m.balance, m.lastSuccess
	m.mu.Unlock()
	if last.IsZero() {
		return
	}
	ch <- prometheus.MustNewConstMetric(platformCreditBalanceDesc, prometheus.GaugeValue, balance)
	ch <- prometheus.MustNewConstMetric(platformCreditLastSuccessDesc, prometheus.GaugeValue, float64(last.UnixNano())/1e9)
}

// openRouterCredits is GET /api/v1/credits's body.
type openRouterCredits struct {
	Data struct {
		TotalCredits *float64 `json:"total_credits"`
		TotalUsage   *float64 `json:"total_usage"`
	} `json:"data"`
}

// Run polls until ctx ends: once at start, then every Interval.
func (m *PlatformCreditMonitor) Run(ctx context.Context) {
	interval := m.Interval
	if interval <= 0 {
		interval = PlatformCreditPollInterval
	}
	core.Poll(ctx, "agent platform model credit", interval, m.Poll)
}

// Poll reads the account once and records the balance. A failed read keeps
// the last balance and its timestamp, which PlatformModelCreditStale watches.
func (m *PlatformCreditMonitor) Poll(ctx context.Context) error {
	balance, err := m.fetch(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.balance, m.lastSuccess = balance, time.Now()
	m.mu.Unlock()
	return nil
}

func (m *PlatformCreditMonitor) fetch(ctx context.Context) (float64, error) {
	url := m.URL
	if url == "" {
		url = openRouterCreditsURL
	}
	var c openRouterCredits
	if err := getOpenRouterJSON(ctx, m.HTTP, url, m.Key, platformCreditFetchTimeout, maxPlatformCreditBody, func(r io.Reader) error {
		if json.NewDecoder(r).Decode(&c) != nil {
			return fmt.Errorf("malformed response")
		}
		return nil
	}); err != nil {
		return 0, fmt.Errorf("read account credits: %w", err)
	}
	valid := func(v *float64) bool { return v != nil && !math.IsNaN(*v) && !math.IsInf(*v, 0) && *v >= 0 }
	if !valid(c.Data.TotalCredits) || !valid(c.Data.TotalUsage) {
		return 0, fmt.Errorf("read account credits: response carries no credits or usage")
	}
	return *c.Data.TotalCredits - *c.Data.TotalUsage, nil
}

// getOpenRouterJSON GETs url (with bearer as the Authorization token when
// set) within timeout and hands decode the body, bounded to maxBytes. Its
// errors never carry the bearer token or the response body: net/http errors
// name only the method and URL, and a non-200 reports just the status.
func getOpenRouterJSON(ctx context.Context, client *http.Client, url, bearer string, timeout time.Duration, maxBytes int64, decode func(io.Reader) error) error {
	if client == nil {
		client = http.DefaultClient
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body := io.LimitReader(resp.Body, maxBytes)
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, body)
		return fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	return decode(body)
}
