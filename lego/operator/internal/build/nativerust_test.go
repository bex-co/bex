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
	"testing"
)

// w8/327: rustup applies Render's Rust signals inside the one pinned image, so
// the line, image and exact pin never change, but the build log names the
// toolchain that builds: RUSTUP_TOOLCHAIN over the toolchain files,
// rust-toolchain before rust-toolchain.toml, else the pinned release.
func TestRustNarratesTheToolchainRustupApplies(t *testing.T) {
	rust, _ := nativeToolchains["rust"].find("1")
	if !fullVersion.MatchString(rust.version) {
		t.Fatalf("the rust line names no release: %q", rust.version)
	}
	const toml = "[toolchain]\nchannel = \"nightly-2026-01-01\"\ncomponents = [\"rustfmt\"]\n"
	for _, tc := range []struct {
		name  string
		env   map[string]string
		files map[string]string
		want  string
	}{
		{"default names the pinned release", nil, nil, fmt.Sprintf("==> Using Rust %s (default)", rust.version)},
		{"RUSTUP_TOOLCHAIN", map[string]string{"RUSTUP_TOOLCHAIN": "1.75.0"}, nil, "==> Using Rust 1.75.0 (from RUSTUP_TOOLCHAIN=1.75.0)"},
		{"rust-toolchain", nil, map[string]string{"rust-toolchain": "# pinned\n1.81.0\n"}, "==> Using Rust 1.81.0 (from rust-toolchain)"},
		{"the variable overrides the file", map[string]string{"RUSTUP_TOOLCHAIN": "stable"}, map[string]string{"rust-toolchain": "1.81.0\n"}, "==> Using Rust stable (from RUSTUP_TOOLCHAIN=stable)"},
		{"rust-toolchain.toml channel", nil, map[string]string{"rust-toolchain.toml": toml}, "==> Using Rust nightly-2026-01-01 (from rust-toolchain.toml)"},
		{"legacy file in TOML form", nil, map[string]string{"rust-toolchain": toml}, "==> Using Rust nightly-2026-01-01 (from rust-toolchain)"},
		{"rust-toolchain before rust-toolchain.toml", nil, map[string]string{"rust-toolchain": "1.81.0", "rust-toolchain.toml": toml}, "==> Using Rust 1.81.0 (from rust-toolchain)"},
		{"a file with no channel leaves the default", nil, map[string]string{"rust-toolchain.toml": "[toolchain]\ncomponents = [\"clippy\"]\n"}, fmt.Sprintf("==> Using Rust %s (default)", rust.version)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveNativeVersion("rust", tc.env, files(tc.files))
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if got.Narration() != tc.want {
				t.Errorf("narration %q, want %q", got.Narration(), tc.want)
			}
			if got.Line != "1" || got.Image != rust.image || got.Exact != "" {
				t.Errorf("choice line %q image %q exact %q, want the one pinned image, unswapped", got.Line, got.Image, got.Exact)
			}
		})
	}
}
