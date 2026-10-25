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
	"encoding/json"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func decodeEnvVars(t *testing.T, body string) []envVarInput {
	t.Helper()
	var in []envVarInput
	if err := json.Unmarshal([]byte(body), &in); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return in
}

func TestEnvVarInputGenerateValue(t *testing.T) {
	env, err := toEnvVars(decodeEnvVars(t, `[{"key":"GEN","generateValue":true},{"key":"LIT","value":"abc"}]`))
	if err != nil {
		t.Fatalf("toEnvVars: %v", err)
	}
	if len(env) != 2 || env[1].Value != "abc" {
		t.Fatalf("env = %+v", env)
	}
	if len(env[0].Value) != 44 {
		t.Fatalf("generated value len = %d, want 44", len(env[0].Value))
	}
	again, err := toEnvVars(decodeEnvVars(t, `[{"key":"GEN","generateValue":true}]`))
	if err != nil || again[0].Value == env[0].Value {
		t.Fatalf("generated values must be fresh: err=%v", err)
	}
}

func TestEnvVarInputBothAndNeitherRefused(t *testing.T) {
	for _, body := range []string{
		`[{"key":"X","value":"v","generateValue":true}]`,
		`[{"key":"X"}]`,
		`[{"key":"X","generateValue":false}]`,
	} {
		_, err := toEnvVars(decodeEnvVars(t, body))
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_VALUE_INPUT_INVALID" {
			t.Fatalf("%s => %v, want ENVIRONMENT_VALUE_INPUT_INVALID", body, err)
		}
	}
	// An explicit empty literal is a value, not "neither".
	env, err := toEnvVars(decodeEnvVars(t, `[{"key":"X","value":""}]`))
	if err != nil || len(env) != 1 || env[0].Value != "" {
		t.Fatalf("empty literal => env=%+v err=%v", env, err)
	}
}
