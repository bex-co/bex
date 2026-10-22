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
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

const accessLine = `10.244.36.215 - - [22/Oct/2026 03:44:30] "GET / HTTP/1.1" 200 -`

// w8/176: Render's `text` filter takes literals, `*` wildcards and /RE2/.
func TestTextFilterWildcardsAndRegex(t *testing.T) {
	for _, tc := range []struct {
		term string
		want bool
	}{
		{"HTTP/1.1", true},
		{"GET*200", true},
		{"*GET*200*", true},
		{"GET*", true},
		{"200*GET", false},
		{"get*200", true}, // wildcards fold case like literals
		{"/GET.*200/", true},
		{"/GET/", true},
		{`/HTTP\/1\.[0-9]/`, true},
		{`/get/`, false}, // a regex is case-sensitive as written
		{`/(?i)get/`, true},
		{`/\D200/`, true},
		{"/nomatch/", false},
	} {
		q := LogQuery{Search: []string{tc.term}}.normalized()
		if got := q.matchesText(accessLine); got != tc.want {
			t.Errorf("text %q = %v, want %v", tc.term, got, tc.want)
		}
	}
	// Several terms stay ORed.
	q := LogQuery{Search: []string{"/nomatch/", "GET*200"}}.normalized()
	if !q.matchesText(accessLine) {
		t.Error("terms must be ORed")
	}
}

func TestInvalidFilterRegexIsANamedBadRequest(t *testing.T) {
	for name, q := range map[string]LogQuery{
		"text":       {Search: []string{"/(/"}},
		"path":       {Path: []string{"/(/"}},
		"statusCode": {StatusCode: []string{"/[/"}},
		"too long":   {Search: []string{strings.Repeat("a", maxFilterPatternLen+1)}},
	} {
		err := q.validate()
		if !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("%s: err = %v, want a bad request", name, err)
		}
	}
	if err := (LogQuery{Search: []string{"/(/"}}).validate(); err == nil || !strings.Contains(err.Error(), "text") || !strings.Contains(err.Error(), "/(/") {
		t.Errorf("error %v must name the filter and value", err)
	}
	if err := (LogQuery{Search: []string{"GET*200", "/a.b/"}, Path: []string{"*", "/^x$/"}}).validate(); err != nil {
		t.Errorf("valid patterns refused: %v", err)
	}
}

// Structured filters pass a /regex/ through as an unanchored expression.
func TestLabelMatcherRegexAndWildcards(t *testing.T) {
	for _, tc := range []struct {
		values []string
		want   string
		match  []string
		reject []string
	}{
		{[]string{"/api"}, `x="/api"`, nil, nil},
		{[]string{"/api/*"}, `x=~"^(/api/.*)$"`, []string{"/api/a"}, []string{"/x/api/a"}},
		{[]string{`/^\/$/`}, "", []string{"/"}, []string{"/a", ""}},
		{[]string{"/4../"}, "", []string{"404", "x500-404-y"}, []string{"200"}},
		{[]string{"/(GET|POST)/", "DELETE"}, "", []string{"POST", "DELETE"}, []string{"PUT"}},
	} {
		m := labelMatcher("x", tc.values)
		if tc.want != "" && m != tc.want {
			t.Errorf("%v: matcher = %s, want %s", tc.values, m, tc.want)
		}
		if tc.match == nil && tc.reject == nil {
			continue
		}
		re := regexp.MustCompile(labelValuePattern(tc.values))
		for _, v := range tc.match {
			if !re.MatchString(v) {
				t.Errorf("%v: %q should match %s", tc.values, v, re)
			}
		}
		for _, v := range tc.reject {
			if re.MatchString(v) {
				t.Errorf("%v: %q should not match %s", tc.values, v, re)
			}
		}
	}
}
