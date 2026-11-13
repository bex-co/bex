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

package agent

import (
	"os"
	"testing"
)

// TestMain runs git the way a CI runner does: no global or system config and
// no identity git can guess, so a test that leans on the developer's own
// ~/.gitconfig fails here rather than only in CI (w2/083). The one identity is
// set here, after useConfigOnly, so a test drops it with GIT_CONFIG_COUNT=1.
func TestMain(m *testing.M) {
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL", "EMAIL"} {
		_ = os.Unsetenv(k)
	}
	for k, v := range map[string]string{
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_CONFIG_COUNT":    "3",
		"GIT_CONFIG_KEY_0":    "user.useConfigOnly",
		"GIT_CONFIG_VALUE_0":  "true",
		"GIT_CONFIG_KEY_1":    "user.name",
		"GIT_CONFIG_VALUE_1":  "Test",
		"GIT_CONFIG_KEY_2":    "user.email",
		"GIT_CONFIG_VALUE_2":  "test@example.com",
	} {
		_ = os.Setenv(k, v)
	}
	os.Exit(m.Run())
}
