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
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/spf13/cobra"
)

// rig is `bex s3` against a fake bex-api (which mints keys) and a fake S3
// endpoint over TLS, the way production is reached.
type rig struct {
	t     *testing.T
	api   *fakeAPI
	s3    *fakeS3
	d     deps
	state string
	env   map[string]string
	stdin *bytes.Buffer
}

func newRig(t *testing.T) *rig {
	t.Helper()
	api := newFakeAPI(t)
	s3 := newFakeS3(t)
	d, state := testDeps(t, api)
	g := &rig{t: t, api: api, s3: s3, d: d, state: state, env: map[string]string{endpointEnv: s3.srv.URL}, stdin: &bytes.Buffer{}}
	g.d.lookupEnv = func(name string) (string, bool) {
		v, ok := g.env[name]
		return v, ok
	}
	g.d.httpClient = func() aws.HTTPClient { return s3.srv.Client() }
	g.d.stdin = g.stdin
	return g
}

// bex runs `bex <args>` and returns what it printed.
func (g *rig) bex(args ...string) (stdout, stderr string, err error) {
	g.t.Helper()
	return bex(g.t, g.d, append([]string{"s3"}, args...)...)
}

// must runs a command that has to succeed and returns its standard output.
func (g *rig) must(args ...string) string {
	g.t.Helper()
	stdout, stderr, err := g.bex(args...)
	if err != nil {
		g.t.Fatalf("bex s3 %s: %v\nstdout: %s\nstderr: %s", strings.Join(args, " "), err, stdout, stderr)
	}
	return stdout
}

// fails runs a command that has to fail and returns its error text.
func (g *rig) fails(args ...string) string {
	g.t.Helper()
	stdout, _, err := g.bex(args...)
	if err == nil {
		g.t.Fatalf("bex s3 %s succeeded, printing %q", strings.Join(args, " "), stdout)
	}
	return err.Error()
}

// writeFile writes size random bytes to dir/name and returns them.
func writeFile(t *testing.T, dir, name string, size int) []byte {
	t.Helper()
	data := make([]byte, size)
	_, _ = rand.Read(data)
	return writeBytes(t, dir, name, data)
}

func writeBytes(t *testing.T, dir, name string, data []byte) []byte {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return data
}

func sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// sameBytes fails the test unless the file holds want.
func sameBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if sum(got) != sum(want) {
		t.Errorf("%s: %d bytes with sha256 %s, want %d bytes with sha256 %s", path, len(got), sum(got), len(want), sum(want))
	}
}

// entriesOf lists the names in dir, for a test that says what is left there.
func entriesOf(t *testing.T, dir string) []string {
	t.Helper()
	var names []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			names = append(names, filepath.ToSlash(rel))
		}
		return nil
	})
	return names
}

func keysOf(n int, format string) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf(format, i)
	}
	return out
}

// newTestCommand is a command with the output flag, for code that wants a run.
func newTestCommand() *cobra.Command {
	c := &cobra.Command{Use: "test"}
	c.Flags().StringP("output", "o", "text", "")
	c.SetContext(context.Background())
	return c
}
