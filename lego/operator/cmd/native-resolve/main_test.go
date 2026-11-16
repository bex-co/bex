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

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/build"
)

// stubExact answers every tag with one digest, so no test reaches the registry.
type stubExact struct{}

func (stubExact) ResolveTag(context.Context, string, string) (string, bool, error) {
	return "sha256:0000000000000000000000000000000000000000000000000000000000000001", true, nil
}

func (stubExact) ListTags(context.Context, string, string) ([]string, error) { return nil, nil }

func (stubExact) NodeSourceDigest(context.Context, string) (string, error) {
	return "", fmt.Errorf("no Node source metadata fixture")
}

// ImageMetadata answers as the registry does for an official runtime image:
// its version environment and Debian history. The common base reads it for
// every selected primary; a line tag (node:24-bookworm) carries a patch of
// its line.
func (stubExact) ImageMetadata(_ context.Context, image string) (env, history []string, err error) {
	ref, _, _ := strings.Cut(image, "@")
	repo, tag, _ := strings.Cut(ref, ":")
	key := map[string]string{"node": "NODE_VERSION", "python": "PYTHON_VERSION", "ruby": "RUBY_VERSION"}[repo]
	if key == "" {
		return nil, nil, fmt.Errorf("no metadata fixture for %s", image)
	}
	version, _, _ := strings.Cut(tag, "-")
	for strings.Count(version, ".") < 2 {
		version += ".0"
	}
	return []string{key + "=" + version, "PATH=/usr/local/bin:/usr/bin"},
		[]string{"# debian.sh --arch 'amd64' out/ 'bookworm' '@1749513600'"}, nil
}

func TestMain(m *testing.M) {
	exactImages = stubExact{}
	os.Exit(m.Run())
}

func TestRunNarratesOrRefusesAsTenantError(t *testing.T) {
	dir := t.TempDir()
	dockerfile := filepath.Join(dir, "Dockerfile")
	bundle := filepath.Join(dir, "render-env")
	// The real python default base, as the preparer writes it.
	base := "# syntax=docker/dockerfile:1.7\n" +
		"FROM python:3.13-bookworm@sha256:cc55e4ded9c95f6ce2133b3d096db43f214d280c2a8af65d83e246c8fa7d168c\n" +
		"RUN true\n"
	write := func(env string) {
		if err := os.WriteFile(dockerfile, []byte(base), 0o600); err != nil {
			t.Fatal(err)
		}
		line := "PYTHON_VERSION=" + base64.StdEncoding.EncodeToString([]byte(env)) + "\n"
		if err := os.WriteFile(bundle, []byte(line), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write("3.11.9")
	var out, errOut bytes.Buffer
	if code := run(time.Time{}, "python", dir, bundle, dockerfile, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	const wantPython = "==> Using Python 3.11.9 (from PYTHON_VERSION=3.11.9)\n==> Using Poetry 2.1.3 (default)\n" +
		"==> Using Node 24.22.3 (default)"
	if got := strings.TrimSpace(out.String()); got != wantPython {
		t.Errorf("narration = %q", got)
	}

	write("2.7.18")
	out.Reset()
	errOut.Reset()
	if code := run(time.Time{}, "python", dir, bundle, dockerfile, &out, &errOut); code != build.ExitTenantError {
		t.Fatalf("unsupported exit = %d, want ExitTenantError", code)
	}
	const want = "PYTHON_VERSION=2.7.18 is not a supported Python version; " +
		"supported Python lines are 3.7, 3.8, 3.9, 3.10, 3.11, 3.12, 3.13, 3.14"
	if !strings.Contains(errOut.String(), want) {
		t.Errorf("refusal = %q", errOut.String())
	}
	if got, _ := os.ReadFile(dockerfile); string(got) != base {
		t.Error("a refused request must leave the Dockerfile untouched")
	}
}

// An opted-in Bun narrates its own line after the primary toolchain (w8/m71).
func TestRunNarratesAnOptedInBun(t *testing.T) {
	dir := t.TempDir()
	dockerfile, bundle := filepath.Join(dir, "Dockerfile"), filepath.Join(dir, "render-env")
	base := "# syntax=docker/dockerfile:1.7\n" +
		"FROM node:24-bookworm@sha256:9d266751ca3d7fdbe651c878ffd5ad0a42fed49a18c283509267c97f025b7496\n" +
		"RUN true\n"
	if err := os.WriteFile(dockerfile, []byte(base), 0o600); err != nil {
		t.Fatal(err)
	}
	env := "BUN_VERSION=" + base64.StdEncoding.EncodeToString([]byte("1.3.4")) + "\n"
	if err := os.WriteFile(bundle, []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := run(time.Time{}, "node", dir, bundle, dockerfile, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	const want = "==> Using Node 24.22.3 (default)\n==> Using Bun 1.3.4 (from BUN_VERSION=1.3.4)"
	if got := strings.TrimSpace(out.String()); got != want {
		t.Errorf("narration = %q", got)
	}
}
