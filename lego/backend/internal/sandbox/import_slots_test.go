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

package sandbox

import (
	"context"
	"testing"
	"time"
)

// w5/m383: a workspace holds one import slot per replica. Its second import
// waits behind its first without taking anything from other workspaces,
// another workspace's import proceeds at once, and the waiter runs as soon as
// the first releases. Nothing is kept once every import has left.
func TestWorkspaceImportSlotsAreFair(t *testing.T) {
	var slots workspaceSlots
	ctx := context.Background()
	releaseA, ok := slots.acquire(ctx, "tea-a")
	if !ok {
		t.Fatal("the first import could not take its workspace's slot")
	}

	waiting := make(chan func(), 1)
	go func() {
		release, ok := slots.acquire(ctx, "tea-a")
		if ok {
			waiting <- release
		}
	}()
	select {
	case <-waiting:
		t.Fatal("a workspace's second import took a slot while its first held one")
	case <-time.After(50 * time.Millisecond):
	}

	releaseB, ok := slots.acquire(ctx, "tea-b")
	if !ok {
		t.Fatal("another workspace's import waited behind tea-a")
	}
	releaseB()

	releaseA()
	select {
	case release := <-waiting:
		release()
	case <-time.After(2 * time.Second):
		t.Fatal("the queued import never ran after the first released")
	}

	gone, cancel := context.WithCancel(ctx)
	cancel()
	hold, ok := slots.acquire(ctx, "tea-c")
	if !ok {
		t.Fatal("tea-c could not take its free slot")
	}
	if _, ok := slots.acquire(gone, "tea-c"); ok {
		t.Fatal("a canceled wait took the slot")
	}
	hold()
	if len(slots.m) != 0 {
		t.Fatalf("slots kept %d workspaces after every import left", len(slots.m))
	}
}
