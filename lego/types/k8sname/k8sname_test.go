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

package k8sname

import (
	"strings"
	"testing"
)

func TestFitLeavesShortNamesReadable(t *testing.T) {
	cases := []struct{ in, want string }{
		{"bld-web-gen-3", "bld-web-gen-3"},
		{"BLD-Web-GEN-3", "bld-web-gen-3"},
		{strings.Repeat("a", MaxLabel), strings.Repeat("a", MaxLabel)},
	}
	for _, c := range cases {
		if got := Fit(c.in); got != c.want {
			t.Errorf("Fit(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestFitKeepsDistinctNamesDistinct is the property a plain slice destroys:
// names that differ only past the cut must not collapse onto each other. Job
// names carry their revision at the END, so a slice makes every revision of a
// long-named resource one name — the caller then reuses the wrong revision's
// object.
func TestFitKeepsDistinctNamesDistinct(t *testing.T) {
	base := strings.Repeat("x", 70)
	seen := map[string]string{}
	for _, suffix := range []string{"-gen-1", "-gen-2", "-gen-10", "-latest"} {
		raw := base + suffix
		got := Fit(raw)
		if len(got) > MaxLabel {
			t.Fatalf("Fit(%q) is %d chars, over the %d limit", raw, len(got), MaxLabel)
		}
		if prev, dup := seen[got]; dup {
			t.Fatalf("%q and %q both truncate to %q", prev, raw, got)
		}
		seen[got] = raw
	}
}

// TestStableBindsTheWholeIdentityTuple: the readable prefix is lossy on
// purpose, so the hash — not the prefix — is what separates two resources whose
// names agree up to the cut.
func TestStableBindsTheWholeIdentityTuple(t *testing.T) {
	readable := strings.Repeat("z", 80)
	first := Stable(readable, "app", "rev-1")
	second := Stable(readable, "app", "rev-2")
	if first == second {
		t.Fatal("distinct identity tuples produced one name")
	}
	if len(first) > MaxLabel || len(second) > MaxLabel {
		t.Fatalf("names exceed the limit: %q (%d), %q (%d)", first, len(first), second, len(second))
	}
	if !strings.HasPrefix(first, "z") {
		t.Fatalf("name %q lost its readable prefix", first)
	}
}

// TestStableNeverEndsTheBaseOnASeparator keeps the result a valid DNS-1123
// label when the cut lands on a "-" or ".".
func TestStableNeverEndsTheBaseOnASeparator(t *testing.T) {
	raw := strings.Repeat("a", 49) + "----------" + strings.Repeat("b", 20)
	got := Stable(raw, raw)
	if strings.Contains(got, "--") {
		t.Fatalf("Stable(%q) = %q, left a separator run at the cut", raw, got)
	}
	if len(got) > MaxLabel {
		t.Fatalf("name %q is %d chars, over the limit", got, len(got))
	}
}

// TestFitCronJobBoundsLongNamesAndKeepsFittingOnes (w7/m154): a 54-character
// App name failed CronJob admission on every reconcile. Names at the boundary
// keep their identity; longer ones fit and stay distinct across shared prefixes.
func TestFitCronJobBoundsLongNamesAndKeepsFittingOnes(t *testing.T) {
	atLimit := strings.Repeat("c", MaxCronJob)
	if got := FitCronJob(atLimit); got != atLimit {
		t.Fatalf("FitCronJob renamed a %d-char name to %q", MaxCronJob, got)
	}
	reported := "tea-daif693dqjvc73e7as3g-qa-20260922-cd9361-longcronxx"
	got := FitCronJob(reported)
	if len(got) > MaxCronJob {
		t.Fatalf("FitCronJob(%q) = %q is %d chars, over %d", reported, got, len(got), MaxCronJob)
	}
	if got != FitCronJob(reported) {
		t.Fatal("FitCronJob is not deterministic")
	}
	seen := map[string]string{}
	for _, name := range []string{reported, reported + "2", strings.Repeat("c", MaxCronJob+1), strings.Repeat("c", MaxCronJob+2), strings.Repeat("c", 63)} {
		out := FitCronJob(name)
		if len(out) > MaxCronJob || strings.HasSuffix(out, "-") {
			t.Fatalf("FitCronJob(%q) = %q is not a valid bounded name", name, out)
		}
		if prev, dup := seen[out]; dup {
			t.Fatalf("%q and %q both fit to %q", prev, name, out)
		}
		seen[out] = name
	}
}
