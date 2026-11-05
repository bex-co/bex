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

package agentsessions

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
)

// The per-session spend ceiling (ADR094 D8) and the cost reading that goes
// with it (D7). Both speak the ADR091 customer price in micro-USD.
const (
	// SpendLimitUnavailableCode refuses a maxSpendUSD on a session bex cannot
	// meter, never silently ignoring it: only the platform model route records
	// a price (ADR091). A session on the workspace's own model key is billed by
	// its provider.
	SpendLimitUnavailableCode = "AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE"
	// SpendLimitInvalidCode refuses a maxSpendUSD that is not a usable amount.
	SpendLimitInvalidCode = "AGENT_SESSION_SPEND_LIMIT_INVALID"
	spendLimitField       = "maxSpendUSD"
	// maxSessionSpendUSD bounds a ceiling. The workspace's monthly cap binds
	// far below it; the bound only keeps the micro-USD value inside int64 and
	// the input sane.
	maxSessionSpendUSD = 100000
)

// parseSessionCeiling validates a create's maxSpendUSD and returns it in
// micro-USD; nil when the request sets none. metered is whether the new
// session's model requests are priced (the platform route, no own key).
func parseSessionCeiling(usd *float64, metered bool) (*int64, error) {
	if usd == nil {
		return nil, nil
	}
	micro := pricing.USDToMicro(*usd)
	if math.IsNaN(*usd) || math.IsInf(*usd, 0) || *usd <= 0 || *usd > maxSessionSpendUSD || micro <= 0 {
		return nil, core.NewBadRequestError(SpendLimitInvalidCode,
			fmt.Sprintf("maxSpendUSD must be more than $0 and at most $%d", maxSessionSpendUSD), map[string]any{"field": spendLimitField})
	}
	if !metered {
		return nil, core.NewBadRequestError(SpendLimitUnavailableCode,
			"maxSpendUSD needs the platform model route, which prices each request; this session runs on the workspace's own model key (or a workspace the route does not cover), whose spend bex cannot meter. Remove maxSpendUSD, or use the platform model",
			map[string]any{"field": spendLimitField})
	}
	return &micro, nil
}

// sessionMetered reports whether a session of agent in workspaceID would run
// on the platform route (and so record a price): the route serves the agent
// profile in a workspace it covers that has not set its own key.
func sessionMetered(workspaceID, agent string, byoKey bool) bool {
	return agentsession.PlatformModelKeyReady(workspaceID, byoKey) && agentsession.PinnedAgent(agent)
}

// microUSDDecimal is micro-USD as an exact decimal string with at least two
// and at most six fractional digits: 420000 -> "0.42", 1234 -> "0.001234".
func microUSDDecimal(micro int64) string {
	whole, frac := micro/1_000_000, micro%1_000_000
	digits := strings.TrimRight(fmt.Sprintf("%06d", frac), "0")
	for len(digits) < 2 {
		digits += "0"
	}
	return strconv.FormatInt(whole, 10) + "." + digits
}
