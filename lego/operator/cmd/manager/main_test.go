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

package main

import "testing"

func TestPositiveEnvInt(t *testing.T) {
	const key = "BEX_TEST_POSITIVE_ENV_INT"
	for _, tc := range []struct {
		name  string
		value string
		want  int
	}{
		{name: "unset", value: "", want: 1},
		{name: "positive", value: "2", want: 2},
		{name: "zero", value: "0", want: 1},
		{name: "negative", value: "-3", want: 1},
		{name: "malformed", value: "many", want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(key, tc.value)
			if got := positiveEnvInt(key, 1); got != tc.want {
				t.Fatalf("positiveEnvInt(%q, 1) = %d, want %d", key, got, tc.want)
			}
		})
	}
}
