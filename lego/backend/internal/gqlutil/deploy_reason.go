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

package gqlutil

import (
	"github.com/graphql-go/graphql"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// DeployReasonType names a deploy reason sentence by its stable code and
// parameters (v1alpha1.ParseDeployReason), so the dashboard can word it in its
// own language (w4/566). A bex extension: REST and MCP keep the English
// sentence, as Render's API does.
var DeployReasonType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DeployReason",
	Fields: graphql.Fields{
		"code": ReqStrField(func(r deployReason) any { return r.code }),
		"params": Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(deployReasonParamType))),
			func(r deployReason) any { return r.params }),
		// w5/m356: the tail from the first sentence no template words, as-is.
		"remainder": ReqStrField(func(r deployReason) any { return r.remainder }),
		// The English sentence, for a client that does not word the code.
		"text": ReqStrField(func(r deployReason) any { return r.text }),
	},
})

// parts are the sentences of a hint or cause parameter, in order (w5/m356):
// DeployReasons too, so the field joins the type once it exists.
func init() {
	DeployReasonType.AddFieldConfig("parts", Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(DeployReasonType))),
		func(r deployReason) any { return r.parts }))
}

var deployReasonParamType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DeployReasonParam",
	Fields: graphql.Fields{
		"name":  ReqStrField(func(p appv1alpha1.DeployReasonParam) any { return p.Name }),
		"value": ReqStrField(func(p appv1alpha1.DeployReasonParam) any { return p.Value }),
	},
})

type deployReason struct {
	code      string
	params    []appv1alpha1.DeployReasonParam
	parts     []deployReason
	remainder string
	text      string
}

// DeployReasonField is a nullable DeployReason for the sentence text reads
// off the source: null when the text is empty or no template words it.
func DeployReasonField[T any](text func(T) string) *graphql.Field {
	return Typed(DeployReasonType, func(v T) any {
		reason, ok := parseDeployReason(text(v))
		if !ok {
			return nil
		}
		return reason
	})
}

// parseDeployReason is text's DeployReason, its hint or cause read as parts.
func parseDeployReason(text string) (deployReason, bool) {
	code, params, ok := appv1alpha1.ParseDeployReason(text)
	if !ok {
		return deployReason{}, false
	}
	reason := deployReason{code: code, params: nonNilParams(params), parts: []deployReason{}, text: text}
	for _, p := range params {
		if p.Name != "hint" && p.Name != "cause" {
			continue
		}
		var parts []appv1alpha1.DeployReasonPart
		parts, reason.remainder = appv1alpha1.ParseDeployReasonParts(p.Value)
		for _, part := range parts {
			reason.parts = append(reason.parts, deployReason{
				code: part.Code, params: nonNilParams(part.Params), parts: []deployReason{}, text: part.Text,
			})
		}
	}
	return reason, true
}

// nonNilParams is params, empty rather than nil: the list is non-null.
func nonNilParams(params []appv1alpha1.DeployReasonParam) []appv1alpha1.DeployReasonParam {
	if params == nil {
		return []appv1alpha1.DeployReasonParam{}
	}
	return params
}
