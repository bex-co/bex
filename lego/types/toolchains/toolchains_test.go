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

package toolchains

import (
	"testing"
	"time"
)

func TestRequestLine(t *testing.T) {
	for request, want := range map[string]string{
		"3.11": "3.11", "3.11.9": "3.11", "v3.11.9": "3.11", "Python-3.12.1": "3.12", "ruby-3.3.6": "3.3",
		"3": "", "latest": "", "3.14.0rc1": "", "": "",
	} {
		got, ok := RequestLine(request)
		if got != want || ok != (want != "") {
			t.Errorf("RequestLine(%q) = %q, %v; want %q", request, got, ok, want)
		}
	}
}

func TestExactVersion(t *testing.T) {
	for _, tc := range []struct{ request, line, patch, want string }{
		{"3.11.9", "3.11", "3.11.18", "3.11.9"},
		{" python-3.11.9 ", "3.11", "3.11.18", "3.11.9"},
		{"v3.11.9", "3.11", "3.11.18", "3.11.9"},
		{"3.11.18", "3.11", "3.11.18", ""}, // the reviewed patch itself
		{"3.11", "3.11", "3.11.18", ""},    // a line, not a version
		{"3.12.1", "3.11", "3.11.18", ""},  // another line's version
		{"~3.11.9", "3.11", "3.11.18", ""}, // a range
	} {
		if got := ExactVersion(tc.request, tc.line, tc.patch); got != tc.want {
			t.Errorf("ExactVersion(%q, %q, %q) = %q, want %q", tc.request, tc.line, tc.patch, got, tc.want)
		}
	}
}

func TestDatedDefaultAt(t *testing.T) {
	first := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	second := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	steps := []DatedDefault{{first, "1.0.0"}, {second, "2.0.0"}}
	for _, tc := range []struct {
		created time.Time
		want    string
	}{
		{time.Time{}, ""}, {first.Add(-time.Second), ""}, {first, "1.0.0"}, {second.Add(-time.Second), "1.0.0"}, {second, "2.0.0"},
	} {
		if got := DatedDefaultAt(steps, tc.created); got != tc.want {
			t.Errorf("DatedDefaultAt(%v) = %q, want %q", tc.created, got, tc.want)
		}
	}
}

func TestImageVariant(t *testing.T) {
	repo, suffix, ok := ImageVariant("python:3.11-bookworm@sha256:abc", "3.11")
	if !ok || repo != "python" || suffix != "-bookworm" {
		t.Fatalf("ImageVariant = %q %q %v", repo, suffix, ok)
	}
	if _, _, ok := ImageVariant("python:3.12-bookworm", "3.11"); ok {
		t.Fatal("an image of another line split")
	}
	if _, _, ok := ImageVariant("python@sha256:abc", "3.11"); ok {
		t.Fatal("an untagged image split")
	}
}
