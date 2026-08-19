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
		"tax":                   gqlutil.Typed(taxReadinessGQLType, func(r Readiness) any { return r.Tax }),
		"lifecycle":             gqlutil.Typed(billingLifecycleGQLType, func(r Readiness) any { return r.Lifecycle }),
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
	},
})

var hostedSessionGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BillingHostedSession",
	Fields: graphql.Fields{
		"url":       gqlutil.StrField(func(s HostedSession) any { return s.URL }),
		"expiresAt": gqlutil.StrField(func(s HostedSession) any { return s.ExpiresAt }),
	},
})

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
