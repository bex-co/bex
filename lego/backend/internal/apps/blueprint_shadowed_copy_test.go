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

package apps

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/102: a service declaring the same key as a literal and as a reference
// gets the reference at runtime (it is appended last), but the copy index
// kept the literal, so a sibling's envVarKey copy silently took the losing
// value. The shadowed literal is not plainly defined: the copy is refused
// before any write, like a pure reference chain.
func TestBlueprintCopyOfAReferenceShadowedLiteralIsRefused(t *testing.T) {
	const image = `"runtime":"image","image":{"url":"nginx:1"}`
	source := `{"name":"source","type":"web",` + image + `,"envVars":[{"key":"SOURCE_VALUE","value":"source value"}]}`
	ref := `{"key":"VALUE","fromService":{"type":"web","name":"source","envVarKey":"SOURCE_VALUE"}}`
	lit := func(v string) string { return `{"key":"VALUE","value":"` + v + `"}` }
	caller := func(envs ...string) string {
		return `{"name":"caller","type":"web",` + image + `,"envVars":[` + strings.Join(envs, ",") + `]}`
	}
	consumer := `{"name":"consumer","type":"web",` + image + `,"envVars":[{"key":"COPY_VALUE","fromService":{"type":"web","name":"caller","envVarKey":"VALUE"}}]}`
	doc := func(services ...string) string { return `{"services":[` + strings.Join(services, ",") + `]}` }

	refused := map[string]string{
		"literal then reference": doc(source, caller(lit("literal value"), ref), consumer),
		"reference then literal": doc(source, caller(ref, lit("literal value")), consumer),
		"consumer first":         doc(consumer, source, caller(lit("literal value"), ref)),
		"empty shadowed literal": doc(source, caller(lit(""), ref), consumer),
		"pure reference chain":   doc(source, caller(ref), consumer),
	}
	for name, manifest := range refused {
		t.Run(name, func(t *testing.T) {
			svc, cl := newService(nil)
			_, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest})
			if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "no such plainly-defined variable") {
				t.Fatalf("DeployStack = %v, want the plainly-defined refusal", err)
			}
			var apps appv1alpha1.AppList
			if err := cl.List(context.Background(), &apps); err != nil || len(apps.Items) != 0 {
				t.Fatalf("apps after refusal = %d (err %v), want none", len(apps.Items), err)
			}
			result, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
			if err != nil || result.Valid || len(result.Errors) != 1 || result.Errors[0].Path == nil ||
				!strings.HasSuffix(*result.Errors[0].Path, "envVars[0]") || !strings.Contains(result.Errors[0].Error, `"consumer"`) {
				t.Fatalf("validation = %+v, %v; want one refusal at the consumer's COPY_VALUE", result, err)
			}
		})
	}

	// Controls: duplicate literals copy the final literal; unique keys copy the intended literal.
	for name, tc := range map[string]struct{ manifest, want string }{
		"duplicate literals": {doc(source, caller(lit("first"), lit("final")), consumer), "final"},
		"unique keys":        {doc(source, caller(lit("literal value"), `{"key":"OTHER","fromService":{"type":"web","name":"source","envVarKey":"SOURCE_VALUE"}}`), consumer), "literal value"},
	} {
		t.Run(name, func(t *testing.T) {
			svc, cl := newService(nil)
			if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: tc.manifest}); err != nil {
				t.Fatalf("DeployStack: %v", err)
			}
			var copied, effective string
			for _, e := range getApp(t, cl, "consumer").Spec.Env {
				if e.Name == "COPY_VALUE" {
					copied = e.Value
				}
			}
			for _, e := range getApp(t, cl, "caller").Spec.Env {
				if e.Name == "VALUE" {
					effective = e.Value // the last entry wins at runtime
				}
			}
			if copied != tc.want || effective != tc.want {
				t.Fatalf("copy %q, caller's effective VALUE %q; want both %q", copied, effective, tc.want)
			}
		})
	}
}
