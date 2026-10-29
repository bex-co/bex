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

// A rollout-stall diagnosis is written while a deploy is still open (the
// in-flight half) and restated as an outcome once the deploy has given up
// (w4/202). The operator words both; bex-api restates a row's last in-flight
// diagnosis when it closes the row from it (w8/219). The pairs are shared so
// the two sides can never drift.
const (
	ProbeStallWaiting         = "has not succeeded, so the rollout is waiting"
	ProbeStallFailed          = "never succeeded within the rollout window, so the deploy failed"
	ImagePullStallWaiting     = "image pull is failing"
	ImagePullStallFailed      = "image pull failed"
	UnschedulableStallWaiting = "the instance starts as soon as capacity frees up"
	UnschedulableStallFailed  = "the rollout window ended before capacity freed up; deploy again later"
	QuotaStallWaiting         = "the new instance starts as soon as capacity frees up"
	QuotaStallFailed          = "the rollout window ended before capacity freed up, so the deploy failed; deploy again once there is room"
)

// TerminalStallText restates an in-flight stall diagnosis as the outcome of a
// deploy that failed. Text without an in-flight phrase is returned unchanged.
func TerminalStallText(msg string) string {
	return strings.NewReplacer(
		ProbeStallWaiting, ProbeStallFailed,
		ImagePullStallWaiting, ImagePullStallFailed,
		UnschedulableStallWaiting, UnschedulableStallFailed,
		QuotaStallWaiting, QuotaStallFailed,
	).Replace(msg)
}
