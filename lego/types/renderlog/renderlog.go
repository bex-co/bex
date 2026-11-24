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

// Package renderlog holds the bytes Render's log writes its `==>` lines in
// (w32/m576). Render's log API, its CLI's `-o json` and its MCP server's
// list_logs return each of these lines with ANSI colour codes. Three Render
// writers use seven forms between them: the platform's deploy narration, the
// build's shell and runtime installer, and the native instance. Each form
// here is copied byte for byte from public captures of Render's log API
// (ivanseibel/tretta docs/work/reports/assets/T014-public/04b-render-logs.json
// at b0f216c985, kush252/Sentry-AI investigation_data.jsonl at 13d4e291cc,
// Ishan2036924/upmyrank-poc reports/smoke_v0207_1_r2_063029.json). bex's
// control plane, build pod and native-init write the same bytes from here, so
// they cannot drift apart.
//
// A line no capture shows with its codes stays Plain until w32/085's capture
// shows it (docs/ADR018/logs.md, Build logs).
package renderlog

import "strings"

// A Form is the bytes one kind of Render `==>` line is written in: Open, the
// line's text, then Close. Open carries the `==>` marker. The zero Form
// writes as Plain, so a line nobody gave a form keeps its marker and no codes.
type Form struct {
	Open, Close string
}

// Line is text written in f.
func (f Form) Line(text string) string {
	f = f.orPlain()
	return f.Open + text + f.Close
}

func (f Form) orPlain() Form {
	if f == (Form{}) {
		return Plain
	}
	return f
}

// Shell is f for a printf format or an awk string literal in a script bex
// generates: each ESC spelled as the octal escape \033, which sh's printf,
// busybox, mawk and gawk all read. The forms carry no quote, backslash or %
// of their own.
func (f Form) Shell() (open, close string) {
	f = f.orPlain()
	return strings.ReplaceAll(f.Open, "\x1b", `\033`), strings.ReplaceAll(f.Close, "\x1b", `\033`)
}

var (
	// Plain is a line with no codes: bex's own lines, and Render's lines no
	// capture shows with their codes yet.
	Plain = Form{Open: "==> "}

	// The platform's deploy narration. A step and a success colour the marker
	// and its space, reset, then bold the text: `==> Deploying...`, the
	// WEB_CONCURRENCY line and the troubleshooting line are blue (tretta,
	// Sentry-AI, upmyrank-poc); `==> Your service is live 🎉`, each line of
	// the primary URL block, `==> Detected service running on port N` and its
	// port docs line are green (tretta, Sentry-AI), the blank one
	// `\x1b[0;32m\x1b[1m==> \x1b[0m\x1b[1m\x1b[0m`. A failure is the whole
	// line in red and bold, no reset after the marker: `==> Exited with status
	// 1`, the port scan's timeout and its docs line (Sentry-AI), `==> Timed
	// Out` (upmyrank-poc).
	PlatformStep    = Form{Open: "\x1b[0;34m\x1b[1m==> \x1b[0m\x1b[1m", Close: "\x1b[0m"}
	PlatformSuccess = Form{Open: "\x1b[0;32m\x1b[1m==> \x1b[0m\x1b[1m", Close: "\x1b[0m"}
	PlatformFailure = Form{Open: "\x1b[0;31m\x1b[1m==> ", Close: "\x1b[0m"}

	// The build's shell lines, as `tput setaf 4; tput bold` … `tput sgr0`
	// write them, `ESC ( B` included: `==> Downloading cache...`, the cache's
	// `Downloaded … Extraction took …` line and `==> Running build command
	// '…'...` (tretta).
	BuildShell = Form{Open: "\x1b[34m\x1b[1m==>\x1b(B\x1b[m \x1b[1m", Close: "\x1b(B\x1b[m"}

	// The build's runtime lines: `==> Using Node.js version … via environment
	// variable NODE_VERSION` and its docs line, `==> Uploading build...` and
	// `==> Uploaded in …` in blue, `==> Build successful 🎉` in green
	// (tretta).
	BuildRuntime        = Form{Open: "\x1b[34;1m==>\x1b[0;22m \x1b[1m", Close: "\x1b[22m"}
	BuildRuntimeSuccess = Form{Open: "\x1b[32;1m==>\x1b[0;22m \x1b[1m", Close: "\x1b[22m"}

	// Instance is the native instance's `==> Running '<command>'`, green in
	// the shell form, at each start of the command (tretta).
	Instance = Form{Open: "\x1b[32m\x1b[1m==>\x1b(B\x1b[m \x1b[1m", Close: "\x1b(B\x1b[m"}
)
