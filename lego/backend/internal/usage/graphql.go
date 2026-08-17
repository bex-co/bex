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
	"time"

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
		"kind": gqlutil.StrField(func(r store.UsageSummaryRow) any { return r.Kind }),
		"tier": gqlutil.StrField(func(r store.UsageSummaryRow) any { return r.Tier }),
		// GraphQL Int is signed 32-bit. Storage GB-seconds (and egress bytes)
		// routinely exceed that, so expose the int64 quantity through Float;
		// IEEE-754 remains exact for every realistic monthly counter here.
		"total": gqlutil.FloatField(func(r store.UsageSummaryRow) any { return r.Total }),
	},
})

var serviceUsageGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ServiceUsage",
	Fields: graphql.Fields{
		"serviceId":    gqlutil.StrField(func(s ServiceUsage) any { return s.ServiceID }),
		"serviceName":  gqlutil.StrField(func(s ServiceUsage) any { return s.ServiceName }),
		"resourceKind": gqlutil.StrField(func(s ServiceUsage) any { return s.ResourceKind }),
		"rows":         gqlutil.Typed(graphql.NewList(usageRowGQLType), func(s ServiceUsage) any { return s.Rows }),
	},
})

var meterEstimateGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "MeterEstimate",
	Fields: graphql.Fields{
		"kind":         gqlutil.StrField(func(m pricing.MeterEstimate) any { return m.Kind }),
		"tier":         gqlutil.StrField(func(m pricing.MeterEstimate) any { return m.Tier }),
		"resourceKind": gqlutil.StrField(func(m pricing.MeterEstimate) any { return m.ResourceKind }),
		"costUsd":      gqlutil.StrField(func(m pricing.MeterEstimate) any { return m.CostUSD }),
	},
})

var estimatedCostGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "EstimatedCost",
	Fields: graphql.Fields{
		"totalUsd": gqlutil.StrField(func(e pricing.EstimatedCost) any { return e.TotalUSD }),
		"meters":   gqlutil.Typed(graphql.NewList(meterEstimateGQLType), func(e pricing.EstimatedCost) any { return e.Meters }),
	},
})

var usageCoverageGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "UsageCoverage",
	Fields: graphql.Fields{
		"state": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(c Coverage) any {
			if c.State == "" {
				return CoverageUnknown
			}
			return c.State
		})},
		"through": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(c Coverage) any {
			if c.Through.IsZero() {
				return nil
			}
			return c.Through.UTC().Format(time.RFC3339)
		})},
		"degradedSources": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))), Resolve: gqlutil.Field(func(c Coverage) any {
			if c.DegradedSources == nil {
				return []string{}
			}
			return c.DegradedSources
		})},
	},
})

// billingAmountGQLType and invoiceGQLType mirror billing.Amount / billing.Invoice
// (the same fields the REST/MCP JSON carries) so the three surfaces stay
// identical (ADR006 one-core/thin-adapters).
var billingAmountGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BillingAmount",
	Fields: graphql.Fields{
		"amountUsd":   gqlutil.StrField(func(a billing.Amount) any { return a.AmountUSD }),
		"currency":    gqlutil.StrField(func(a billing.Amount) any { return a.Currency }),
		"periodStart": gqlutil.StrField(func(a billing.Amount) any { return a.PeriodStart }),
		"periodEnd":   gqlutil.StrField(func(a billing.Amount) any { return a.PeriodEnd }),
	},
})

var billingInvoiceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BillingInvoice",
	Fields: graphql.Fields{
		"id":          gqlutil.StrField(func(i billing.Invoice) any { return i.ID }),
		"status":      gqlutil.StrField(func(i billing.Invoice) any { return i.Status }),
		"amountUsd":   gqlutil.StrField(func(i billing.Invoice) any { return i.AmountUSD }),
		"currency":    gqlutil.StrField(func(i billing.Invoice) any { return i.Currency }),
		"periodStart": gqlutil.StrField(func(i billing.Invoice) any { return i.PeriodStart }),
		"periodEnd":   gqlutil.StrField(func(i billing.Invoice) any { return i.PeriodEnd }),
	},
})

// billingCreditGrantGQLType and billingCreditsGQLType mirror
// billing.CreditGrant / billing.Credits (the same fields the REST/MCP JSON
// carries) so the three surfaces stay identical (ADR006, w5/m70).
var billingCreditGrantGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BillingCreditGrant",
	Fields: graphql.Fields{
		"name":         gqlutil.StrField(func(g billing.CreditGrant) any { return g.Name }),
		"remainingUsd": gqlutil.StrField(func(g billing.CreditGrant) any { return g.RemainingUSD }),
		"expiresAt":    gqlutil.StrField(func(g billing.CreditGrant) any { return g.ExpiresAt }),
	},
})

var billingCreditsGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BillingCredits",
	Fields: graphql.Fields{
		"availableUsd": gqlutil.StrField(func(c billing.Credits) any { return c.AvailableUSD }),
		"currency":     gqlutil.StrField(func(c billing.Credits) any { return c.Currency }),
		"grants":       gqlutil.Typed(graphql.NewList(billingCreditGrantGQLType), func(c billing.Credits) any { return c.Grants }),
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
		"invoices": gqlutil.Typed(graphql.NewList(billingInvoiceGQLType), func(b billing.Billing) any { return b.Invoices }),
		"credits": &graphql.Field{Type: billingCreditsGQLType, Resolve: gqlutil.Field(func(b billing.Billing) any {
			if b.Credits == nil {
				return nil
			}
			return *b.Credits
		})},
	},
})

var usageSummaryGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "UsageSummary",
	Fields: graphql.Fields{
		"workspaceId":   gqlutil.StrField(func(s Summary) any { return s.WorkspaceID }),
		"period":        gqlutil.StrField(func(s Summary) any { return s.Period }),
		"services":      gqlutil.Typed(graphql.NewList(serviceUsageGQLType), func(s Summary) any { return s.Services }),
		"estimatedCost": gqlutil.Typed(estimatedCostGQLType, func(s Summary) any { return s.EstimatedCost }),
		"coverage":      gqlutil.Typed(graphql.NewNonNull(usageCoverageGQLType), func(s Summary) any { return s.Coverage }),
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
				"ownerId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				asOf, err := ResolvePeriodEnd(gqlutil.Str(p.Args, "period"), s.Now().UTC())
				if err != nil {
					return nil, err
				}
				return s.monthToDateAt(p.Context, gqlutil.Str(p.Args, "ownerId"), asOf)
			},
		},
	}
}
