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

package avatars

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// avatarTypeGQLEnum is Render's AvatarType: ACCOUNT or WORKSPACE. Render has
// no organization avatar.
var avatarTypeGQLEnum = graphql.NewEnum(graphql.EnumConfig{
	Name: "AvatarType",
	Values: graphql.EnumValueConfigMap{
		core.AvatarTypeAccount:   &graphql.EnumValueConfig{Value: core.AvatarTypeAccount},
		core.AvatarTypeWorkspace: &graphql.EnumValueConfig{Value: core.AvatarTypeWorkspace},
	},
})

var presignedUploadGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "PresignedUpload",
	Fields: graphql.Fields{
		"url":  gqlutil.ReqStrField(func(u PresignedUpload) any { return u.URL }),
		"verb": gqlutil.ReqStrField(func(u PresignedUpload) any { return u.Verb }),
	},
})

func ownerArgs() graphql.FieldConfigArgument {
	return graphql.FieldConfigArgument{
		"avatarOwnerId": gqlutil.ReqArg(graphql.String),
		"avatarType":    gqlutil.ReqArg(avatarTypeGQLEnum),
	}
}

// GraphQLQuery is Render's avatarPresignedUpload. Render makes it a query
// although it mints an upload slot; the scope matrix classes it as a write.
func (s *Service) GraphQLQuery() graphql.Fields {
	args := ownerArgs()
	args["avatarSizeBytes"] = gqlutil.ReqArg(graphql.Int)
	return graphql.Fields{
		"avatarPresignedUpload": &graphql.Field{
			Type:        graphql.NewNonNull(presignedUploadGQLType),
			Description: "A one-time URL to upload an account's or a workspace's avatar to (PUT, exactly avatarSizeBytes of PNG or JPEG, within 10 minutes). The account's own, or a workspace's as its Admin.",
			Args:        args,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.PresignedUpload(p.Context, gqlutil.Str(p.Args, "avatarOwnerId"),
					gqlutil.Str(p.Args, "avatarType"), p.Args["avatarSizeBytes"].(int))
			},
		},
	}
}

// GraphQLMutation is Render's deleteAvatar.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"deleteAvatar": &graphql.Field{
			Type:        graphql.NewNonNull(graphql.Boolean),
			Description: "Reset an account's or a workspace's avatar to its default.",
			Args:        ownerArgs(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.DeleteAvatar(p.Context, gqlutil.Str(p.Args, "avatarOwnerId"), gqlutil.Str(p.Args, "avatarType"))
			},
		},
	}
}
