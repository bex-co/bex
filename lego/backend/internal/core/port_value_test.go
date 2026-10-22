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
