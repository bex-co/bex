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
package keyvalue

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// w4/m263, w4/m316: the API accepts the plan and maxmemory-policy writes
// without a confirmation, as Render's does, so each one tells its caller what
// the dashboard's confirmation tells a user: it restarts the store, and a
// Persistence Off store loses every key.
func TestRestartingWritesDiscloseThePersistenceOffDataLoss(t *testing.T) {
	const loss = "persistenceMode off loses every key"
	svc, _ := newService()
	schema, err := kvGQLSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"updateKeyValuePlan", "setKeyValueMaxmemoryPolicy"} {
		field := schema.MutationType().Fields()[name]
		if field == nil || !strings.Contains(field.Description, "restarts the store") || !strings.Contains(field.Description, loss) {
			t.Errorf("GraphQL %s description does not disclose the restart and the Off data loss: %+v", name, field)
		}
	}
	// w4/570: a persistence switch to or from off is itself the restart that
	// deletes the keys, in Render's words.
	const switchLoss = "to or from off deletes every key"
	if field := schema.MutationType().Fields()["setKeyValuePersistenceMode"]; field == nil ||
		!strings.Contains(field.Description, "restarts the store") || !strings.Contains(field.Description, switchLoss) {
		t.Errorf("GraphQL setKeyValuePersistenceMode description does not disclose the restart and the switch loss: %+v", field)
	}

	cs := kvMCPSession(t, svc)
	defer func() { _ = cs.Close() }()
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "update_key_value" {
			continue
		}
		if !strings.Contains(tool.Description, "maxmemoryPolicy change RESTARTS") {
			t.Errorf("update_key_value description does not say a policy change restarts the store:\n%s", tool.Description)
		}
		if !strings.Contains(tool.Description, "persistenceMode change RESTARTS") || !strings.Contains(tool.Description, switchLoss) {
			t.Errorf("update_key_value description does not disclose a persistence switch:\n%s", tool.Description)
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Properties map[string]struct {
				Description string `json:"description"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		for _, arg := range []string{"plan", "maxmemoryPolicy"} {
			if d := schema.Properties[arg].Description; !strings.Contains(d, "RESTARTS the store") || !strings.Contains(d, loss) {
				t.Errorf("update_key_value %s argument does not disclose the restart and the Off data loss: %q", arg, d)
			}
		}
		if d := schema.Properties["persistenceMode"].Description; !strings.Contains(d, "RESTARTS the store") || !strings.Contains(d, switchLoss) {
			t.Errorf("update_key_value persistenceMode argument does not disclose the restart and the switch loss: %q", d)
		}
		return
	}
	t.Fatal("update_key_value is not registered")
}
