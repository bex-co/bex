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
	"math/rand"
	"slices"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// portSetInvariantSeed fixes every sweep below: the same generated cases run
// on every checkout, so a failure reproduces by seed, not by luck.
const portSetInvariantSeed = 20261126

// invariantDeclaredSet is one generated in-contract discovery result: TCP
// ports with nothing the network cannot publish (the registry already left
// reserved ports out, privateTCPPorts). Sizes run past 75 so the sweep
// exercises the cap, and most sets declare 3000 so the primary rule meets
// both its branches.
func invariantDeclaredSet(rng *rand.Rand) []int32 {
	set := map[int32]bool{}
	for len(set) < 1+rng.Intn(200) {
		p := int32(1 + rng.Intn(65535))
		if appv1alpha1.IsReservedImagePort(p) {
			continue
		}
		set[p] = true
	}
	if rng.Intn(4) != 0 {
		set[imagePortPreferred] = true
	}
	out := make([]int32, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

// wantPublished is the published set by definition, written plainly rather
// than through the helpers under test: the primary beside at most the 74
// lowest other declared ports, ascending.
func wantPublished(declared []int32, primary int32) []int32 {
	seen := map[int32]bool{primary: true}
	others := make([]int32, 0, len(declared))
	for _, p := range declared {
		if !seen[p] {
			seen[p] = true
			others = append(others, p)
		}
	}
	slices.Sort(others)
	if len(others) > appv1alpha1.MaxImagePorts-1 {
		others = others[:appv1alpha1.MaxImagePorts-1]
	}
	out := append([]int32{primary}, others...)
	slices.Sort(out)
	return out
}

// The published set (w6/181) always carries its primary, never a reserved
// port, and never more than Render's 75 open ports: the primary the image
// chose beside the lowest others, or a configured primary beside the lowest
// declared ones even when it sits above them all.
func TestPublishedImagePortsHoldTheInvariant(t *testing.T) {
	rng := rand.New(rand.NewSource(portSetInvariantSeed))
	for i := range 2000 {
		declared := invariantDeclaredSet(rng)
		// Half the cases shuffle and duplicate the input: the published set
		// is defined over the declared ports, not over their order.
		if i%2 == 0 {
			rng.Shuffle(len(declared), func(a, b int) { declared[a], declared[b] = declared[b], declared[a] })
			declared = append(declared, declared[:rng.Intn(len(declared))]...)
		}
		var primary int32
		switch i % 3 {
		case 0:
			primary = preferredPort(dedupSorted(declared))
		case 1:
			primary = declared[rng.Intn(len(declared))]
		default:
			primary = int32(1 + rng.Intn(65535))
			if appv1alpha1.IsReservedImagePort(primary) {
				primary = appv1alpha1.DefaultPort
			}
		}
		got := publishedImagePorts(declared, primary)
		want := wantPublished(declared, primary)
		if !slices.Equal(got, want) {
			t.Fatalf("case %d: publishedImagePorts(%v, %d) = %v, want %v", i, declared, primary, got, want)
		}
		if len(got) > appv1alpha1.MaxImagePorts {
			t.Fatalf("case %d: %d ports published, want at most %d", i, len(got), appv1alpha1.MaxImagePorts)
		}
		if !slices.Contains(got, primary) {
			t.Fatalf("case %d: primary %d missing from %v", i, primary, got)
		}
		for _, p := range got {
			if appv1alpha1.IsReservedImagePort(p) {
				t.Fatalf("case %d: reserved port %d published in %v", i, p, got)
			}
		}
	}
}

// dedupSorted is the test's own ascending-unique copy, for inputs the
// preferred-port rule reads.
func dedupSorted(ports []int32) []int32 {
	seen := map[int32]bool{}
	out := make([]int32, 0, len(ports))
	for _, p := range ports {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
}

// invariantScanPorts is one generated listener-scan answer: real ports mixed
// with everything a scan cannot produce on its own but the filters must still
// refuse — reserved ports, out-of-range numbers, duplicates, unsorted.
func invariantScanPorts(rng *rand.Rand) []int {
	n := rng.Intn(300)
	out := make([]int, 0, n)
	for len(out) < n {
		switch rng.Intn(10) {
		case 0:
			out = append(out, []int{18012, 18013, 19099}[rng.Intn(3)])
		case 1:
			out = append(out, []int{-100, -1, 0, 65536, 70000}[rng.Intn(5)])
		default:
			out = append(out, 1+rng.Intn(65535))
		}
	}
	return out
}

// wantListenerSet is a scan's answer as the status keeps it (listenerSet's
// own contract): sorted, deduplicated, without reserved ports, within the
// CRD's 75 entries.
func wantListenerSet(open []int) []int32 {
	seen := map[int32]bool{}
	out := make([]int32, 0, len(open))
	for _, p := range open {
		if p < 1 || p > 65535 || appv1alpha1.IsReservedImagePort(int32(p)) || seen[int32(p)] {
			continue
		}
		seen[int32(p)] = true
		out = append(out, int32(p))
	}
	slices.Sort(out)
	if len(out) > appv1alpha1.MaxImagePorts {
		out = out[:appv1alpha1.MaxImagePorts]
	}
	return out
}

// wantHTTPCandidates is a scan's open ports as HTTP candidates
// (httpCandidates' own contract): sorted, without reserved ports, stopping at
// the lowest 75.
func wantHTTPCandidates(open []int) []int32 {
	out := make([]int32, 0, len(open))
	for _, p := range open {
		if p < 1 || p > 65535 || appv1alpha1.IsReservedImagePort(int32(p)) {
			continue
		}
		out = append(out, int32(p))
	}
	slices.Sort(out)
	if len(out) > appv1alpha1.MaxImagePorts {
		out = out[:appv1alpha1.MaxImagePorts]
	}
	return out
}

// Scans see everything, so the scan-derived sets hold the invariant on
// hostile input: no reserved port and no more than 75 entries survive into
// the status record, the HTTP candidate list, or any set built from them.
func TestScanDerivedPortSetsHoldTheInvariant(t *testing.T) {
	rng := rand.New(rand.NewSource(portSetInvariantSeed + 1))
	for i := range 2000 {
		open := invariantScanPorts(rng)
		if got, want := listenerSet(open), wantListenerSet(open); !slices.Equal(got, want) {
			t.Fatalf("case %d: listenerSet(%v) = %v, want %v", i, open, got, want)
		}
		if got, want := httpCandidates(open), wantHTTPCandidates(open); !slices.Equal(got, want) {
			t.Fatalf("case %d: httpCandidates(%v) = %v, want %v", i, open, got, want)
		}
	}
}
