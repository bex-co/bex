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

package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/billing"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

// failingBillingProvider fails every call with an error that names an
// internal address and a provider account, and counts the calls.
type failingBillingProvider struct{ asked int }

func (p *failingBillingProvider) fail() error {
	p.asked++
	return errors.New("dial tcp 10.0.3.7:443: connection refused (acct_1Restricted)")
}

func (p *failingBillingProvider) Readiness(context.Context, string) (billing.Readiness, error) {
	return billing.Readiness{}, p.fail()
}

func (p *failingBillingProvider) CreateCheckoutSession(context.Context, string, billing.CheckoutRequest) (billing.HostedSession, error) {
	return billing.HostedSession{}, p.fail()
}

func (p *failingBillingProvider) CreatePortalSession(context.Context, string, billing.PortalRequest) (billing.HostedSession, error) {
	return billing.HostedSession{}, p.fail()
}

// TestABillingFailureAnswersWithoutItsCause (w5/123): a failing billing
// provider answers an uncoded "billing integration unavailable" (503 on REST)
// on REST, GraphQL and MCP, without the provider's text. The provider is asked
// on every surface, so the answer is its failure, not a missing workspace.
func TestABillingFailureAnswersWithoutItsCause(t *testing.T) {
	provider := &failingBillingProvider{}
	base := &core.Base{Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{"client-1": "tea-cli", "dana": "tea-cli"}}
	h, srv := serverWith(t, base, Deps{Billing: provider})
	unavailable := codedRefusal{status: http.StatusServiceUnavailable, msg: core.ErrBillingUnavailable.Error()}
	unavailable.onREST(t, h, http.MethodGet, "/v1/workspaces/tea-cli/billing", "")
	unavailable.onGraphQL(t, h, `{ workspaceBillingReadiness(workspaceId: "tea-cli") { mode } }`)
	unavailable.onMCP(t, mcpSessionAs(t, srv, "dana"), "get_billing_readiness", map[string]any{})
	if provider.asked != 3 {
		t.Fatalf("the provider was asked %d times, want once per surface", provider.asked)
	}
}
