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

package dbrole

import (
	"strings"
	"testing"
)

func TestRequiredPrivilegesAreDerivedFromGrantDDL(t *testing.T) {
	privileges, err := requiredPrivileges()
	if err != nil {
		t.Fatal(err)
	}
	grantLines := 0
	for _, line := range strings.Split(roleGrantsSQL, "\n") {
		if strings.HasPrefix(line, "GRANT ") {
			grantLines++
		}
	}
	if matches := grantPattern.FindAllStringSubmatch(roleGrantsSQL, -1); len(matches) != grantLines {
		t.Fatalf("parsed %d of %d embedded GRANT statements", len(matches), grantLines)
	}
	want := map[string]bool{
		"public:USAGE":                      false,
		"public.agent_session_turns:SELECT": false,
		"public.ssh_sessions:UPDATE":        false,
	}
	for _, privilege := range privileges {
		key := privilege.relation + ":" + privilege.privilege
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for key, found := range want {
		if !found {
			t.Errorf("embedded grant %s was not parsed", key)
		}
	}
}
