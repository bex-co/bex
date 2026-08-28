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

package controller

import (
	"regexp"
	"testing"
)

// TestPodNameRegexSurvivesGeneratedNameTruncation keeps the operator's copy of
// the pod-name selector in step with the backend's egressquery.PodNameRegex
// (w6/m110). The Prometheus MetricsReader is the fallback for a cluster without
// metrics-server; when it is the one in use, a two-segment-only matcher makes
// the autoscaler read an empty series — indistinguishable from an idle App —
// for every App whose object name crossed Kubernetes' 58-char truncation point.
func TestPodNameRegexSurvivesGeneratedNameTruncation(t *testing.T) {
	cases := []struct {
		app, pod string
		want     bool
	}{
		{"tea-d98210cbbpdc73dcrkvg-block-eden-mono", "tea-d98210cbbpdc73dcrkvg-block-eden-mono-5f557c45fd-ll7x9", true},
		{"tea-d98210cbbpdc73dcrkvg-qa-20260826-webhook-svc", "tea-d98210cbbpdc73dcrkvg-qa-20260826-webhook-svc-55d855bcb7sxgd", true},
		{"tea-d98210cbbpdc73dcrkvg-web", "tea-d98210cbbpdc73dcrkvg-web-api-5f557c45fd-ll7x9", false},
		{"tea-d98210cbbpdc73dcrkvg-web", "tea-d98210cbbpdc73dcrkvg-web-api-55d855bcb7sxgd", false},
	}
	for _, tc := range cases {
		re := regexp.MustCompile(`^(?:` + podNameRegex(tc.app) + `)$`) // Prometheus anchors as ^(?:re)$
		if got := re.MatchString(tc.pod); got != tc.want {
			t.Errorf("podNameRegex(%q) match %q = %v, want %v", tc.app, tc.pod, got, tc.want)
		}
	}
}
