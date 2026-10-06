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
	"os"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation"
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

// w4/m168: names are capped at Kubernetes' 253-character Secret-key limit, and
// the refusal names the rule without echoing the over-long name.
func TestConfigKeysAreCappedAtTheSecretKeyLimit(t *testing.T) {
	ok, long := "A"+strings.Repeat("B", validation.DNS1123SubdomainMaxLength-1), "A"+strings.Repeat("B", validation.DNS1123SubdomainMaxLength)
	if !ValidEnvKey(ok) || ValidEnvKey(long) {
		t.Fatalf("ValidEnvKey: 253 => %v, 254 => %v", ValidEnvKey(ok), ValidEnvKey(long))
	}
	if !ValidSecretFileName(strings.ToLower(ok)) || ValidSecretFileName(strings.ToLower(long)) {
		t.Fatal("ValidSecretFileName must accept 253 and refuse 254 characters")
	}
	for name, err := range map[string]error{
		"env key":     CheckEnvKey(long),
		"secret file": CheckSecretFileName(strings.ToLower(long)),
	} {
		if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "longer than 253 characters") || strings.Contains(err.Error(), long[:40]) {
			t.Errorf("%s refusal = %v, want a 400 naming the 253 limit without the name", name, err)
		}
	}
	if err := CheckSecretFileName("../etc"); !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "invalid secret file name") {
		t.Errorf("charset refusal = %v, want the existing invalid-name message", err)
	}
}

// configKeyVectorsPath is the one table of environment variable and secret-file
// names. The dashboard's VALID_ENV_KEY and isValidSecretFileName are tested
// against the same file (dashboard/src/features/services/lib/__tests__/
// environment-draft.test.ts), and bex-api's answers are the source of truth.
const configKeyVectorsPath = "testdata/config-key-vectors.json"

// TestConfigKeyVectors (w5/m118): every row is what bex-api accepts, and every
// refusal is a coded 400 naming the field and the length limit.
func TestConfigKeyVectors(t *testing.T) {
	raw, err := os.ReadFile(configKeyVectorsPath)
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Key        string `json:"key"`
		Repeat     int    `json:"repeat"`
		EnvVar     bool   `json:"envVar"`
		SecretFile bool   `json:"secretFile"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	refused := 0
	for _, row := range rows {
		key := row.Key
		if row.Repeat > 0 {
			key = strings.Repeat(key, row.Repeat)
		}
		for _, tc := range []struct {
			kind  string
			want  bool
			valid func(string) bool
			check func(string) error
			code  string
		}{
			{"environment variable", row.EnvVar, ValidEnvKey, CheckEnvKey, "ENVIRONMENT_VARIABLE_INVALID"},
			{"secret file", row.SecretFile, ValidSecretFileName, CheckSecretFileName, "SECRET_FILE_INVALID"},
		} {
			if got := tc.valid(key); got != tc.want {
				t.Errorf("%s name %.40q (%d chars): valid = %v, the table says %v", tc.kind, key, len(key), got, tc.want)
			}
			if tc.want {
				continue
			}
			refused++
			var coded *CodedError
			if err := tc.check(key); !errors.As(err, &coded) || coded.Code != tc.code || !errors.Is(err, ErrBadRequest) ||
				coded.Params["maxLength"] != validation.DNS1123SubdomainMaxLength {
				t.Errorf("%s name %.40q: refusal = %v, want a 400 coded %s carrying maxLength", tc.kind, key, err, tc.code)
			}
		}
	}
	if refused < 10 {
		t.Fatalf("vector table too small: %d refusals", refused)
	}
}

// A name stored before the cap can still be deleted or renamed away through a
// patch; writing one stays refused.
func TestStoredOverLongNamesStayRemovable(t *testing.T) {
	long := strings.Repeat("f", validation.DNS1123SubdomainMaxLength+1)
	files := map[string]string{long: "1", "g" + long[1:]: "2"}
	if err := ApplySecretFilePatch(files, []SecretFilePatch{{Name: long, Delete: true}, {Name: "fixed.txt", FromName: "g" + long[1:]}}); err != nil {
		t.Fatalf("delete/rename of stored over-long names = %v", err)
	}
	if len(files) != 1 || files["fixed.txt"] != "2" {
		t.Fatalf("files = %+v, want only fixed.txt", files)
	}
	if err := ApplySecretFilePatch(map[string]string{}, []SecretFilePatch{{Name: long, Content: "x"}}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("writing an over-long name = %v, want 400", err)
	}
	if err := ApplySecretFilePatch(map[string]string{}, []SecretFilePatch{{Name: long, Delete: true}}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("deleting an over-long name that is not stored = %v, want 400", err)
	}
}
