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

package main_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// rendersForms are a `==>` line in each of the seven forms Render's log API
// returns (w32/m576), byte for byte from public captures of it
// (ivanseibel/tretta's `render logs -o json`, kush252/Sentry-AI's,
// Ishan2036924/upmyrank-poc's). bex-api returns its own `==>` lines in the
// same bytes; the CLI must show them as `render logs` does.
var rendersForms = []string{
	"\x1b[0;34m\x1b[1m==> \x1b[0m\x1b[1mDeploying...\x1b[0m",
	"\x1b[0;32m\x1b[1m==> \x1b[0m\x1b[1mYour service is live 🎉\x1b[0m",
	"\x1b[0;31m\x1b[1m==> Exited with status 1\x1b[0m",
	"\x1b[34m\x1b[1m==>\x1b(B\x1b[m \x1b[1mDownloading cache...\x1b(B\x1b[m",
	"\x1b[34;1m==>\x1b[0;22m \x1b[1mUploading build...\x1b[22m",
	"\x1b[32;1m==>\x1b[0;22m \x1b[1mBuild successful 🎉\x1b[22m",
	"\x1b[32m\x1b[1m==>\x1b(B\x1b[m \x1b[1mRunning 'node main.js'\x1b(B\x1b[m",
}

// servesForms is a bex-api whose GET /v1/logs returns rendersForms in order.
func servesForms(t *testing.T) string {
	t.Helper()
	type label struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	type entry struct {
		ID        string  `json:"id"`
		Timestamp string  `json:"timestamp"`
		Message   string  `json:"message"`
		Labels    []label `json:"labels"`
	}
	logs := make([]entry, 0, len(rendersForms))
	for i, message := range rendersForms {
		logs = append(logs, entry{
			ID: "log-" + string(rune('a'+i)), Timestamp: "2026-11-22T10:00:0" + string(rune('0'+i)) + "Z", Message: message,
			Labels: []label{{"resource", "srv-abcdefghij0123456789"}, {"type", "build"}},
		})
	}
	body, err := json.Marshal(map[string]any{"hasMore": false, "logs": logs})
	if err != nil {
		t.Fatal(err)
	}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/logs":
			_, _ = w.Write(body)
		case "/v1/owners":
			_, _ = w.Write([]byte(`[{"owner":{"id":"tea-bex","name":"Bex","email":"bex@example.test","type":"team"}}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)
	return api.URL
}

// bexLogs runs `bex logs` against apiURL with stdout to out.
func bexLogs(t *testing.T, apiURL string, out io.Writer, output string) {
	t.Helper()
	command := bexAgainst(t, apiURL, "logs", "-r", "srv-abcdefghij0123456789", "-o", output)
	var stderr bytes.Buffer
	command.Stdout, command.Stderr = out, &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("bex logs -o %s: %v\nstderr:\n%s", output, err, stderr.String())
	}
}

// TestLogsJSONAndYAMLCarryRendersCodes is `render logs -o json` and `-o yaml`:
// each message as the API returned it, the codes escaped, as tretta's capture
// of `render logs -o json` shows them (`\u001b[0;34m\u001b[1m==\u003e `).
func TestLogsJSONAndYAMLCarryRendersCodes(t *testing.T) {
	api := servesForms(t)
	var out bytes.Buffer
	bexLogs(t, api, &out, "json")
	if bytes.IndexByte(out.Bytes(), 0x1b) >= 0 {
		t.Fatalf("-o json wrote a raw ESC:\n%q", out.String())
	}
	if want := `"message": "\u001b[0;34m\u001b[1m==\u003e \u001b[0m\u001b[1mDeploying...\u001b[0m"`; !strings.Contains(out.String(), want) {
		t.Errorf("-o json does not escape the codes as render logs does (%s):\n%s", want, out.String())
	}
	decoder := json.NewDecoder(&out)
	var got []string
	for {
		var log struct{ Message string }
		if err := decoder.Decode(&log); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		got = append(got, log.Message)
	}
	if strings.Join(got, "\n") != strings.Join(rendersForms, "\n") {
		t.Errorf("-o json messages:\n%q\nwant Render's bytes:\n%q", got, rendersForms)
	}

	out.Reset()
	bexLogs(t, api, &out, "yaml")
	if bytes.IndexByte(out.Bytes(), 0x1b) >= 0 {
		t.Fatalf("-o yaml wrote a raw ESC:\n%q", out.String())
	}
	// The pinned CLI writes one YAML mapping per record with no document
	// separator, as `render logs -o yaml` does, so each record opens at `id:`.
	got = nil
	for _, record := range strings.Split("\n"+out.String(), "\nid: ")[1:] {
		var log struct{ Message string }
		if err := yaml.Unmarshal([]byte("id: "+record), &log); err != nil {
			t.Fatalf("%v in record:\n%s", err, record)
		}
		got = append(got, log.Message)
	}
	if strings.Join(got, "\n") != strings.Join(rendersForms, "\n") {
		t.Errorf("-o yaml messages:\n%q\nwant Render's bytes:\n%q", got, rendersForms)
	}
}

// TestLogsTextShowsRendersColours is `render logs` in text: to a terminal or a
// pipe it keeps the colours (SGR), drops every other escape, `ESC ( B`
// included, and resets after each styled message (SanitizeTerminal); to a
// regular file it writes the message untouched.
func TestLogsTextShowsRendersColours(t *testing.T) {
	api := servesForms(t)
	var piped bytes.Buffer
	bexLogs(t, api, &piped, "text")
	lines := strings.Split(strings.TrimSuffix(piped.String(), "\n"), "\n")
	if len(lines) != len(rendersForms) {
		t.Fatalf("piped text has %d lines, want %d:\n%q", len(lines), len(rendersForms), piped.String())
	}
	for i, form := range rendersForms {
		want := strings.ReplaceAll(form, "\x1b(B", "") + "\x1b[0m"
		if _, message, _ := strings.Cut(lines[i], "  "); message != want {
			t.Errorf("piped line %d = %q, want %q", i, message, want)
		}
	}

	file, err := os.Create(filepath.Join(t.TempDir(), "logs.txt"))
	if err != nil {
		t.Fatal(err)
	}
	bexLogs(t, api, file, "text")
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	lines = strings.Split(strings.TrimSuffix(string(written), "\n"), "\n")
	if len(lines) != len(rendersForms) {
		t.Fatalf("redirected text has %d lines, want %d:\n%q", len(lines), len(rendersForms), written)
	}
	for i, form := range rendersForms {
		if _, message, _ := strings.Cut(lines[i], "  "); message != form {
			t.Errorf("redirected line %d = %q, want the message untouched %q", i, message, form)
		}
	}
}
