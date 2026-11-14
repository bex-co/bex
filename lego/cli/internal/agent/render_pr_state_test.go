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
	"bytes"
	"strings"
	"testing"
)

// The session view names its pull request's state beside the link (w9/m386),
// and shows the link alone until bex has read it.
func TestSessionShowsPullRequestState(t *testing.T) {
	var out bytes.Buffer
	writeSession(&out, session{ID: "ags-x", State: "succeeded", PRURL: "https://github.com/acme/api/pull/7", PRState: "merged"})
	if !strings.Contains(out.String(), "https://github.com/acme/api/pull/7 (merged)") {
		t.Fatalf("view:\n%s", out.String())
	}
	out.Reset()
	writeSession(&out, session{ID: "ags-x", State: "succeeded", PRURL: "https://github.com/acme/api/pull/7"})
	if strings.Contains(out.String(), "()") || !strings.Contains(out.String(), "pull/7") {
		t.Fatalf("view without a state:\n%s", out.String())
	}
}
