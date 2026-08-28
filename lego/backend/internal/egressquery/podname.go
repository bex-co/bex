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

import "fmt"

// Kubernetes' generateName rules, copied from
// k8s.io/apiserver/pkg/storage/names/generate.go (the version this repo pins):
//
//	maxNameLength          = 63
//	randomLength           = 5
//	MaxGeneratedNameLength = maxNameLength - randomLength  // 58
//
//	func (simpleNameGenerator) GenerateName(base string) string {
//		if len(base) > MaxGeneratedNameLength {
//			base = base[:MaxGeneratedNameLength]
//		}
//		return fmt.Sprintf("%s%s", base, utilrand.String(randomLength))
//	}
const (
	maxPodNameLength       = 63
	podRandomSuffixLength  = 5
	maxGeneratedNameLength = maxPodNameLength - podRandomSuffixLength
)

// PodNameRegex returns the PromQL regex (for a fully anchored `pod=~"…"`
// matcher) that selects the pods a workload named objectName owns, as they
// appear in cAdvisor/kubelet series. kubelet metrics carry pod NAMES but no pod
// labels — there is no app.bex.co/app to match on — so pod identity has to be
// reconstructed from the name shape.
//
// Two shapes, because Kubernetes generates only one of them reliably:
//
//   - `<obj>-<replicaset-hash>-<5 random>` — the untruncated Deployment pod (and
//     `<cronjob>-<timestamp>-<5 random>` for a CronJob's Job pods).
//
//   - `<obj>-<N alphanumerics>` — what is left once GenerateName truncates. The
//     ReplicaSet controller passes base = "<obj>-<hash>-"; when that exceeds 58
//     chars the cut eats the separating hyphen and the name collapses from two
//     segments to one, which the two-segment pattern alone cannot match. A
//     truncated name is ALWAYS exactly 63 chars (58 kept + 5 random), so N is
//     pinned to 62-len(obj) rather than left open — that exact length plus the
//     "no hyphen" character class is what keeps a sibling App out: any other
//     workload whose name starts with "<obj>-" carries a hyphen after its own
//     extra segment, so it can never fill N alphanumerics.
//
// This is the selector docs/ADR010-observability.md describes; w6/m110 replaced
// its two-segment-only form, which silently returned no series for every App
// whose Kubernetes object name pushed past the truncation threshold (a service
// name of ~22 characters, well inside the 30 ValidAppName allows).
//
// objectName is the KUBERNETES object name (core.CRName(tenant, app) for an
// App), never the workspace-scoped public service name — the pods are named
// after the object.
func PodNameRegex(objectName string) string {
	escaped := RegexEscape(objectName)
	twoSegment := fmt.Sprintf(`%s-[a-z0-9]+-[a-z0-9]{%d}`, escaped, podRandomSuffixLength)
	truncated := maxPodNameLength - len(objectName) - 1
	if truncated <= 0 || len(objectName) > maxGeneratedNameLength {
		// No room for a suffix at all: the object name already fills the pod
		// name, so only the (unreachable) two-segment form is worth emitting.
		return twoSegment
	}
	return fmt.Sprintf(`%s|%s-[a-z0-9]{%d}`, twoSegment, escaped, truncated)
}

// PodNameMatcher returns the full `namespace=…,pod=~…,container!=""` matcher
// list shared by every cAdvisor read: the usage meter's instance-seconds query
// and the metrics feature's CPU/memory/instance-count queries must select the
// same pods, or a service charts data it is not billed for (or the reverse).
// container!="" drops the pod-sandbox/aggregate rows that survive scrape-side
// relabeling.
func PodNameMatcher(namespace, objectName string) string {
	return fmt.Sprintf(`namespace=%q,pod=~%q,container!=""`, namespace, PodNameRegex(objectName))
}
