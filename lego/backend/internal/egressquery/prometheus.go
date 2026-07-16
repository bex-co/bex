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
)

// Instant evaluates one Prometheus instant query and returns the sum of all
// finite vector values. An empty vector is a successful zero.
func Instant(ctx context.Context, hc *http.Client, base, query string, at time.Time) (float64, error) {
	if hc == nil {
		hc = http.DefaultClient
	}
	u := fmt.Sprintf("%s/api/v1/query?%s", strings.TrimRight(base, "/"), url.Values{
		"query": {query},
		"time":  {strconv.FormatInt(at.Unix(), 10)},
	}.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("prometheus: status %d", resp.StatusCode)
	}
	var result instantResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode prometheus response: %w", err)
	}
	if result.Status != "success" {
		return 0, fmt.Errorf("prometheus status %q", result.Status)
	}
	if result.Data == nil {
		return 0, fmt.Errorf("prometheus response has no data")
	}
	if result.Data.ResultType != "vector" {
		return 0, fmt.Errorf("prometheus result type %q, want vector", result.Data.ResultType)
	}
	if len(result.Data.Result) == 0 || bytes.Equal(bytes.TrimSpace(result.Data.Result), []byte("null")) {
		return 0, fmt.Errorf("prometheus vector result is missing")
	}
	var samples []instantSample
	if err := json.Unmarshal(result.Data.Result, &samples); err != nil {
		return 0, fmt.Errorf("decode prometheus vector result: %w", err)
	}
	var total float64
	for _, item := range samples {
		if len(item.Value) != 2 {
			return 0, fmt.Errorf("prometheus vector sample has %d value fields, want 2", len(item.Value))
		}
		timestamp, ok := item.Value[0].(float64)
		if !ok || math.IsNaN(timestamp) || math.IsInf(timestamp, 0) {
			return 0, fmt.Errorf("prometheus vector sample timestamp is %T, want finite number", item.Value[0])
		}
		value, ok := item.Value[1].(string)
		if !ok {
			return 0, fmt.Errorf("prometheus vector sample value is %T, want string", item.Value[1])
		}
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, fmt.Errorf("parse prometheus vector sample %q: %w", value, err)
		}
		if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return 0, fmt.Errorf("prometheus vector sample %q is not finite", value)
		}
		if parsed < 0 {
			return 0, fmt.Errorf("prometheus vector sample %q is negative", value)
		}
		total += parsed
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
