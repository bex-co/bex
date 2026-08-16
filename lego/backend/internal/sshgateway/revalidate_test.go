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

package sshgateway

import (
	"context"
	"errors"
	"testing"
	"time"
)

// codex round-9 #6: an established stream's authorization is a lease, not an
// admission-time fact. WithRevalidation must cancel the stream context as soon
// as the periodic check fails — the revocation window closes to the interval,
// not to the session cap.
func TestWithRevalidationCancelsOnFailedCheck(t *testing.T) {
	revoked := make(chan struct{})
	check := func(context.Context) error {
		select {
		case <-revoked:
			return errors.New("membership gone")
		default:
			return nil
		}
	}
	ctx, cancel := WithRevalidation(context.Background(), 5*time.Millisecond, check)
	defer cancel()

	select {
	case <-ctx.Done():
		t.Fatal("cancelled before any check failed")
	case <-time.After(20 * time.Millisecond):
	}
	close(revoked)
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("a failed check did not cancel the stream context")
	}
}

// A passing check never cancels the stream; only the caller's Cancel does.
func TestWithRevalidationKeepsHealthyStreamsAlive(t *testing.T) {
	ctx, cancel := WithRevalidation(context.Background(), 5*time.Millisecond, func(context.Context) error { return nil })
	defer cancel()
	select {
	case <-ctx.Done():
		t.Fatal("healthy stream was cancelled")
	case <-time.After(30 * time.Millisecond):
	}
	cancel()
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Cancel did not cancel the stream context")
	}
}

// interval <= 0 disables the watchdog entirely (the pre-round-9
// admission-only behavior) — the context is then a plain WithCancel.
func TestWithRevalidationDisabledDegradesToPlainCancel(t *testing.T) {
	calls := 0
	ctx, cancel := WithRevalidation(context.Background(), 0, func(context.Context) error {
		calls++
		return errors.New("deny")
	})
	defer cancel()
	select {
	case <-ctx.Done():
		t.Fatal("disabled watchdog still cancelled the context")
	case <-time.After(20 * time.Millisecond):
	}
	if calls != 0 {
		t.Fatalf("disabled watchdog ran the check %d times", calls)
	}
}
