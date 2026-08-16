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

package registrycreds

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// credentialGQLType mirrors the REST credentialWire shape — see rest.go's
// doc comment for the host-vs-Render's-registry-enum divergence note.
var credentialGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "RegistryCredential",
	Fields: graphql.Fields{
		"id":        gqlutil.StrField(func(v CredentialView) any { return v.ID }),
		"name":      gqlutil.StrField(func(v CredentialView) any { return v.Name }),
		"host":      gqlutil.StrField(func(v CredentialView) any { return v.Host }),
		"username":  gqlutil.StrField(func(v CredentialView) any { return v.Username }),
		"ownerId":   gqlutil.StrField(func(v CredentialView) any { return v.OwnerID }),
		"expiresAt": gqlutil.StrField(func(v CredentialView) any { return v.ExpiresAt }),
		// status is a bex extension (w2/m14/t007) — Render's registryCredential
		// object has no equivalent expiry-staleness field.
		"status":    gqlutil.StrField(func(v CredentialView) any { return v.Status }),
		"createdAt": gqlutil.StrField(func(v CredentialView) any { return v.CreatedAt }),
		"updatedAt": gqlutil.StrField(func(v CredentialView) any { return v.UpdatedAt }),
	},
})

// GraphQLQuery returns registryCredentials(ownerId) + registryCredential(id).
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"registryCredentials": &graphql.Field{
			Type: graphql.NewList(credentialGQLType),
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.List(p.Context, gqlutil.Str(p.Args, "ownerId"))
			},
		},
		"registryCredential": gqlutil.IDVerb(credentialGQLType, s.Get),
	}
}

// GraphQLMutation returns create/update/delete for registry credentials.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"createRegistryCredential": &graphql.Field{
			Type: credentialGQLType,
			Args: graphql.FieldConfigArgument{
				"ownerId":   gqlutil.Arg(graphql.String),
				"name":      gqlutil.Arg(graphql.String),
				"host":      gqlutil.ReqArg(graphql.String),
				"username":  gqlutil.ReqArg(graphql.String),
				"authToken": gqlutil.ReqArg(graphql.String),
				"expiresAt": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				expiresAt, err := parseExpiresAt(gqlutil.Str(p.Args, "expiresAt"))
				if err != nil {
					return nil, err
				}
				return s.Create(p.Context, CreateRequest{
					OwnerID: gqlutil.Str(p.Args, "ownerId"), Name: gqlutil.Str(p.Args, "name"),
					Host: p.Args["host"].(string), Username: p.Args["username"].(string),
					Secret: p.Args["authToken"].(string), ExpiresAt: expiresAt,
				})
			},
		},
		"updateRegistryCredential": &graphql.Field{
			Type: credentialGQLType,
			Args: graphql.FieldConfigArgument{
				"id":        gqlutil.ReqArg(graphql.String),
				"name":      gqlutil.Arg(graphql.String),
				"username":  gqlutil.Arg(graphql.String),
				"authToken": gqlutil.Arg(graphql.String),
				"expiresAt": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				req := UpdateRequest{
					Name: gqlutil.StrPtr(p.Args, "name"), Username: gqlutil.StrPtr(p.Args, "username"),
					Secret: gqlutil.StrPtr(p.Args, "authToken"),
				}
				if raw, ok := p.Args["expiresAt"]; ok {
					expiresAt, err := parseExpiresAt(raw.(string))
					if err != nil {
						return nil, err
					}
					req.ExpiresAtSet = true
					req.ExpiresAt = expiresAt
				}
				return s.Update(p.Context, p.Args["id"].(string), req)
			},
		},
		"deleteRegistryCredential": &graphql.Field{
			Type: graphql.Boolean,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.Delete(p.Context, p.Args["id"].(string))
				return err == nil, err
			},
		},
	}
}
