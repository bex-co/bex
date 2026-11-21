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

package egressquery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Instant evaluates one Prometheus instant query and returns the sum of all
// finite vector values. An empty vector is a successful zero.
func Instant(ctx context.Context, hc *http.Client, base, query string, at time.Time) (float64, error) {
	samples, err := fetchInstantVector(ctx, hc, base, query, at)
	if err != nil {
		return 0, err
	}
	return sumSamples(samples)
}

// fetchInstantVector performs the query and unwraps Prometheus' envelope down
// to the raw vector samples, rejecting every shape that is not a populated
// vector result.
func fetchInstantVector(ctx context.Context, hc *http.Client, base, query string, at time.Time) ([]instantSample, error) {
	raw, err := fetchResult(ctx, hc, base, query, at, "vector")
	if err != nil {
		return nil, err
	}
	var samples []instantSample
	if err := json.Unmarshal(raw, &samples); err != nil {
		return nil, fmt.Errorf("decode prometheus vector result: %w", err)
	}
	return samples, nil
}

// fetchResult evaluates one query at one instant and returns the result of
// the wanted type, still encoded.
func fetchResult(ctx context.Context, hc *http.Client, base, query string, at time.Time, resultType string) (json.RawMessage, error) {
	if hc == nil {
		hc = core.UpstreamClient
	}
	u := fmt.Sprintf("%s/api/v1/query?%s", strings.TrimRight(base, "/"), url.Values{
		"query": {query},
		"time":  {strconv.FormatInt(at.Unix(), 10)},
	}.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus: status %d", resp.StatusCode)
	}
	var result instantResponse
	if err := core.DecodeUpstreamJSON(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("decode prometheus response: %w", err)
	}
	if result.Status != "success" {
		return nil, fmt.Errorf("prometheus status %q", result.Status)
	}
	if result.Data == nil {
		return nil, fmt.Errorf("prometheus response has no data")
	}
	if result.Data.ResultType != resultType {
		return nil, fmt.Errorf("prometheus result type %q, want %s", result.Data.ResultType, resultType)
	}
	if len(result.Data.Result) == 0 || bytes.Equal(bytes.TrimSpace(result.Data.Result), []byte("null")) {
		return nil, fmt.Errorf("prometheus %s result is missing", resultType)
	}
	return result.Data.Result, nil
}

// Sample is one scraped value of a series, at its scrape time in Unix seconds.
type Sample struct {
	Time, Value float64
}

// RawSamples evaluates a range selector (`metric{...}[300s]`) at one instant
// and returns every matching series' scraped samples, oldest first: the values
// as Prometheus stored them, with nothing interpolated or extrapolated. A
// selector that matches nothing is an empty result, not an error. A sample
// that is malformed, not finite or negative fails the read, as in Instant.
func RawSamples(ctx context.Context, hc *http.Client, base, selector string, at time.Time) ([][]Sample, error) {
	raw, err := fetchResult(ctx, hc, base, selector, at, "matrix")
	if err != nil {
		return nil, err
	}
	var matrix []struct {
		Values [][]any `json:"values"`
	}
	if err := json.Unmarshal(raw, &matrix); err != nil {
		return nil, fmt.Errorf("decode prometheus matrix result: %w", err)
	}
	out := make([][]Sample, 0, len(matrix))
	for _, series := range matrix {
		samples := make([]Sample, 0, len(series.Values))
		for _, pair := range series.Values {
			sample, err := parseSample(pair)
			if err != nil {
				return nil, err
			}
			if n := len(samples); n > 0 && sample.Time <= samples[n-1].Time {
				return nil, fmt.Errorf("prometheus matrix samples are not in time order")
			}
			samples = append(samples, sample)
		}
		out = append(out, samples)
	}
	return out, nil
}

// parseSample reads one [timestamp, "value"] pair, refusing any that is
// malformed, not finite, or negative.
func parseSample(pair []any) (Sample, error) {
	if len(pair) != 2 {
		return Sample{}, fmt.Errorf("prometheus sample has %d value fields, want 2", len(pair))
	}
	timestamp, ok := pair[0].(float64)
	if !ok || math.IsNaN(timestamp) || math.IsInf(timestamp, 0) {
		return Sample{}, fmt.Errorf("prometheus sample timestamp is %T, want finite number", pair[0])
	}
	value, ok := pair[1].(string)
	if !ok {
		return Sample{}, fmt.Errorf("prometheus sample value is %T, want string", pair[1])
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return Sample{}, fmt.Errorf("parse prometheus sample %q: %w", value, err)
	}
	if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return Sample{}, fmt.Errorf("prometheus sample %q is not finite", value)
	}
	if parsed < 0 {
		return Sample{}, fmt.Errorf("prometheus sample %q is negative", value)
	}
	return Sample{Time: timestamp, Value: parsed}, nil
}

// sumSamples totals the vector, refusing any sample that is malformed, not
// finite, or negative — an egress meter that silently absorbed a NaN or a
// negative counter would under-bill.
func sumSamples(samples []instantSample) (float64, error) {
	var total float64
	for _, item := range samples {
		sample, err := parseSample(item.Value)
		if err != nil {
			return 0, err
		}
		total += sample.Value
		if math.IsInf(total, 0) {
			return 0, fmt.Errorf("prometheus vector sum overflowed")
		}
	}
	return total, nil
}

type instantResponse struct {
	Status string `json:"status"`
	Data   *struct {
		ResultType string          `json:"resultType"`
		Result     json.RawMessage `json:"result"`
	} `json:"data"`
}

type instantSample struct {
	Value []any `json:"value"`
}

// RegexEscape escapes a literal value embedded in a PromQL regex matcher.
func RegexEscape(value string) string {
	return regexp.QuoteMeta(value)
}
