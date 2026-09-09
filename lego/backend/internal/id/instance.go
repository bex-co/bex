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

package id

// Instance identity (w5/m87): one public id for live listing, metrics, logs, and
// SSH. Prometheus and Loki retain only Kubernetes pod (or PVC ordinal) names, so
// the canonical derivation is name-based. Pre-m87 live SSH selectors hashed the
// Pod UID instead; MatchServiceInstance and ResolveInstanceSelector keep those
// working without letting a retired or foreign selector fall through to another
// live pod. Raw pod-name filters already bookmarked in the dashboard remain
// accepted as inputs; new outputs never emit raw names.

// ServiceInstanceID is the canonical public instance id for one pod/ordinal of
// resourceID (a service, Postgres, or Key Value public id). Derivation uses the
// Kubernetes name so historical Prom/Loki series resolve to the same id as
// live ListInstances without a live Pod UID lookup.
func ServiceInstanceID(resourceID, podName string) string {
	return DeriveServiceInstance(resourceID, podName)
}

// LegacyServiceInstanceID is the pre-m87 live id derived from Pod UID. Emit
// paths must not use it; resolve paths may accept it for already-issued SSH
// selectors and similar bookmarks.
func LegacyServiceInstanceID(resourceID, podUID string) string {
	return DeriveServiceInstance(resourceID, podUID)
}

// InstanceCandidate is one authorized pod (or ordinal) that may satisfy an
// instance selector. Name is required; UID is optional and enables legacy
// live-id matching when present.
type InstanceCandidate struct {
	Name string
	UID  string
}

// MatchServiceInstance reports whether candidate identifies this pod: the
// canonical name-derived id, the legacy UID-derived id, or the raw pod name
// (bookmarked log filters). Empty candidate never matches. A foreign or
// ambiguous selector must not be remapped onto a different pod — callers that
// need uniqueness use ResolveInstanceSelector over a bounded candidate set.
func MatchServiceInstance(candidate, resourceID, podName, podUID string) bool {
	if candidate == "" || podName == "" {
		return false
	}
	if candidate == podName {
		return true
	}
	if candidate == ServiceInstanceID(resourceID, podName) {
		return true
	}
	if podUID != "" && candidate == LegacyServiceInstanceID(resourceID, podUID) {
		return true
	}
	return false
}

// ResolveInstanceSelector maps one public instance id (or legacy raw pod name /
// UID-derived id) onto a unique pod name from the authorized candidate set.
// ok is false when no candidate matches or more than one does — callers must
// not broaden the query.
func ResolveInstanceSelector(selector, resourceID string, candidates []InstanceCandidate) (podName string, ok bool) {
	if selector == "" {
		return "", false
	}
	var match string
	for _, c := range candidates {
		if c.Name == "" {
			continue
		}
		if !MatchServiceInstance(selector, resourceID, c.Name, c.UID) {
			continue
		}
		if match != "" && match != c.Name {
			return "", false
		}
		match = c.Name
	}
	if match == "" {
		return "", false
	}
	return match, true
}

// ResolveInstanceSelectors translates a filter's public/legacy selectors into
// the internal pod names Loki and the live tail match on. Unresolved or
// ambiguous selectors are dropped (they match nothing) rather than widening
// the query; an all-unresolved input yields a nil slice so the caller can
// fail closed.
func ResolveInstanceSelectors(selectors []string, resourceID string, candidates []InstanceCandidate) []string {
	if len(selectors) == 0 {
		return nil
	}
	out := make([]string, 0, len(selectors))
	seen := make(map[string]struct{}, len(selectors))
	for _, sel := range selectors {
		name, ok := ResolveInstanceSelector(sel, resourceID, candidates)
		if !ok {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}
