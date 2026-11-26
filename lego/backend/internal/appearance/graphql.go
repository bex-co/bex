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

package appearance

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func enum(name string, values []string) *graphql.Enum {
	config := graphql.EnumValueConfigMap{}
	for _, value := range values {
		config[value] = &graphql.EnumValueConfig{Value: value}
	}
	return graphql.NewEnum(graphql.EnumConfig{Name: name, Values: config})
}

// Render's two enums, so a value outside them is refused by the schema.
var (
	themeSettingGQLEnum    = enum("ThemeSetting", store.ThemeSettings)
	logThemeSettingGQLEnum = enum("LogThemeSetting", store.LogThemeSettings)
)

// userGQLType is Render's User with the fields bex keeps on the account.
var userGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "User",
	Fields: graphql.Fields{
		"id":              gqlutil.ReqStrField(func(v User) any { return v.ID }),
		"themeSetting":    gqlutil.Typed(graphql.NewNonNull(themeSettingGQLEnum), func(v User) any { return v.ThemeSetting }),
		"logThemeSetting": gqlutil.Typed(graphql.NewNonNull(logThemeSettingGQLEnum), func(v User) any { return v.LogThemeSetting }),
		"avatarDownload":  gqlutil.AvatarDownloadField(func(v User) core.AvatarOwner { return core.AccountAvatar(v.ID) }),
	},
})

func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"user": &graphql.Field{Type: userGQLType, Resolve: func(p graphql.ResolveParams) (any, error) {
			return s.GetUser(p.Context)
		}},
	}
}

func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"updateUserThemeSetting": &graphql.Field{
			Type: userGQLType,
			Args: graphql.FieldConfigArgument{
				"id":           gqlutil.ReqArg(graphql.String),
				"themeSetting": gqlutil.ReqArg(themeSettingGQLEnum),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.UpdateUser(p.Context, gqlutil.Str(p.Args, "id"),
					Patch{ThemeSetting: gqlutil.StrPtr(p.Args, "themeSetting")})
			},
		},
		"updateUserLogThemeSetting": &graphql.Field{
			Type: userGQLType,
			Args: graphql.FieldConfigArgument{
				"id":              gqlutil.ReqArg(graphql.String),
				"logThemeSetting": gqlutil.ReqArg(logThemeSettingGQLEnum),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.UpdateUser(p.Context, gqlutil.Str(p.Args, "id"),
					Patch{LogThemeSetting: gqlutil.StrPtr(p.Args, "logThemeSetting")})
			},
		},
	}
}
