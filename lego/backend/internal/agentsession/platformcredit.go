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
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Platform credit backstop (ADR091 D5 "Global backstop"). The platform
// OpenRouter key carries a credit limit set in the OpenRouter console (an ops
// step, not code). bex-api reads the key's own limit and usage from
// OpenRouter's GET /api/v1/key every 10 minutes and exports them, and the
// PlatformModelCreditHigh alert fires at 80% of the limit. The key never
// leaves the Authorization header: it is not logged, labelled or returned.

// openRouterKeyURL is OpenRouter's "current API key" endpoint.
const openRouterKeyURL = openRouterEndpoint + "/v1/key"

const (
	// PlatformCreditPollInterval is how often the key's usage is read.
	PlatformCreditPollInterval = 10 * time.Minute
	platformCreditFetchTimeout = 15 * time.Second
	maxPlatformCreditBody      = 1 << 20
)

// PlatformCreditMonitor polls the platform key's credit limit and usage.
type PlatformCreditMonitor struct {
	Key      string
	URL      string
	HTTP     *http.Client
	Interval time.Duration

	limit, usage prometheus.Gauge
}

// NewPlatformCreditMonitor registers the two gauges on registerer:
// bex_agent_platform_model_credit_limit_usd (0 = no limit set) and
// bex_agent_platform_model_credit_usage_usd (usage counted against it).
func NewPlatformCreditMonitor(registerer prometheus.Registerer, key string) *PlatformCreditMonitor {
	m := &PlatformCreditMonitor{
		Key: key,
		limit: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "bex", Subsystem: "agent_platform_model", Name: "credit_limit_usd",
			Help: "Credit limit (USD) set on the platform OpenRouter key; 0 when the key has no limit.",
		}),
		usage: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "bex", Subsystem: "agent_platform_model", Name: "credit_usage_usd",
			Help: "Platform OpenRouter key spend (USD) counted against its credit limit (limit - limit_remaining; all-time usage when no limit is set).",
		}),
	}
	registerer.MustRegister(m.limit, m.usage)
	return m
}

// openRouterKeyInfo is the subset of GET /api/v1/key's data bex reads.
// limit and limit_remaining are null for an unlimited key.
type openRouterKeyInfo struct {
	Data struct {
		Limit          *float64 `json:"limit"`
		LimitRemaining *float64 `json:"limit_remaining"`
		Usage          *float64 `json:"usage"`
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

// Poll reads the key once and sets the gauges. A failed read keeps the last
// values (the alert's own staleness bound covers a long outage).
func (m *PlatformCreditMonitor) Poll(ctx context.Context) error {
	limit, usage, err := m.fetch(ctx)
	if err != nil {
		return err
	}
	m.limit.Set(limit)
	m.usage.Set(usage)
	return nil
}

func (m *PlatformCreditMonitor) fetch(ctx context.Context) (limit, usage float64, err error) {
	url := m.URL
	if url == "" {
		url = openRouterKeyURL
	}
	var info openRouterKeyInfo
	if err := getOpenRouterJSON(ctx, m.HTTP, url, m.Key, platformCreditFetchTimeout, maxPlatformCreditBody, func(r io.Reader) error {
		if json.NewDecoder(r).Decode(&info) != nil {
			return fmt.Errorf("malformed response")
		}
		return nil
	}); err != nil {
		return 0, 0, fmt.Errorf("read key usage: %w", err)
	}
	valid := func(v *float64) bool { return v != nil && !math.IsNaN(*v) && !math.IsInf(*v, 0) && *v >= 0 }
	d := info.Data
	switch {
	case valid(d.Limit) && *d.Limit > 0 && valid(d.LimitRemaining):
		return *d.Limit, math.Max(*d.Limit-*d.LimitRemaining, 0), nil
	case valid(d.Limit) && *d.Limit > 0 && valid(d.Usage):
		return *d.Limit, *d.Usage, nil
	case valid(d.Usage):
		return 0, *d.Usage, nil // no limit set: nothing to alert against
	}
	return 0, 0, fmt.Errorf("read key usage: response carries no usage")
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
