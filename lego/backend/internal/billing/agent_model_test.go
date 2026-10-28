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
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const agentModelTestModel = "deepseek/deepseek-v4.1-flash-1017"

func agentModelTokenRow() store.HourlyRow {
	return store.HourlyRow{
		WorkspaceID: "tea-a", ServiceID: "ags-x", Kind: store.UsageKindAgentModelOutputTokens,
		Tier: agentModelTestModel, ResourceKind: store.ResourceKindAgentSession,
		WindowStart: time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC), Quantity: 4321,
	}
}

// With pricing.yaml's agentModel.models empty (the shipped state) a token
// row is accounted locally at zero: no Stripe meter event, never a reject.
func TestAgentModelTokensAreNotExportedWhileUnpriced(t *testing.T) {
	if _, _, skip := stripeMeterEvent(toEvent(agentModelTokenRow())); !skip {
		t.Fatal("an unpriced token row must map to the local zero-rate path")
	}
	c, stub := newStripeTest(t, func(_ string, _ string) (int, string) {
		return 200, `{"id":"cus_1","object":"customer"}`
	})
	c.storeCustomer("tea-a", "cus_1")
	event := toEvent(agentModelTokenRow())
	result := c.IngestBatch(context.Background(), []Event{event})
	if len(result.Accepted) != 1 || result.Accepted[0] != event.TransactionID || len(result.Failed) != 0 {
		t.Fatalf("IngestBatch = %+v, want the row accepted locally", result)
	}
	if got := stub.count("/billing/meter_events"); got != 0 {
		t.Fatalf("meter events posted = %d, want 0 while unpriced", got)
	}
}

// Once a model is priced its (kind × model) lookup key is billable, and the
// sealed row ships as one Stripe meter event carrying the raw token count.
func TestPricedAgentModelTokensExportAsMeterEvents(t *testing.T) {
	name := pricing.AgentModelMeterName(store.UsageKindAgentModelOutputTokens, agentModelTestModel)
	saved := billableMeterNames
	billableMeterNames = map[string]struct{}{name: {}}
	t.Cleanup(func() { billableMeterNames = saved })

	gotName, value, skip := stripeMeterEvent(toEvent(agentModelTokenRow()))
	if skip || gotName != "agent_model_output_tokens.deepseek_deepseek-v4.1-flash-1017" || value != "4321" {
		t.Fatalf("stripeMeterEvent = (%q,%q,%v)", gotName, value, skip)
	}
	// A different model on the same kind stays unbilled.
	other := agentModelTokenRow()
	other.Tier = "openai/gpt-x"
	if _, _, skip := stripeMeterEvent(toEvent(other)); !skip {
		t.Fatal("an unpriced model must not export")
	}

	c, stub := newStripeTest(t, func(_ string, _ string) (int, string) {
		return 200, `{"id":"cus_1","object":"customer"}`
	})
	c.storeCustomer("tea-a", "cus_1")
	result := c.IngestBatch(context.Background(), []Event{toEvent(agentModelTokenRow())})
	if len(result.Accepted) != 1 || len(result.Failed) != 0 {
		t.Fatalf("IngestBatch = %+v, want one accepted event", result)
	}
	if got := stub.count("/billing/meter_events"); got != 1 {
		t.Fatalf("meter events posted = %d, want 1", got)
	}
	stub.mu.Lock()
	joined := strings.Join(stub.bodies, "\n")
	stub.mu.Unlock()
	for _, want := range []string{"event_name=agent_model_output_tokens.deepseek_deepseek-v4.1-flash-1017", "payload[value]=4321"} {
		if decoded, _ := url.QueryUnescape(joined); !strings.Contains(decoded, want) {
			t.Fatalf("meter-event body missing %q: %s", want, joined)
		}
	}
}
