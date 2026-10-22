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
	"fmt"
	"regexp"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Every Render log filter value is one of three forms (api-docs.render.com
// list-logs: "wildcards and regex are supported", w8/176):
//
//   - `/…/` is an RE2 regular expression, matched unanchored;
//   - a value containing `*` is a wildcard pattern, `*` matching any run of characters;
//   - anything else is a literal.
//
// maxFilterPatternLen bounds one value so a pattern cannot be used to make the
// store or the line matcher expensive.
const maxFilterPatternLen = 1024

// regexFilterBody reports a `/…/` value's expression.
func regexFilterBody(v string) (string, bool) {
	if len(v) >= 3 && strings.HasPrefix(v, "/") && strings.HasSuffix(v, "/") {
		return v[1 : len(v)-1], true
	}
	return "", false
}

// validateFilterPatterns refuses a filter value that is too long or a regex
// that does not compile, naming the filter and the value, instead of letting
// the search silently match nothing.
func (q LogQuery) validateFilterPatterns() error {
	for _, f := range []struct {
		name   string
		values []string
	}{
		{"text", q.Search}, {"host", q.Host}, {"statusCode", q.StatusCode},
		{"method", q.Method}, {"path", q.Path}, {"instance", q.Instance},
	} {
		for _, v := range f.values {
			if len(v) > maxFilterPatternLen {
				return fmt.Errorf("%w: invalid %s filter: the value is longer than %d bytes", core.ErrBadRequest, f.name, maxFilterPatternLen)
			}
			if body, ok := regexFilterBody(v); ok {
				if _, err := regexp.Compile(body); err != nil {
					return fmt.Errorf("%w: invalid %s filter %q: %v", core.ErrBadRequest, f.name, v, err)
				}
			}
		}
	}
	return nil
}

// textMatcher compiles one normalized `text` term. Regexes run on the message
// as written; literals and wildcards on its lowercase form (normalizeLogText
// lowercased them), as the literal search always has.
func textMatcher(term string) func(original, lower string) bool {
	if body, ok := regexFilterBody(term); ok {
		re, err := regexp.Compile(body)
		if err != nil {
			return func(string, string) bool { return false }
		}
		return func(original, _ string) bool { return re.MatchString(original) }
	}
	if strings.Contains(term, "*") {
		parts := strings.Split(term, "*")
		for i, p := range parts {
			parts[i] = regexp.QuoteMeta(p)
		}
		re := regexp.MustCompile(strings.Join(parts, ".*"))
		return func(_, lower string) bool { return re.MatchString(lower) }
	}
	return func(_, lower string) bool { return strings.Contains(lower, term) }
}

// labelValueAlternative is one filter value as an alternative of an anchored
// matcher: a literal is quoted, `*` becomes `.*`, and a `/…/` regex matches
// unanchored (padded with `.*`, since Loki's `=~` is fully anchored).
func labelValueAlternative(v string) string {
	if body, ok := regexFilterBody(v); ok {
		return ".*(?:" + body + ").*"
	}
	return strings.ReplaceAll(regexp.QuoteMeta(v), `\*`, ".*")
}
