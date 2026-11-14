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

package apps

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var yamlUnknownAnchor = regexp.MustCompile(`unknown anchor '([^']+)' referenced`)

// yamlParserProblems are the reasons yaml.v3 v3.0.1's parser (parserc.go)
// reports. Its message carries the 0-based line of the error's mark, where a
// scanner reason carries the same mark 1-based ((*parser).fail in decode.go).
var yamlParserProblems = map[string]bool{
	"did not find expected <stream-start>":   true,
	"did not find expected <document start>": true,
	"found undefined tag handle":             true,
	"did not find expected node content":     true,
	"did not find expected '-' indicator":    true,
	"did not find expected key":              true,
	"did not find expected ',' or ']'":       true,
	"did not find expected ',' or '}'":       true,
	"found duplicate %YAML directive":        true,
	"found incompatible YAML document":       true,
	"found duplicate %TAG directive":         true,
}

// yamlQuotedScalarProblems are the scanner reasons raised "while scanning a
// quoted scalar"; their mark is the scalar's opening quote.
var yamlQuotedScalarProblems = map[string]bool{
	"found unexpected end of stream":              true,
	"found unexpected document indicator":         true,
	"found unknown escape character":              true,
	"did not find expected hexdecimal number":     true,
	"found invalid Unicode character escape code": true,
}

// yamlSyntaxLocation translates a yaml.v3 syntax error into Render's position
// contract: the 1-indexed line and column of the construct the message names
// (w4/632). yaml.v3 puts only a line in its message, 0-based for a parser
// reason, and that line is the error's context mark (the construct being
// parsed) unless the construct starts on line 1, when it is the offending
// token's. The column is recovered from the source:
//
//   - "did not find expected ',' or ']'" (or '}'): the unclosed `[` (or `{`);
//   - a quoted-scalar reason: the scalar's opening quote;
//   - a tab that breaks indentation: that tab;
//   - anything else: the first token on the line. For a block collection or
//     mapping that an under-indented line breaks, that is the collection's
//     start (PyYAML's context mark), not the bad line.
//
// An error yaml.v3 reports with no line and no construct to find (a reader
// error, excessive aliasing) has no position: 0, 0.
func yamlSyntaxLocation(err error, manifest string) (line, column int) {
	reason := yamlSyntaxReason(err)
	match := yamlSyntaxLine.FindStringSubmatch(err.Error())
	reported := 0
	if match != nil {
		reported, _ = strconv.Atoi(match[1])
	}
	var opens string
	switch {
	case reason == "did not find expected ',' or ']'":
		opens = "["
	case reason == "did not find expected ',' or '}'":
		opens = "{"
	case yamlQuotedScalarProblems[reason]:
		opens = `"'`
	}
	switch {
	case yamlParserProblems[reason]:
		line = reported + 1
	case match != nil:
		line = reported
	case opens != "":
		line = 1 // yaml.v3 omits a mark on its first line
	default:
		// yaml.v3 reports an alias to an undefined anchor without a position,
		// so point at the first `*name` token in the source (w6/092).
		if anchor := yamlUnknownAnchor.FindStringSubmatch(err.Error()); anchor != nil {
			alias := regexp.MustCompile(`(?:^|[\s\[{,:-])(\*` + regexp.QuoteMeta(anchor[1]) + `)(?:$|[\s\]},])`)
			if at := alias.FindStringSubmatchIndex(manifest); at != nil {
				before := manifest[:at[2]]
				return strings.Count(before, "\n") + 1, at[2] - strings.LastIndex(before, "\n")
			}
		}
		return 0, 0
	}
	lines := strings.Split(manifest, "\n")
	if opens != "" {
		constructs := scanYAMLConstructs(lines)
		// A construct on line 1 left yaml.v3 reporting the problem's line.
		for _, at := range []int{line, 1} {
			if column := yamlConstructColumn(constructs, at, opens); column > 0 {
				return at, column
			}
		}
	}
	if strings.HasPrefix(reason, "found a tab character") {
		for n := max(line, 1); n <= len(lines); n++ {
			text := lines[n-1]
			lead := text[:len(text)-len(strings.TrimLeft(text, " \t"))]
			if i := strings.IndexByte(lead, '\t'); i >= 0 {
				return n, i + 1
			}
		}
	}
	return line, yamlFirstTokenColumn(lines, line)
}

// yamlConstruct is a flow collection's opening bracket or a quoted scalar's
// opening quote, found by scanYAMLConstructs.
type yamlConstruct struct {
	open         byte
	line, column int // 1-based; the column counts characters
	closedLine   int // 0 when the source ends with it still open
}

// scanYAMLConstructs walks the source the way YAML tokenizes it, closely
// enough to find where flow collections and quoted scalars open and close:
// it skips comments and block scalar content, opens a bracket or quote only
// where a node may start, and honors quoting escapes. It is used only to
// place an error yaml.v3 has already reported.
func scanYAMLConstructs(lines []string) []yamlConstruct {
	var (
		found       []yamlConstruct
		flow        []int // open flow collections, innermost last
		quote       = -1  // the open quoted scalar
		blockIndent = -1  // a block scalar's parent indentation while its content runs
	)
	for n, text := range lines {
		line := n + 1
		indent := len(text) - len(strings.TrimLeft(text, " "))
		if blockIndent >= 0 {
			if strings.TrimSpace(text) == "" || indent > blockIndent {
				continue
			}
			blockIndent = -1
		}
		prev := byte(0)   // the last significant byte on this line; 0 at its start
		spaced := false   // blanks separate prev from the current byte
		property := false // the last node token was an anchor or a tag
		for i := 0; i < len(text); i++ {
			c := text[i]
			if quote >= 0 {
				q := found[quote].open
				switch {
				case q == '"' && c == '\\':
					i++
				case q == '\'' && c == '\'' && i+1 < len(text) && text[i+1] == '\'':
					i++
				case c == q:
					found[quote].closedLine = line
					quote = -1
					prev, spaced = c, false
				}
				continue
			}
			if c == ' ' || c == '\t' || c == '\r' {
				spaced = true
				continue
			}
			if c == '#' && (prev == 0 || spaced) {
				break // a comment runs to the end of the line
			}
			nodeStart := prev == 0 ||
				(len(flow) > 0 && strings.IndexByte("[{,:", prev) >= 0) ||
				(spaced && (property || strings.IndexByte(":-?", prev) >= 0))
			if nodeStart {
				property = c == '&' || c == '!'
			}
			if (c == '|' || c == '>') && nodeStart && len(flow) == 0 {
				blockIndent = indent
				break
			}
			switch {
			case (c == '[' || c == '{') && (nodeStart || len(flow) > 0):
				flow = append(flow, len(found))
				found = append(found, yamlConstruct{open: c, line: line, column: utf8.RuneCountInString(text[:i]) + 1})
			case (c == ']' || c == '}') && len(flow) > 0:
				found[flow[len(flow)-1]].closedLine = line
				flow = flow[:len(flow)-1]
			case (c == '"' || c == '\'') && nodeStart:
				quote = len(found)
				found = append(found, yamlConstruct{open: c, line: line, column: utf8.RuneCountInString(text[:i]) + 1})
			}
			prev, spaced = c, false
		}
	}
	return found
}

// yamlConstructColumn is the column of the construct on line, opened by one
// of opens, that an error there names: the last one still open at the end of
// the line, else the last one. 0 when the line opens none.
func yamlConstructColumn(constructs []yamlConstruct, line int, opens string) int {
	last, lastOpen := 0, 0
	for _, c := range constructs {
		if c.line != line || strings.IndexByte(opens, c.open) < 0 {
			continue
		}
		last = c.column
		if c.closedLine != line {
			lastOpen = c.column
		}
	}
	if lastOpen > 0 {
		return lastOpen
	}
	return last
}

// yamlFirstTokenColumn is the column of line's first non-space character (a
// tab counts: it is what a "cannot start any token" error names), 1 when the
// line is blank or past the end of the source.
func yamlFirstTokenColumn(lines []string, line int) int {
	if line < 1 || line > len(lines) {
		return 1
	}
	text := strings.TrimRight(lines[line-1], "\r")
	i := strings.IndexFunc(text, func(r rune) bool { return r != ' ' })
	if i < 0 {
		return 1
	}
	return utf8.RuneCountInString(text[:i]) + 1
}
