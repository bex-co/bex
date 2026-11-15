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

package objectstorage

import (
	"errors"
	"sync"
	"testing"
)

func TestStreamLimiterCapsPerWorkspaceAndPerReplica(t *testing.T) {
	l := NewStreamLimiter(2, 3)
	r1, err := l.Acquire("tea-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Acquire("tea-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Acquire("tea-a"); !errors.Is(err, ErrTooManyStreams) {
		t.Fatalf("a third stream for one workspace = %v, want ErrTooManyStreams", err)
	}
	// Another workspace still has room, until the replica is full.
	if _, err := l.Acquire("tea-b"); err != nil {
		t.Fatalf("a different workspace was refused: %v", err)
	}
	if _, err := l.Acquire("tea-c"); !errors.Is(err, ErrTooManyStreams) {
		t.Fatalf("a fourth stream on a replica capped at 3 = %v, want ErrTooManyStreams", err)
	}
	r1()
	if _, err := l.Acquire("tea-c"); err != nil {
		t.Fatalf("a released slot was not reusable: %v", err)
	}
}

func TestStreamLimiterReleaseIsIdempotent(t *testing.T) {
	l := NewStreamLimiter(1, 10)
	release, err := l.Acquire("tea-a")
	if err != nil {
		t.Fatal(err)
	}
	release()
	release() // a double release must not hand out a slot that was never taken
	if _, err := l.Acquire("tea-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Acquire("tea-a"); !errors.Is(err, ErrTooManyStreams) {
		t.Fatalf("a double release freed a second slot: %v", err)
	}
}

func TestStreamLimiterForgetsIdleWorkspaces(t *testing.T) {
	l := NewStreamLimiter(2, 10)
	for i := range 50 {
		release, err := l.Acquire("tea-" + string(rune('a'+i%26)) + string(rune('a'+i/26)))
		if err != nil {
			t.Fatal(err)
		}
		release()
	}
	if len(l.active) != 0 || l.total != 0 {
		t.Fatalf("limiter kept state for idle workspaces: %d workspaces, %d in flight", len(l.active), l.total)
	}
}

func TestStreamLimiterUnderConcurrency(t *testing.T) {
	l := NewStreamLimiter(5, 100)
	var wg sync.WaitGroup
	var mu sync.Mutex
	granted := 0
	for range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := l.Acquire("tea-a"); err == nil {
				mu.Lock()
				granted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if granted != 5 {
		t.Fatalf("40 racing acquires granted %d, want exactly the cap of 5", granted)
	}
}

func TestServiceAllowMintUsesTheWorkspaceBucket(t *testing.T) {
	s := NewService(nil, nil)
	allowed := 0
	for range MintBurst + 10 {
		if s.AllowMint("tea-a") == nil {
			allowed++
		}
	}
	if allowed != MintBurst {
		t.Fatalf("a burst of %d mints allowed %d, want %d", MintBurst+10, allowed, MintBurst)
	}
	if err := s.AllowMint("tea-a"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("over the rate = %v, want ErrRateLimited", err)
	}
	if err := s.AllowMint("tea-b"); err != nil {
		t.Fatalf("another workspace was limited: %v", err)
	}
}
