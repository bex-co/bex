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

package logs

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// applevel.go is the live tail's copy of the `level` the log shipper stamps on
// an App line (deploy/gitops/base/log-shipper.yaml, `loki.process "app_logs"`),
// so `subscribe?level=` keeps the lines a history query with the same level
// returns (w8/m147). The shipper parses only the line's own structured severity
// — a JSON object's top-level `level`/`severity`, else a logfmt `level=` or
// `severity=` — and never guesses from words in the message.
// TestAppLineLevelMatchesShipper reads the shipper's config and fails when its
// field names, logfmt gate or bucket vocabulary drift from this file.

// The shipper's stage.match selector for the logfmt stage: a line that does not
// open a JSON object and carries a top-level level=/severity= token.
var (
	shipperJSONLine   = regexp.MustCompile(`^\s*[{]`)
	shipperLogfmtLine = regexp.MustCompile(`(^|\s)(level|severity)=`)
)

// shipperLevelBuckets is the shipper's stage.template vocabulary: a lowercased
// severity (pino's and bunyan's numbers included) and the bucket it stores.
// Anything else is `unknown`.
var shipperLevelBuckets = map[string]string{
	"error": "error", "err": "error", "fatal": "error", "panic": "error", "critical": "error",
	"crit": "error", "alert": "error", "emergency": "error", "50": "error", "60": "error",
	"warn": "warning", "warning": "warning", "40": "warning",
	"info": "info", "notice": "info", "30": "info",
	"debug": "debug", "trace": "debug", "10": "debug", "20": "debug",
}

const unknownLevel = "unknown"

// appLineLevel is the level the shipper stores for an App line's message, or ""
// when it stores none: a line whose `level` is present but empty, with no
// `severity`, fails the shipper's template and keeps that empty value, which
// stage.labels then omits.
func appLineLevel(msg string) string {
	var level, severity string
	var hasLevel, hasSeverity bool
	extract := func(key, value string) {
		switch key {
		case "level":
			level, hasLevel = value, true
		case "severity":
			severity, hasSeverity = value, true
		}
	}
	if shipperJSONLine.MatchString(msg) {
		jsonSeverity(msg, extract)
	} else if (strings.Contains(msg, "level=") || strings.Contains(msg, "severity=")) && shipperLogfmtLine.MatchString(msg) {
		scanLogfmt(msg, extract)
	}
	// The template: `$l := ToLower (or .Value .severity)` with `.Value` the
	// level field. With neither field the template fails and the static
	// `unknown` stands; with only an empty level it fails and "" stands.
	if level == "" {
		switch {
		case hasSeverity:
			level = severity
		case hasLevel:
			return ""
		default:
			return unknownLevel
		}
	}
	if bucket, known := shipperLevelBuckets[strings.ToLower(level)]; known {
		return bucket
	}
	return unknownLevel
}

// jsonSeverity is the shipper's stage.json with expressions `level` and
// `severity`: on a line that is one JSON object, it yields each of those
// top-level fields that is present and not null, as Alloy stringifies it — a
// number in its shortest decimal form (pino's 30 is "30"), a bool as
// true/false, an object or array as its JSON text.
func jsonSeverity(msg string, yield func(key, value string)) {
	var object map[string]json.RawMessage
	if json.Unmarshal([]byte(msg), &object) != nil {
		return
	}
	for _, key := range []string{"level", "severity"} {
		raw, ok := object[key]
		if !ok {
			continue
		}
		var value any
		if json.Unmarshal(raw, &value) != nil || value == nil {
			continue
		}
		switch v := value.(type) {
		case string:
			yield(key, v)
		case float64:
			yield(key, strconv.FormatFloat(v, 'f', -1, 64))
		case bool:
			yield(key, strconv.FormatBool(v))
		default:
			yield(key, string(raw))
		}
	}
}

// scanLogfmt yields each key and value go-logfmt's Decoder reads from one line,
// in order, and stops at its first syntax error — Alloy's stage.logfmt keeps
// what it extracted before the error. A key without `=` has an empty value.
func scanLogfmt(line string, yield func(key, value string)) {
	pos := 0
	for {
		for pos < len(line) && line[pos] <= ' ' {
			pos++
		}
		if pos >= len(line) {
			return
		}
		start, multibyte := pos, false
		for pos < len(line) && line[pos] > ' ' && line[pos] != '=' {
			if line[pos] == '"' {
				return
			}
			if line[pos] >= utf8.RuneSelf {
				multibyte = true
			}
			pos++
		}
		key := line[start:pos]
		if key == "" || (multibyte && strings.ContainsRune(key, utf8.RuneError)) {
			return // a bare `=`, or a key that is not valid UTF-8
		}
		if pos >= len(line) || line[pos] != '=' {
			yield(key, "")
			continue
		}
		pos++ // the '='
		switch {
		case pos >= len(line) || line[pos] <= ' ':
			yield(key, "")
		case line[pos] == '"':
			value, end, ok := logfmtQuoted(line, pos)
			if !ok {
				return
			}
			pos = end
			yield(key, value)
		default:
			start = pos
			for pos < len(line) && line[pos] > ' ' {
				if line[pos] == '=' || line[pos] == '"' {
					return
				}
				pos++
			}
			yield(key, line[start:pos])
		}
	}
}

// logfmtQuoted reads the quoted value opening at line[open] and returns it
// unquoted with the index just past its closing quote. An unterminated value,
// or an escape go-logfmt (encoding/json's unquote) rejects, is a syntax error.
func logfmtQuoted(line string, open int) (string, int, bool) {
	escaped, esc := false, false
	for i := open + 1; i < len(line); i++ {
		switch c := line[i]; {
		case esc:
			esc = false
		case c == '\\':
			escaped, esc = true, true
		case c == '"':
			if !escaped {
				return line[open+1 : i], i + 1, true
			}
			var value string
			if json.Unmarshal([]byte(line[open:i+1]), &value) != nil {
				return "", 0, false
			}
			return value, i + 1, true
		}
	}
	return "", 0, false
}
