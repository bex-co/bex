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

// w4/m263: the API accepts a plan write without a confirmation, as Render's
// does, so it tells its caller what the dashboard's confirmation tells a user:
// it restarts the store, and a Persistence Off store loses every key. A
// maxmemory-policy write restarts nothing since w4/m330, and says so.
func TestRestartingWritesDiscloseThePersistenceOffDataLoss(t *testing.T) {
	const loss = "persistenceMode off loses every key"
	const noRestart = "without a restart"
	svc, _ := newService()
	schema, err := kvGQLSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	if field := schema.MutationType().Fields()["updateKeyValuePlan"]; field == nil ||
		!strings.Contains(field.Description, "restarts the store") || !strings.Contains(field.Description, loss) {
		t.Errorf("GraphQL updateKeyValuePlan description does not disclose the restart and the Off data loss: %+v", field)
	}
	if field := schema.MutationType().Fields()["setKeyValueMaxmemoryPolicy"]; field == nil ||
		!strings.Contains(field.Description, noRestart) || strings.Contains(field.Description, "restarts the store") {
		t.Errorf("GraphQL setKeyValueMaxmemoryPolicy description does not say the policy applies without a restart: %+v", field)
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
		if !strings.Contains(tool.Description, "maxmemoryPolicy change applies to the running store "+noRestart) ||
			strings.Contains(tool.Description, "maxmemoryPolicy change RESTARTS") {
			t.Errorf("update_key_value description does not say a policy change applies without a restart:\n%s", tool.Description)
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
		if d := schema.Properties["plan"].Description; !strings.Contains(d, "RESTARTS the store") || !strings.Contains(d, loss) {
			t.Errorf("update_key_value plan argument does not disclose the restart and the Off data loss: %q", d)
		}
		if d := schema.Properties["maxmemoryPolicy"].Description; !strings.Contains(d, noRestart) || strings.Contains(d, "RESTARTS") {
			t.Errorf("update_key_value maxmemoryPolicy argument does not say it applies without a restart: %q", d)
		}
		if d := schema.Properties["persistenceMode"].Description; !strings.Contains(d, "RESTARTS the store") || !strings.Contains(d, switchLoss) {
			t.Errorf("update_key_value persistenceMode argument does not disclose the restart and the switch loss: %q", d)
		}
		return
	}
	t.Fatal("update_key_value is not registered")
}
