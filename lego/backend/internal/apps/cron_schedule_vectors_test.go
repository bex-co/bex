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
	"os"
	"testing"
)

// cronScheduleVectorsPath is the one acceptance table for a cron schedule. The
// dashboard's isValidCron is tested against the same file
// (dashboard/src/features/services/lib/__tests__/cron.test.ts), so the form
// and bex-api cannot drift apart silently again: w1/m145 found the form
// previewing "0 0 * * 7" as "Every Sunday" while bex-api refused it.
const cronScheduleVectorsPath = "testdata/cron-schedule-vectors.json"

// loadCronScheduleVectors reads the shared table as schedule → accepted.
func loadCronScheduleVectors(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(cronScheduleVectorsPath)
	if err != nil {
		t.Fatalf("read %s: %v", cronScheduleVectorsPath, err)
	}
	var vectors []struct {
		Schedule string `json:"schedule"`
		Valid    bool   `json:"valid"`
	}
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatalf("parse %s: %v", cronScheduleVectorsPath, err)
	}
	table := make(map[string]bool, len(vectors))
	for _, v := range vectors {
		if _, dup := table[v.Schedule]; dup {
			t.Fatalf("%s lists %q twice", cronScheduleVectorsPath, v.Schedule)
		}
		table[v.Schedule] = v.Valid
	}
	return table
}

// TestValidCronScheduleVectors makes bex-api the source of truth for the shared
// table: every expectation in it is what validCronSchedule actually returns.
func TestValidCronScheduleVectors(t *testing.T) {
	var valid, invalid int
	for schedule, want := range loadCronScheduleVectors(t) {
		if got := validCronSchedule(schedule); got != want {
			t.Errorf("validCronSchedule(%q) = %v, table says %v", schedule, got, want)
		}
		if want {
			valid++
		} else {
			invalid++
		}
	}
	// Keeps the loop from passing vacuously if the file is emptied or reshaped.
	if valid < 10 || invalid < 10 {
		t.Fatalf("vector table too small: %d valid, %d invalid", valid, invalid)
	}
}
