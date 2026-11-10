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
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakeMeterMinter struct {
	calls atomic.Int32
	// failFirst is how many leading calls fail.
	failFirst int32
}

func (f *fakeMeterMinter) EnsureSandboxMeterCreds(context.Context) error {
	if n := f.calls.Add(1); n <= f.failFirst {
		return errors.New("zot-htpasswd not there yet")
	}
	return nil
}

// A failed ensure is retried on the short interval until it works, then the
// credentials are re-ensured on the long one; the runnable stops with its
// context.
func TestSandboxMeterCredsRunnableRetriesThenResyncs(t *testing.T) {
	minter := &fakeMeterMinter{failFirst: 2}
	r := &SandboxMeterCredsRunnable{Registry: minter, Retry: time.Millisecond, Resync: 5 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Start(ctx) }()

	deadline := time.After(2 * time.Second)
	for minter.calls.Load() < 5 { // two failures, the success, and two resyncs
		select {
		case <-deadline:
			t.Fatalf("only %d ensures ran", minter.calls.Load())
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Start returned %v on cancel", err)
		}
	case <-time.After(time.Second):
		t.Fatal("the runnable did not stop with its context")
	}
}
