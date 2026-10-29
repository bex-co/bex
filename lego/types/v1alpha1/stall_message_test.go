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

package v1alpha1

import "testing"

func TestTerminalStallTextRestatesEachInFlightPhrase(t *testing.T) {
	for waiting, failed := range map[string]string{
		ProbeStallWaiting:         ProbeStallFailed,
		ImagePullStallWaiting:     ImagePullStallFailed,
		UnschedulableStallWaiting: UnschedulableStallFailed,
	} {
		if got := TerminalStallText("x " + waiting + " y"); got != "x "+failed+" y" {
			t.Errorf("TerminalStallText(%q) = %q", waiting, got)
		}
	}
	if got := TerminalStallText("build failed: exit 1"); got != "build failed: exit 1" {
		t.Errorf("unrelated text changed: %q", got)
	}
}
