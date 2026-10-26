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

package controller

import (
	"regexp"
	"strings"
)

// shellPrefixed matches Render's documented multi-command form, a Docker
// Command that starts with a shell and -c: `/bin/sh -c python manage.py
// migrate && gunicorn …`. Render hands everything after -c to the shell as one
// argument, quotes or not.
var shellPrefixed = regexp.MustCompile(`(?s)^\s*((?:/(?:usr/)?(?:s?bin)/)?(?:ba|da|a)?sh)\s+-c\s+(.+?)\s*$`)

// shellMeta are the characters that make a command text need a shell: a
// list, pipe, redirection, expansion, glob, comment or subshell. Only quoting
// protects them.
const shellMeta = "&|;<>()$`*?[]{}~#!\n"

// splitShellWords splits command into argv by POSIX word rules (whitespace,
// single and double quotes, backslash) and reports ok=false when it needs a
// shell to mean what it says: an unquoted metacharacter, an expansion inside
// double quotes, an unquoted leading VAR=value assignment, or an unterminated quote.
func splitShellWords(command string) (words []string, ok bool) {
	var cur strings.Builder
	inWord := false
	// quoted[i] records whether word i contained any quoting: only an
	// entirely unquoted leading NAME=value is a shell assignment (w4/412).
	var quoted []bool
	curQuoted := false
	flush := func() {
		if inWord {
			words = append(words, cur.String())
			quoted = append(quoted, curQuoted)
			cur.Reset()
			inWord = false
			curQuoted = false
		}
	}
	for i := 0; i < len(command); i++ {
		c := command[i]
		switch {
		case c == ' ' || c == '\t':
			flush()
		case c == '\'':
			end := strings.IndexByte(command[i+1:], '\'')
			if end < 0 {
				return nil, false
			}
			cur.WriteString(command[i+1 : i+1+end])
			inWord = true
			curQuoted = true
			i += end + 1
		case c == '"':
			inWord = true
			curQuoted = true
			for i++; ; i++ {
				if i >= len(command) {
					return nil, false
				}
				d := command[i]
				if d == '"' {
					break
				}
				if d == '$' || d == '`' {
					return nil, false
				}
				if d == '\\' && i+1 < len(command) && strings.IndexByte("\\\"$`", command[i+1]) >= 0 {
					i++
					d = command[i]
				}
				cur.WriteByte(d)
			}
		case c == '\\':
			if i+1 >= len(command) || command[i+1] == '\n' {
				return nil, false
			}
			i++
			cur.WriteByte(command[i])
			inWord = true
		case strings.IndexByte(shellMeta, c) >= 0:
			return nil, false
		default:
			cur.WriteByte(c)
			inWord = true
		}
	}
	flush()
	if len(words) == 0 || (isAssignment(words[0]) && !quoted[0]) {
		return nil, false
	}
	return words, true
}

// unwrapQuotedScript strips the outer quoting when a `sh -c` script is
// exactly one single- or double-quoted token, with shell semantics: single
// quotes are literal; inside double quotes a backslash escapes only ", \, $
// and `, and $ / backtick pass through untouched — the shell receiving the
// script expands them itself (w4/412). splitShellWords cannot do this: it
// rightly refuses an expansion for direct exec, but here a shell runs the
// result either way.
func unwrapQuotedScript(script string) (string, bool) {
	if len(script) < 2 {
		return "", false
	}
	switch script[0] {
	case '\'':
		inner := script[1:]
		end := strings.IndexByte(inner, '\'')
		if end < 0 || end != len(inner)-1 {
			return "", false
		}
		return inner[:end], true
	case '"':
		if script[len(script)-1] != '"' {
			return "", false
		}
		inner := script[1 : len(script)-1]
		var out strings.Builder
		for i := 0; i < len(inner); i++ {
			c := inner[i]
			if c == '"' {
				return "", false
			}
			if c == '\\' && i+1 < len(inner) && strings.IndexByte("\\\"$`", inner[i+1]) >= 0 {
				i++
				c = inner[i]
			}
			out.WriteByte(c)
		}
		return out.String(), true
	}
	return "", false
}

// isAssignment reports a leading `NAME=value` word, which the shell treats as
// an environment assignment for the command that follows.
func isAssignment(word string) bool {
	name, _, found := strings.Cut(word, "=")
	if !found || name == "" {
		return false
	}
	for i, c := range name {
		if c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9' {
			continue
		}
		return false
	}
	return true
}

// shellCommand is the app container's Command for a Docker Command, as Render
// runs it (w8/164): a command written `sh -c <script>` is the shell invoked
// with everything after -c as its one script; a plain command (no shell syntax)
// is exec'd directly, so a shell-less image (scratch, distroless) can start;
// anything that needs a shell keeps running through /bin/sh -c, as it always
// has here.
func shellCommand(command string) []string {
	if m := shellPrefixed.FindStringSubmatch(command); m != nil {
		script := m[2]
		// A script that is one quoted word (`sh -c 'a && b'`) is that word.
		if inner, ok := unwrapQuotedScript(script); ok {
			script = inner
		} else if words, ok := splitShellWords(script); ok && len(words) == 1 {
			script = words[0]
		}
		return []string{m[1], "-c", script}
	}
	if words, ok := splitShellWords(command); ok {
		return words
	}
	return legacyShellCommand(command)
}

// legacyShellCommand is how every Docker Command used to launch; release
// records written before w8/164 hold it.
func legacyShellCommand(command string) []string {
	return []string{shellBinary, "-c", command}
}
