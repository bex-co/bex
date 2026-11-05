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

package api

import (
	"os"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/rendertest"
)

// TestMain fails a full run that leaves an internal/api Allowlist entry
// unexercised (rendertest.RunSuite): every test building its handler with
// buildHandler feeds the tally.
func TestMain(m *testing.M) {
	os.Exit(rendertest.RunSuite(m, rendertest.SuiteAPI))
}
