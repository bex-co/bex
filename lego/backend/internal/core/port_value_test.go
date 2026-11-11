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
	"errors"
	"strings"
	"testing"
)

// An unusable PORT is refused where it is written, not silently replaced by
// the operator (w8/170); a usable one and other keys pass.
func TestCheckEnvValueRefusesUnusablePort(t *testing.T) {
	for _, bad := range []string{"abc", "0", "70000", "-1", "", " 80", "80.5", "18012", "19099"} {
		err := CheckEnvValue("PORT", bad)
		var coded *CodedError
		if !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_VARIABLE_PORT_INVALID" {
			t.Errorf("PORT=%q: %v, want ENVIRONMENT_VARIABLE_PORT_INVALID", bad, err)
		}
	}
	for _, good := range []string{"80", "8080", "65535", "1"} {
		if err := CheckEnvValue("PORT", good); err != nil {
			t.Errorf("PORT=%q refused: %v", good, err)
		}
	}
	if err := CheckEnvValue("LOG_LEVEL", "abc"); err != nil {
		t.Errorf("a non-PORT key was judged as a port: %v", err)
	}
}

// A KEY=value entry the kernel would refuse to exec is refused at write,
// one byte past the limit and not before (w4/422).
func TestCheckEnvValueRefusesAnEntryNoProcessCanStartWith(t *testing.T) {
	const key = "BIG"
	fits := strings.Repeat("v", MaxEnvEntryBytes-len(key)-len("=")-1)
	if err := CheckEnvValue(key, fits); err != nil {
		t.Fatalf("an entry at the limit = %v, want accepted", err)
	}
	var coded *CodedError
	err := CheckEnvValue(key, fits+"v")
	if !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_VARIABLE_TOO_LARGE" || coded.Params["key"] != key || !strings.Contains(err.Error(), "secret file") {
		t.Fatalf("one byte over = %v, want ENVIRONMENT_VARIABLE_TOO_LARGE naming %s", err, key)
	}
}

// w21/047: the dashboard's Environment editors mark these rules in the row with
// the same vectors (dashboard/src/features/services/lib/__tests__/
// environment-draft.test.ts "environment value rules"). Change one side and
// change the other.
func TestEnvValueRulesVectors(t *testing.T) {
	code := func(err error) string {
		var coded *CodedError
		if errors.As(err, &coded) {
			return coded.Code
		}
		return ""
	}
	for value, ok := range map[string]bool{
		"8080": true, "1": true, "65535": true, "00080": true,
		// strconv.Atoi takes a sign and no surrounding space.
		"+80": true,
		"abc": false, "": false, "0": false, "65536": false, "-1": false, "80 ": false, " 80": false, "8.0": false,
		"99999999999999999999": false, "18012": false, "18013": false, "18014": false, "19099": false,
	} {
		if got := ValidPortValue(value); got != ok {
			t.Errorf("ValidPortValue(%q) = %v, want %v", value, got, ok)
		}
	}
	// Only the exact key PORT is judged as a port.
	for _, key := range []string{"PORTAL", "port"} {
		if err := CheckEnvValue(key, "abc"); err != nil {
			t.Errorf("%s=abc judged as a port: %v", key, err)
		}
	}
	// The entry limit counts UTF-8 bytes: 43,690 Chinese characters plus
	// "K", "=" and the terminator are 131,073 bytes, 43,689 are 131,070.
	if got := code(CheckEnvValue("K", strings.Repeat("中", 43690))); got != "ENVIRONMENT_VARIABLE_TOO_LARGE" {
		t.Errorf("43,690 characters => %q, want TOO_LARGE", got)
	}
	if err := CheckEnvValue("K", strings.Repeat("中", 43689)); err != nil {
		t.Errorf("43,689 characters refused: %v", err)
	}
	// A NUL is judged first, then the size, then the PORT value.
	if got := code(CheckEnvValue("PORT", "8\x000")); got != "ENVIRONMENT_VARIABLE_VALUE_INVALID" {
		t.Errorf("PORT=8\\x000 => %q, want VALUE_INVALID", got)
	}
	if got := code(CheckEnvValue("K", strings.Repeat("a", 140000)+"\x00")); got != "ENVIRONMENT_VARIABLE_VALUE_INVALID" {
		t.Errorf("a NUL beyond the size limit => %q, want VALUE_INVALID", got)
	}
}
