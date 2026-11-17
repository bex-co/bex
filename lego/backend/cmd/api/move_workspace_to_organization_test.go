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

package main

import "testing"

func TestParseMoveWorkspaceArgs(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want moveWorkspaceRequest
	}{
		{[]string{"tea-a", "org-b"}, moveWorkspaceRequest{workspaceID: "tea-a", organizationID: "org-b"}},
		{[]string{"--apply", "tea-a", "org-b"}, moveWorkspaceRequest{workspaceID: "tea-a", organizationID: "org-b", apply: true}},
	} {
		got, err := parseMoveWorkspaceArgs(tc.args)
		if err != nil || got != tc.want {
			t.Errorf("parse %q = %+v, %v; want %+v", tc.args, got, err, tc.want)
		}
	}
	for _, bad := range [][]string{nil, {"tea-a"}, {"org-b", "tea-a"}, {"tea-a", "org-b", "extra"}, {"tea-a", "org-b", "--force"}} {
		if _, err := parseMoveWorkspaceArgs(bad); err == nil {
			t.Errorf("parse %q accepted", bad)
		}
	}
}
