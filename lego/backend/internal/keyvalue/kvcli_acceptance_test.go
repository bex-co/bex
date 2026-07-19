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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode"

	"github.com/creack/pty"
)

const maxKVCLITranscriptBytes = 1 << 20

var (
	ansiCSI  = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
	ansiOSC  = regexp.MustCompile("\\x1b\\][^\\x07]*(\\x07|\\x1b\\\\)")
	redisURI = regexp.MustCompile(`(?i)(rediss?://)[^@[:space:]]+@`)
	bearer   = regexp.MustCompile(`(?i)(bearer[[:space:]]+)[a-z0-9._~+/=-]+`)
)

type cappedTranscript struct {
	buf       bytes.Buffer
	truncated bool
}

func (w *cappedTranscript) Write(p []byte) (int, error) {
	written := len(p)
	remaining := maxKVCLITranscriptBytes - w.buf.Len()
	if remaining > 0 {
		if len(p) > remaining {
			_, _ = w.buf.Write(p[:remaining])
			w.truncated = true
		} else {
			_, _ = w.buf.Write(p)
		}
	} else {
		w.truncated = true
	}
	return written, nil
}

type ptyTranscript struct {
	bytes     []byte
	truncated bool
}

// runPTYCommand gives an otherwise headless child a real terminal on all three
// streams. The PTY package starts a fresh session/process group; a deadline can
// therefore kill the exact spawned tree without touching the caller.
func runPTYCommand(ctx context.Context, binary string, args, env []string) (ptyTranscript, error) {
	cmd := exec.Command(binary, args...)
	cmd.Env = env
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 120})
	if err != nil {
		return ptyTranscript{}, fmt.Errorf("start PTY command: %w", err)
	}

	readDone := make(chan ptyTranscript, 1)
	go func() {
		var transcript cappedTranscript
		_, _ = io.Copy(&transcript, ptmx) // PTYs commonly finish with EIO, not EOF.
		readDone <- ptyTranscript{bytes: transcript.buf.Bytes(), truncated: transcript.truncated}
	}()
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()

	select {
	case waitErr := <-waitDone:
		return finishPTYRead(ptmx, readDone), waitErr
	case <-ctx.Done():
		// StartWithSize uses Setsid, making the child pid its process-group id.
		// Killing the group also reaps redis-cli if the Render CLI is waiting on it.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Process.Kill()
		<-waitDone
		transcript := finishPTYRead(ptmx, readDone)
		return transcript, fmt.Errorf("PTY command deadline: %w", ctx.Err())
	}
}

func finishPTYRead(ptmx *os.File, readDone <-chan ptyTranscript) ptyTranscript {
	select {
	case transcript := <-readDone:
		_ = ptmx.Close()
		return transcript
	case <-time.After(time.Second):
		_ = ptmx.Close()
		return <-readDone
	}
}

func renderKVCLIArgs(target, addressFamily string, redisArgs []string) []string {
	args := []string{"kv-cli", "--output", "interactive", target, "--", addressFamily, "--raw"}
	return append(args, redisArgs...)
}

func runOfficialKVCLICommand(
	ctx context.Context,
	binary, target, addressFamily string,
	redisArgs []string,
	wantLine string,
	env, secrets []string,
) error {
	transcript, runErr := runPTYCommand(ctx, binary, renderKVCLIArgs(target, addressFamily, redisArgs), env)
	return validateKVCLIOutcome(transcript, runErr, wantLine, secrets)
}

func validateKVCLIOutcome(transcript ptyTranscript, runErr error, wantLine string, secrets []string) error {
	diagnostic := sanitizeKVCLIDiagnostic(transcript.bytes, secrets)
	if runErr != nil {
		return fmt.Errorf("official kv-cli process failed: %w (%s)", runErr, diagnostic)
	}
	if !terminalHasLine(transcript.bytes, wantLine) {
		return fmt.Errorf("official kv-cli result mismatch: expected one exact result line (%s)", diagnostic)
	}
	if transcript.truncated {
		return errors.New("official kv-cli produced more than the bounded terminal transcript")
	}
	return nil
}

func terminalHasLine(raw []byte, want string) bool {
	clean := cleanTerminalText(raw)
	for _, line := range strings.Split(strings.ReplaceAll(clean, "\r", "\n"), "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

func cleanTerminalText(raw []byte) string {
	clean := ansiOSC.ReplaceAll(raw, nil)
	clean = ansiCSI.ReplaceAll(clean, nil)
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || (!unicode.IsControl(r) && r != unicode.ReplacementChar) {
			return r
		}
		return -1
	}, string(clean))
}

func sanitizeKVCLIDiagnostic(raw []byte, secrets []string) string {
	clean := cleanTerminalText(raw)
	clean = redisURI.ReplaceAllString(clean, `${1}***@`)
	clean = bearer.ReplaceAllString(clean, `${1}***`)
	for _, secret := range secrets {
		if secret != "" {
			clean = strings.ReplaceAll(clean, secret, "***")
		}
	}

	lines := make([]string, 0, 8)
	for _, line := range strings.Split(strings.ReplaceAll(clean, "\r", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) > 8 {
		lines = lines[len(lines)-8:]
	}
	diagnostic := strings.Join(lines, " | ")
	if diagnostic == "" {
		return "no safe terminal output"
	}
	if len(diagnostic) > 800 {
		diagnostic = diagnostic[:800] + "…"
	}
	return diagnostic
}

// TestOfficialCLIKeyValueAcceptance is the opt-in public-edge probe driven by
// scripts/keyvalue-cli-verify.sh. Raw PTY bytes stay in memory; only stable PASS
// labels are emitted when -v is used, and failures pass through the sanitizer.
func TestOfficialCLIKeyValueAcceptance(t *testing.T) {
	required := map[string]string{
		"BEX_TEST_KV_CLI_BIN":    os.Getenv("BEX_TEST_KV_CLI_BIN"),
		"BEX_TEST_KV_CLI_ID":     os.Getenv("BEX_TEST_KV_CLI_ID"),
		"BEX_TEST_KV_CLI_NAME":   os.Getenv("BEX_TEST_KV_CLI_NAME"),
		"BEX_TEST_KV_CLI_KEY":    os.Getenv("BEX_TEST_KV_CLI_KEY"),
		"BEX_TEST_KV_CLI_VALUE":  os.Getenv("BEX_TEST_KV_CLI_VALUE"),
		"BEX_TEST_KV_CLI_FAMILY": os.Getenv("BEX_TEST_KV_CLI_FAMILY"),
		"RENDER_HOST":            os.Getenv("RENDER_HOST"),
		"RENDER_API_KEY":         os.Getenv("RENDER_API_KEY"),
		"RENDER_WORKSPACE":       os.Getenv("RENDER_WORKSPACE"),
		"RENDER_CLI_CONFIG_PATH": os.Getenv("RENDER_CLI_CONFIG_PATH"),
	}
	for _, value := range required {
		if value == "" {
			t.Skip("official kv-cli acceptance environment is not configured")
		}
	}

	timeout := 45 * time.Second
	if raw := os.Getenv("BEX_TEST_KV_CLI_TIMEOUT"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 {
			t.Fatalf("invalid BEX_TEST_KV_CLI_TIMEOUT")
		}
		timeout = parsed
	}
	binary := required["BEX_TEST_KV_CLI_BIN"]
	id := required["BEX_TEST_KV_CLI_ID"]
	name := required["BEX_TEST_KV_CLI_NAME"]
	key := required["BEX_TEST_KV_CLI_KEY"]
	value := required["BEX_TEST_KV_CLI_VALUE"]
	addressFamily := required["BEX_TEST_KV_CLI_FAMILY"]
	if addressFamily != "-4" && addressFamily != "-6" {
		t.Fatalf("BEX_TEST_KV_CLI_FAMILY must be -4 or -6")
	}
	secrets := []string{
		required["RENDER_API_KEY"],
		required["RENDER_WORKSPACE"],
		id,
		name,
		key,
		value,
	}

	checks := []struct {
		label     string
		target    string
		redisArgs []string
		want      string
	}{
		{label: "opaque id PING", target: id, redisArgs: []string{"PING"}, want: "PONG"},
		{label: "opaque id SET", target: id, redisArgs: []string{"SET", key, value}, want: "OK"},
		{label: "display name GET", target: name, redisArgs: []string{"GET", key}, want: value},
		{label: "display name DEL", target: name, redisArgs: []string{"DEL", key}, want: strconv.Itoa(1)},
	}
	for _, check := range checks {
		t.Run(check.label, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			if err := runOfficialKVCLICommand(ctx, binary, check.target, addressFamily, check.redisArgs, check.want, os.Environ(), secrets); err != nil {
				t.Fatal(err)
			}
			t.Log("PASS official kv-cli " + check.label)
		})
	}
}
