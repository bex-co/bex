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

package build

import "github.com/bex-co/bex/lego/types/renderlog"

// The build pod writes the lines Render's captures show in Render's forms
// (w32/m576): its shell lines (`==> Downloading cache...`, the cache's
// second line, `==> Running build command '…'...`) in renderlog.BuildShell,
// and its runtime lines (the Node.js version and docs lines, `==> Uploading
// build...`, `==> Uploaded in …`) in renderlog.BuildRuntime, with `==> Build
// successful 🎉` in renderlog.BuildRuntimeSuccess. Every other line it
// writes stays plain until w32/085's capture shows Render's.

// shellLine is a script command that prints text as one line in form f. text
// is the inside of a double-quoted shell word, so it expands variables and
// commands as the echo it replaces did.
func shellLine(f renderlog.Form, text string) string {
	open, closing := f.Shell()
	return `printf '` + open + `%s` + closing + `\n' "` + text + `"`
}

// awkString is text in form f as the inside of an awk string literal: a
// print of it writes the line. text must hold no quote or backslash, and a
// printf of it no % but its own conversions.
func awkString(f renderlog.Form, text string) string {
	open, closing := f.Shell()
	return open + text + closing
}
