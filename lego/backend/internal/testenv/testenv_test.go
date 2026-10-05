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

package testenv

import (
	"fmt"
	"strings"
	"testing"
)

// recorder captures what Skip did without ending the calling test.
type recorder struct {
	testing.TB
	skipped, failed bool
	msg             string
}

func (r *recorder) Helper() {}
func (r *recorder) Skipf(format string, args ...any) {
	r.skipped, r.msg = true, fmt.Sprintf(format, args...)
}
func (r *recorder) Fatalf(format string, args ...any) {
	r.failed, r.msg = true, fmt.Sprintf(format, args...)
}

func TestSkipSkipsLocallyWithTheSetupHint(t *testing.T) {
	t.Setenv(RequireEnv, "")
	r := &recorder{TB: t}
	Skip(r, "BEX_TEST_DB_URI not set")
	if !r.skipped || r.failed || !strings.Contains(r.msg, "BEX_TEST_DB_URI not set") || !strings.Contains(r.msg, "scripts/backend-test-deps.sh up") {
		t.Fatalf("skipped=%v failed=%v msg=%q, want a skip naming the reason and the setup command", r.skipped, r.failed, r.msg)
	}
}

func TestSkipFailsWhereTheJobProvisionsDependencies(t *testing.T) {
	t.Setenv(RequireEnv, "1")
	r := &recorder{TB: t}
	Skip(r, "BEX_TEST_DB_URI not set")
	if !r.failed || r.skipped || !strings.Contains(r.msg, "BEX_TEST_DB_URI not set") || !strings.Contains(r.msg, "must provide") {
		t.Fatalf("failed=%v skipped=%v msg=%q, want a failure naming the job's duty", r.failed, r.skipped, r.msg)
	}
}
