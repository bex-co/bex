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

package usage

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/billing"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// graphql.go is the usage GraphQL fragment. The `usage` query is a bex
// dashboard companion alongside `monthToDateBandwidth` — workspace-scoped (no
// per-resource filter needed) and returning the same data as GET /v1/usage,
// including the estimatedCost breakdown (w8/m7).

var usageRowGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "UsageRow",
	Fields: graphql.Fields{
		"kind": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r store.UsageSummaryRow) any { return r.Kind })},
		"tier": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r store.UsageSummaryRow) any { return r.Tier })},
		// GraphQL Int is signed 32-bit. Storage GB-seconds (and egress bytes)
		// routinely exceed that, so expose the int64 quantity through Float;
		// IEEE-754 remains exact for every realistic monthly counter here.
		"total": &graphql.Field{Type: graphql.Float, Resolve: gqlutil.Field(func(r store.UsageSummaryRow) any { return r.Total })},
	},
})

var serviceUsageGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ServiceUsage",
	Fields: graphql.Fields{
		"serviceId":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(s ServiceUsage) any { return s.ServiceID })},
		"serviceName":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(s ServiceUsage) any { return s.ServiceName })},
		"resourceKind": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(s ServiceUsage) any { return s.ResourceKind })},
		"rows":         &graphql.Field{Type: graphql.NewList(usageRowGQLType), Resolve: gqlutil.Field(func(s ServiceUsage) any { return s.Rows })},
	},
})

var meterEstimateGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "MeterEstimate",
	Fields: graphql.Fields{
		"kind":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(m pricing.MeterEstimate) any { return m.Kind })},
		"tier":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(m pricing.MeterEstimate) any { return m.Tier })},
		"resourceKind": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(m pricing.MeterEstimate) any { return m.ResourceKind })},
		"costUsd":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(m pricing.MeterEstimate) any { return m.CostUSD })},
	},
})

var estimatedCostGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "EstimatedCost",
	Fields: graphql.Fields{
		"totalUsd": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e pricing.EstimatedCost) any { return e.TotalUSD })},
		"meters":   &graphql.Field{Type: graphql.NewList(meterEstimateGQLType), Resolve: gqlutil.Field(func(e pricing.EstimatedCost) any { return e.Meters })},
	},
})

// billingAmountGQLType and invoiceGQLType mirror billing.Amount / billing.Invoice
// (the same fields the REST/MCP JSON carries) so the three surfaces stay
// identical (ADR006 one-core/thin-adapters).
var billingAmountGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BillingAmount",
	Fields: graphql.Fields{
		"amountUsd":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a billing.Amount) any { return a.AmountUSD })},
		"currency":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a billing.Amount) any { return a.Currency })},
		"periodStart": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a billing.Amount) any { return a.PeriodStart })},
		"periodEnd":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a billing.Amount) any { return a.PeriodEnd })},
	},
})

var billingInvoiceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BillingInvoice",
	Fields: graphql.Fields{
		"id":          &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(i billing.Invoice) any { return i.ID })},
		"status":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(i billing.Invoice) any { return i.Status })},
		"amountUsd":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(i billing.Invoice) any { return i.AmountUSD })},
		"currency":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(i billing.Invoice) any { return i.Currency })},
		"periodStart": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(i billing.Invoice) any { return i.PeriodStart })},
		"periodEnd":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(i billing.Invoice) any { return i.PeriodEnd })},
	},
})

// billingGQLType is the real Stripe billing object; null when estimate-only.
var billingGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Billing",
	Fields: graphql.Fields{
		"currentCost": &graphql.Field{Type: billingAmountGQLType, Resolve: gqlutil.Field(func(b billing.Billing) any {
			if b.CurrentCost == nil {
				return nil
			}
			return *b.CurrentCost
		})},
		"invoices": &graphql.Field{Type: graphql.NewList(billingInvoiceGQLType), Resolve: gqlutil.Field(func(b billing.Billing) any { return b.Invoices })},
	},
})

var usageSummaryGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "UsageSummary",
	Fields: graphql.Fields{
		"workspaceId":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(s Summary) any { return s.WorkspaceID })},
		"period":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(s Summary) any { return s.Period })},
		"services":      &graphql.Field{Type: graphql.NewList(serviceUsageGQLType), Resolve: gqlutil.Field(func(s Summary) any { return s.Services })},
		"estimatedCost": &graphql.Field{Type: estimatedCostGQLType, Resolve: gqlutil.Field(func(s Summary) any { return s.EstimatedCost })},
		// billing is the real Stripe cost/invoices (m48/m50); null ⇒ estimate-only.
		"billing": &graphql.Field{Type: billingGQLType, Resolve: gqlutil.Field(func(s Summary) any {
			if s.Billing == nil {
				return nil
			}
			return *s.Billing
		})},
	},
})

// GraphQLQuery contributes the `usage` query to the root Query. ownerId
// (w6/m18) names the workspace to query; omitted means the caller's default
// workspace — the same arg m14 gave the write-side create mutations.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"usage": &graphql.Field{
			Type: usageSummaryGQLType,
			Args: graphql.FieldConfigArgument{
				"period": &graphql.ArgumentConfig{
					Type:        graphql.String,
					Description: "Calendar month YYYY-MM; defaults to the current month.",
				},
				"ownerId": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				period, _ := p.Args["period"].(string)
				now := resolvePeriodEnd(period, s.Now().UTC())
				return s.monthToDateAt(p.Context, gqlutil.Str(p.Args, "ownerId"), now)
			},
		},
	}
}
