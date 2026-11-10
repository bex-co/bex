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

package workspaces

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
)

// A workspace's Build Pipeline settings (w6/m309, docs/ADR030 §13): Render's
// Workspace Settings → Build Pipeline → Set spend limit. A monthly cap on what
// the workspace spends on build minutes beyond its plan's included ones; once
// reached, its builds stop for the rest of the month (internal/buildminutes).
// Render's public API and MCP have no such operation, so it is GraphQL and
// dashboard only (parity by absence).

// maxPipelineSpendLimitCents bounds a limit to $1,000,000 a month: far above
// any real build bill, low enough that cents fit comfortably.
const maxPipelineSpendLimitCents = 100_000_000

// PipelineSettings is a workspace's Build Pipeline settings.
type PipelineSettings struct {
	WorkspaceID string
	// SpendLimitUSD is the monthly pipeline spend limit ("25.00"); nil when
	// the workspace has none.
	SpendLimitUSD *string
}

type pipelineSpendLimitStore interface {
	PipelineSpendLimit(ctx context.Context, workspaceID string) (*int64, error)
	SetPipelineSpendLimit(ctx context.Context, workspaceID string, cents *int64) error
}

func (s *Service) pipelineStore() (pipelineSpendLimitStore, error) {
	st, ok := s.Store.(pipelineSpendLimitStore)
	if !ok {
		return nil, core.ErrWorkspacesUnavailable
	}
	return st, nil
}

// PipelineSettings reads a workspace's Build Pipeline settings. Spend is
// billing data, so it is read with the billing relation, as billing.Status
// is; that relation is audited, and the dashboard asks only for billing
// managers, so a page view never records a denial.
func (s *Service) PipelineSettings(ctx context.Context, workspaceID string) (PipelineSettings, error) {
	id, err := s.AuthorizeWorkspace(ctx, core.RelCanManageBilling, workspaceID)
	if err != nil {
		return PipelineSettings{}, err
	}
	st, err := s.pipelineStore()
	if err != nil {
		return PipelineSettings{}, err
	}
	cents, err := st.PipelineSpendLimit(ctx, id)
	if err != nil {
		return PipelineSettings{}, mapStoreErr(err)
	}
	return pipelineSettings(id, cents), nil
}

// SetPipelineSpendLimit sets a workspace's monthly pipeline spend limit in
// dollars; nil removes it.
func (s *Service) SetPipelineSpendLimit(ctx context.Context, workspaceID string, limitUSD *string) (PipelineSettings, error) {
	id, err := s.AuthorizeWorkspace(ctx, core.RelCanManageBilling, workspaceID)
	if err != nil {
		return PipelineSettings{}, err
	}
	st, err := s.pipelineStore()
	if err != nil {
		return PipelineSettings{}, err
	}
	var cents *int64
	if limitUSD != nil {
		c, err := parseUSDCents(*limitUSD)
		if err != nil {
			return PipelineSettings{}, err
		}
		cents = &c
	}
	if err := st.SetPipelineSpendLimit(ctx, id, cents); err != nil {
		return PipelineSettings{}, mapStoreErr(err)
	}
	return pipelineSettings(id, cents), nil
}

func pipelineSettings(id string, cents *int64) PipelineSettings {
	out := PipelineSettings{WorkspaceID: id}
	if cents != nil {
		usd := pricing.FormatUSD(float64(*cents) / 100)
		out.SpendLimitUSD = &usd
	}
	return out
}

// usdAmount is a dollar amount with at most two decimals: "25", "25.5",
// "25.50", optionally after a "$".
var usdAmount = regexp.MustCompile(`^\$?(\d{1,9})(?:\.(\d{1,2}))?$`)

// parseUSDCents reads a dollar amount as cents, within
// [0, maxPipelineSpendLimitCents].
func parseUSDCents(raw string) (int64, error) {
	m := usdAmount.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return 0, invalidSpendLimit()
	}
	dollars, _ := strconv.ParseInt(m[1], 10, 64) // at most nine digits
	cents, _ := strconv.ParseInt((m[2] + "00")[:2], 10, 64)
	total := dollars*100 + cents
	if total > maxPipelineSpendLimitCents {
		return 0, invalidSpendLimit()
	}
	return total, nil
}

func invalidSpendLimit() error {
	return core.NewBadRequestError("INVALID_SPEND_LIMIT",
		fmt.Sprintf("spendLimitUsd must be a dollar amount between 0 and %d with at most two decimals", maxPipelineSpendLimitCents/100), nil)
}
