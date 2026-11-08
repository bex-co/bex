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
	"fmt"
	"strings"
	"testing"
)

// TestHideCauseLogsWhatItHides (w5/123): the refusal reaches the client as
// it is, and the cause it hides reaches the server log, so an operator can
// still diagnose the failure. A canceled request is not a failure to diagnose.
func TestHideCauseLogsWhatItHides(t *testing.T) {
	logged := captureLog(t)
	err := HideCause(ErrBillingUnavailable, errors.New("dial tcp 10.0.3.7:5432: connection refused"))
	if err != ErrBillingUnavailable {
		t.Fatalf("HideCause = %v, want the refusal itself", err)
	}
	if !strings.Contains(logged(), "billing integration unavailable: dial tcp 10.0.3.7:5432: connection refused") {
		t.Fatalf("log = %q, want the refusal and its cause", logged())
	}

	before := logged()
	if err := HideCause(ErrSandboxesUnavailable, fmt.Errorf("exec stream: %w", context.Canceled)); err != ErrSandboxesUnavailable {
		t.Fatalf("HideCause = %v, want the refusal itself", err)
	}
	if logged() != before {
		t.Fatalf("a canceled request was logged: %q", strings.TrimPrefix(logged(), before))
	}
}

// w5/m286: a timed-out call answers REQUEST_TIMEOUT on MCP as on REST, never
// the dependency's wrapped text; a coded refusal that wraps a deadline keeps
// its own code.
func TestMCPErrorReportsATimeout(t *testing.T) {
	err := MCPError(fmt.Errorf("get pod: Get https://10.0.0.1/api: %w", context.DeadlineExceeded))
	if err == nil || err.Error() != RequestTimeoutCode+": "+RequestTimedOutMessage {
		t.Fatalf("MCPError = %v, want REQUEST_TIMEOUT", err)
	}
	coded := NewConflictError("BUSY", "busy", nil)
	if got := MCPError(fmt.Errorf("%w: %w", coded, context.DeadlineExceeded)); !strings.HasPrefix(got.Error(), "BUSY: ") {
		t.Fatalf("coded timeout = %v, want its own code", got)
	}
}
