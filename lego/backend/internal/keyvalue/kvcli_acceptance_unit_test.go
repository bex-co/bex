//go:build !windows

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

package keyvalue

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRenderKVCLIArgs(t *testing.T) {
	want := []string{"kv-cli", "--output", "interactive", "red-example", "--", "--raw", "SET", "key", "value"}
	got := renderKVCLIArgs("red-example", []string{"SET", "key", "value"})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("arguments = %#v, want %#v", got, want)
	}
}

func TestRunPTYCommand(t *testing.T) {
	t.Run("captures terminal output and exit", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		transcript, err := runPTYCommand(ctx, "/bin/sh", []string{"-c", `printf 'PONG\r\n'`}, os.Environ())
		if err != nil {
			t.Fatalf("runPTYCommand: %v", err)
		}
		if !terminalHasLine(transcript.bytes, "PONG") {
			t.Fatalf("missing exact output line in %q", cleanTerminalText(transcript.bytes))
		}
	})

	t.Run("preserves a nonzero exit", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err := runPTYCommand(ctx, "/bin/sh", []string{"-c", "exit 23"}, os.Environ())
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 23 {
			t.Fatalf("error = %v, want exit 23", err)
		}
	})

	t.Run("kills the bounded process group on timeout", func(t *testing.T) {
		pidFile := filepath.Join(t.TempDir(), "pid")
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_, err := runPTYCommand(ctx, "/bin/sh", []string{"-c", `echo $$ > "$1"; trap '' TERM; while :; do :; done`, "sh", pidFile}, os.Environ())
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %v, want deadline", err)
		}
		rawPID, readErr := os.ReadFile(pidFile)
		if readErr != nil {
			t.Fatalf("read child pid: %v", readErr)
		}
		pid, parseErr := strconv.Atoi(strings.TrimSpace(string(rawPID)))
		if parseErr != nil {
			t.Fatalf("parse child pid: %v", parseErr)
		}
		if killErr := syscall.Kill(pid, 0); !errors.Is(killErr, syscall.ESRCH) {
			t.Fatalf("timed-out child still exists: %v", killErr)
		}
	})
}

func TestValidateKVCLIOutcome(t *testing.T) {
	if err := validateKVCLIOutcome(ptyTranscript{bytes: []byte("\x1b[2JPONG\r\n")}, nil, "PONG", nil); err != nil {
		t.Fatalf("valid outcome: %v", err)
	}
	err := validateKVCLIOutcome(ptyTranscript{bytes: []byte("WRONG\r\n")}, nil, "PONG", nil)
	if err == nil || !strings.Contains(err.Error(), "result mismatch") {
		t.Fatalf("mismatch error = %v", err)
	}
	err = validateKVCLIOutcome(ptyTranscript{truncated: true, bytes: []byte("PONG\r\n")}, nil, "PONG", nil)
	if err == nil || !strings.Contains(err.Error(), "bounded") {
		t.Fatalf("truncation error = %v", err)
	}
}

func TestSanitizeKVCLIDiagnostic(t *testing.T) {
	const token = "secret-bearer-value"
	raw := []byte("\x1b[2JAuthorization: Bearer abc.def\r\n" +
		"redis-cli -u rediss://default:password@red.example.test:6379\r\n" +
		"RENDER_API_KEY=" + token + "\r\n")
	got := sanitizeKVCLIDiagnostic(raw, []string{token})
	for _, forbidden := range []string{"abc.def", "default:password", token, "\x1b"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("diagnostic leaked %q: %q", forbidden, got)
		}
	}
	for _, want := range []string{"Bearer ***", "rediss://***@red.example.test:6379", "RENDER_API_KEY=***"} {
		if !strings.Contains(got, want) {
			t.Fatalf("diagnostic %q missing %q", got, want)
		}
	}
}
