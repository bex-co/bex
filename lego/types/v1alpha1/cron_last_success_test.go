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

// w3/m366: a cron job's last success outlives the ten-run history. After one
// success and ten failures the success has been evicted from status.runs, and
// status.lastSuccessfulRunAt must still name it.
func TestNewestSuccessfulRunAtOutlivesTheHistory(t *testing.T) {
	run := func(status, finished string) CronRun {
		return CronRun{Name: "r-" + finished, Status: status, FinishedAt: finished}
	}
	const success = "2026-11-12T08:00:30Z"
	got := NewestSuccessfulRunAt("", []CronRun{run(CronRunFailed, "2026-11-12T08:01:30Z"), run(CronRunSucceeded, success)})
	if got != success {
		t.Fatalf("first observation = %q, want the success %q", got, success)
	}
	var failures []CronRun
	for i := range 10 {
		failures = append(failures, run(CronRunFailed, "2026-11-12T09:0"+string(rune('0'+i))+":30Z"))
	}
	if got = NewestSuccessfulRunAt(got, failures); got != success {
		t.Errorf("after ten failures = %q, want the evicted success %q", got, success)
	}
	// A newer success wins; an older one, a canceled run or a running one never does.
	newer := "2026-11-12T10:00:30Z"
	if got = NewestSuccessfulRunAt(got, []CronRun{run(CronRunSucceeded, newer)}); got != newer {
		t.Errorf("newer success = %q, want %q", got, newer)
	}
	others := []CronRun{
		run(CronRunSucceeded, "2026-11-12T07:00:30Z"),
		run(CronRunCanceled, "2026-11-12T11:00:00Z"),
		{Name: "running", Status: CronRunRunning},
		run(CronRunSucceeded, ""),
		run(CronRunSucceeded, "not a time"),
	}
	if got = NewestSuccessfulRunAt(got, others); got != newer {
		t.Errorf("older/canceled/running/unparseable = %q, want %q kept", got, newer)
	}
	// A cron job that never succeeded has none, and a corrupt record is replaced.
	if got := NewestSuccessfulRunAt("", failures); got != "" {
		t.Errorf("never succeeded = %q, want empty", got)
	}
	if got := NewestSuccessfulRunAt("garbage", nil); got != "" {
		t.Errorf("a corrupt record kept: %q", got)
	}
	if got := NewestSuccessfulRunAt("garbage", []CronRun{run(CronRunSucceeded, success)}); got != success {
		t.Errorf("a corrupt record was not replaced: %q", got)
	}
}
