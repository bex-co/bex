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
	"maps"
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

// w4/m195: an opaque rename may refill a name the same batch explicitly
// deleted. The source's exact bytes replace the deleted destination's and the
// source disappears, for variables and files alike.
func TestOpaqueRenameRefillsAnExplicitlyDeletedName(t *testing.T) {
	env := map[string]string{"NEW": "left value\n", "RIGHT": `right\nvalue`, "KEEP": "k"}
	if err := ApplyEnvVarPatch(env, []EnvVarPatch{{Key: "RIGHT", Delete: true}, {Key: "RIGHT", FromKey: "NEW"}}); err != nil {
		t.Fatalf("delete-then-rename variables = %v", err)
	}
	if len(env) != 2 || env["RIGHT"] != "left value\n" || env["KEEP"] != "k" {
		t.Fatalf("env = %#v, want RIGHT carrying NEW's bytes and KEEP untouched", env)
	}

	files := map[string]string{"new": "left-file\n", "right": "right-file\n"}
	if err := ApplySecretFilePatch(files, []SecretFilePatch{{Name: "right", Delete: true}, {Name: "right", FromName: "new"}}); err != nil {
		t.Fatalf("delete-then-rename files = %v", err)
	}
	if len(files) != 1 || files["right"] != "left-file\n" {
		t.Fatalf("files = %#v, want right carrying new's bytes", files)
	}
}

// Everything around the one admitted sequence keeps its refusal, and a refused
// batch is reported before a caller could commit it.
func TestDeletedNameRefillKeepsItsRefusals(t *testing.T) {
	stored := func() map[string]string {
		return map[string]string{"A": "a", "B": "b", "C": "c"}
	}
	for _, test := range []struct {
		name   string
		writes []EnvVarPatch
		want   error
		reason string
	}{
		{
			name:   "rename first into an occupied name",
			writes: []EnvVarPatch{{Key: "B", FromKey: "A"}, {Key: "B", Delete: true}},
			want:   ErrBadRequest, reason: "already exists",
		},
		{
			name:   "cycle of occupied names",
			writes: []EnvVarPatch{{Key: "B", FromKey: "A"}, {Key: "A", FromKey: "B"}},
			want:   ErrBadRequest, reason: "already exists",
		},
		{
			name:   "second rename into the refilled name",
			writes: []EnvVarPatch{{Key: "B", Delete: true}, {Key: "B", FromKey: "A"}, {Key: "B", FromKey: "C"}},
			want:   ErrBadRequest, reason: "duplicate",
		},
		{
			name:   "delete then recreate with a value",
			writes: []EnvVarPatch{{Key: "B", Delete: true}, {Key: "B", Value: "fresh"}},
			want:   ErrBadRequest, reason: "duplicate",
		},
		{
			name:   "duplicate delete",
			writes: []EnvVarPatch{{Key: "B", Delete: true}, {Key: "B", Delete: true}},
			want:   ErrBadRequest, reason: "duplicate",
		},
		{
			name:   "rename from the deleted name",
			writes: []EnvVarPatch{{Key: "B", Delete: true}, {Key: "D", FromKey: "B"}},
			want:   ErrBadRequest, reason: "conflicting",
		},
		{
			name:   "rename of the deleted name onto itself",
			writes: []EnvVarPatch{{Key: "B", Delete: true}, {Key: "B", FromKey: "B"}},
			want:   ErrBadRequest, reason: "duplicate",
		},
		{
			name:   "repeated source",
			writes: []EnvVarPatch{{Key: "B", Delete: true}, {Key: "B", FromKey: "A"}, {Key: "D", FromKey: "A"}},
			want:   ErrBadRequest, reason: "conflicting",
		},
		{
			name:   "missing source",
			writes: []EnvVarPatch{{Key: "B", Delete: true}, {Key: "B", FromKey: "MISSING"}},
			want:   ErrNotFound, reason: "MISSING",
		},
		{
			name:   "rename into the deleted name with a value",
			writes: []EnvVarPatch{{Key: "B", Delete: true}, {Key: "B", FromKey: "A", Value: "x", ValueSet: true}},
			want:   ErrBadRequest, reason: "cannot combine",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := ApplyEnvVarPatch(stored(), test.writes)
			if !errors.Is(err, test.want) || !strings.Contains(err.Error(), test.reason) {
				t.Fatalf("error = %v, want %v mentioning %q", err, test.want, test.reason)
			}
		})
	}

	files := map[string]string{"a": "a", "b": "b"}
	err := ApplySecretFilePatch(files, []SecretFilePatch{{Name: "b", Delete: true}, {Name: "b", Content: "fresh"}})
	if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), "duplicate secret file operation") {
		t.Fatalf("file delete then recreate = %v", err)
	}
}

// w4/234: a staged editor that moves a stored value and puts a new one at its
// old name sends the rename and then a plain write. Both values survive under
// their own names, byte for byte, for files and variables alike.
func TestValueWriteRefillsARenamedAwayName(t *testing.T) {
	original := "qa-a49 initial file\nopaque original snow 雪\n"
	replacement := "qa-a49 uploaded replacement\r\nUnicode 雪\r\n\r\n"
	unicode := "qa-a49 Unicode 雪, café and 🙂\nlast line without newline"
	files := map[string]string{"qa-a49-original.txt": original, "keep.txt": "k"}
	err := ApplySecretFilePatch(files, []SecretFilePatch{
		{Name: "qa-a49-renamed.txt", FromName: "qa-a49-original.txt"},
		{Name: "qa-a49-original.txt", Content: replacement},
		{Name: "qa-a49-empty.txt", Content: ""},
		{Name: "qa-a49-unicode.txt", Content: unicode},
	})
	if err != nil {
		t.Fatalf("rename then refill the old file name = %v", err)
	}
	want := map[string]string{
		"qa-a49-renamed.txt":  original,
		"qa-a49-original.txt": replacement,
		"qa-a49-empty.txt":    "",
		"qa-a49-unicode.txt":  unicode,
		"keep.txt":            "k",
	}
	if !maps.Equal(files, want) {
		t.Fatalf("files = %#v, want %#v", files, want)
	}

	env := map[string]string{"OLD": "opaque", "KEEP": "k"}
	if err := ApplyEnvVarPatch(env, []EnvVarPatch{{Key: "NEW", FromKey: "OLD"}, {Key: "OLD", Value: "", ValueSet: true}}); err != nil {
		t.Fatalf("rename then refill the old variable = %v", err)
	}
	if want := map[string]string{"NEW": "opaque", "OLD": "", "KEEP": "k"}; !maps.Equal(env, want) {
		t.Fatalf("env = %#v, want %#v", env, want)
	}
	generated := map[string]string{"OLD": "opaque"}
	if err := ApplyEnvVarPatch(generated, []EnvVarPatch{{Key: "NEW", FromKey: "OLD"}, {Key: "OLD", GenerateValue: true}}); err != nil {
		t.Fatalf("rename then generate the old variable = %v", err)
	}
	if generated["NEW"] != "opaque" || generated["OLD"] == "" || generated["OLD"] == "opaque" {
		t.Fatalf("generated = %#v, want NEW opaque and a fresh OLD", generated)
	}
}

// Only one plain write may refill a vacated name; everything else that
// touches it again stays refused.
func TestRenamedAwayNameRefillKeepsItsRefusals(t *testing.T) {
	for _, test := range []struct {
		name   string
		writes []EnvVarPatch
		want   error
		reason string
	}{
		{
			name:   "two writes into the vacated name",
			writes: []EnvVarPatch{{Key: "B2", FromKey: "B"}, {Key: "B", Value: "x"}, {Key: "B", Value: "y"}},
			want:   ErrBadRequest, reason: "duplicate",
		},
		{
			name:   "delete of the vacated name",
			writes: []EnvVarPatch{{Key: "B2", FromKey: "B"}, {Key: "B", Delete: true}},
			want:   ErrBadRequest, reason: "duplicate",
		},
		{
			name:   "rename into the vacated name",
			writes: []EnvVarPatch{{Key: "B2", FromKey: "B"}, {Key: "B", FromKey: "A"}},
			want:   ErrBadRequest, reason: "duplicate",
		},
		{
			name:   "rename away the refilled value",
			writes: []EnvVarPatch{{Key: "B2", FromKey: "B"}, {Key: "B", Value: "x"}, {Key: "B3", FromKey: "B"}},
			want:   ErrBadRequest, reason: "conflicting",
		},
		{
			name:   "write before the move",
			writes: []EnvVarPatch{{Key: "B", Value: "x"}, {Key: "B2", FromKey: "B"}},
			want:   ErrBadRequest, reason: "conflicting",
		},
		{
			name:   "write into the vacated name without a value",
			writes: []EnvVarPatch{{Key: "B2", FromKey: "B"}, {Key: "B"}},
			want:   ErrBadRequest, reason: "exactly one of value or generateValue",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := ApplyEnvVarPatch(map[string]string{"A": "a", "B": "b"}, test.writes)
			if !errors.Is(err, test.want) || !strings.Contains(err.Error(), test.reason) {
				t.Fatalf("error = %v, want %v mentioning %q", err, test.want, test.reason)
			}
		})
	}
}

// w4/243: the dashboard used to send Delete+Add of one stored name as a
// delete plus a write, which this contract refuses. It now sends only the
// write, which replaces the stored value in place and leaves the rest alone.
func TestDeleteThenReAddSendsOneReplacingWrite(t *testing.T) {
	stored := func() map[string]string { return map[string]string{"QA_A118_RENAMED": "old", "KEEP": "k"} }
	refused := stored()
	if err := ApplyEnvVarPatch(refused, []EnvVarPatch{
		{Key: "QA_A118_RENAMED", Delete: true},
		{Key: "QA_A118_RENAMED", Value: "qa-a118-readd-literal"},
	}); err == nil || !strings.Contains(err.Error(), `duplicate environment variable operation for "QA_A118_RENAMED"`) {
		t.Fatalf("delete then write the same name = %v, want the duplicate refusal kept", err)
	}

	literal := stored()
	if err := ApplyEnvVarPatch(literal, []EnvVarPatch{{Key: "QA_A118_RENAMED", Value: "qa-a118-readd-literal"}}); err != nil {
		t.Fatalf("literal replacement = %v", err)
	}
	if want := map[string]string{"QA_A118_RENAMED": "qa-a118-readd-literal", "KEEP": "k"}; !maps.Equal(literal, want) {
		t.Fatalf("env = %#v, want %#v", literal, want)
	}
	generated := stored()
	if err := ApplyEnvVarPatch(generated, []EnvVarPatch{{Key: "QA_A118_RENAMED", GenerateValue: true}}); err != nil {
		t.Fatalf("generated replacement = %v", err)
	}
	if v := generated["QA_A118_RENAMED"]; v == "" || v == "old" || generated["KEEP"] != "k" || len(generated) != 2 {
		t.Fatalf("generated = %#v, want a fresh value and KEEP intact", generated)
	}

	for _, content := range []string{"", "qa-a165-file-v2\nnew β\n"} {
		files := map[string]string{"qa-a165-renamed.txt": "qa-a165-file-v1\nβ sample\n", "keep.txt": "k"}
		if err := ApplySecretFilePatch(files, []SecretFilePatch{{Name: "qa-a165-renamed.txt", Content: content}}); err != nil {
			t.Fatalf("file replacement %q = %v", content, err)
		}
		if want := map[string]string{"qa-a165-renamed.txt": content, "keep.txt": "k"}; !maps.Equal(files, want) {
			t.Fatalf("files = %#v, want %#v", files, want)
		}
	}
}
