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

package s3

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// objectKeyCases are the cases of lego/cli/testdata/object-keys.txt, which
// lego/backend's ValidateKey test reads too.
func objectKeyCases(t *testing.T) (valid, invalid []string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "object-keys.txt"))
	if err != nil {
		t.Fatal(err)
	}
	limit := 0
	for n, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kind, rest, _ := strings.Cut(line, " ")
		if kind == "max-key-bytes" {
			if limit, err = strconv.Atoi(rest); err != nil {
				t.Fatalf("object-keys.txt:%d: %v", n+1, err)
			}
			continue
		}
		key, err := strconv.Unquote(rest)
		if err != nil {
			t.Fatalf("object-keys.txt:%d: %s is not a Go string literal: %v", n+1, rest, err)
		}
		switch kind {
		case "valid":
			valid = append(valid, key)
		case "invalid":
			invalid = append(invalid, key)
		default:
			t.Fatalf("object-keys.txt:%d: %q is not max-key-bytes, valid or invalid", n+1, kind)
		}
	}
	if limit != maxKeyBytes {
		t.Fatalf("object-keys.txt has max-key-bytes %d, and the CLI's maxKeyBytes is %d", limit, maxKeyBytes)
	}
	if len(valid) == 0 || len(invalid) == 0 {
		t.Fatalf("object-keys.txt has %d valid and %d invalid keys", len(valid), len(invalid))
	}
	valid = append(valid, strings.Repeat("k", limit))
	invalid = append(invalid, strings.Repeat("k", limit+1), strings.Repeat("é", limit/2+1))
	return valid, invalid
}

func TestKeyRuleIsTheGatewaysTable(t *testing.T) {
	valid, invalid := objectKeyCases(t)
	for _, key := range valid {
		if problem := keyProblem(key); problem != "" {
			t.Errorf("keyProblem(%q) = %q, want none: bex-s3 serves the key", key, problem)
		}
	}
	for _, key := range invalid {
		if keyProblem(key) == "" {
			t.Errorf("keyProblem(%q) found nothing: bex-s3 refuses the key", key)
		}
	}
}

func exitCode(err error) int {
	var coded interface{ ExitCode() int }
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}
	return 1
}

// TestAKeyBexCannotServeIsAWrongCommandLine: Traefik cleans a dot segment or a
// doubled slash out of a request's path, so such a request would fail its
// signature check and blame the clock and the secret (w32/063). Each command
// refuses the key with exit 2 and the rule, before a key is made or a request
// sent.
func TestAKeyBexCannotServeIsAWrongCommandLine(t *testing.T) {
	g := newRig(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	writeFile(t, dir, "a.txt", 10)
	for _, tc := range []struct {
		args    []string
		problem string
	}{
		{[]string{"cp", file, "s3://photos/x/../y"}, `the key "x/../y" has a ".." segment`},
		{[]string{"cp", file, "s3://photos//dbl"}, `the key "/dbl" starts with a slash`},
		{[]string{"cp", file, "s3://photos/a//b"}, `the key "a//b" has a doubled slash`},
		{[]string{"cp", file, "s3://photos/./a"}, `the key "./a" has a "." segment`},
		{[]string{"cp", file, `s3://photos/a\b`}, `the key "a\\b" has a backslash`},
		{[]string{"cp", file, "s3://photos/a\tb"}, `the key "a\tb" has a control character`},
		{[]string{"cp", file, "s3://photos/a\xffb"}, `the key "a\xffb" is not UTF-8`},
		{[]string{"cp", file, "s3://photos/" + strings.Repeat("k", maxKeyBytes+1)}, "is 1025 bytes long"},
		{[]string{"cp", "s3://photos/x/../y", filepath.Join(dir, "out")}, `".." segment`},
		{[]string{"cp", "s3://photos/a", "s3://photos//b"}, "starts with a slash"},
		{[]string{"cp", "--recursive", dir, "s3://photos/site//"}, "doubled slash"},
		{[]string{"mv", file, "s3://photos/x/../y"}, `".." segment`},
		{[]string{"mv", "s3://photos/x/./y", "s3://photos/b"}, `"." segment`},
		{[]string{"rm", "s3://photos/x/../y"}, `".." segment`},
		{[]string{"rm", "--recursive", "--yes", "s3://photos/a//"}, "doubled slash"},
		{[]string{"ls", "s3://photos/x/../"}, `".." segment`},
		{[]string{"ls", "s3://photos//"}, "starts with a slash"},
		{[]string{"sync", dir, "s3://photos/x/../y"}, `".." segment`},
		{[]string{"sync", "s3://photos//site", dir}, "starts with a slash"},
		{[]string{"presign", "s3://photos/x/../y"}, `".." segment`},
		{[]string{"info", "s3://photos/x/../y"}, `".." segment`},
	} {
		stdout, _, err := g.bex(tc.args...)
		if err == nil {
			t.Errorf("bex s3 %q succeeded, printing %q", tc.args, stdout)
			continue
		}
		if code := exitCode(err); code != 2 {
			t.Errorf("bex s3 %q exited %d, want 2: %v", tc.args, code, err)
		}
		for _, want := range []string{"cannot name an object or a prefix", tc.problem, keyRule} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("bex s3 %q: %v\nwant it to say %q", tc.args, err, want)
			}
		}
	}
	if n := g.s3.count(""); n != 0 {
		t.Errorf("a refused key still sent %d S3 requests: %v", n, g.s3.requests(""))
	}
	if n := g.api.mintCount(); n != 0 {
		t.Errorf("a refused key still made %d access keys", n)
	}
	if _, err := os.Stat(file); err != nil {
		t.Errorf("a refused mv touched its source: %v", err)
	}
}

// TestADirectoryOrDottedKeyStillPasses: a trailing slash names a directory, and
// dots inside a name are ordinary.
func TestADirectoryOrDottedKeyStillPasses(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	dir := t.TempDir()
	data := writeFile(t, dir, "a.txt", 10)
	file := filepath.Join(dir, "a.txt")
	g.must("cp", file, "s3://photos/dir/")
	g.must("cp", file, "s3://photos/a.b/c")
	g.must("cp", file, "s3://photos/x/...y/.z")
	if got, want := g.s3.keys("photos"), []string{"a.b/c", "dir/a.txt", "x/...y/.z"}; !slices.Equal(got, want) {
		t.Errorf("keys = %q, want %q", got, want)
	}
	g.must("cp", "s3://photos/a.b/c", filepath.Join(dir, "back.txt"))
	sameBytes(t, filepath.Join(dir, "back.txt"), data)
	if out := g.must("ls", "s3://photos/dir/"); !strings.Contains(out, " a.txt") {
		t.Errorf("ls s3://photos/dir/ =\n%s", out)
	}
}

// TestAFileWhoseNameCannotBeAKeyFailsAlone: a local name may hold a backslash
// or a control character, which no key may. A recursive upload or a sync
// reports that file and copies the rest; a single upload is refused as a
// wrong command line.
func TestAFileWhoseNameCannotBeAKeyFailsAlone(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	dir := t.TempDir()
	writeFile(t, dir, "site/ok.txt", 10)
	writeFile(t, dir, `site/back\slash.txt`, 10)
	writeFile(t, dir, "site/new\nline.txt", 10)
	site := filepath.Join(dir, "site")

	for _, verb := range []string{"cp", "sync"} {
		args := []string{verb, site, "s3://photos/" + verb}
		if verb == "cp" {
			args = append(args, "--recursive")
		}
		_, stderr, err := g.bex(args...)
		if err == nil || exitCode(err) != 1 || !strings.Contains(err.Error(), "2 of 3 operations failed") {
			t.Errorf("bex s3 %q = %v (exit %d), want two of three failed and exit 1", args, err, exitCode(err))
		}
		for _, want := range []string{`back\slash.txt: the key "` + verb + `/back\\slash.txt" has a backslash`, `has a control character`, keyRule} {
			if !strings.Contains(stderr, want) {
				t.Errorf("bex s3 %q printed\n%s\nwant it to say %q", args, stderr, want)
			}
		}
	}
	if got, want := g.s3.keys("photos"), []string{"cp/ok.txt", "sync/ok.txt"}; !slices.Equal(got, want) {
		t.Errorf("keys = %q, want %q", got, want)
	}

	puts := g.s3.count("PUT ")
	_, _, err := g.bex("cp", filepath.Join(site, `back\slash.txt`), "s3://photos/one/")
	if err == nil || exitCode(err) != 2 || !strings.Contains(err.Error(), `the key "one/back\\slash.txt" has a backslash`) {
		t.Errorf("a single upload of a file whose name cannot be a key = %v (exit %d), want exit 2 with the rule", err, exitCode(err))
	}
	if n := g.s3.count("PUT "); n != puts {
		t.Errorf("a refused single upload still sent %d PUT requests", n-puts)
	}
}
