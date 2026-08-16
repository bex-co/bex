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

package sshkeys

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

var sshKeyGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SSHKey",
	Fields: graphql.Fields{
		"id":          gqlutil.ReqStrField(func(k store.SSHKey) any { return k.ID }),
		"name":        gqlutil.ReqStrField(func(k store.SSHKey) any { return k.Name }),
		"publicKey":   gqlutil.ReqStrField(func(k store.SSHKey) any { return k.PublicKey }),
		"fingerprint": gqlutil.ReqStrField(func(k store.SSHKey) any { return k.Fingerprint }),
		"createdAt":   gqlutil.ReqStrField(func(k store.SSHKey) any { return k.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00") }),
	},
})

func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"sshKeys": &graphql.Field{Type: graphql.NewList(sshKeyGQLType), Resolve: func(p graphql.ResolveParams) (any, error) {
			return s.List(p.Context)
		}},
	}
}

func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"createSSHKey": &graphql.Field{
			Type: sshKeyGQLType,
			Args: graphql.FieldConfigArgument{
				"name":      gqlutil.ReqArg(graphql.String),
				"publicKey": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Create(p.Context, p.Args["name"].(string), p.Args["publicKey"].(string))
			},
		},
		"deleteSSHKey": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if err := s.Delete(p.Context, p.Args["id"].(string)); err != nil {
					return false, err
				}
				return true, nil
			},
		},
	}
}
