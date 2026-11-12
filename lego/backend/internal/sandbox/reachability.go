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
	"errors"
	"fmt"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// CodeSandboxUnreachable is the 503 an exec, a file transfer or a session
// read answers while a sandbox that exists cannot be reached: its Pod is not
// ready (a NotReady or unreachable node), the gateway is restarting, or a
// stream broke mid-read. Retrying is the remedy. It never means the sandbox
// is gone (core.ErrNotFound) or that the runtime is not configured
// (core.ErrSandboxesUnavailable).
const CodeSandboxUnreachable = "SANDBOX_UNREACHABLE"

// CodeSandboxStateUnknown is a session read refused because the lifecycle
// server reports a state bex does not know: not a dead sandbox, and not one
// to exec into either, so it is retried like an unreachable one.
const CodeSandboxStateUnknown = "SANDBOX_STATE_UNKNOWN"

const unreachableRetryAfterSeconds = 15

// unreachable is a CodeSandboxUnreachable answer naming what was out of reach.
// detail is bex's own fixed wording or the gateway's, never a transport error.
func unreachable(detail string) error {
	return core.NewUnavailableError(CodeSandboxUnreachable,
		"the sandbox cannot be reached right now ("+detail+"); retry shortly",
		map[string]any{"retryAfterSeconds": unreachableRetryAfterSeconds})
}

var errStateUnknown = core.NewUnavailableError(CodeSandboxStateUnknown,
	"the sandbox runtime reports a state bex does not know; retry shortly",
	map[string]any{"retryAfterSeconds": unreachableRetryAfterSeconds})

// errExecSessionLimit is the gateway's per-caller concurrent exec cap. Every
// Completer read shares one system subject, so a burst of reads can meet it.
var errExecSessionLimit = fmt.Errorf("%w: too many concurrent exec sessions", core.ErrBadRequest)

// Transient reports whether a session read's err says only that the sandbox
// could not be read right now: it is unreachable (its node is not ready, the
// gateway is restarting, a stream broke), its runtime reports a state bex does
// not know, the lifecycle server is not answering, too many execs ran at once,
// or the read timed out. It is false for a gone sandbox (core.ErrNotFound) and
// for the platform being unable to read sandboxes at all.
func Transient(err error) bool {
	return hasCode(err, CodeSandboxUnreachable) || hasCode(err, CodeSandboxStateUnknown) ||
		hasCode(err, CodeSandboxRuntimeUnavailable) ||
		errors.Is(err, errExecSessionLimit) || errors.Is(err, context.DeadlineExceeded)
}

// StateUnknown reports a session read refused because the sandbox's state
// is one bex does not know.
func StateUnknown(err error) bool { return hasCode(err, CodeSandboxStateUnknown) }

// Termination is how a sandbox's container ended, as the kubelet recorded it
// and the gateway's target check read it: the terminated reason (OOMKilled,
// Error, …; the Pod's own reason, e.g. Evicted, when no container recorded
// one) and the exit code. The zero value records nothing.
type Termination struct {
	Reason   string
	ExitCode *int
}

// maxTerminationReasonLen bounds a reason taken from the kubelet; the
// vocabulary is short CamelCase words.
const maxTerminationReasonLen = 64

// Describe words t as an agent session's failure reason, "" when t records
// nothing.
func (t Termination) Describe() string {
	reason := cleanTerminationReason(t.Reason)
	if t.ExitCode == nil {
		switch reason {
		case "":
			return ""
		case "OOMKilled":
			return "the sandbox ran out of memory (exit 137)"
		case "Evicted":
			return "the sandbox was evicted from its node"
		case "DeletionByTaintManager":
			return "the sandbox was evicted from its node after the node stopped responding"
		}
		return fmt.Sprintf("the sandbox stopped (%s)", reason)
	}
	code := *t.ExitCode
	suffix := ""
	if reason != "" && reason != "Error" && reason != "Completed" {
		suffix = " (" + reason + ")"
	}
	switch {
	case reason == "OOMKilled":
		return fmt.Sprintf("the sandbox ran out of memory (exit %d)", code)
	case reason == "Evicted":
		return fmt.Sprintf("the sandbox was evicted from its node (exit %d)", code)
	case code > 128 && code < 128+65:
		return fmt.Sprintf("the sandbox's agent was killed by signal %d (exit %d)%s", code-128, code, suffix)
	case code != 0:
		return fmt.Sprintf("the sandbox's agent exited with code %d%s", code, suffix)
	}
	return "the sandbox's agent exited before finishing its turn (exit 0)" + suffix
}

// cleanTerminationReason keeps a kubelet reason only when it is one short
// alphanumeric word, so the platform never stores free text from the node.
func cleanTerminationReason(reason string) string {
	if len(reason) > maxTerminationReasonLen {
		return ""
	}
	for _, r := range reason {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return ""
		}
	}
	return reason
}

// TerminatedError is the NotFound an exec or a session read answers when the
// gateway saw the sandbox's target end, with how its container ended. It reads
// exactly as the bare "not found: sandbox terminated" it replaces, so every
// surface answers unchanged.
type TerminatedError struct{ Termination Termination }

func (e *TerminatedError) Error() string { return core.ErrNotFound.Error() + ": sandbox terminated" }

func (e *TerminatedError) Unwrap() error { return core.ErrNotFound }

// TerminationOf is the Termination err carries when the gateway saw the
// sandbox's target end. ok is false for any other error, including a NotFound
// that records nothing about the container.
func TerminationOf(err error) (Termination, bool) {
	var terminated *TerminatedError
	if errors.As(err, &terminated) {
		return terminated.Termination, true
	}
	return Termination{}, false
}
