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

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/bex-co/bex/lego/types/k8sname"
)

// CronJobName is the scheduled CronJob backing a cron App (w7/m154). The App
// name itself while it fits the API server's 52-character CronJob limit — so
// every existing schedule keeps its identity — and a stable bounded fit beyond
// it. Every operator path that creates, reads or patches the CronJob uses this.
func CronJobName(appName string) string {
	return k8sname.FitCronJob(appName)
}

// ManualCronRunJobName is the deterministic Kubernetes Job name for a manual
// cron run. It lives in the leaf contract module because both sides of the App
// CR boundary need the same identity: the backend returns the run id before the
// operator has materialized the Job, and the operator must later create exactly
// that Job. The public crr- id remains a backend concern derived from this name.
//
// It is the CronJob's own name plus one segment, `-r<8 hex>` (at most 62
// characters, so never hashed), so a manual run's pods are named like a
// scheduled run's (`<cronjob>-<minute>`): one segment after the CronJob name,
// then the generated suffix, or the single segment the 63-character cut leaves.
// The metrics and usage pod selector matches both (egressquery.PodNameRegex,
// w4/m322). The former `<app>-run-<8 hex>` added two segments, and no run it
// created was charted or metered.
func ManualCronRunJobName(appName, runAt string) string {
	sum := sha256.Sum256([]byte(runAt))
	return fmt.Sprintf("%s-r%x", CronJobName(appName), sum[:4])
}

// LegacyManualCronRunJobName is ManualCronRunJobName before w4/m322. An
// operator that meets a Job of this name for the current runAt (created by
// the previous release while a deploy rolled) adopts it instead of starting
// the run a second time.
func LegacyManualCronRunJobName(appName, runAt string) string {
	sum := sha256.Sum256([]byte(runAt))
	return k8sname.Fit(fmt.Sprintf("%s-run-%x", appName, sum[:4]))
}

// NewestSuccessfulRunAt is the newer of a recorded last success and any
// Succeeded run in runs: the operator folds it into status.lastSuccessfulRunAt
// so the last success outlives the ten-run history, and bex-api applies it to
// read an operator from before that field (w3/m366). An unparseable time is
// never kept and never wins.
func NewestSuccessfulRunAt(recorded string, runs []CronRun) string {
	out := ""
	best, err := time.Parse(time.RFC3339Nano, recorded)
	if err == nil {
		out = recorded
	}
	for _, run := range runs {
		if run.Status != CronRunSucceeded {
			continue
		}
		if at, perr := time.Parse(time.RFC3339Nano, run.FinishedAt); perr == nil && (out == "" || at.After(best)) {
			best, out = at, run.FinishedAt
		}
	}
	return out
}
