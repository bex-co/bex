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
	"math/rand/v2"
	"testing"
	"time"
)

func TestSandboxIntervalChunksSplitHoursAndCarryFractionalUnits(t *testing.T) {
	hour := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	chunks, remainder := sandboxIntervalChunks(553, sandboxWeightDivisor,
		hour.Add(time.Hour-500*time.Millisecond),
		hour.Add(time.Hour+500*time.Millisecond), 0)
	if remainder != 0 || len(chunks) != 2 {
		t.Fatalf("chunks=%+v remainder=%d", chunks, remainder)
	}
	if !chunks[0].window.Equal(hour) || chunks[0].quantity != 276 {
		t.Errorf("first chunk = %+v", chunks[0])
	}
	if !chunks[1].window.Equal(hour.Add(time.Hour)) || chunks[1].quantity != 277 {
		t.Errorf("second chunk = %+v", chunks[1])
	}
	if chunks[0].quantity+chunks[1].quantity != 553 {
		t.Errorf("one weighted second split to %d units", chunks[0].quantity+chunks[1].quantity)
	}
}

func TestSandboxIntervalChunksDefaultShapeHalfHour(t *testing.T) {
	start := time.Date(2026, 8, 1, 12, 15, 0, 0, time.UTC)
	chunks, remainder := sandboxIntervalChunks(553, sandboxWeightDivisor, start, start.Add(30*time.Minute), 0)
	if remainder != 0 || len(chunks) != 1 || chunks[0].quantity != 553*30*60 {
		t.Fatalf("chunks=%+v remainder=%d", chunks, remainder)
	}
}

// TestSandboxIntervalChunksConserveUsageAcrossHourBoundaries sweeps random
// intervals through sandboxIntervalChunks and pins its conservation contract:
// elapsedNanos*perSecond sub-units enter, and exactly
// sum(quantity)*divisor + outRemainder - inRemainder come out, so splitting at
// UTC hour boundaries neither creates nor destroys billable usage and a fast
// poll cadence cannot systematically round it down. Chunk windows must stay
// hour-aligned, strictly increasing, and inside the interval's own hours, and
// the carried remainder must stay a proper fraction of a unit. A zero-length
// interval writes nothing and carries the remainder through.
func TestSandboxIntervalChunksConserveUsageAcrossHourBoundaries(t *testing.T) {
	divisors := []int64{sandboxWeightDivisor, sandboxMemoryDivisor, 1, 2, 3, 1000}
	base := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	for seed := uint64(1); seed <= 2000; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0x5a5d))
		perSecond := rng.Int64N(1_000_001) // the observation gate's bound
		divisor := divisors[rng.IntN(len(divisors))]
		inRem := rng.Int64N(divisor)
		start := base.
			Add(time.Duration(rng.Int64N(int64(72 * time.Hour)))).
			Add(time.Duration(rng.Int64N(int64(time.Hour))))
		end := start.Add(time.Duration(rng.Int64N(int64(2 * time.Hour)))).Add(time.Duration(rng.Int64N(1000)))
		switch {
		case seed%7 == 0:
			end = start // a replayed sample accrues nothing
		case seed%11 == 0:
			start = start.UTC().Truncate(time.Hour) // an interval opened on the hour
			end = start.Add(time.Duration(rng.Int64N(int64(2 * time.Hour))))
		}
		// Two hours at the observation gate's bound stays in int64, so the
		// oracle below cannot overflow where the splitter does not.
		chunks, outRem := sandboxIntervalChunks(perSecond, divisor, start, end, inRem)
		if outRem < 0 || outRem >= divisor {
			t.Fatalf("seed %d: remainder %d escapes [0, %d)", seed, outRem, divisor)
		}
		if start.Equal(end) {
			if len(chunks) != 0 || outRem != inRem {
				t.Fatalf("seed %d: zero-length interval wrote chunks=%+v remainder=%d", seed, chunks, outRem)
			}
			continue
		}
		var total int64
		for i, c := range chunks {
			if c.quantity < 0 {
				t.Fatalf("seed %d: negative quantity %+v", seed, c)
			}
			total += c.quantity
			if !c.window.Equal(c.window.UTC().Truncate(time.Hour)) {
				t.Fatalf("seed %d: window %s is not hour-aligned", seed, c.window)
			}
			if i > 0 && !c.window.After(chunks[i-1].window) {
				t.Fatalf("seed %d: windows not strictly increasing: %+v", seed, chunks)
			}
		}
		if len(chunks) > 0 {
			first, last := chunks[0].window, chunks[len(chunks)-1].window
			if first.Before(start.UTC().Truncate(time.Hour)) || last.After(end.UTC().Truncate(time.Hour)) {
				t.Fatalf("seed %d: windows [%s, %s] escape interval [%s, %s]",
					seed, first, last, start, end)
			}
		}
		elapsed := end.Sub(start).Nanoseconds()
		if total*divisor+outRem-inRem != elapsed*perSecond {
			t.Fatalf("seed %d: perSecond=%d divisor=%d inRem=%d: %d chunks sum to %d units + rem %d, want %d nanos of usage",
				seed, perSecond, divisor, inRem, len(chunks), total, outRem, elapsed*perSecond)
		}
	}
}
