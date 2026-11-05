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

package rendertest

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// RunSuite is a validating suite's TestMain: it runs the package, then fails a
// passing full run in which one of suite's Allowlist entries excused no
// response. That divergence is gone, and its entry must go too, or it would
// silently excuse the next regression at the same pointer. The tally is only
// complete when every test ran with its dependencies — a full run (no -run or
// -skip) against the real test Postgres, which CI's backend job and /ship
// provide — so other runs skip the check.
func RunSuite(m *testing.M, suite string) int {
	code := m.Run()
	if code != 0 || !fullRun() {
		return code
	}
	if stale := Unexercised(suite); len(stale) > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: %d rendertest.Allowlist entries for %s matched no response in a full run; bex now conforms there, so remove them (and lower allowlistSize):\n  %s\n",
			len(stale), suite, strings.Join(stale, "\n  "))
		return 1
	}
	return 0
}

func fullRun() bool {
	for _, name := range []string{"test.run", "test.skip"} {
		if f := flag.Lookup(name); f != nil && f.Value.String() != "" {
			return false
		}
	}
	return os.Getenv(testenv.RequireEnv) == "1" || os.Getenv("BEX_TEST_DB_URI") != ""
}
