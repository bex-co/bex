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
	"strings"
	"testing"
)

func TestSkopeoCopyArgsPassesExplicitCreds(t *testing.T) {
	args := skopeoCopyArgs("docker://h/src:t", "docker://h/dst:t", "bex-builder", "secret")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"copy",
		"--src-tls-verify=false",
		"--dest-tls-verify=false",
		"--src-creds",
		"bex-builder:secret",
		"--dest-creds",
		"bex-builder:secret",
		"docker://h/src:t",
		"docker://h/dst:t",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args %v missing %q", args, want)
		}
	}
}

func TestSkopeoCopyArgsOmitsCredsWhenEmpty(t *testing.T) {
	args := skopeoCopyArgs("docker://h/src:t", "docker://h/dst:t", "", "")
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "--src-creds") || strings.Contains(joined, "--dest-creds") {
		t.Fatalf("empty user/password must omit creds flags: %v", args)
	}
}
