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

package api

import (
	"encoding/json"
	"testing"
)

// TestMCPSchemasHaveNoByteArrays guards w6/m398. jsonschema-go infers a
// []byte, json.RawMessage included, as an array of 0-255 integers, but
// encoding/json writes a []byte as a base64 string and a json.RawMessage as the
// JSON it holds. The SDK validates every typed result against the inferred
// output schema, so a tool whose result carried one failed every call that had
// a value there: get_agent_session_transcript refused any transcript with a
// part on production. mcputil.AddTool maps json.RawMessage to any JSON value;
// this fails a tool that reaches the byte-array shape some other way.
func TestMCPSchemasHaveNoByteArrays(t *testing.T) {
	for _, tool := range listBexTools(t, fullyWiredServer()).Tools {
		for side, schema := range map[string]any{"input": tool.InputSchema, "output": tool.OutputSchema} {
			if schema == nil {
				continue
			}
			raw, err := json.Marshal(schema)
			if err != nil {
				t.Fatalf("marshal %s schema of %s: %v", side, tool.Name, err)
			}
			var tree any
			if err := json.Unmarshal(raw, &tree); err != nil {
				t.Fatalf("parse %s schema of %s: %v", side, tool.Name, err)
			}
			for _, path := range byteArrays(tree, "") {
				t.Errorf("%s: %s schema at %s is a byte array; a []byte or json.RawMessage never encodes as one", tool.Name, side, path)
			}
		}
	}
}

// byteArrays returns the path of every node in a schema tree whose items are
// integers from 0 to 255: the schema jsonschema-go infers for a []byte.
func byteArrays(node any, path string) []string {
	var found []string
	switch n := node.(type) {
	case map[string]any:
		if items, ok := n["items"].(map[string]any); ok && items["type"] == "integer" && items["minimum"] == 0.0 && items["maximum"] == 255.0 {
			found = append(found, path)
		}
		for key, child := range n {
			found = append(found, byteArrays(child, path+"/"+key)...)
		}
	case []any:
		for _, child := range n {
			found = append(found, byteArrays(child, path)...)
		}
	}
	return found
}
