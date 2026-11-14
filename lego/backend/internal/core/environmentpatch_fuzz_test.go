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

package core

import (
	"encoding/json"
	"testing"
)

// envPatchFuzzSeeds are the sparse env-patch bodies the service and
// Environment Group REST adapters accept: a plain write, an explicit empty
// literal, a generated value, a rename, a delete, and the combinations the
// shared applier must refuse. They live as source so a reviewer sees the
// untrusted shape; the on-disk testdata/fuzz cache is never committed.
func envPatchFuzzSeeds(f *testing.F) {
	f.Helper()
	seeds := []string{
		`[]`,
		`[{"key":"FOO","value":"bar"}]`,
		`[{"key":"FOO","value":""}]`,
		`[{"key":"FOO","generateValue":true}]`,
		`[{"key":"FOO","fromKey":"BAR"}]`,
		`[{"key":"FOO","delete":true}]`,
		`[{"key":"FOO","delete":true,"value":"bar"}]`,
		`[{"key":"FOO","fromKey":"BAR","value":"bar"}]`,
		`[{"key":"FOO"},{"key":"FOO","value":"bar"}]`,
		`[{"key":"FOO","fromKey":"FOO"}]`,
		`[{"key":"","value":"bar"}]`,
		`[{"key":"RENDER","value":"x"}]`,
		`[{"key":"../escape","value":"x"}]`,
		`[{"key":"FOO","value":null}]`,
		`[{"key":"FOO","value":42}]`,
		`[null]`,
		`{}`,
		`null`,
		`not json`,
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
}

// FuzzApplyEnvVarPatch drives the sparse env-patch JSON decoder and the shared
// applier with arbitrary request bodies. Decoding, validation and application
// must never panic; a body the decoder rejects is simply not applied, and an
// applied body leaves every key and value a string.
func FuzzApplyEnvVarPatch(f *testing.F) {
	envPatchFuzzSeeds(f)
	f.Fuzz(func(t *testing.T, body string) {
		var writes []EnvVarPatch
		if err := json.Unmarshal([]byte(body), &writes); err != nil {
			return
		}
		env := map[string]string{"BAR": "existing", "FOO": "old", "RENDER": "platform"}
		_ = ApplyEnvVarPatch(env, writes)
		for key := range env {
			if key == "" {
				t.Fatalf("an applied patch left an empty key in %q", body)
			}
		}
	})
}
