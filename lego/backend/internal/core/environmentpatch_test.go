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
	"errors"
	"testing"
)

func TestApplyEnvVarPatchDistinguishesEmptyLiteralFromOmittedValue(t *testing.T) {
	var writes []EnvVarPatch
	if err := json.Unmarshal([]byte(`[{"key":"EMPTY","value":""}]`), &writes); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{}
	if err := ApplyEnvVarPatch(env, writes); err != nil {
		t.Fatalf("explicit empty literal: %v", err)
	}
	if value, exists := env["EMPTY"]; !exists || value != "" {
		t.Fatalf("empty literal was not stored: %#v", env)
	}
}

func TestApplyEnvVarPatchRequiresExactlyOneLiteralOrGenerationIntent(t *testing.T) {
	for _, test := range []struct {
		name  string
		write EnvVarPatch
	}{
		{name: "neither", write: EnvVarPatch{Key: "TOKEN"}},
		{name: "both", write: EnvVarPatch{Key: "TOKEN", Value: "literal", ValueSet: true, GenerateValue: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := ApplyEnvVarPatch(map[string]string{}, []EnvVarPatch{test.write})
			var coded *CodedError
			if !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_VALUE_INPUT_INVALID" || !errors.Is(err, ErrBadRequest) {
				t.Fatalf("error = %v, want coded bad request", err)
			}
			if coded.Params["key"] != "TOKEN" {
				t.Fatalf("params = %#v", coded.Params)
			}
		})
	}
}
