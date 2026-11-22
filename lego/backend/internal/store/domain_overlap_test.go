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

package store

import (
	"slices"
	"testing"
)

// A wildcard and every name under it, at any depth, take one overlap lock
// (w4/m326). For an ordinary host it is the registrable domain.
func TestDomainOverlapDomainIsSharedUnderAWildcard(t *testing.T) {
	for _, tc := range []struct{ host, want string }{
		{"example.com", "example.com"},
		{"api.example.com", "example.com"},
		{"*.example.com", "example.com"},
		{"login.eu.example.com", "example.com"},
		{"*.eu.example.com", "example.com"},
		{"api.example.co.uk", "example.co.uk"},
		{"*.example.co.uk", "example.co.uk"},
		{"a.foo.github.io", "foo.github.io"},
		// s3.amazonaws.com is on the public-suffix list, so the registrable
		// domain of a name under it is the name itself. Its overlap domain
		// stays the one a wildcard over amazonaws.com takes.
		{"x.s3.amazonaws.com", "amazonaws.com"},
		{"*.amazonaws.com", "amazonaws.com"},
	} {
		if got := domainOverlapDomain(tc.host); got != tc.want {
			t.Errorf("domainOverlapDomain(%q) = %q, want %q", tc.host, got, tc.want)
		}
	}
}

func TestWildcardsOverListEveryDepth(t *testing.T) {
	for _, tc := range []struct {
		name string
		want []string
	}{
		{"login.eu.example.com", []string{"*.eu.example.com", "*.example.com"}},
		{"api.example.com", []string{"*.example.com"}},
		{"example.com", []string{}},
	} {
		if got := wildcardsOver(tc.name); !slices.Equal(got, tc.want) {
			t.Errorf("wildcardsOver(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}
