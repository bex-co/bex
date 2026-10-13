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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w8/096: classifyRefs used fromService.type only to pick the Key Value
// branch, so a reference typed worker resolved against a web service. A
// supplied type must now describe the declared same-file target.
func TestBlueprintFromServiceTypeMustMatchItsTarget(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	target := func(kind string) string {
		switch kind {
		case "worker":
			return `{"name":"source","type":"worker","plan":"starter","runtime":"image","image":{"url":"nginx:1"},"envVars":[{"key":"QA_SOURCE","value":"v"}]}`
		case "cron":
			return `{"name":"source","type":"cron","plan":"starter","schedule":"*/15 * * * *","runtime":"image","image":{"url":"nginx:1"},"envVars":[{"key":"QA_SOURCE","value":"v"}]}`
		case "static":
			return `{"name":"source","type":"web","runtime":"static","repo":"https://github.com/acme/site","staticPublishPath":"dist","envVars":[{"key":"QA_SOURCE","value":"v"}]}`
		}
		return `{"name":"source","type":"web","runtime":"image","image":{"url":"nginx:1"},"envVars":[{"key":"QA_SOURCE","value":"v"}]}`
	}
	caller := func(refType, ref string) string {
		return `{"name":"caller","type":"web","runtime":"image","image":{"url":"nginx:1"},"envVars":[{"key":"QA_TARGET","fromService":{"name":"source","type":"` + refType + `",` + ref + `}}]}`
	}
	const port, copyKey = `"property":"port"`, `"envVarKey":"QA_SOURCE"`
	for _, tc := range []struct {
		name, target, refType, ref string
		valid                      bool
	}{
		{"web copy typed web", "web", "web", copyKey, true},
		{"web port typed web", "web", "web", port, true},
		{"worker copy typed worker", "worker", "worker", copyKey, true},
		{"cron copy typed cron", "cron", "cron", copyKey, true},
		{"static copy typed web", "static", "web", copyKey, true},
		{"static copy typed static", "static", "static", copyKey, true},
		{"web port typed worker", "web", "worker", port, false},
		{"web copy typed worker", "web", "worker", copyKey, false},
		{"web copy typed cron", "web", "cron", copyKey, false},
		{"web port typed pserv", "web", "pserv", port, false},
		{"worker copy typed web", "worker", "web", copyKey, false},
	} {
		for _, order := range []string{"caller first", "target first"} {
			services := caller(tc.refType, tc.ref) + "," + target(tc.target)
			if order == "target first" {
				services = target(tc.target) + "," + caller(tc.refType, tc.ref)
			}
			for _, wrap := range []func(string) string{
				func(s string) string { return `{"services":[` + s + `]}` },
				func(s string) string { return `{"projects":[{"name":"p","environments":[{"name":"e","services":[` + s + `]}]}]}` },
			} {
				result, err := svc.ValidateBlueprint(context.Background(), "", wrap(services), "")
				if err != nil {
					t.Fatalf("%s (%s): %v", tc.name, order, err)
				}
				joined := strings.Join(messagesOf(result), " | ")
				if tc.valid && !result.Valid {
					t.Errorf("%s (%s) = %s, want valid", tc.name, order, joined)
				}
				if !tc.valid && (result.Valid || !strings.Contains(joined, `fromService.type "`+tc.refType+`" does not match "source"`)) {
					t.Errorf("%s (%s) = valid %v %s, want the type mismatch", tc.name, order, result.Valid, joined)
				}
			}
		}
	}
}
