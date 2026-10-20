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

package metrics

import (
	"fmt"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// CPU interval aggregations other than the default AVG (MetricQuery.
// CPUAggregation).
const (
	cpuAggregationMax = "MAX"
	cpuAggregationMin = "MIN"
)

// parseCPUAggregation reads Render's CPU interval aggregation under the
// caller's parameter name — REST and GraphQL aggregationMethod, MCP
// cpuUsageAggregationMethod — so the three surfaces share one vocabulary and
// one refusal. Empty and AVG are the default ("").
func parseCPUAggregation(param, method string) (string, error) {
	switch m := strings.ToUpper(strings.TrimSpace(method)); m {
	case "", "AVG":
		return "", nil
	case cpuAggregationMax, cpuAggregationMin:
		return m, nil
	default:
		return "", fmt.Errorf("%w: unknown %s %q (want AVG|MAX|MIN)", core.ErrBadRequest, param, method)
	}
}

// checkCPUAggregation refuses a CPU interval aggregation on any metric but cpu,
// as Render scopes aggregationMethod to /metrics/cpu.
func checkCPUAggregation(metric, agg string) error {
	if agg != "" && metric != MetricCPU {
		return fmt.Errorf("%w: aggregationMethod applies only to cpu, not %s", core.ErrBadRequest, metric)
	}
	return nil
}

// cpuAggregationFor is agg for the cpu metric and "" for every other read that
// shares a funnel with it (limits, instance count).
func cpuAggregationFor(metric, agg string) string {
	if metric == MetricCPU {
		return agg
	}
	return ""
}
