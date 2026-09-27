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
	"context"
	"testing"
	"time"
)

func TestWithTenantAdvisoryLockSerializesSameWorkspace(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- st.WithTenantAdvisoryLock(ctx, "tea-api-key-quota", func() error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()
	<-firstEntered

	secondEntered := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- st.WithTenantAdvisoryLock(ctx, "tea-api-key-quota", func() error {
			close(secondEntered)
			return nil
		})
	}()
	select {
	case <-secondEntered:
		t.Fatal("second callback entered before first transaction released its lock")
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("first lock: %v", err)
	}
	select {
	case <-secondEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("second callback did not enter after first lock released")
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second lock: %v", err)
	}
}

// w8/m46: one app's deploy triggers serialize across bex-api replicas, and a
// different app's trigger never waits on them.
func TestWithAppAdvisoryLockSerializesOneApp(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	firstEntered, releaseFirst := make(chan struct{}), make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- st.WithAppAdvisoryLock(ctx, "srv-race", func() error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()
	<-firstEntered

	if err := st.WithAppAdvisoryLock(ctx, "srv-other", func() error { return nil }); err != nil {
		t.Fatalf("another app's trigger: %v", err)
	}
	secondEntered := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- st.WithAppAdvisoryLock(ctx, "srv-race", func() error {
			close(secondEntered)
			return nil
		})
	}()
	select {
	case <-secondEntered:
		t.Fatal("second trigger entered while the first held the app's lock")
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("first lock: %v", err)
	}
	select {
	case <-secondEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("second trigger did not enter after the first released")
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second lock: %v", err)
	}
}
