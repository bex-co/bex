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

package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Readiness is the provider-neutral customer-billing state returned unchanged
// by REST, GraphQL, MCP, and the dashboard. Stripe object ids stay server-side:
// callers need state and hosted URLs, never provider credentials or topology.
type Readiness struct {
	WorkspaceID        string `json:"workspaceId"`
	Mode               string `json:"mode"`
	CustomerReady      bool   `json:"customerReady"`
	SubscriptionReady  bool   `json:"subscriptionReady"`
	PaymentMethodReady bool   `json:"paymentMethodReady"`
	// PaymentMethodBrand and PaymentMethodLast4 describe the card on file
	// ("visa", "4242") so a billing page can name it rather than say only that
	// one exists. Both empty when there is no method, when it is not a card, or
	// when the provider did not expand it — presenters fall back to the boolean.
	// Neither is a credential: the provider's payment-method id stays
	// server-side, as does every other Stripe object id.
	PaymentMethodBrand string `json:"paymentMethodBrand,omitempty"`
	PaymentMethodLast4 string `json:"paymentMethodLast4,omitempty"`
	// PaymentMethodRequired is the platform's BEX_REQUIRE_PAYMENT_METHOD gate
	// (ADR046), not whether this workspace already has a card. The dashboard
	// create-workspace flow uses it to disable Create on Pro/Scale until the
	// current workspace can bind a method; false ⇒ prior ungated behavior.
	PaymentMethodRequired bool `json:"paymentMethodRequired"`
	// PaymentMethodOnboardingRequired is the sign-up wall's one input: true when
	// this workspace cannot use ANY resource until a payment method is bound —
	// the gate runs in `all` mode (ADR075 D7) and the same marker read every
	// create consults (PaymentGate.RequirePaymentMethod) refuses the workspace
	// right now (not bound, not Mode-A excluded, not comped). Derived from the
	// gate itself rather than from PaymentMethodReady so the dashboard's wall
	// and the API's 402 cannot disagree; false whenever a create would pass.
	PaymentMethodOnboardingRequired bool          `json:"paymentMethodOnboardingRequired"`
	Tax                             TaxReadiness  `json:"tax"`
	Lifecycle                       LifecycleView `json:"lifecycle"`
}

type LifecycleView struct {
	Status           string   `json:"status"`
	Reason           string   `json:"reason,omitempty"`
	GraceDeadline    string   `json:"graceDeadline,omitempty"`
	EnforcementOwned bool     `json:"enforcementOwned"`
	RecoveryPending  bool     `json:"recoveryPending"`
	AllowedActions   []string `json:"allowedActions"`
	UpdatedAt        string   `json:"updatedAt,omitempty"`
}

// TaxReadiness separates operator configuration from per-subscription
// activation. Configured is true only after the canonical product code,
// explicit price behavior, and an active registration are verified.
type TaxReadiness struct {
	Configured        bool   `json:"configured"`
	Enabled           bool   `json:"enabled"`
	Reason            string `json:"reason,omitempty"`
	ProductTaxCode    string `json:"productTaxCode,omitempty"`
	TaxBehavior       string `json:"taxBehavior,omitempty"`
	RegistrationCount int    `json:"registrationCount"`
}

// HostedSession is the only data bex returns for Stripe-hosted interactions.
// URL is short-lived and contains no bex or Stripe server credential.
type HostedSession struct {
	URL       string `json:"url"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}

// CheckoutRequest and PortalRequest carry only redirect destinations. The
// provider validates them against its configured first-party dashboard origin.
type CheckoutRequest struct {
	SuccessURL string `json:"successUrl"`
	CancelURL  string `json:"cancelUrl"`
}

type PortalRequest struct {
	ReturnURL string `json:"returnUrl"`
}

// HostedProvider is the narrow Stripe seam used by the authorized core. It is
// intentionally not a generic payments abstraction: bex already owns exactly
// one metered Subscription and setup-mode Checkout augments that contract.
type HostedProvider interface {
	Readiness(ctx context.Context, workspaceID string) (Readiness, error)
	CreateCheckoutSession(ctx context.Context, workspaceID string, req CheckoutRequest) (HostedSession, error)
	CreatePortalSession(ctx context.Context, workspaceID string, req PortalRequest) (HostedSession, error)
}

// Service owns customer-billing authorization and workspace resolution. Every
// adapter calls these three verbs so session ownership and error semantics
// cannot drift between surfaces.
type Service struct {
	*core.Base
	Provider HostedProvider
	State    LifecycleReader
}

type LifecycleReader interface {
	GetBillingLifecycle(context.Context, string) (store.BillingLifecycle, error)
}

func (s *Service) Status(ctx context.Context, workspaceID string) (Readiness, error) {
	ctx, tenantID, err := s.authorize(ctx, workspaceID)
	if err != nil {
		return Readiness{}, err
	}
	status, err := s.Provider.Readiness(ctx, tenantID)
	if err != nil {
		return Readiness{}, fmt.Errorf("%w: %v", core.ErrBillingUnavailable, err)
	}
	status.WorkspaceID = tenantID
	status.PaymentMethodRequired = s.Payment != nil
	status.PaymentMethodOnboardingRequired, err = s.onboardingRequired(ctx, tenantID)
	if err != nil {
		return Readiness{}, fmt.Errorf("%w: %v", core.ErrBillingUnavailable, err)
	}
	status.Lifecycle, err = s.lifecycle(ctx, tenantID)
	if err != nil {
		return Readiness{}, fmt.Errorf("%w: %v", core.ErrBillingUnavailable, err)
	}
	return status, nil
}

// onboardingRequired asks the injected PaymentGate the exact question a create
// would ask, but only when the gate covers every plan (`all` mode): in
// paid-intent-only mode a card-less workspace can still run the free tier, so
// there is nothing to wall at sign-up. A refused workspace reads true; a marker
// read failure surfaces as the same ErrBillingUnavailable as the rest of Status.
func (s *Service) onboardingRequired(ctx context.Context, workspaceID string) (bool, error) {
	if s.Payment == nil || !s.PaymentAllPlans {
		return false, nil
	}
	err := s.RequirePaymentMethod(ctx, workspaceID)
	switch {
	case err == nil:
		return false, nil
	case errors.Is(err, core.ErrPaymentRequired):
		return true, nil
	default:
		return false, err
	}
}

func (s *Service) lifecycle(ctx context.Context, workspaceID string) (LifecycleView, error) {
	if s.State == nil {
		return lifecycleView(store.BillingLifecycle{Status: store.BillingHealthy}), nil
	}
	state, err := s.State.GetBillingLifecycle(ctx, workspaceID)
	if errors.Is(err, store.ErrNotFound) {
		return lifecycleView(store.BillingLifecycle{Status: store.BillingHealthy}), nil
	}
	if err != nil {
		return LifecycleView{}, err
	}
	return lifecycleView(state), nil
}

func lifecycleView(state store.BillingLifecycle) LifecycleView {
	status := state.Status
	if status == "" {
		status = store.BillingHealthy
	}
	v := LifecycleView{Status: status, Reason: state.Reason, AllowedActions: []string{"update_payment_method", "open_portal"}}
	if state.GraceDeadline != nil {
		v.GraceDeadline = state.GraceDeadline.UTC().Format(time.RFC3339)
	}
	if !state.UpdatedAt.IsZero() {
		v.UpdatedAt = state.UpdatedAt.UTC().Format(time.RFC3339)
	}
	v.EnforcementOwned = status == store.BillingEnforcing || status == store.BillingEnforced || status == store.BillingRecovering
	v.RecoveryPending = status == store.BillingRecovering
	return v
}

func (s *Service) Checkout(ctx context.Context, workspaceID string, req CheckoutRequest) (HostedSession, error) {
	ctx, tenantID, err := s.authorize(ctx, workspaceID)
	if err != nil {
		return HostedSession{}, err
	}
	// codex round-8 #8: a hosted Checkout Session binds a payment method to the
	// workspace — durable financial-capability issuance, so re-assert
	// can_manage_billing uncached (a member revoked inside PositiveTTL must not
	// mint one last session).
	if err := s.AuthorizeFresh(ctx, core.RelCanManageBilling); err != nil {
		return HostedSession{}, err
	}
	session, err := s.Provider.CreateCheckoutSession(ctx, tenantID, req)
	if err != nil {
		return HostedSession{}, classifyProviderError(err)
	}
	return session, nil
}

func (s *Service) Portal(ctx context.Context, workspaceID string, req PortalRequest) (HostedSession, error) {
	ctx, tenantID, err := s.authorize(ctx, workspaceID)
	if err != nil {
		return HostedSession{}, err
	}
	// codex round-8 #8: same class as Checkout — the Portal Session is a durable
	// financial surface, re-asserted uncached.
	if err := s.AuthorizeFresh(ctx, core.RelCanManageBilling); err != nil {
		return HostedSession{}, err
	}
	session, err := s.Provider.CreatePortalSession(ctx, tenantID, req)
	if err != nil {
		return HostedSession{}, classifyProviderError(err)
	}
	return session, nil
}

func (s *Service) authorize(ctx context.Context, workspaceID string) (context.Context, string, error) {
	if s == nil || s.Base == nil {
		return ctx, "", core.ErrBillingUnavailable
	}
	if workspaceID != "" {
		ctx = core.WithWorkspace(ctx, workspaceID)
	}
	// Billing setup and hosted-session links expose organization-level financial
	// controls, gated on the dedicated can_manage_billing relation — held by
	// Render's BILLING role and by admin (model.fga: `billing or admin`), so a
	// billing-role member manages billing without workspace-admin (w1/m60).
	if err := s.Authorize(ctx, core.RelCanManageBilling); err != nil {
		return ctx, "", err
	}
	if s.Provider == nil {
		return ctx, "", core.ErrBillingUnavailable
	}
	tenantID, ok := s.Tenant(ctx)
	if !ok || tenantID == "" || tenantID == core.DefaultTenant {
		return ctx, "", core.ErrBillingUnavailable
	}
	return ctx, tenantID, nil
}

func classifyProviderError(err error) error {
	if err == nil {
		return nil
	}
	if isInputError(err) {
		return fmt.Errorf("%w: %v", core.ErrBadRequest, err)
	}
	if isStateError(err) {
		return fmt.Errorf("%w: %v", core.ErrConflict, err)
	}
	return fmt.Errorf("%w: %v", core.ErrBillingUnavailable, err)
}
