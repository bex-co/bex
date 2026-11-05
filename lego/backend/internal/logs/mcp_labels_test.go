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

package logs

import "testing"

// MCP list_logs carries no container label, as the REST line does not
// (Render's label vocabulary is closed, w3/053); every other label stays and
// the source entry is not mutated.
func TestMCPLogEntriesDropContainer(t *testing.T) {
	in := []LogEntry{
		{Message: "a", Labels: map[string]string{"container": "app", LabelInstance: "web-1", "host": "x.example"}},
		{Message: "b"},
	}
	out := mcpLogEntries(in)
	if _, ok := out[0].Labels["container"]; ok || out[0].Labels[LabelInstance] != "web-1" || out[0].Labels["host"] != "x.example" {
		t.Fatalf("labels = %v, want container dropped and the rest kept", out[0].Labels)
	}
	if in[0].Labels["container"] != "app" {
		t.Fatal("the source entry was mutated")
	}
	if out[1].Message != "b" || out[1].Labels != nil {
		t.Fatalf("unlabelled entry = %+v", out[1])
	}
}
