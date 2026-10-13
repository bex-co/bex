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

// w8/098: a Key Value fromService with envVarKey and no property passed
// validation and resolved to an empty Secret key, which the Pod would refuse.
// Key Value references take a connection property; the resolver also refuses
// an unmapped property instead of returning an empty selector.
func TestBlueprintKeyValueReferenceNeedsAProperty(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	manifest := func(kvType, ref string) string {
		return `{"services":[
  {"name":"source","type":"keyvalue","plan":"free","ipAllowList":[]},
  {"name":"caller","type":"web","runtime":"image","image":{"url":"nginx:1"},"envVars":[{"key":"QA_TARGET","fromService":{"type":"` + kvType + `","name":"source",` + ref + `}}]}]}`
	}
	for _, kvType := range []string{"keyvalue", "redis"} {
		for _, tc := range []struct {
			ref, wantErr string
		}{
			{`"envVarKey":"QA_UNDECLARED"`, "takes a property"},
			{`"property":"host","envVarKey":"QA_UNDECLARED"`, "takes a property"},
			{`"property":"hostport"`, "is not supported"},
			{`"property":"connectionString"`, ""},
			{`"property":"host"`, ""},
			{`"property":"port"`, ""},
		} {
			result, err := svc.ValidateBlueprint(context.Background(), "", manifest(kvType, tc.ref), "")
			if err != nil {
				t.Fatalf("%s %s: %v", kvType, tc.ref, err)
			}
			joined := strings.Join(messagesOf(result), " | ")
			if tc.wantErr == "" && !result.Valid {
				t.Errorf("%s %s = %s, want valid", kvType, tc.ref, joined)
			}
			if tc.wantErr != "" && (result.Valid || !strings.Contains(joined, tc.wantErr)) {
				t.Errorf("%s %s = valid %v %s, want %q", kvType, tc.ref, result.Valid, joined, tc.wantErr)
			}
		}
	}

	names := map[string]string{"source": "red-source"}
	for property, want := range map[string]string{"connectionString": "uri", "host": "host", "port": "port", "password": "password"} {
		ev, err := resolveKeyValueRef(bexEnvVar{Key: "K", FromService: &bexFromRef{Name: "source", Type: "keyvalue", Property: property}}, names)
		if err != nil || ev.ValueFrom == nil || ev.ValueFrom.SecretKeyRef.Key != want {
			t.Errorf("property %s = %+v, %v; want key %q", property, ev, err, want)
		}
	}
	for _, bad := range []bexFromRef{{Name: "source", Type: "keyvalue", EnvVarKey: "X"}, {Name: "source", Type: "redis", Property: "hostport"}} {
		if _, err := resolveKeyValueRef(bexEnvVar{Key: "K", FromService: &bad}, names); err == nil {
			t.Errorf("malformed ref %+v resolved, want an error", bad)
		}
	}
}
