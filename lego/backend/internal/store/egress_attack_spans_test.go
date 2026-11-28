package store

import (
	"math/rand"
	"testing"
	"time"
)

// Property: EgressAttackHourSpans covers exactly the whole hours the SQL
// predicate (egressAttackHour) counts — a window overlaps hour [h, h+1h) when
// it starts before h+1h and ends after h — clipped to [from, to). The
// Outbound Bandwidth footer subtracts exactly these spans, so any disagreement
// would show the customer a footer that does not match the bill.
func TestEgressAttackHourSpansMatchTheSQLPredicate(t *testing.T) {
	rng := rand.New(rand.NewSource(0x5eed07))
	const trials = 200

	var hitPartialHour, hitExactBoundary, hitAdjacentMerge int
	var hitEmpty, hitMultiWindow, hitClipped int

	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < trials; i++ {
		// Random [from, to): sometimes hour-aligned, sometimes not.
		from := base.Add(time.Duration(rng.Intn(72)) * time.Hour)
		if rng.Intn(100) < 30 {
			from = from.Add(time.Duration(rng.Intn(60)) * time.Minute)
		}
		spanHours := 1 + rng.Intn(72)
		to := from.Add(time.Duration(spanHours) * time.Hour)
		if rng.Intn(100) < 30 {
			to = to.Add(time.Duration(rng.Intn(60)) * time.Minute)
		}
		if !from.Before(to) {
			to = from.Add(time.Hour)
		}

		nw := 1 + rng.Intn(3)
		if nw > 1 {
			hitMultiWindow++
		}
		windows := make([]EgressAttackWindow, 0, nw)
		for k := 0; k < nw; k++ {
			// Starts around [from, to), ends after the start; sometimes
			// exact-hour, sometimes mid-hour, sometimes degenerate.
			start := from.Add(-time.Duration(rng.Intn(48)) * time.Hour).Add(time.Duration(rng.Intn(120)) * time.Minute)
			end := start.Add(time.Duration(rng.Intn(72)) * time.Hour).Add(time.Duration(rng.Intn(120)) * time.Minute)
			switch rng.Intn(100) {
			case 0, 1, 2:
				start = start.Truncate(time.Hour)
				end = end.Truncate(time.Hour)
				if !start.Before(end) {
					end = start.Add(time.Hour)
				}
				hitExactBoundary++
			case 3:
				// Degenerate: zero-length and inverted windows.
				if rng.Intn(2) == 0 {
					end = start
				} else {
					start = end.Add(time.Hour)
				}
			}
			if start != start.Truncate(time.Hour) || end != end.Truncate(time.Hour) {
				hitPartialHour++
			}
			windows = append(windows, EgressAttackWindow{StartsAt: start, EndsAt: end})
		}

		got := EgressAttackHourSpans(windows, from, to)

		// Independent expectation: brute-force every hour, SQL predicate
		// plus [from, to) overlap.
		wantHours := map[time.Time]bool{}
		for h := from.Truncate(time.Hour); h.Before(to); h = h.Add(time.Hour) {
			if h.Add(time.Hour).After(from) && h.Before(to) {
				for _, w := range windows {
					if w.StartsAt.Before(h.Add(time.Hour)) && w.EndsAt.After(h) {
						wantHours[h] = true
						break
					}
				}
			}
		}
		gotHours := map[time.Time]bool{}
		for _, s := range got {
			if !s.Start.Before(s.End) {
				t.Fatalf("trial %d: span [%s, %s) is empty or inverted", i, s.Start, s.End)
			}
			// Spans clip to [from, to), which may cut mid-hour: iterate
			// from the truncated start and keep hours overlapping both.
			for h := s.Start.Truncate(time.Hour); h.Before(s.End); h = h.Add(time.Hour) {
				if h.Add(time.Hour).After(s.Start) && h.Before(s.End) &&
					h.Add(time.Hour).After(from) && h.Before(to) {
					gotHours[h] = true
				}
			}
		}
		if len(gotHours) != len(wantHours) {
			t.Fatalf("trial %d: spans cover %d hours, want %d (windows %+v, [%s, %s))",
				i, len(gotHours), len(wantHours), windows, from, to)
		}
		for h := range wantHours {
			if !gotHours[h] {
				t.Fatalf("trial %d: hour %s missing from spans (windows %+v, [%s, %s))",
					i, h, windows, from, to)
			}
		}

		// Spans are merged and disjoint: adjacent or overlapping windows
		// read as one footer subtraction.
		for k := 1; k < len(got); k++ {
			if !got[k].Start.After(got[k-1].Start) {
				t.Fatalf("trial %d: spans out of order: %+v", i, got)
			}
			if !got[k].Start.After(got[k-1].End) {
				t.Fatalf("trial %d: spans overlap: %+v", i, got)
			}
			if got[k].Start.Equal(got[k-1].End) {
				t.Fatalf("trial %d: adjacent spans not merged: %+v", i, got)
			}
		}
		if len(got) == 0 {
			hitEmpty++
		}
		if len(got) < nw {
			hitAdjacentMerge++
		}
		for _, w := range windows {
			if len(wantHours) > 0 && (w.StartsAt.Before(from) || w.EndsAt.After(to)) {
				hitClipped++
				break
			}
		}
	}

	if hitPartialHour == 0 || hitExactBoundary == 0 || hitAdjacentMerge == 0 ||
		hitEmpty == 0 || hitMultiWindow == 0 || hitClipped == 0 {
		t.Fatalf("property missed branches: partial=%d exact=%d merged=%d empty=%d multi=%d clipped=%d",
			hitPartialHour, hitExactBoundary, hitAdjacentMerge, hitEmpty, hitMultiWindow, hitClipped)
	}
}
