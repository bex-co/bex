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
	"net/http"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Platform model route metering and quota (w10/m14 t003/t005, ADR090 §
// Platform OpenRouter route). The gateway model proxy reads each
// platform-key exchange's token usage from the provider response and reports
// it here over the same gateway-only HMAC channel as the mint; bex-api
// records it into usage_hourly (ADR023), where the estimate and the billing
// outbox (ADR040) read it like every other meter. The mint compares the
// workspace's month-to-date tokens with its plan quota before releasing the
// platform key.

// InternalModelUsagePath is the usage-report sibling of the model mint on
// bex-api's :8091 listener.
const InternalModelUsagePath = InternalModelMintPath + "/usage"

// ModelQuotaExceededCode is the stable code of a platform-route quota refusal:
// the session's failureReasonCode and the gateway's error body both carry it.
const ModelQuotaExceededCode = "AGENT_MODEL_QUOTA_EXCEEDED"

// ModelQuotaExceededReason is the terminal failureReason of a session whose
// workspace exhausted its plan's monthly platform model token quota.
const ModelQuotaExceededReason = "this workspace has used its plan's monthly platform model token quota (" + ModelQuotaExceededCode + "); upgrade the workspace plan or wait for the quota to reset on the 1st (UTC), then start a new session"

// ModelFailureCauseQuotaExceeded is ModelMintRequest.Cause on a report that a
// mint was refused for quota. Empty Cause is the original auth-failure report.
const ModelFailureCauseQuotaExceeded = "quota_exceeded"

// RefusalModelQuotaExceeded is RefusalHeader's value for ErrModelQuotaExceeded.
const RefusalModelQuotaExceeded = "model-quota-exceeded"

// ErrModelQuotaExceeded refuses a platform-route mint once the workspace's
// month-to-date platform tokens reach its plan quota. It deliberately does
// NOT wrap ErrForbidden: it is a standing state, never the create-time race
// the gateway retries, and it travels as HTTP 429.
var ErrModelQuotaExceeded = errors.New("agent model quota exceeded")

// maxReportedTokens bounds one exchange's reported count per meter; no
// provider response comes near it, so a larger value is a bug, not usage.
const maxReportedTokens = 1 << 32

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
	Nonce            string `json:"nonce,omitempty"`
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
	recorded, err := r.Store.RecordAgentModelUsage(ctx, store.AgentModelUsage{
		RequestID: rep.RequestID, WorkspaceID: rep.Workspace, SessionID: rep.SessionID,
		Model: rep.Model, At: r.now(),
		InputTokens: rep.InputTokens, OutputTokens: rep.OutputTokens,
		CacheReadTokens: rep.CacheReadTokens, CacheWriteTokens: rep.CacheWriteTokens,
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

// ReportQuotaExceeded tells bex-api to fail the session whose platform-route
// mint was refused for quota, with ModelQuotaExceededReason. It is the
// auth-failure verb with a cause, so the same live-sandbox binding applies.
func (c *ModelClient) ReportQuotaExceeded(ctx context.Context, req ModelMintRequest) error {
	req.Cause = ModelFailureCauseQuotaExceeded
	return c.ReportAuthFailure(ctx, req)
}

// ModelQuotaStore is the store surface of the mint-time quota check;
// *store.PGStore satisfies it.
type ModelQuotaStore interface {
	GetTenant(context.Context, string) (store.Tenant, error)
	AgentModelTokensMonthToDate(context.Context, string, time.Time) (int64, error)
}

// checkModelQuota refuses with ErrModelQuotaExceeded once workspaceID's
// month-to-date platform tokens reach its plan's quota. With no quota on the
// sheet for any plan (the shipped default) it reads nothing and allows.
func (m *ModelMinter) checkModelQuota(ctx context.Context, workspaceID string) error {
	quotaFor := m.QuotaFor
	if quotaFor == nil {
		if !pricing.Default.AgentModelQuotaConfigured() {
			return nil
		}
		quotaFor = pricing.Default.AgentModelMonthlyTokenQuota
	}
	if m.Quota == nil {
		return nil
	}
	tenant, err := m.Quota.GetTenant(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("agent model quota: read workspace plan: %w", err)
	}
	limit, limited := quotaFor(tenant.Plan)
	if !limited {
		return nil
	}
	used, err := m.Quota.AgentModelTokensMonthToDate(ctx, workspaceID, m.now())
	if err != nil {
		return fmt.Errorf("agent model quota: read month-to-date tokens: %w", err)
	}
	if used >= limit {
		return ErrModelQuotaExceeded
	}
	return nil
}

// modelFailureReason maps an auth-failure report's cause to the terminal
// reason and stable code it records. An unknown cause is refused.
func modelFailureReason(cause string) (reason, code string, err error) {
	switch cause {
	case "":
		return ModelAuthFailureReason, "", nil
	case ModelFailureCauseQuotaExceeded:
		return ModelQuotaExceededReason, ModelQuotaExceededCode, nil
	default:
		return "", "", fmt.Errorf("%w: unknown failure cause", ErrInvalidRequest)
	}
}

// failureCodeStamper is the optional store surface that records a stable
// failureReasonCode after the terminal CAS; *store.PGStore satisfies it.
type failureCodeStamper interface {
	StampAgentSessionFailureCode(ctx context.Context, id, reason, code string) error
}
