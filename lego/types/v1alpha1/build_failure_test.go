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

// TestIsBuildFailedReadsOnlyWhatFormatBuildFailedWrote is w8/393: bex-api
// closes a build's log with Render's `==> Build failed 😞` only for a build
// whose own work failed, so the reason the operator formats must read back as
// one, and the build's other failures must not.
func TestIsBuildFailedReadsOnlyWhatFormatBuildFailedWrote(t *testing.T) {
	for _, c := range []struct{ step, tail, want string }{
		{"build command", "error: build command 'exit 3' exited with code 3",
			"build failed in the build command step:\nerror: build command 'exit 3' exited with code 3"},
		{"docker build", "", "build failed in the docker build step — check the build logs for the failing step's output"},
		{"", "boom", "build failed:\nboom"},
		{"", "", "build failed — check the build logs for the failing step's output"},
	} {
		got := FormatBuildFailed(c.step, c.tail)
		if got != c.want {
			t.Fatalf("FormatBuildFailed(%q, %q) = %q, want %q", c.step, c.tail, got, c.want)
		}
		if !IsBuildFailed(got) {
			t.Fatalf("IsBuildFailed(%q) = false, want true", got)
		}
	}
	for _, reason := range []string{
		"",
		"build failed",
		"boom",
		"build failed for an infrastructure reason and was retried — this was not caused by a change in your code; trigger a new deploy to retry, and contact support if it keeps happening",
		"build exceeded its time limit (60 minutes) — reduce work done at build time, or enable the build cache so unchanged layers are reused",
		FormatDeployReason(DeployReasonBuildTimedOut),
		FormatDeployReason(DeployReasonBuildAbandoned),
		BuildMinutesExhaustedMessage,
		"build source bld-1: not found",
		"build failed in the build command",
	} {
		if IsBuildFailed(reason) {
			t.Fatalf("IsBuildFailed(%q) = true, want false", reason)
		}
	}
}

// TestPublishOutputReadsOnlyThePublishStep is w8/395: the publish Job's output
// never reaches the build log, so bex-api writes what its step said, and only
// that step's; a build's own output is in the log already.
func TestPublishOutputReadsOnlyThePublishStep(t *testing.T) {
	const said = "Publish directory nope does not exist!"
	reason := FormatBuildFailed(PublishStep, said)
	if !IsBuildFailed(reason) {
		t.Fatalf("IsBuildFailed(%q) = false, want true", reason)
	}
	if got, ok := PublishOutput(reason); !ok || got != said {
		t.Fatalf("PublishOutput(%q) = %q, %v; want %q", reason, got, ok, said)
	}
	for _, other := range []string{
		FormatBuildFailed("publish directory", said),
		FormatBuildFailed("", said),
		FormatBuildFailed(PublishStep, ""),
		said,
	} {
		if got, ok := PublishOutput(other); ok {
			t.Fatalf("PublishOutput(%q) = %q, true; want false", other, got)
		}
	}
}
