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

package accounts

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

var dispositionType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AccountWorkspaceDisposition",
	Fields: graphql.Fields{
		"id":     gqlutil.ReqStrField(func(v store.AccountWorkspaceDisposition) any { return v.ID }),
		"name":   gqlutil.ReqStrField(func(v store.AccountWorkspaceDisposition) any { return v.Name }),
		"action": gqlutil.ReqStrField(func(v store.AccountWorkspaceDisposition) any { return v.Action }),
	},
})

var previewType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AccountDeletionPreview",
	Fields: graphql.Fields{
		"delete": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(dispositionType))), Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(Preview).Delete, nil
		}},
		"leave": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(dispositionType))), Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(Preview).Leave, nil
		}},
		"blocked": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(dispositionType))), Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(Preview).Blocked, nil
		}},
	},
})

var deletionType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AccountDeletion",
	Fields: graphql.Fields{
		"state": gqlutil.ReqStrField(func(v DeletionView) any { return v.State }),
	},
})

func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"accountDeletionPreview": &graphql.Field{Type: previewType, Resolve: func(p graphql.ResolveParams) (any, error) {
			return s.Preview(p.Context)
		}},
	}
}

func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"deleteAccount": &graphql.Field{
			Type: deletionType,
			Args: graphql.FieldConfigArgument{"confirmation": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Request(p.Context, p.Args["confirmation"].(string))
			},
		},
	}
}
