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

package apps

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// cronScheduleVectorsPath is the one acceptance table for a cron schedule. Each
// row's code is what bex-api answers for it: null for a schedule it accepts,
// else the refusal's error code. The dashboard's cronScheduleProblem is tested
// against the same file (dashboard/src/features/services/lib/__tests__/
// cron.test.ts), so the form and bex-api cannot drift apart silently again:
// w1/m145 found the form previewing "0 0 * * 7" as "Every Sunday" while bex-api
// refused it, and w5/m118 pinned why each schedule is refused, not just whether.
const cronScheduleVectorsPath = "testdata/cron-schedule-vectors.json"

// loadCronScheduleVectors reads the shared table as schedule → refusal code
// ("" for an accepted schedule).
func loadCronScheduleVectors(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(cronScheduleVectorsPath)
	if err != nil {
		t.Fatalf("read %s: %v", cronScheduleVectorsPath, err)
	}
	var vectors []struct {
		Schedule string  `json:"schedule"`
		Code     *string `json:"code"`
	}
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatalf("parse %s: %v", cronScheduleVectorsPath, err)
	}
	table := make(map[string]string, len(vectors))
	for _, v := range vectors {
		if _, dup := table[v.Schedule]; dup {
			t.Fatalf("%s lists %q twice", cronScheduleVectorsPath, v.Schedule)
		}
		table[v.Schedule] = ""
		if v.Code != nil {
			table[v.Schedule] = *v.Code
		}
	}
	return table
}

// TestValidCronScheduleVectors makes bex-api the source of truth for the shared
// table: every row's code is the one checkCronSchedule actually answers.
func TestValidCronScheduleVectors(t *testing.T) {
	counts := map[string]int{}
	for schedule, want := range loadCronScheduleVectors(t) {
		got := ""
		if err := checkCronSchedule(schedule); err != nil {
			var coded *core.CodedError
			if !errors.As(err, &coded) || !errors.Is(err, core.ErrBadRequest) {
				t.Errorf("checkCronSchedule(%q) = %v: an uncoded refusal", schedule, err)
				continue
			}
			got = coded.Code
		}
		if got != want {
			t.Errorf("checkCronSchedule(%q) answers code %q, the table says %q", schedule, got, want)
		}
		counts[want]++
	}
	// Keeps the loop from passing vacuously if the file is emptied or reshaped.
	if counts[""] < 10 || counts[cronScheduleInvalid] < 10 || counts[cronScheduleNeverFires] < 5 {
		t.Fatalf("vector table too small: %v", counts)
	}
}
