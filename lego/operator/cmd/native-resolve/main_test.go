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
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/operator/internal/build"
)

func TestRunNarratesOrRefusesAsTenantError(t *testing.T) {
	dir := t.TempDir()
	dockerfile := filepath.Join(dir, "Dockerfile")
	bundle := filepath.Join(dir, "render-env")
	// The real python default base, as the preparer writes it.
	base := "# syntax=docker/dockerfile:1.7\n" +
		"FROM python:3.13-bookworm@sha256:227b6570d6ee07061ae6ca2eb04dedfb6d2b34045835f343065b9869e4d427ea\n" +
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
	if code := run("python", dir, bundle, dockerfile, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if got := strings.TrimSpace(out.String()); got != "==> Using Python 3.11 (from PYTHON_VERSION=3.11.9)" {
		t.Errorf("narration = %q", got)
	}

	write("2.7.18")
	out.Reset()
	errOut.Reset()
	if code := run("python", dir, bundle, dockerfile, &out, &errOut); code != build.ExitTenantError {
		t.Fatalf("unsupported exit = %d, want ExitTenantError", code)
	}
	const want = "PYTHON_VERSION=2.7.18 is not a supported Python version; " +
		"supported Python lines are 3.10, 3.11, 3.12, 3.13, 3.14"
	if !strings.Contains(errOut.String(), want) {
		t.Errorf("refusal = %q", errOut.String())
	}
	if got, _ := os.ReadFile(dockerfile); string(got) != base {
		t.Error("a refused request must leave the Dockerfile untouched")
	}
}
