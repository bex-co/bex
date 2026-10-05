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

// Package testenv decides what a test does when a dependency its CI job
// provisions (the test Postgres, OpenFGA, OpenBao, promtool) is missing.
// Locally it skips, saying how to provide the dependency. In a job that
// provisions them — RequireEnv=1, set by .github/workflows/backend-test.yml —
// it fails instead, so that job can never quietly stop running a test because
// a dependency went missing (w5/m112). Other jobs that run backend packages
// without the dependencies (the CLI's adapter tests) keep skipping.
package testenv

import (
	"os"
	"testing"
)

// RequireEnv names the variable a dependency-provisioning job sets to "1".
const RequireEnv = "BEX_TEST_REQUIRE_DEPS"

// SetupHint is how to provide the dependencies locally.
const SetupHint = `bash scripts/backend-test-deps.sh up && eval "$(bash scripts/backend-test-deps.sh env)"`

// Skip stands in for t.Skip when the reason is a missing provisioned
// dependency: it skips locally and fails where RequireEnv is "1".
func Skip(t testing.TB, reason string) {
	t.Helper()
	if os.Getenv(RequireEnv) == "1" {
		t.Fatalf("%s, but %s=1: this job must provide the dependency", reason, RequireEnv)
		return
	}
	t.Skipf("%s (to run it: %s)", reason, SetupHint)
}
