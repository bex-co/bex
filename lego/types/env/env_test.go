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

package env

import (
	"os"
	"testing"
)

func TestOr(t *testing.T) {
	t.Setenv("BEX_TEST_ENV_OR_SET", "value")
	t.Setenv("BEX_TEST_ENV_OR_EMPTY", "")
	t.Setenv("BEX_TEST_ENV_OR_UNSET", "placeholder")
	if err := os.Unsetenv("BEX_TEST_ENV_OR_UNSET"); err != nil {
		t.Fatal(err)
	}

	cases := []struct{ name, def, want string }{
		{"BEX_TEST_ENV_OR_SET", "def", "value"},
		{"BEX_TEST_ENV_OR_EMPTY", "def", "def"},
		{"BEX_TEST_ENV_OR_UNSET", "def", "def"},
	}
	for _, c := range cases {
		if got := Or(c.name, c.def); got != c.want {
			t.Errorf("Or(%q, %q) = %q, want %q", c.name, c.def, got, c.want)
		}
	}
}
