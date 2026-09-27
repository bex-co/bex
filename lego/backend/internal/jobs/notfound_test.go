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

package jobs

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// missingJobStore answers every job lookup as absent.
type missingJobStore struct{ recordingJobStore }

func (missingJobStore) GetJob(context.Context, string, string, string) (store.Job, error) {
	return store.Job{}, store.ErrNotFound
}

// w4/154: a missing job on a real service names the job, so a caller can tell
// "no such job" from "no such service" (w8/021 did this for deploys).
func TestMissingJobOnARealServiceSaysJobNotFound(t *testing.T) {
	_, mux := filterHarnessWith(t, &missingJobStore{})

	rec := getJobs(t, mux, "/v1/services/web/jobs/job-doesnotexist00000000")
	var body struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusNotFound || body.Message != "job not found" {
		t.Fatalf("GET missing job => %d %q, want 404 \"job not found\"", rec.Code, body.Message)
	}

	rec = getJobs(t, mux, "/v1/services/nope/jobs/job-doesnotexist00000000")
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusNotFound || body.Message == "job not found" {
		t.Fatalf("GET job on a missing service => %d %q, want the neutral service miss", rec.Code, body.Message)
	}
}
