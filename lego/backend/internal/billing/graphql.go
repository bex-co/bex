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
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

var taxReadinessGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BillingTaxReadiness",
	Fields: graphql.Fields{
		"configured":        gqlutil.BoolField(func(t TaxReadiness) any { return t.Configured }),
		"enabled":           gqlutil.BoolField(func(t TaxReadiness) any { return t.Enabled }),
		"reason":            gqlutil.StrField(func(t TaxReadiness) any { return t.Reason }),
		"productTaxCode":    gqlutil.StrField(func(t TaxReadiness) any { return t.ProductTaxCode }),
		"taxBehavior":       gqlutil.StrField(func(t TaxReadiness) any { return t.TaxBehavior }),
		"registrationCount": gqlutil.IntField(func(t TaxReadiness) any { return t.RegistrationCount }),
	},
})

// paymentMethodRejectionGQLEnum is why the last card offered was refused at
// binding (ADR091 D10.5–D10.6); null when none is pending.
var paymentMethodRejectionGQLEnum = graphql.NewEnum(graphql.EnumConfig{
	Name:        "PaymentMethodRejection",
	Description: "Why the last card offered was refused at binding: PREPAID_CARD (prepaid cards aren't accepted) or UNPAID_BALANCE (the card is linked to an unpaid balance).",
	Values: graphql.EnumValueConfigMap{
		"PREPAID_CARD":   &graphql.EnumValueConfig{Value: store.PaymentMethodRejectedPrepaid},
		"UNPAID_BALANCE": &graphql.EnumValueConfig{Value: store.PaymentMethodRejectedUnpaidBalance},
	},
})

var billingReadinessGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "WorkspaceBillingReadiness",
	Fields: graphql.Fields{
		"workspaceId":           gqlutil.StrField(func(r Readiness) any { return r.WorkspaceID }),
		"mode":                  gqlutil.StrField(func(r Readiness) any { return r.Mode }),
		"customerReady":         gqlutil.BoolField(func(r Readiness) any { return r.CustomerReady }),
		"subscriptionReady":     gqlutil.BoolField(func(r Readiness) any { return r.SubscriptionReady }),
		"paymentMethodReady":    gqlutil.BoolField(func(r Readiness) any { return r.PaymentMethodReady }),
		"paymentMethodBrand":    gqlutil.StrField(func(r Readiness) any { return r.PaymentMethodBrand }),
		"paymentMethodLast4":    gqlutil.StrField(func(r Readiness) any { return r.PaymentMethodLast4 }),
		"paymentMethodRequired": gqlutil.BoolField(func(r Readiness) any { return r.PaymentMethodRequired }),
		"paymentMethodOnboardingRequired": gqlutil.BoolField(func(r Readiness) any {
			return r.PaymentMethodOnboardingRequired
		}),
		"paymentMethodRejection": gqlutil.Typed(paymentMethodRejectionGQLEnum, func(r Readiness) any {
			if r.PaymentMethodRejection == "" {
				return nil
			}
			return r.PaymentMethodRejection
		}),
		"tax":       gqlutil.Typed(taxReadinessGQLType, func(r Readiness) any { return r.Tax }),
		"lifecycle": gqlutil.Typed(billingLifecycleGQLType, func(r Readiness) any { return r.Lifecycle }),
	},
})

var billingLifecycleGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BillingLifecycle",
	Fields: graphql.Fields{
		"status":           gqlutil.StrField(func(v LifecycleView) any { return v.Status }),
		"reason":           gqlutil.StrField(func(v LifecycleView) any { return v.Reason }),
		"graceDeadline":    gqlutil.StrField(func(v LifecycleView) any { return v.GraceDeadline }),
		"enforcementOwned": gqlutil.BoolField(func(v LifecycleView) any { return v.EnforcementOwned }),
		"recoveryPending":  gqlutil.BoolField(func(v LifecycleView) any { return v.RecoveryPending }),
		"allowedActions":   gqlutil.StrsField(func(v LifecycleView) any { return v.AllowedActions }),
		"updatedAt":        gqlutil.StrField(func(v LifecycleView) any { return v.UpdatedAt }),
		"overdueInvoice": gqlutil.Typed(overdueInvoiceGQLType, func(v LifecycleView) any {
			if v.OverdueInvoice == nil {
				return nil
			}
			return *v.OverdueInvoice
		}),
	},
})

// overdueInvoiceGQLType is the invoice a workspace in dunning owes, as REST
// and MCP serve it; hostedInvoiceUrl is null where Stripe gives none
// (w6/m626).
var overdueInvoiceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BillingOverdueInvoice",
	Fields: graphql.Fields{
		"id":               gqlutil.StrField(func(v OverdueInvoice) any { return v.ID }),
		"hostedInvoiceUrl": gqlutil.OptionalStrField(func(v OverdueInvoice) any { return v.HostedInvoiceURL }),
	},
})

var hostedSessionGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BillingHostedSession",
	Fields: graphql.Fields{
		"url":       gqlutil.StrField(func(s HostedSession) any { return s.URL }),
		"expiresAt": gqlutil.StrField(func(s HostedSession) any { return s.ExpiresAt }),
	},
})

// Billing info (w6/m628): address and taxId are null when none is set.
var billingAddressGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BillingAddress",
	Fields: graphql.Fields{
		"line1":      gqlutil.ReqStrField(func(a BillingAddress) any { return a.Line1 }),
		"line2":      gqlutil.ReqStrField(func(a BillingAddress) any { return a.Line2 }),
		"city":       gqlutil.ReqStrField(func(a BillingAddress) any { return a.City }),
		"state":      gqlutil.ReqStrField(func(a BillingAddress) any { return a.State }),
		"postalCode": gqlutil.ReqStrField(func(a BillingAddress) any { return a.PostalCode }),
		"country":    gqlutil.ReqStrField(func(a BillingAddress) any { return a.Country }),
	},
})

var billingTaxIDGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BillingTaxId",
	Fields: graphql.Fields{
		"type":  gqlutil.ReqStrField(func(t BillingTaxID) any { return t.Type }),
		"value": gqlutil.ReqStrField(func(t BillingTaxID) any { return t.Value }),
	},
})

var billingInfoGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "WorkspaceBillingInfo",
	Description: "The company name, address and VAT or tax ID a workspace's invoices carry, as Render's Billing page billing info. billTo is the name invoices bill: the company name, else the workspace's name.",
	Fields: graphql.Fields{
		"workspaceId":   gqlutil.ReqStrField(func(v BillingInfo) any { return v.WorkspaceID }),
		"customerReady": gqlutil.ReqBoolField(func(v BillingInfo) any { return v.CustomerReady }),
		"billTo":        gqlutil.ReqStrField(func(v BillingInfo) any { return v.BillTo }),
		"companyName":   gqlutil.ReqStrField(func(v BillingInfo) any { return v.CompanyName }),
		"address": gqlutil.Typed(billingAddressGQLType, func(v BillingInfo) any {
			if v.Address == nil {
				return nil
			}
			return *v.Address
		}),
		"taxId": gqlutil.Typed(billingTaxIDGQLType, func(v BillingInfo) any {
			if v.TaxID == nil {
				return nil
			}
			return *v.TaxID
		}),
	},
})

var billingAddressInputGQLType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "BillingAddressInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"line1":      &graphql.InputObjectFieldConfig{Type: graphql.String},
		"line2":      &graphql.InputObjectFieldConfig{Type: graphql.String},
		"city":       &graphql.InputObjectFieldConfig{Type: graphql.String},
		"state":      &graphql.InputObjectFieldConfig{Type: graphql.String},
		"postalCode": &graphql.InputObjectFieldConfig{Type: graphql.String},
		"country":    &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
	},
})

var billingTaxIDInputGQLType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "BillingTaxIdInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"type":  &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"value": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
	},
})

// billingInfoInput reads setWorkspaceBillingInfo's arguments; an omitted
// address or taxId removes it, as REST's PUT does.
func billingInfoInput(args map[string]any) BillingInfoInput {
	in := BillingInfoInput{CompanyName: gqlutil.Str(args, "companyName")}
	if a, ok := args["address"].(map[string]any); ok {
		in.Address = &BillingAddress{
			Line1: gqlutil.Str(a, "line1"), Line2: gqlutil.Str(a, "line2"), City: gqlutil.Str(a, "city"),
			State: gqlutil.Str(a, "state"), PostalCode: gqlutil.Str(a, "postalCode"), Country: gqlutil.Str(a, "country"),
		}
	}
	if t, ok := args["taxId"].(map[string]any); ok {
		in.TaxID = &BillingTaxID{Type: gqlutil.Str(t, "type"), Value: gqlutil.Str(t, "value")}
	}
	return in
}

func billingWorkspaceArg() graphql.FieldConfigArgument {
	return graphql.FieldConfigArgument{"workspaceId": gqlutil.ReqArg(graphql.String)}
}

func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"workspaceBillingReadiness": &graphql.Field{
			Type: billingReadinessGQLType,
			Args: billingWorkspaceArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Status(p.Context, gqlutil.Str(p.Args, "workspaceId"))
			},
		},
		"workspaceBillingInfo": &graphql.Field{
			Type: billingInfoGQLType,
			Args: billingWorkspaceArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.BillingInfo(p.Context, gqlutil.Str(p.Args, "workspaceId"))
			},
		},
	}
}

func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"createBillingCheckoutSession": &graphql.Field{
			Type: hostedSessionGQLType,
			Args: graphql.FieldConfigArgument{
				"workspaceId": gqlutil.ReqArg(graphql.String),
				"successUrl":  gqlutil.ReqArg(graphql.String),
				"cancelUrl":   gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Checkout(p.Context, gqlutil.Str(p.Args, "workspaceId"), CheckoutRequest{SuccessURL: gqlutil.Str(p.Args, "successUrl"), CancelURL: gqlutil.Str(p.Args, "cancelUrl")})
			},
		},
		// setWorkspaceBillingInfo replaces the billing info every later
		// invoice carries (w6/m628); an omitted address or taxId is removed.
		"setWorkspaceBillingInfo": &graphql.Field{
			Type: billingInfoGQLType,
			Args: graphql.FieldConfigArgument{
				"workspaceId": gqlutil.ReqArg(graphql.String),
				"companyName": gqlutil.Arg(graphql.String),
				"address":     gqlutil.Arg(billingAddressInputGQLType),
				"taxId":       gqlutil.Arg(billingTaxIDInputGQLType),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetBillingInfo(p.Context, gqlutil.Str(p.Args, "workspaceId"), billingInfoInput(p.Args))
			},
		},
		"createBillingPortalSession": &graphql.Field{
			Type: hostedSessionGQLType,
			Args: graphql.FieldConfigArgument{
				"workspaceId": gqlutil.ReqArg(graphql.String),
				"returnUrl":   gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Portal(p.Context, gqlutil.Str(p.Args, "workspaceId"), PortalRequest{ReturnURL: gqlutil.Str(p.Args, "returnUrl")})
			},
		},
	}
}
