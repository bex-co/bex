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
		"configured":        &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(t TaxReadiness) any { return t.Configured })},
		"enabled":           &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(t TaxReadiness) any { return t.Enabled })},
		"reason":            &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(t TaxReadiness) any { return t.Reason })},
		"productTaxCode":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(t TaxReadiness) any { return t.ProductTaxCode })},
		"taxBehavior":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(t TaxReadiness) any { return t.TaxBehavior })},
		"registrationCount": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(t TaxReadiness) any { return t.RegistrationCount })},
	},
})

var billingReadinessGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "WorkspaceBillingReadiness",
	Fields: graphql.Fields{
		"workspaceId":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r Readiness) any { return r.WorkspaceID })},
		"mode":               &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r Readiness) any { return r.Mode })},
		"customerReady":      &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(r Readiness) any { return r.CustomerReady })},
		"subscriptionReady":  &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(r Readiness) any { return r.SubscriptionReady })},
		"paymentMethodReady": &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(r Readiness) any { return r.PaymentMethodReady })},
		"tax":                &graphql.Field{Type: taxReadinessGQLType, Resolve: gqlutil.Field(func(r Readiness) any { return r.Tax })},
		"lifecycle":          &graphql.Field{Type: billingLifecycleGQLType, Resolve: gqlutil.Field(func(r Readiness) any { return r.Lifecycle })},
	},
})

var billingLifecycleGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BillingLifecycle",
	Fields: graphql.Fields{
		"status":           &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v LifecycleView) any { return v.Status })},
		"reason":           &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v LifecycleView) any { return v.Reason })},
		"graceDeadline":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v LifecycleView) any { return v.GraceDeadline })},
		"enforcementOwned": &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v LifecycleView) any { return v.EnforcementOwned })},
		"recoveryPending":  &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v LifecycleView) any { return v.RecoveryPending })},
		"allowedActions":   &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(v LifecycleView) any { return v.AllowedActions })},
		"updatedAt":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v LifecycleView) any { return v.UpdatedAt })},
	},
})

var hostedSessionGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BillingHostedSession",
	Fields: graphql.Fields{
		"url":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(s HostedSession) any { return s.URL })},
		"expiresAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(s HostedSession) any { return s.ExpiresAt })},
	},
})

func billingWorkspaceArg() graphql.FieldConfigArgument {
	return graphql.FieldConfigArgument{"workspaceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}}
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
				"workspaceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"successUrl":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"cancelUrl":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Checkout(p.Context, gqlutil.Str(p.Args, "workspaceId"), CheckoutRequest{SuccessURL: gqlutil.Str(p.Args, "successUrl"), CancelURL: gqlutil.Str(p.Args, "cancelUrl")})
			},
		},
		"createBillingPortalSession": &graphql.Field{
			Type: hostedSessionGQLType,
			Args: graphql.FieldConfigArgument{
				"workspaceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"returnUrl":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Portal(p.Context, gqlutil.Str(p.Args, "workspaceId"), PortalRequest{ReturnURL: gqlutil.Str(p.Args, "returnUrl")})
			},
		},
	}
}
