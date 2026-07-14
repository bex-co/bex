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
		"id":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v CredentialView) any { return v.ID })},
		"name":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v CredentialView) any { return v.Name })},
		"host":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v CredentialView) any { return v.Host })},
		"username":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v CredentialView) any { return v.Username })},
		"ownerId":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v CredentialView) any { return v.OwnerID })},
		"expiresAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v CredentialView) any { return v.ExpiresAt })},
		// status is a bex extension (w2/m14/t007) — Render's registryCredential
		// object has no equivalent expiry-staleness field.
		"status":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v CredentialView) any { return v.Status })},
		"createdAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v CredentialView) any { return v.CreatedAt })},
		"updatedAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v CredentialView) any { return v.UpdatedAt })},
	},
})

// gqlStr / gqlStrPtr read an optional string argument (mirrors apps/graphql.go's
// helpers — graphql-go omits an unset optional arg from the map, so ok=false
// distinguishes "absent" from an explicit, possibly-empty value).
func gqlStr(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func gqlStrPtr(args map[string]any, key string) *string {
	v, ok := args[key].(string)
	if !ok {
		return nil
	}
	return &v
}

// GraphQLQuery returns registryCredentials(ownerId) + registryCredential(id).
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"registryCredentials": &graphql.Field{
			Type: graphql.NewList(credentialGQLType),
			Args: graphql.FieldConfigArgument{
				"ownerId": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.List(p.Context, gqlStr(p.Args, "ownerId"))
			},
		},
		"registryCredential": &graphql.Field{
			Type: credentialGQLType,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Get(p.Context, p.Args["id"].(string))
			},
		},
	}
}

// GraphQLMutation returns create/update/delete for registry credentials.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"createRegistryCredential": &graphql.Field{
			Type: credentialGQLType,
			Args: graphql.FieldConfigArgument{
				"ownerId":   &graphql.ArgumentConfig{Type: graphql.String},
				"name":      &graphql.ArgumentConfig{Type: graphql.String},
				"host":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"username":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"authToken": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"expiresAt": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				expiresAt, err := parseExpiresAt(gqlStr(p.Args, "expiresAt"))
				if err != nil {
					return nil, err
				}
				return s.Create(p.Context, CreateRequest{
					OwnerID: gqlStr(p.Args, "ownerId"), Name: gqlStr(p.Args, "name"),
					Host: p.Args["host"].(string), Username: p.Args["username"].(string),
					Secret: p.Args["authToken"].(string), ExpiresAt: expiresAt,
				})
			},
		},
		"updateRegistryCredential": &graphql.Field{
			Type: credentialGQLType,
			Args: graphql.FieldConfigArgument{
				"id":        &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"name":      &graphql.ArgumentConfig{Type: graphql.String},
				"username":  &graphql.ArgumentConfig{Type: graphql.String},
				"authToken": &graphql.ArgumentConfig{Type: graphql.String},
				"expiresAt": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				req := UpdateRequest{
					Name: gqlStrPtr(p.Args, "name"), Username: gqlStrPtr(p.Args, "username"),
					Secret: gqlStrPtr(p.Args, "authToken"),
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
