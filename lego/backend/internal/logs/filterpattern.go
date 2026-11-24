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
	"slices"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
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

// isInstancePattern reports whether selector uses wildcard (`*`) or `/RE2/`
// regex syntax rather than an exact instance identifier.
func isInstancePattern(selector string) bool {
	if _, ok := regexFilterBody(selector); ok {
		return true
	}
	return strings.Contains(selector, "*")
}

// hasInstancePattern reports whether any selector uses wildcard or regex syntax.
func hasInstancePattern(selectors []string) bool {
	return slices.ContainsFunc(selectors, isInstancePattern)
}

// compileInstancePattern compiles one wildcard (`*`) or `/RE2/` regex instance
// selector. A `/…/` regex matches unanchored (matching Render's log filter
// syntax); a `*` wildcard is anchored across the whole candidate string.
func compileInstancePattern(selector string) *regexp.Regexp {
	if body, ok := regexFilterBody(selector); ok {
		re, err := regexp.Compile(body)
		if err != nil {
			return nil
		}
		return re
	}
	re, err := regexp.Compile("^" + labelValueAlternative(selector) + "$")
	if err != nil {
		return nil
	}
	return re
}

// matchInstancePattern reports whether re matches candidate's canonical public
// instance ID, legacy UID-derived ID (when UID is non-empty), or raw pod name.
func matchInstancePattern(re *regexp.Regexp, resourceID string, c ids.InstanceCandidate) bool {
	if c.Name == "" {
		return false
	}
	if re.MatchString(c.Name) {
		return true
	}
	if resourceID != "" {
		if re.MatchString(ids.ServiceInstanceID(resourceID, c.Name)) {
			return true
		}
		if c.UID != "" && re.MatchString(ids.LegacyServiceInstanceID(resourceID, c.UID)) {
			return true
		}
	}
	return false
}

// resolveInstanceSelectors translates public, legacy, raw-name, wildcard (`*`),
// and `/RE2/` regex instance selectors into the internal pod or deploy names
// Loki and live tails match on, bounded strictly to the authorized candidate
// set (w32/114). Exact selectors keep ids.ResolveInstanceSelector's uniqueness
// check; wildcard and regex selectors return every matching candidate name
// (deduplicated, preserving candidate order per selector).
func resolveInstanceSelectors(selectors []string, resourceID string, candidates []ids.InstanceCandidate) []string {
	if len(selectors) == 0 {
		return nil
	}
	var out []string
	seen := make(map[string]struct{}, len(selectors))
	add := func(name string) {
		if name == "" {
			return
		}
		if _, dup := seen[name]; dup {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for _, sel := range selectors {
		if sel == "" {
			continue
		}
		if !isInstancePattern(sel) {
			if name, ok := ids.ResolveInstanceSelector(sel, resourceID, candidates); ok {
				add(name)
			}
			continue
		}
		re := compileInstancePattern(sel)
		if re == nil {
			continue
		}
		for _, c := range candidates {
			if matchInstancePattern(re, resourceID, c) {
				add(c.Name)
			}
		}
	}
	return out
}

// mergeResolvedInstances resolves selectors against newly discovered live
// candidates and appends any newly matched internal names to q.Instance
// without mutating an earlier backing array shared with a concurrent goroutine.
func (q *LogQuery) mergeResolvedInstances(selectors []string, resourceID string, candidates []ids.InstanceCandidate) {
	if len(selectors) == 0 {
		return
	}
	resolved := resolveInstanceSelectors(selectors, resourceID, candidates)
	var added []string
	for _, name := range resolved {
		if !slices.Contains(q.Instance, name) {
			added = append(added, name)
		}
	}
	if len(added) > 0 {
		q.Instance = append(slices.Clip(q.Instance), added...)
	}
}
