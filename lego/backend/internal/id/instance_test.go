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

import "testing"

func TestServiceInstanceIDIsNameDerivedAndStable(t *testing.T) {
	resource := "srv-c185th5c2rvvnhbfiltg"
	pod := "tea-ws-web-8645b77f5c-hg25t"
	got := ServiceInstanceID(resource, pod)
	if got != ServiceInstanceID(resource, pod) {
		t.Fatalf("ServiceInstanceID is not deterministic: %q", got)
	}
	if got == ServiceInstanceID(resource, "other-pod") {
		t.Fatalf("different pod names produced the same instance id %q", got)
	}
	if got == LegacyServiceInstanceID(resource, "fd57f2ee-57a4-4dc5-98aa-fd972f098a34") {
		t.Fatalf("name-derived and UID-derived ids must differ: %q", got)
	}
}

func TestMatchServiceInstanceAcceptsCanonicalLegacyAndRawName(t *testing.T) {
	resource := "srv-c185th5c2rvvnhbfiltg"
	pod := "web-rs-pod01"
	uid := "fd57f2ee-57a4-4dc5-98aa-fd972f098a34"
	canonical := ServiceInstanceID(resource, pod)
	legacy := LegacyServiceInstanceID(resource, uid)

	cases := []struct {
		name      string
		candidate string
		want      bool
	}{
		{"canonical", canonical, true},
		{"legacy uid", legacy, true},
		{"raw name", pod, true},
		{"foreign", ServiceInstanceID(resource, "other"), false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		if got := MatchServiceInstance(tc.candidate, resource, pod, uid); got != tc.want {
			t.Fatalf("%s: MatchServiceInstance(%q) = %v, want %v", tc.name, tc.candidate, got, tc.want)
		}
	}
}

func TestResolveInstanceSelectorUniqueAndAmbiguous(t *testing.T) {
	resource := "srv-c185th5c2rvvnhbfiltg"
	a := InstanceCandidate{Name: "pod-a", UID: "uid-a"}
	b := InstanceCandidate{Name: "pod-b", UID: "uid-b"}
	dupUID := InstanceCandidate{Name: "pod-c", UID: "uid-a"}

	name, ok := ResolveInstanceSelector(ServiceInstanceID(resource, a.Name), resource, []InstanceCandidate{a, b})
	if !ok || name != a.Name {
		t.Fatalf("canonical resolve = %q, %v", name, ok)
	}
	name, ok = ResolveInstanceSelector(a.Name, resource, []InstanceCandidate{a, b})
	if !ok || name != a.Name {
		t.Fatalf("raw-name resolve = %q, %v", name, ok)
	}
	if _, ok := ResolveInstanceSelector(LegacyServiceInstanceID(resource, a.UID), resource, []InstanceCandidate{a, dupUID}); ok {
		t.Fatal("ambiguous legacy UID selector must not resolve")
	}
	if _, ok := ResolveInstanceSelector(ServiceInstanceID(resource, "missing"), resource, []InstanceCandidate{a, b}); ok {
		t.Fatal("foreign selector must not resolve")
	}
}

func TestResolveInstanceSelectorsDropsUnresolved(t *testing.T) {
	resource := "dpg-c185th5c2rvvnhbfiltg"
	cands := []InstanceCandidate{{Name: "pg-1"}, {Name: "pg-2"}}
	got := ResolveInstanceSelectors(
		[]string{ServiceInstanceID(resource, "pg-1"), "pg-2", "foreign", ServiceInstanceID(resource, "pg-1")},
		resource,
		cands,
	)
	if len(got) != 2 || got[0] != "pg-1" || got[1] != "pg-2" {
		t.Fatalf("ResolveInstanceSelectors = %#v", got)
	}
	if got := ResolveInstanceSelectors([]string{"nobody"}, resource, cands); len(got) != 0 {
		t.Fatalf("all-unresolved = %#v, want empty", got)
	}
}
