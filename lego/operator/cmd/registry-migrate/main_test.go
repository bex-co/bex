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
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The registry password is a credential for the whole registry: an argument of
// the skopeo process is readable by every process in the container through
// /proc/<pid>/cmdline, so it goes in a private auth file, never in argv.
func TestSkopeoCopyArgsPointAtTheAuthFileAndCarryNoPassword(t *testing.T) {
	args := skopeoCopyArgs("docker://h/src:t", "docker://h/dst:t", "/run/x/auth.json")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"copy",
		"--src-tls-verify=false",
		"--dest-tls-verify=false",
		"--src-authfile /run/x/auth.json",
		"--dest-authfile /run/x/auth.json",
		"docker://h/src:t",
		"docker://h/dst:t",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args %v missing %q", args, want)
		}
	}
	for _, arg := range args {
		if strings.Contains(arg, "creds") {
			t.Fatalf("argument %q passes credentials on the command line", arg)
		}
	}
}

func TestSkopeoCopyArgsOmitsTheAuthFileWhenThereIsNone(t *testing.T) {
	args := skopeoCopyArgs("docker://h/src:t", "docker://h/dst:t", "")
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "authfile") || strings.Contains(joined, "creds") {
		t.Fatalf("no credentials must mean no credential flags: %v", args)
	}
}

func TestSkopeoAuthFileHoldsTheCredentialsPrivatelyAndIsRemoved(t *testing.T) {
	path, cleanup, err := writeSkopeoAuthFile("zot.svc:5000", "bex-builder", "s3cr3t")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(path)
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("auth file: %v, %v, want mode 0600", info, err)
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("auth dir: %v, %v, want mode 0700", info, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	want := base64.StdEncoding.EncodeToString([]byte("bex-builder:s3cr3t"))
	if got := parsed.Auths["zot.svc:5000"].Auth; got != want {
		t.Fatalf("auth = %q, want %q", got, want)
	}
	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("auth dir still exists after cleanup: %v", err)
	}
}

func TestSkopeoAuthFileIsSkippedWithoutCredentials(t *testing.T) {
	path, cleanup, err := writeSkopeoAuthFile("zot.svc:5000", "", "")
	if err != nil || path != "" {
		t.Fatalf("path = %q, err = %v, want none", path, err)
	}
	cleanup() // a no-op, but callable
}
