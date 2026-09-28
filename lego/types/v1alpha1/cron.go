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
func ManualCronRunJobName(appName, runAt string) string {
	sum := sha256.Sum256([]byte(runAt))
	return k8sname.Fit(fmt.Sprintf("%s-run-%x", appName, sum[:4]))
}
