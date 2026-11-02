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

package desktop

import (
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestShorthand(t *testing.T) {
	// Every directory below exists — including ones named like commands —
	// so the table proves the rule, not the filesystem, decides.
	existing := map[string]bool{
		".": true, "..": true, "./docs": true, "docs": true, "services": true, "ws": true,
		"../x": true, "/abs/p": true, "/home/u": true, "/home/u/p": true, "src/a.go": true,
		"./notes:2024": true,
	}
	stat := func(p string) (os.FileInfo, error) {
		if existing[p] {
			return fakeInfo{}, nil
		}
		return nil, fs.ErrNotExist
	}
	t.Setenv("HOME", "/home/u")

	cases := []struct {
		args []string
		want []string // nil: unchanged
		err  string
	}{
		{args: []string{"."}, want: []string{"edit", "."}},
		{args: []string{"..", "-n"}, want: []string{"edit", "..", "-n"}},
		{args: []string{"./docs"}, want: []string{"edit", "./docs"}},
		{args: []string{"../x"}, want: []string{"edit", "../x"}},
		{args: []string{"/abs/p"}, want: []string{"edit", "/abs/p"}},
		// A quoted ~ is expanded before it is forwarded.
		{args: []string{"~"}, want: []string{"edit", "/home/u"}},
		{args: []string{"~/p:3"}, want: []string{"edit", "/home/u/p:3"}},
		// An existing name with a colon is a file, not a position.
		{args: []string{"./notes:2024"}, want: []string{"edit", "./notes:2024"}},
		{args: []string{"src/a.go:4"}, want: []string{"edit", "src/a.go:4"}},
		{args: []string{"src/a.go:4:2", "--wait"}, want: []string{"edit", "src/a.go:4:2", "--wait"}},
		// A bare word is always cobra's, even with a same-named directory.
		{args: []string{"docs"}},
		{args: []string{"services", "list"}},
		{args: []string{"ws"}},
		{args: []string{"servcies"}},
		{args: []string{}},
		{args: []string{"-o", "json"}},
		{args: []string{"ssh://ags-a@ssh.bex.co/workspace"}},
		// A path-shaped name that does not exist is refused, never created.
		{args: []string{"./nope"}, err: "no such file or directory: ./nope"},
		{args: []string{"src/missing.go:3"}, err: "run `bex edit src/missing.go`"},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			got, err := shorthand(tc.args, stat)
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("err = %v, want %q", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := tc.want
			if want == nil {
				want = tc.args
			}
			if !slices.Equal(got, want) {
				t.Fatalf("shorthand(%q) = %q, want %q", tc.args, got, want)
			}
		})
	}
}
