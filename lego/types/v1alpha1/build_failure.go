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

import "strings"

// BuildFailedMessage opens the reason of a build whose own work failed: its
// build command, a Dockerfile step, or another phase of the build Job, as
// FormatBuildFailed words it (w6/m123). Render's deploy log closes such a
// build with `==> Build failed 😞` (w8/393).
const BuildFailedMessage = "build failed"

// buildFailedNoTail ends a build failure whose output capture did not survive.
const buildFailedNoTail = " — check the build logs for the failing step's output"

// FormatBuildFailed is the reason of a build whose own work failed: the step
// that failed ("" when none is named), then that step's output tail, or a
// pointer at the build logs when the capture did not survive.
func FormatBuildFailed(step, tail string) string {
	msg := BuildFailedMessage
	if step != "" {
		msg += " in the " + step + " step"
	}
	if tail == "" {
		return msg + buildFailedNoTail
	}
	return msg + ":\n" + tail
}

// IsBuildFailed reports a reason FormatBuildFailed wrote. A build that ran out
// of its time limit, failed for an infrastructure fault, or never ran reads
// otherwise, and so does any reason the control plane wrote.
func IsBuildFailed(reason string) bool {
	rest, ok := strings.CutPrefix(reason, BuildFailedMessage)
	if !ok {
		return false
	}
	if step, named := strings.CutPrefix(rest, " in the "); named {
		if _, rest, ok = strings.Cut(step, " step"); !ok {
			return false
		}
	}
	return strings.HasPrefix(rest, ":\n") || rest == buildFailedNoTail
}
