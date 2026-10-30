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

package apps

import (
	"bytes"
	"context"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

// w3/m248: a delivery slower than slowDeliveryLog names each step's time and
// whether the request was cancelled; a fast one stays quiet.
func TestDeliveryTraceLogsOnlySlowDeliveries(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	fast := newDeliveryTrace()
	fast.mark("scopes")
	fast.logIfSlow(context.Background(), "fast-key")
	if buf.Len() != 0 {
		t.Fatalf("a fast delivery logged: %q", buf.String())
	}

	slow := newDeliveryTrace()
	slow.start = slow.start.Add(-time.Minute)
	slow.last = slow.start
	slow.mark("candidates")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	slow.logIfSlow(ctx, "slow-key")
	out := buf.String()
	for _, want := range []string{"slow delivery slow-key", "total=1m", "ctxErr=context canceled", "steps=[candidates=1m"} {
		if !strings.Contains(out, want) {
			t.Errorf("log %q lacks %q", out, want)
		}
	}
}
