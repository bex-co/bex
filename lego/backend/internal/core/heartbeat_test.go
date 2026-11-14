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

package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A heartbeat keeps touching through a failed touch and stops the work once
// the touch says another owner settled it.
func TestHeartbeatStopsWhenTheWorkIsNoLongerLive(t *testing.T) {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	touches := 0
	done := make(chan struct{})
	go func() {
		defer close(done)
		Heartbeat(ctx, "test", time.Millisecond, stop, func(context.Context) (bool, error) {
			touches++
			if touches == 1 {
				return false, errors.New("store blip")
			}
			return touches < 3, nil
		})
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the heartbeat never stopped")
	}
	if touches != 3 || ctx.Err() == nil {
		t.Fatalf("touches = %d, stopped = %v; want the work stopped on the third touch", touches, ctx.Err() != nil)
	}
}
