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

package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The lockfile is read at the service's root directory, as every version file
// is: a nested service's own lockfile decides, and a checkout-root one does not
// reach it (w8/m85).
func TestResolveNativeBuildReadsTheServiceRootLockfile(t *testing.T) {
	o := nativeOptions()
	o.Runtime = "ruby"
	for _, tc := range []struct{ lockAt string }{{"apps/web/Gemfile.lock"}, {"Gemfile.lock"}} {
		bundle, dockerfile, checkout := writeResolveFixture(t, o, nil, nil)
		root := filepath.Join(checkout, "apps", "web")
		must(t, os.MkdirAll(root, 0o755))
		lock := strings.Replace(discourseLock, "3.4.7p58", "3.3.6p108", 1)
		must(t, os.WriteFile(filepath.Join(checkout, tc.lockAt), []byte(lock), 0o644))
		choices, err := ResolveNativeBuild("ruby", bundle, dockerfile, root)
		if err != nil {
			t.Fatal(err)
		}
		want, source := "3.3", "Gemfile.lock"
		if tc.lockAt == "Gemfile.lock" {
			want, source = nativeToolchains["ruby"].defaultLine, ""
		}
		if choices[0].Line != want || choices[0].Source != source {
			t.Errorf("lockfile at %s: Ruby %s from %q, want %s from %q", tc.lockAt, choices[0].Line, choices[0].Source, want, source)
		}
	}
}

// TestNativeImageRunsTheLockedRuby is w8/m85's execution gate: sweep354's
// Discourse shape — a Gemfile allowing >= 3.4, < 4.1 beside a lockfile that
// records Ruby 3.4.7p58 — builds and starts on the reviewed 3.4 line, not the
// newest line the Gemfile range allows. Opt-in: needs Docker.
func TestNativeImageRunsTheLockedRuby(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build and run the generated image")
	}
	o := nativeOptions()
	o.Runtime = "ruby"
	o.BuildCommand = `ruby -e 'print RUBY_VERSION' > built.txt`
	o.StartCommand = `echo "built $(cat built.txt) runtime $(ruby -e 'print RUBY_VERSION')"`
	src := map[string]string{"Gemfile": "source 'https://rubygems.org'\nruby '>= 3.4', '< 4.1'\n", "Gemfile.lock": discourseLock}
	tag := buildResolvedImage(t, o, nil, src, fmt.Sprintf("bex-native-ruby-lock-it:%d", os.Getpid()))
	out, err := exec.Command("docker", "run", "--rm", "--network", "none", tag).CombinedOutput()
	ruby34, _ := nativeToolchains["ruby"].find("3.4")
	if want := "built " + ruby34.version + " runtime " + ruby34.version; err != nil || !strings.Contains(string(out), want) {
		t.Fatalf("run = %v\n%s; want %q", err, out, want)
	}
}
