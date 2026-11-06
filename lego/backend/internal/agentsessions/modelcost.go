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
	"context"
	"encoding/json"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ModelCostStore reads a session and its platform-route model usage per turn
// in one query (ADR091 D6); *store.PGStore satisfies it.
type ModelCostStore interface {
	AgentSessionModelUsage(ctx context.Context, sessionID string) (store.AgentSessionModelCost, error)
}

// ComputeStore reads the weighted milli-vCPU seconds the sandbox compute meter
// attributed to one session's sandboxes (store, migration 0206).
type ComputeStore interface {
	AgentSessionComputeMilliSeconds(ctx context.Context, sessionID string) (int64, error)
}

// ModelCost is what one session's platform-route model requests cost the
// customer (ADR091 D6), in total and per turn. A session on the workspace's
// own model key (BYO) records no requests: its provider bills it directly.
type ModelCost struct {
	SessionID string `json:"sessionId"`
	// ModelSource is the model route the session runs on: platform (bex
	// bills its requests) or byo (the workspace's provider bills them).
	ModelSource ModelSource `json:"modelSource"`
	// CostUSD is the session's charge so far (provider cost + 30%), rounded
	// to the cent like the usage read's agentModel amounts ("1.56").
	CostUSD  string `json:"costUsd"`
	Requests int    `json:"requests"`
	// Turns lists the turns that made model requests, oldest first. Turn 0
	// holds requests started before the first turn was accepted.
	Turns []TurnModelCost `json:"turns"`
}

// TurnModelCost is one turn's model requests, summed.
type TurnModelCost struct {
	Turn             int       `json:"turn"`
	CostUSD          string    `json:"costUsd"`
	Requests         int       `json:"requests"`
	InputTokens      int64     `json:"inputTokens"`
	OutputTokens     int64     `json:"outputTokens"`
	CacheReadTokens  int64     `json:"cacheReadTokens"`
	CacheWriteTokens int64     `json:"cacheWriteTokens"`
	Models           []string  `json:"models"`
	FirstAt          time.Time `json:"firstAt"`
	LastAt           time.Time `json:"lastAt"`
}

// ModelCost reads sessionID's platform-route model cost per turn. Requests
// are attributed to the turn that was the session's latest when they started
// (turns run one at a time). Usage events are kept 13 months.
func (s *Service) ModelCost(ctx context.Context, sessionID string) (ModelCost, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanOperate, sessionObject(sessionID)); err != nil {
		return ModelCost{}, err
	}
	if !s.enabled() {
		return ModelCost{}, core.ErrAgentSessionsUnavailable
	}
	if err := validateSessionID(sessionID); err != nil {
		return ModelCost{}, err
	}
	var read store.AgentSessionModelCost
	if s.ModelCosts != nil {
		var err error
		if read, err = s.ModelCosts.AgentSessionModelUsage(ctx, sessionID); err != nil {
			return ModelCost{}, mapStoreError(sessionID, err)
		}
	} else {
		// No usage store: the session still has to exist, and reads zero.
		record, err := s.Store.GetAgentSession(ctx, sessionID)
		if err != nil {
			return ModelCost{}, mapStoreError(sessionID, err)
		}
		read.AgentConfig = record.AgentConfig
	}
	out := ModelCost{SessionID: sessionID, Turns: make([]TurnModelCost, 0, len(read.Turns))}
	var total int64
	for _, r := range read.Turns {
		models := r.Models
		if models == nil {
			models = []string{}
		}
		total += r.PriceMicroUSD
		out.Requests += int(r.Requests)
		out.Turns = append(out.Turns, TurnModelCost{
			Turn: r.Turn, CostUSD: microUSDString(r.PriceMicroUSD), Requests: int(r.Requests),
			InputTokens: r.InputTokens, OutputTokens: r.OutputTokens,
			CacheReadTokens: r.CacheReadTokens, CacheWriteTokens: r.CacheWriteTokens,
			Models: models, FirstAt: r.FirstAt.UTC(), LastAt: r.LastAt.UTC(),
		})
	}
	out.CostUSD = microUSDString(total)
	out.ModelSource = sessionModelSource(read.AgentConfig, out.Requests)
	return out, nil
}

// microUSDString is micro-USD in the usage read's USD format, rounded
// half-up to the cent in integers first (a float 1.555 would round down).
func microUSDString(micro int64) string {
	cents := (micro + 5_000) / 10_000
	return pricing.FormatUSD(float64(cents) / 100)
}

// sessionModelSource is the model route a session was created on, from its
// stored model endpoint: the platform route's endpoint ⇒ platform, else the
// workspace's own provider. A session with billed requests is platform even
// if it was later re-dispatched onto the workspace's own key, so a non-zero
// cost is never labelled as billed by the provider.
func sessionModelSource(config json.RawMessage, requests int) ModelSource {
	var stored struct {
		ModelEndpoint string `json:"modelEndpoint"`
	}
	_ = json.Unmarshal(config, &stored)
	if requests > 0 || agentsession.IsPlatformModelEndpoint(stored.ModelEndpoint) {
		return ModelSourcePlatform
	}
	return ModelSourceBYO
}
