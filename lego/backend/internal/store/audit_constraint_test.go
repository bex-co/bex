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

package store

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestAuditRelationCheckMatchesRelCan pins the audit_events CHECK lists to
// the Go vocabularies. recordAudit swallows sink errors, so a CHECK miss
// would silently drop authorization (including denial) rows. A follow-on
// migration that replaces either constraint is picked up automatically as
// long as it keeps those constraint names.
func TestAuditRelationCheckMatchesRelCan(t *testing.T) {
	body := latestMatchingSQL(t, `audit_events_relation_check`)
	got := quotedList(t, body, regexp.MustCompile(`relation IN \(([\s\S]*?)\)`))
	if !slices.Equal(got, core.RelCanRelations()) {
		t.Errorf("audit_events relation CHECK = %q, want RelCanRelations %q", got, core.RelCanRelations())
	}

	body = latestMatchingSQL(t, `audit_events_oauth_scopes_check`)
	got = quotedList(t, body, regexp.MustCompile(`ARRAY\[([^\]]+)\]`))
	if !slices.Equal(got, core.ClosedOAuthScopes()) {
		t.Errorf("audit_events oauth_scopes CHECK = %q, want ClosedOAuthScopes %q", got, core.ClosedOAuthScopes())
	}
}

func latestMatchingSQL(t *testing.T, needle string) string {
	t.Helper()
	files, err := filepath.Glob("migrations/*.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(files)
	var last string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), needle) {
			last = string(b)
		}
	}
	if last == "" {
		t.Fatalf("no migration mentions %s", needle)
	}
	return last
}

func quotedList(t *testing.T, body string, list *regexp.Regexp) []string {
	t.Helper()
	m := list.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no quoted list matching %s", list)
	}
	quoted := regexp.MustCompile(`'([^']+)'`)
	var out []string
	for _, hit := range quoted.FindAllStringSubmatch(m[1], -1) {
		out = append(out, hit[1])
	}
	if len(out) == 0 {
		t.Fatalf("empty quoted list matching %s", list)
	}
	return out
}
