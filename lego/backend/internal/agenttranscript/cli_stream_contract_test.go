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

package agenttranscript

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReplayChunksMatchCLIStreamContract is the backend twin of the `bex
// agent` stream fixture (lego/cli/testdata/agent-stream-replay.sse, ADR106
// D9): every chunk Expand replays for a text, reasoning or tool item must be a
// type the fixture shows the CLI, carrying the fields the CLI reads from it.
func TestReplayChunksMatchCLIStreamContract(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "..", "..", "cli", "testdata", "agent-stream-replay.sse"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	cli := map[string]map[string]bool{} // chunk type → the keys the fixture gives it
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		payload, ok := strings.CutPrefix(scanner.Text(), "data: ")
		if !ok || payload == "[DONE]" {
			continue
		}
		var chunk map[string]any
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			t.Fatalf("fixture line %q: %v", payload, err)
		}
		keys := map[string]bool{}
		for key := range chunk {
			keys[key] = true
		}
		cli[chunk["type"].(string)] = keys
	}
	items := []string{
		`{"type":"text","id":"t1","text":"Fixed it."}`,
		`{"type":"reasoning","id":"r1","text":"Read the log."}`,
		`{"type":"tool-Bash","toolCallId":"call-1","toolName":"Bash","state":"output-error","input":{"command":"go test"},"errorText":"exit status 1"}`,
		`{"type":"dynamic-tool","toolCallId":"call-2","toolName":"edit","state":"output-available","input":{"path":"main.go"},"output":"ok"}`,
	}
	for _, item := range items {
		for _, chunk := range Expand(json.RawMessage(item)) {
			var got map[string]any
			if err := json.Unmarshal(chunk, &got); err != nil {
				t.Fatal(err)
			}
			typ, _ := got["type"].(string)
			want, ok := cli[typ]
			if !ok {
				t.Errorf("replay emits %q for %s, which the CLI's stream fixture does not cover", typ, item)
				continue
			}
			for key := range want {
				if _, ok := got[key]; !ok && key != "dynamic" {
					t.Errorf("replayed %q lacks %q, which the CLI reads", typ, key)
				}
			}
		}
	}
}
