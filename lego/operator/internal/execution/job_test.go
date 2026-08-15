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

package execution

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

func jobWith(conditions ...batchv1.JobCondition) *batchv1.Job {
	return &batchv1.Job{Status: batchv1.JobStatus{Conditions: conditions}}
}

func condition(t batchv1.JobConditionType, status corev1.ConditionStatus, reason, message string) batchv1.JobCondition {
	return batchv1.JobCondition{Type: t, Status: status, Reason: reason, Message: message}
}

func TestJobHasConditionRequiresBothTypeAndTrueStatus(t *testing.T) {
	for _, tc := range []struct {
		name string
		job  *batchv1.Job
		want bool
	}{
		{name: "no conditions", job: jobWith(), want: false},
		{
			name: "matching type and true",
			job:  jobWith(condition(batchv1.JobComplete, corev1.ConditionTrue, "", "")),
			want: true,
		},
		{
			// The status check is the half that is easy to drop: Kubernetes
			// leaves a condition in the list with status False, so matching on
			// type alone reports a Job complete the moment it is created.
			name: "matching type but false",
			job:  jobWith(condition(batchv1.JobComplete, corev1.ConditionFalse, "", "")),
			want: false,
		},
		{
			name: "matching type but unknown",
			job:  jobWith(condition(batchv1.JobComplete, corev1.ConditionUnknown, "", "")),
			want: false,
		},
		{
			name: "other type is true",
			job:  jobWith(condition(batchv1.JobFailed, corev1.ConditionTrue, "", "")),
			want: false,
		},
		{
			name: "found among several",
			job: jobWith(
				condition(batchv1.JobSuspended, corev1.ConditionFalse, "", ""),
				condition(batchv1.JobFailed, corev1.ConditionFalse, "", ""),
				condition(batchv1.JobComplete, corev1.ConditionTrue, "", ""),
			),
			want: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := JobHasCondition(tc.job, batchv1.JobComplete); got != tc.want {
				t.Fatalf("JobHasCondition(JobComplete) = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestJobFinishedCoversBothTerminalConditions(t *testing.T) {
	for _, tc := range []struct {
		name string
		job  *batchv1.Job
		want bool
	}{
		{name: "still running", job: jobWith(), want: false},
		{
			name: "complete",
			job:  jobWith(condition(batchv1.JobComplete, corev1.ConditionTrue, "", "")),
			want: true,
		},
		{
			// The reason JobFinished exists: a caller that watched only
			// JobComplete would wait on a failed Job until its own deadline.
			name: "failed",
			job:  jobWith(condition(batchv1.JobFailed, corev1.ConditionTrue, "BackoffLimitExceeded", "")),
			want: true,
		},
		{
			name: "suspended is not finished",
			job:  jobWith(condition(batchv1.JobSuspended, corev1.ConditionTrue, "", "")),
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := JobFinished(tc.job); got != tc.want {
				t.Fatalf("JobFinished = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestJobFailureMessageFormatsReasonAndMessage(t *testing.T) {
	const fallback = "unknown build failure"
	for _, tc := range []struct {
		name string
		job  *batchv1.Job
		want string
	}{
		{
			name: "reason and message are joined",
			job:  jobWith(condition(batchv1.JobFailed, corev1.ConditionTrue, "BackoffLimitExceeded", "Job has reached the specified backoff limit")),
			want: "BackoffLimitExceeded: Job has reached the specified backoff limit",
		},
		{
			name: "reason alone when Kubernetes supplied no message",
			job:  jobWith(condition(batchv1.JobFailed, corev1.ConditionTrue, "DeadlineExceeded", "")),
			want: "DeadlineExceeded",
		},
		// The fallback is per-caller because this string reaches the tenant on
		// the App's status: a pre-deploy failure must not be reported with the
		// build's wording.
		{name: "no failed condition falls back", job: jobWith(), want: fallback},
		{
			name: "a false failed condition falls back",
			job:  jobWith(condition(batchv1.JobFailed, corev1.ConditionFalse, "Ignored", "not really failed")),
			want: fallback,
		},
		{
			name: "a complete job falls back",
			job:  jobWith(condition(batchv1.JobComplete, corev1.ConditionTrue, "", "")),
			want: fallback,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := JobFailureMessage(tc.job, fallback); got != tc.want {
				t.Fatalf("JobFailureMessage = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestNoPackageReimplementsTheJobConditionScan guards the dedupe. Four packages
// (build, predeploy, publish, controller's cleanup Job) each carried a verbatim
// copy of this scan before it graduated here, and a fifth Job dispatcher is far
// more likely to be written by copying a neighbour than by finding this file.
//
// The database-export helpers are deliberately exempt and named below: they
// answer genuinely different questions (one also consults Status.Succeeded, the
// other prefers Message over Reason and reports presence separately).
func TestNoPackageReimplementsTheJobConditionScan(t *testing.T) {
	root, err := filepath.Abs("../..") // internal/execution -> operator/
	if err != nil {
		t.Fatal(err)
	}
	exempt := map[string]bool{
		// Its own scan is intentional; see this test's doc comment.
		filepath.Join("internal", "controller", "database_exports.go"): true,
		// Projects each condition into a CronRun status + timestamp rather than
		// answering a yes/no question.
		filepath.Join("internal", "controller", "app_controller.go"): true,
	}
	// Matches the scan whether the condition type is a literal
	// (`c.Type == batchv1.JobComplete`) or a parameter (`c.Type == t`) — the
	// removed copies used the parameter form, so a regex keyed on the literal
	// would have let the exact duplication this guards against walk straight
	// back in. The match is then scoped to functions that actually take a
	// *batchv1.Job, because the identical shape is legitimately used to read
	// Pod and PVC conditions elsewhere in the operator.
	scan := regexp.MustCompile(`\.Type == [\w.]+ &&\s*\w+\.Status == corev1\.ConditionTrue`)
	scansAJob := func(source string) bool {
		for fn := range strings.SplitSeq(source, "\nfunc ") {
			if strings.Contains(fn, "*batchv1.Job") && scan.MatchString(fn) {
				return true
			}
		}
		return false
	}
	var offenders []string
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if exempt[rel] || rel == filepath.Join("internal", "execution", "job.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if scansAJob(string(source)) {
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("these files re-implement the Job condition scan instead of calling "+
			"execution.JobHasCondition/JobFinished/JobFailureMessage:\n\t%s",
			strings.Join(offenders, "\n\t"))
	}
}
