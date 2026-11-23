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

package renderlog

import (
	"os/exec"
	"strings"
	"testing"
)

// renderLines are messages from public captures of Render's log API, as the
// API returned them (package comment), with the form each is written in.
var renderLines = []struct {
	form    Form
	text    string
	capture string
}{
	{PlatformStep, "Deploying...", "\x1b[0;34m\x1b[1m==> \x1b[0m\x1b[1mDeploying...\x1b[0m"},
	{PlatformStep, "Setting WEB_CONCURRENCY=1 by default, based on available CPUs in the instance",
		"\x1b[0;34m\x1b[1m==> \x1b[0m\x1b[1mSetting WEB_CONCURRENCY=1 by default, based on available CPUs in the instance\x1b[0m"},
	{PlatformStep, "Common ways to troubleshoot your deploy: https://render.com/docs/troubleshooting-deploys",
		"\x1b[0;34m\x1b[1m==> \x1b[0m\x1b[1mCommon ways to troubleshoot your deploy: https://render.com/docs/troubleshooting-deploys\x1b[0m"},
	{PlatformSuccess, "Your service is live 🎉", "\x1b[0;32m\x1b[1m==> \x1b[0m\x1b[1mYour service is live 🎉\x1b[0m"},
	{PlatformSuccess, "", "\x1b[0;32m\x1b[1m==> \x1b[0m\x1b[1m\x1b[0m"},
	{PlatformSuccess, "///////////////////////////////////////////////////////////",
		"\x1b[0;32m\x1b[1m==> \x1b[0m\x1b[1m///////////////////////////////////////////////////////////\x1b[0m"},
	{PlatformSuccess, "Available at your primary URL https://tretta-server.onrender.com",
		"\x1b[0;32m\x1b[1m==> \x1b[0m\x1b[1mAvailable at your primary URL https://tretta-server.onrender.com\x1b[0m"},
	{PlatformSuccess, "Detected service running on port 8000",
		"\x1b[0;32m\x1b[1m==> \x1b[0m\x1b[1mDetected service running on port 8000\x1b[0m"},
	{PlatformSuccess, "Docs on specifying a port: https://render.com/docs/web-services#port-binding",
		"\x1b[0;32m\x1b[1m==> \x1b[0m\x1b[1mDocs on specifying a port: https://render.com/docs/web-services#port-binding\x1b[0m"},
	{PlatformFailure, "Exited with status 1", "\x1b[0;31m\x1b[1m==> Exited with status 1\x1b[0m"},
	{PlatformFailure, "Timed Out", "\x1b[0;31m\x1b[1m==> Timed Out\x1b[0m"},
	{PlatformFailure, "Port scan timeout reached, no open ports detected. Bind your service to at least one port. " +
		"If you don't need to receive traffic on any port, create a background worker instead.",
		"\x1b[0;31m\x1b[1m==> Port scan timeout reached, no open ports detected. Bind your service to at least one port. " +
			"If you don't need to receive traffic on any port, create a background worker instead.\x1b[0m"},
	{PlatformFailure, "Docs on specifying a port: https://render.com/docs/web-services#port-binding",
		"\x1b[0;31m\x1b[1m==> Docs on specifying a port: https://render.com/docs/web-services#port-binding\x1b[0m"},
	{BuildShell, "Downloading cache...", "\x1b[34m\x1b[1m==>\x1b(B\x1b[m \x1b[1mDownloading cache...\x1b(B\x1b[m"},
	{BuildShell, "Downloaded 160MB in 7s. Extraction took 4s.",
		"\x1b[34m\x1b[1m==>\x1b(B\x1b[m \x1b[1mDownloaded 160MB in 7s. Extraction took 4s.\x1b(B\x1b[m"},
	{BuildShell, "Running build command 'pnpm install'...",
		"\x1b[34m\x1b[1m==>\x1b(B\x1b[m \x1b[1mRunning build command 'pnpm install'...\x1b(B\x1b[m"},
	{BuildRuntime, "Using Node.js version 22.23.1 via environment variable NODE_VERSION",
		"\x1b[34;1m==>\x1b[0;22m \x1b[1mUsing Node.js version 22.23.1 via environment variable NODE_VERSION\x1b[22m"},
	{BuildRuntime, "Docs on specifying a Node.js version: https://render.com/docs/node-version",
		"\x1b[34;1m==>\x1b[0;22m \x1b[1mDocs on specifying a Node.js version: https://render.com/docs/node-version\x1b[22m"},
	{BuildRuntime, "Uploading build...", "\x1b[34;1m==>\x1b[0;22m \x1b[1mUploading build...\x1b[22m"},
	{BuildRuntime, "Uploaded in 9.9s. Compression took 2.4s", "\x1b[34;1m==>\x1b[0;22m \x1b[1mUploaded in 9.9s. Compression took 2.4s\x1b[22m"},
	{BuildRuntimeSuccess, "Build successful 🎉", "\x1b[32;1m==>\x1b[0;22m \x1b[1mBuild successful 🎉\x1b[22m"},
	{Instance, "Running 'pnpm --filter @tretta/server start'",
		"\x1b[32m\x1b[1m==>\x1b(B\x1b[m \x1b[1mRunning 'pnpm --filter @tretta/server start'\x1b(B\x1b[m"},
	{Plain, "Build queued", "==> Build queued"},
	{Form{}, "Build queued", "==> Build queued"},
}

func TestEachFormWritesRendersCapturedBytes(t *testing.T) {
	for _, l := range renderLines {
		if got := l.form.Line(l.text); got != l.capture {
			t.Errorf("Line(%q) = %q, want Render's %q", l.text, got, l.capture)
		}
	}
}

// A script's printf writes the same bytes from Shell's spelling, so the build
// pod's lines are Render's too.
func TestShellSpellingPrintsTheSameBytes(t *testing.T) {
	for _, l := range renderLines {
		open, closing := l.form.Shell()
		if strings.ContainsAny(open+closing, "\x1b'\"%") {
			t.Fatalf("Shell() = %q, %q: an ESC, a quote or a %% would break the script it goes into", open, closing)
		}
		out, err := exec.Command("sh", "-c", `printf "$1%s$2" "$3"`, "sh", open, closing, l.text).Output()
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != l.capture {
			t.Errorf("printf of Shell() for %q = %q, want %q", l.text, out, l.capture)
		}
	}
}
