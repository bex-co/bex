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
	"testing"
	"time"
)

func TestSandboxIntervalChunksSplitHoursAndCarryFractionalUnits(t *testing.T) {
	hour := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	chunks, remainder := sandboxIntervalChunks(553,
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
	chunks, remainder := sandboxIntervalChunks(553, start, start.Add(30*time.Minute), 0)
	if remainder != 0 || len(chunks) != 1 || chunks[0].quantity != 553*30*60 {
		t.Fatalf("chunks=%+v remainder=%d", chunks, remainder)
	}
}
