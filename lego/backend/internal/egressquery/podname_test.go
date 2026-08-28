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

package egressquery

import (
	"fmt"
	"regexp"
	"testing"
)

// mustMatchPod compiles the selector the way Prometheus does — fully anchored,
// `^(?:re)$` — so a test can never pass on a substring match the real matcher
// would reject.
func mustMatchPod(t *testing.T, objectName, pod string) bool {
	t.Helper()
	re, err := regexp.Compile(`^(?:` + PodNameRegex(objectName) + `)$`)
	if err != nil {
		t.Fatalf("PodNameRegex(%q) does not compile: %v", objectName, err)
	}
	return re.MatchString(pod)
}

// TestPodNameRegexMatchesTruncatedNames is w6/m110 Defect B: Kubernetes'
// generateName truncates "<obj>-<hash>-" at 58 chars, and when the cut eats the
// separating hyphen the pod name collapses from two segments to one. The pod
// names below are the ones observed live in the QA workspace (Loki's `instance`
// label), which is why the long one is exactly 63 characters.
func TestPodNameRegexMatchesTruncatedNames(t *testing.T) {
	cases := []struct {
		name       string
		objectName string
		pod        string
		want       bool
	}{{
		name:       "untruncated two-segment pod",
		objectName: "tea-d98210cbbpdc73dcrkvg-block-eden-mono",
		pod:        "tea-d98210cbbpdc73dcrkvg-block-eden-mono-5f557c45fd-ll7x9",
		want:       true,
	}, {
		name:       "truncated single-segment pod (23-char service name)",
		objectName: "tea-d98210cbbpdc73dcrkvg-qa-20260826-webhook-svc",
		pod:        "tea-d98210cbbpdc73dcrkvg-qa-20260826-webhook-svc-55d855bcb7sxgd",
		want:       true,
	}, {
		name:       "truncated single-segment pod, second replica set",
		objectName: "tea-d98210cbbpdc73dcrkvg-qa-20260826-webhook-svc",
		pod:        "tea-d98210cbbpdc73dcrkvg-qa-20260826-webhook-svc-6d6cfb74ddh5dr",
		want:       true,
	}, {
		name:       "cron Job pod: <cronjob>-<timestamp>-<5 random>",
		objectName: "tea-d98210cbbpdc73dcrkvg-nightly",
		pod:        "tea-d98210cbbpdc73dcrkvg-nightly-29168400-k4w25",
		want:       true,
	}, {
		// The property the old two-segment-only selector was defended on, and
		// which the truncation alternative must not give up.
		name:       "sibling App's untruncated pod is not this App's",
		objectName: "tea-d98210cbbpdc73dcrkvg-web",
		pod:        "tea-d98210cbbpdc73dcrkvg-web-api-5f557c45fd-ll7x9",
		want:       false,
	}, {
		name:       "sibling App's truncated pod is not this App's",
		objectName: "tea-d98210cbbpdc73dcrkvg-web",
		pod:        "tea-d98210cbbpdc73dcrkvg-web-api-55d855bcb7sxgd",
		want:       false,
	}, {
		name:       "another tenant's same-named App is not this App's",
		objectName: "tea-d98210cbbpdc73dcrkvg-web",
		pod:        "tea-aaaaaaaaaaaaaaaaaaaa-web-5f557c45fd-ll7x9",
		want:       false,
	}, {
		name:       "a name that merely starts the same is not this App's",
		objectName: "tea-d98210cbbpdc73dcrkvg-web",
		pod:        "tea-d98210cbbpdc73dcrkvg-webhook-5f557c45fd-ll7x9",
		want:       false,
	}, {
		name:       "the workload's own object name is not one of its pods",
		objectName: "tea-d98210cbbpdc73dcrkvg-web",
		pod:        "tea-d98210cbbpdc73dcrkvg-web",
		want:       false,
	}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mustMatchPod(t, tc.objectName, tc.pod); got != tc.want {
				t.Errorf("PodNameRegex(%q) match %q = %v, want %v\nregex: %s",
					tc.objectName, tc.pod, got, tc.want, PodNameRegex(tc.objectName))
			}
		})
	}
}

// TestPodNameRegexCoversEveryGeneratedName walks the generator itself rather
// than a sample: for every service-name length ValidAppName allows and every
// pod-template-hash length Kubernetes produces, it reproduces
// SimpleNameGenerator exactly and asserts the selector matches what came out.
// This is the check that would have caught Defect B at the 22-character
// threshold, where the two-segment-only selector started returning nothing.
func TestPodNameRegexCoversEveryGeneratedName(t *testing.T) {
	const tenant = "tea-d98210cbbpdc73dcrkvg" // 24 chars, the real id shape
	for svcLen := 1; svcLen <= 30; svcLen++ { // ValidAppName caps at 30
		service := ""
		for len(service) < svcLen {
			service += "a"
		}
		object := tenant + "-" + service
		for hashLen := 6; hashLen <= 10; hashLen++ { // observed live: 6–10
			hash := ""
			for len(hash) < hashLen {
				hash += "b"
			}
			base := object + "-" + hash + "-"
			if len(base) > maxGeneratedNameLength {
				base = base[:maxGeneratedNameLength]
			}
			pod := base + "c2d4e" // utilrand.String(5), alphanumeric
			if !mustMatchPod(t, object, pod) {
				t.Errorf("service name %d chars, hash %d chars: pod %q (%d chars) unmatched\nregex: %s",
					svcLen, hashLen, pod, len(pod), PodNameRegex(object))
			}
			if len(pod) > maxPodNameLength {
				t.Errorf("generated pod name %q exceeds %d chars — test model is wrong", pod, maxPodNameLength)
			}
		}
	}
}

// TestPodNameMatcherIsShared pins the matcher string both the usage meter and
// the metrics feature build from, so the two can never drift into charging for
// pods the Metrics page does not chart.
func TestPodNameMatcherIsShared(t *testing.T) {
	got := PodNameMatcher("tea-abc", "tea-abc-web")
	want := fmt.Sprintf(`namespace="tea-abc",pod=~%q,container!=""`, PodNameRegex("tea-abc-web"))
	if got != want {
		t.Errorf("PodNameMatcher:\n got %s\nwant %s", got, want)
	}
}
