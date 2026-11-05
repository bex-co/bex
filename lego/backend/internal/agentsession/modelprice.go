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
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// Computed-cost fallback (ADR091 D2.2). When OpenRouter reports no
// usage.cost for a platform-route exchange, bex-api prices it from
// OpenRouter's public model list: per-token USD prices plus time-of-day
// `overrides` (UTC weekday/hour windows, e.g. a weekday 2× peak). The table
// is refreshed in the background at most hourly, and the last good copy is
// kept when a refresh fails; pricing never waits on the network.

// openRouterModelsURL is OpenRouter's public model list.
const openRouterModelsURL = openRouterEndpoint + "/v1/models"

const (
	// priceTableRefresh is how long a fetched table is used before a refresh.
	priceTableRefresh = time.Hour
	// priceTableRetry spaces refresh attempts after a failure, so a down
	// endpoint costs one request a minute, not one per usage report.
	priceTableRetry = time.Minute
	// priceTableFetchTimeout bounds one background fetch.
	priceTableFetchTimeout = 10 * time.Second
	// maxPriceTableBytes bounds the model list body (a few MB today).
	maxPriceTableBytes = 64 << 20
)

// tokenPrices is one set of USD-per-token prices. A nil field is absent from
// the source (for an override: inherit the base price).
type tokenPrices struct {
	prompt, completion, cacheRead, cacheWrite *float64
}

// priceOverride is one OpenRouter pricing override window. Minutes are
// minutes after UTC midnight; end is exclusive and 24*60 means midnight. A
// window with start > end wraps past midnight.
type priceOverride struct {
	days       [7]bool // by time.Weekday; all false ⇒ every day
	anyDay     bool
	start, end int
	prices     tokenPrices
}

func (o priceOverride) matches(at time.Time) bool {
	if !o.anyDay && !o.days[at.Weekday()] {
		return false
	}
	minute := at.Hour()*60 + at.Minute()
	if o.start <= o.end {
		return minute >= o.start && minute < o.end
	}
	return minute >= o.start || minute < o.end
}

type modelPrice struct {
	base      tokenPrices
	overrides []priceOverride
}

// effective resolves the prices in force at at: the first matching override,
// field by field over the base. Cache write is input_cache_write when listed,
// else the (effective) prompt price; cache read likewise falls back to
// prompt, so a missing cache price never under-recovers cost.
func (m modelPrice) effective(at time.Time) (prompt, completion, cacheRead, cacheWrite float64) {
	p := m.base
	for _, o := range m.overrides {
		if !o.matches(at) {
			continue
		}
		if o.prices.prompt != nil {
			p.prompt = o.prices.prompt
		}
		if o.prices.completion != nil {
			p.completion = o.prices.completion
		}
		if o.prices.cacheRead != nil {
			p.cacheRead = o.prices.cacheRead
		}
		if o.prices.cacheWrite != nil {
			p.cacheWrite = o.prices.cacheWrite
		}
		break
	}
	val := func(v *float64, fallback float64) float64 {
		if v == nil {
			return fallback
		}
		return *v
	}
	prompt = val(p.prompt, 0)
	return prompt, val(p.completion, 0), val(p.cacheRead, prompt), val(p.cacheWrite, prompt)
}

// OpenRouterPriceTable is the cached OpenRouter price list. The zero value
// fetches OpenRouter's public model list with http.DefaultClient; it is safe
// for concurrent use.
type OpenRouterPriceTable struct {
	URL  string
	HTTP *http.Client
	Now  func() time.Time

	snapshot atomic.Pointer[priceSnapshot]
	// refreshing admits one background refresh at a time.
	refreshing atomic.Bool
	// attemptedAt is the UnixNano start of the last refresh attempt.
	attemptedAt atomic.Int64
}

// priceSnapshot is one successfully fetched table.
type priceSnapshot struct {
	models    map[string]modelPrice
	fetchedAt time.Time
}

// Start begins the first fetch in the background, so the table is usually
// warm before the first exchange needs a computed cost.
func (t *OpenRouterPriceTable) Start(ctx context.Context) {
	t.refreshInBackground(ctx)
}

// Cost is the USD cost of one exchange of model starting at at, from the
// table's prices in force then. ok is false when the model is not in the
// table (or no table was fetched yet). It reads the current snapshot only; a
// stale or missing table triggers a background refresh.
func (t *OpenRouterPriceTable) Cost(model string, at time.Time, in, out, cacheRead, cacheWrite int64) (usd float64, ok bool) {
	snap := t.snapshot.Load()
	if t.due(snap, t.now()) {
		t.refreshInBackground(context.Background())
	}
	if snap == nil {
		return 0, false
	}
	price, ok := snap.models[model]
	if !ok {
		return 0, false
	}
	prompt, completion, read, write := price.effective(at.UTC())
	return float64(in)*prompt + float64(out)*completion + float64(cacheRead)*read + float64(cacheWrite)*write, true
}

func (t *OpenRouterPriceTable) now() time.Time {
	if t.Now != nil {
		return t.Now()
	}
	return time.Now()
}

// due reports whether snap needs a refresh at now: it is missing or older
// than priceTableRefresh, and the last attempt is at least priceTableRetry ago.
func (t *OpenRouterPriceTable) due(snap *priceSnapshot, now time.Time) bool {
	if snap != nil && now.Sub(snap.fetchedAt) < priceTableRefresh {
		return false
	}
	last := t.attemptedAt.Load()
	return last == 0 || now.Sub(time.Unix(0, last)) >= priceTableRetry
}

// refreshInBackground starts a refresh unless one is already running.
func (t *OpenRouterPriceTable) refreshInBackground(ctx context.Context) {
	if !t.refreshing.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer t.refreshing.Store(false)
		t.refresh(ctx)
	}()
}

// refresh fetches the table once. A failed fetch keeps the last good copy.
func (t *OpenRouterPriceTable) refresh(ctx context.Context) {
	now := t.now()
	t.attemptedAt.Store(now.UnixNano())
	models, err := t.fetch(ctx)
	if err != nil {
		kept := 0
		if snap := t.snapshot.Load(); snap != nil {
			kept = len(snap.models)
		}
		log.Printf("agent model price table: refresh failed, keeping the last good copy (%d models): %v", kept, err)
		return
	}
	t.snapshot.Store(&priceSnapshot{models: models, fetchedAt: now})
}

func (t *OpenRouterPriceTable) fetch(ctx context.Context) (map[string]modelPrice, error) {
	url := t.URL
	if url == "" {
		url = openRouterModelsURL
	}
	var models map[string]modelPrice
	err := getOpenRouterJSON(ctx, t.HTTP, url, "", priceTableFetchTimeout, maxPriceTableBytes, func(r io.Reader) error {
		var err error
		models, err = parseOpenRouterModels(r)
		return err
	})
	return models, err
}

// priceString is an OpenRouter price: a decimal string of USD per token
// (a number is accepted too). Absent or null stays nil.
type priceString struct{ v *float64 }

func (p *priceString) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("price %s: %w", b, err)
	}
	p.v = &v
	return nil
}

type openRouterPrices struct {
	Prompt          priceString `json:"prompt"`
	Completion      priceString `json:"completion"`
	InputCacheRead  priceString `json:"input_cache_read"`
	InputCacheWrite priceString `json:"input_cache_write"`
}

func (p openRouterPrices) tokenPrices() (tokenPrices, bool) {
	out := tokenPrices{prompt: p.Prompt.v, completion: p.Completion.v, cacheRead: p.InputCacheRead.v, cacheWrite: p.InputCacheWrite.v}
	for _, v := range []*float64{out.prompt, out.completion, out.cacheRead, out.cacheWrite} {
		// OpenRouter lists "-1" for routers whose price is not fixed; such
		// a model cannot be priced from the table.
		if v != nil && *v < 0 {
			return tokenPrices{}, false
		}
	}
	return out, true
}

var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "monday": time.Monday, "tuesday": time.Tuesday, "wednesday": time.Wednesday,
	"thursday": time.Thursday, "friday": time.Friday, "saturday": time.Saturday,
}

// hhmmMinutes converts an OpenRouter HHMM time (100 = 01:00) to minutes.
func hhmmMinutes(v int) (int, bool) {
	h, m := v/100, v%100
	if v < 0 || h > 24 || m > 59 || (h == 24 && m != 0) {
		return 0, false
	}
	return h*60 + m, true
}

func parseOpenRouterModels(r io.Reader) (map[string]modelPrice, error) {
	var body struct {
		Data []struct {
			ID            string `json:"id"`
			CanonicalSlug string `json:"canonical_slug"`
			Pricing       struct {
				openRouterPrices
				Overrides []struct {
					openRouterPrices
					UTCDays  []string `json:"utc_days"`
					UTCStart *int     `json:"utc_start"`
					UTCEnd   *int     `json:"utc_end"`
				} `json:"overrides"`
			} `json:"pricing"`
		} `json:"data"`
	}
	if err := json.NewDecoder(r).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode models: %w", err)
	}
	if len(body.Data) == 0 {
		return nil, fmt.Errorf("models list is empty")
	}
	models := make(map[string]modelPrice, len(body.Data))
model:
	for _, m := range body.Data {
		base, ok := m.Pricing.tokenPrices()
		if m.ID == "" || !ok || base.prompt == nil || base.completion == nil {
			continue
		}
		price := modelPrice{base: base}
		for _, o := range m.Pricing.Overrides {
			prices, ok := o.tokenPrices()
			if !ok {
				continue model
			}
			po := priceOverride{prices: prices, start: 0, end: 24 * 60, anyDay: len(o.UTCDays) == 0}
			for _, d := range o.UTCDays {
				wd, known := weekdays[strings.ToLower(strings.TrimSpace(d))]
				if !known {
					continue model // an unreadable window must not misprice
				}
				po.days[wd] = true
			}
			if o.UTCStart != nil {
				if po.start, ok = hhmmMinutes(*o.UTCStart); !ok {
					continue model
				}
			}
			if o.UTCEnd != nil {
				if po.end, ok = hhmmMinutes(*o.UTCEnd); !ok {
					continue model
				}
				if po.end == 0 {
					po.end = 24 * 60 // 0 ends the window at midnight
				}
			}
			price.overrides = append(price.overrides, po)
		}
		models[m.ID] = price
		if m.CanonicalSlug != "" && m.CanonicalSlug != m.ID {
			if _, taken := models[m.CanonicalSlug]; !taken {
				models[m.CanonicalSlug] = price
			}
		}
	}
	return models, nil
}
