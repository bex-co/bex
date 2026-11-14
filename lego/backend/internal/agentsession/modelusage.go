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
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Platform model route metering (ADR090 §
// Platform OpenRouter route). The gateway model proxy reads each
// platform-key exchange's token usage from the provider response and reports
// it here over the same gateway-only HMAC channel as the mint; bex-api
// records it into usage_hourly (ADR023), where the estimate and the billing
// outbox (ADR040) read it like every other meter. The mint compares the
// workspace's month-to-date cost with its spend cap (spendcap.go) before
// releasing the platform key.

// InternalModelUsagePath is the usage-report sibling of the model mint on
// bex-api's :8091 listener.
const InternalModelUsagePath = InternalModelMintPath + "/usage"

// maxReportedTokens bounds one exchange's reported count per meter; no
// provider response comes near it, so a larger value is a bug, not usage.
const maxReportedTokens = 1 << 32

// maxReportedCostUSD bounds one exchange's reported upstream cost. No single
// Messages exchange comes near $100, so a larger value is a bug, not spend:
// it is ignored and the exchange priced from the table instead.
const maxReportedCostUSD = 100

// startedAtSkew bounds how far a report's StartedAt may sit from the record
// time before the record time is used to pick the price instead.
const startedAtSkew = 2 * time.Hour

// ModelUsageReport is one metered platform-route exchange. It carries no
// credential. RequestID is the gateway-minted idempotency key (an `amr-` id);
// Nonce is the per-attempt replay guard, so a retry signs a fresh nonce but
// the same RequestID and is recorded once.
type ModelUsageReport struct {
	RequestID        string `json:"requestId"`
	SessionID        string `json:"sessionId"`
	Workspace        string `json:"workspace"`
	Model            string `json:"model"`
	InputTokens      int64  `json:"inputTokens"`
	OutputTokens     int64  `json:"outputTokens"`
	CacheReadTokens  int64  `json:"cacheReadTokens"`
	CacheWriteTokens int64  `json:"cacheWriteTokens"`
	// UpstreamCostUSD is OpenRouter's reported usage.cost for the exchange
	// (ADR091 D2.1); nil when the response carried none.
	UpstreamCostUSD *float64 `json:"upstreamCostUsd,omitempty"`
	// StartedAt is when the gateway sent the exchange upstream: the instant
	// whose time-of-day price applies to a computed cost (ADR091 D2.2).
	StartedAt time.Time `json:"startedAt,omitzero"`
	Nonce     string    `json:"nonce,omitempty"`
}

// ModelUsageResponse acknowledges a report; Duplicate is true when the
// request id was already recorded (a retry after a lost acknowledgement).
type ModelUsageResponse struct {
	Acknowledged bool `json:"acknowledged"`
	Duplicate    bool `json:"duplicate,omitempty"`
}

// ModelUsageStore is the store surface the usage verb needs; *store.PGStore
// satisfies it.
type ModelUsageStore interface {
	GetAgentSession(context.Context, string) (store.AgentSession, error)
	RecordAgentModelUsage(context.Context, store.AgentModelUsage) (bool, error)
}

// ModelUsageRecorder is bex-api's internal usage verb. It trusts only an
// HMAC-authenticated gateway report, and still binds it to a real session of
// the reported workspace. It does not require the session to be live: a
// stream's usage is reported after the exchange, possibly after the turn.
type ModelUsageRecorder struct {
	Store ModelUsageStore
	Now   func() time.Time
	// Prices computes the upstream cost of an exchange that reported none
	// (ADR091 D2.2); nil ⇒ such an exchange is recorded unpriced.
	Prices ModelPricer
	// Markup is the customer price multiplier on upstream cost (ADR091 D1);
	// 0 ⇒ pricing.Default's agentModel.markup.
	Markup float64

	unpricedMu sync.Mutex
	// unpricedLogged is when each model's UNPRICED line was last logged.
	unpricedLogged map[string]time.Time
}

// unpricedLogEvery rate-limits the UNPRICED log line per model.
const unpricedLogEvery = time.Hour

// ModelPricer prices one exchange from a price table; *OpenRouterPriceTable
// satisfies it. ok is false when the model is not in the table.
type ModelPricer interface {
	Cost(model string, at time.Time, in, out, cacheRead, cacheWrite int64) (usd float64, ok bool)
}

// microUSD converts USD to micro-USD, rounded half-up.
func microUSD(usd float64) int64 {
	return int64(math.Floor(usd*1e6 + 0.5))
}

// price resolves an exchange's upstream cost, its source and the customer
// price (ADR091 D1/D2): OpenRouter's reported cost first, else a cost
// computed from pricing.yaml's list price or the price table at the request's
// start, else unpriced at zero — logged loudly, because it is spend bex
// cannot bill.
func (r *ModelUsageRecorder) price(rep ModelUsageReport, now time.Time) (upstream int64, source string, price int64) {
	var cost float64
	reported := rep.UpstreamCostUSD
	if c := reported; c != nil && (math.IsNaN(*c) || math.IsInf(*c, 0) || *c < 0 || *c > maxReportedCostUSD) {
		log.Printf("agent model usage: reported cost %v out of range, pricing from the table instead (workspace=%s session=%s request=%s)",
			*c, rep.Workspace, rep.SessionID, rep.RequestID)
		reported = nil
	}
	switch {
	case reported != nil:
		cost, source = *reported, store.AgentModelCostReported
	default:
		at := startedAt(rep, now)
		// A model bex calls directly (Muse on Meta's API, ADR108) has a list
		// price in pricing.yaml; everything else is an OpenRouter model.
		computed, ok := pricing.Default.AgentModelUpstreamCost(rep.Model, rep.InputTokens, rep.OutputTokens, rep.CacheReadTokens, rep.CacheWriteTokens)
		if !ok && r.Prices != nil {
			computed, ok = r.Prices.Cost(rep.Model, at, rep.InputTokens, rep.OutputTokens, rep.CacheReadTokens, rep.CacheWriteTokens)
		}
		if !ok || math.IsNaN(computed) || math.IsInf(computed, 0) || computed < 0 {
			if r.shouldLogUnpriced(rep.Model, now) {
				log.Printf("agent model usage: UNPRICED platform-route exchange recorded at $0 — no reported cost and no price for model %q (workspace=%s session=%s request=%s; logged once per model per hour)",
					rep.Model, rep.Workspace, rep.SessionID, rep.RequestID)
			}
			return 0, store.AgentModelCostUnpriced, 0
		}
		cost, source = computed, store.AgentModelCostComputed
	}
	markup := r.Markup
	if markup == 0 {
		markup = pricing.Default.AgentModelMarkup()
	}
	return microUSD(cost), source, microUSD(cost * markup)
}

// startedAt is the report's StartedAt, or now when it is absent or sits
// further from the record time than the gateway's clock can drift: one
// validated start drives both the time-of-day price and the per-turn
// attribution (ADR091 D2.2, D6).
func startedAt(rep ModelUsageReport, now time.Time) time.Time {
	at := rep.StartedAt.UTC()
	if at.IsZero() || at.After(now.Add(time.Minute)) || at.Before(now.Add(-startedAtSkew)) {
		return now
	}
	return at
}

// shouldLogUnpriced reports whether model's UNPRICED line is due: at most
// once per model per unpricedLogEvery, so a burst stays one loud line.
func (r *ModelUsageRecorder) shouldLogUnpriced(model string, now time.Time) bool {
	r.unpricedMu.Lock()
	defer r.unpricedMu.Unlock()
	if last, ok := r.unpricedLogged[model]; ok && now.Sub(last) < unpricedLogEvery {
		return false
	}
	if r.unpricedLogged == nil {
		r.unpricedLogged = map[string]time.Time{}
	}
	r.unpricedLogged[model] = now
	return true
}

func (r *ModelUsageRecorder) Record(ctx context.Context, rep ModelUsageReport) (ModelUsageResponse, error) {
	if r.Store == nil {
		return ModelUsageResponse{}, fmt.Errorf("agent model usage unavailable")
	}
	if kind, ok := id.KindOf(rep.RequestID); !ok || kind != id.AgentModelRequest ||
		rep.SessionID == "" || rep.Workspace == "" || !devModelIDPattern.MatchString(rep.Model) {
		return ModelUsageResponse{}, fmt.Errorf("%w: malformed usage report", ErrInvalidRequest)
	}
	for _, n := range []int64{rep.InputTokens, rep.OutputTokens, rep.CacheReadTokens, rep.CacheWriteTokens} {
		if n < 0 || n > maxReportedTokens {
			return ModelUsageResponse{}, fmt.Errorf("%w: token count out of range", ErrInvalidRequest)
		}
	}
	session, err := r.Store.GetAgentSession(ctx, rep.SessionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ModelUsageResponse{}, ErrForbidden
		}
		return ModelUsageResponse{}, err
	}
	if session.WorkspaceID != rep.Workspace {
		return ModelUsageResponse{}, ErrForbidden
	}
	// At is the record time, never the report's StartedAt: the cost meter's
	// hourly row must only change while its hour is open, which is what lets
	// the billing outbox export it right after the hour (ADR091 D3).
	now := r.now()
	upstream, source, price := r.price(rep, now)
	recorded, err := r.Store.RecordAgentModelUsage(ctx, store.AgentModelUsage{
		RequestID: rep.RequestID, WorkspaceID: rep.Workspace, SessionID: rep.SessionID,
		Model: rep.Model, At: now, StartedAt: startedAt(rep, now),
		InputTokens: rep.InputTokens, OutputTokens: rep.OutputTokens,
		CacheReadTokens: rep.CacheReadTokens, CacheWriteTokens: rep.CacheWriteTokens,
		UpstreamCostMicroUSD: upstream, CostSource: source, PriceMicroUSD: price,
	})
	if err != nil {
		return ModelUsageResponse{}, err
	}
	return ModelUsageResponse{Acknowledged: true, Duplicate: !recorded}, nil
}

func (r *ModelUsageRecorder) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

// ModelUsageHandler authenticates the gateway→bex-api usage report. Same
// signed envelope, nonce guard and :8091-only mount as ModelHandler.
type ModelUsageHandler struct {
	Secret   []byte
	Recorder *ModelUsageRecorder
	Nonce    NonceClaimer
	Now      func() time.Time
}

func (h *ModelUsageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var record func(context.Context, ModelUsageReport) (ModelUsageResponse, error)
	if h.Recorder != nil {
		record = h.Recorder.Record
	}
	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	serveSignedMint(w, r, h.Secret, now, h.Nonce, func(req ModelUsageReport) string { return req.Nonce }, record)
}

// ReportUsage posts one exchange's usage to bex-api's usage verb. It signs a
// fresh nonce per call, so the caller may retry the same report.
func (c *ModelClient) ReportUsage(ctx context.Context, rep ModelUsageReport) error {
	rep.Nonce = newNonce()
	url := strings.TrimRight(c.URL, "/") + "/usage"
	_, err := postSignedMint(ctx, url, c.Secret, c.now(), c.HTTP, rep,
		func(out ModelUsageResponse) bool { return out.Acknowledged })
	return err
}

// modelFailureReason maps an auth-failure report's cause to the terminal
// reason and stable code it records. An unknown cause is refused.
func modelFailureReason(cause string) (reason, code string, err error) {
	switch cause {
	case "":
		return ModelAuthFailureReason, "", nil
	case ModelFailureCauseSpendCapReached:
		return ModelSpendCapReachedReason, ModelSpendCapReachedCode, nil
	case ModelFailureCauseSessionSpendCapReached:
		return ModelSessionSpendCapReachedReason, ModelSpendCapReachedCode, nil
	case ModelFailureCauseBillingBalanceDue:
		return BillingBalanceDueReason, BillingBalanceDueCode, nil
	default:
		return "", "", fmt.Errorf("%w: unknown failure cause", ErrInvalidRequest)
	}
}

// failureCodeStamper is the optional store surface that records a stable
// failureReasonCode after the terminal CAS; *store.PGStore satisfies it.
type failureCodeStamper interface {
	StampAgentSessionFailureCode(ctx context.Context, id, reason, code string) error
}
