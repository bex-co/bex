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

// Command claudeplugin regenerates bex's Claude Code plugin and the
// repository's plugin marketplace from the skills pack (w4/m631). Run it from
// lego/cli after changing the pack or packgen/plugin: go run ./hack/claudeplugin.
package main

import (
	"fmt"
	"os"

	"github.com/bex-co/bex/lego/cli/internal/skills/packgen"
)

func main() {
	if err := packgen.GeneratePlugin("internal/skills/pack", "../.."); err != nil {
		fmt.Fprintln(os.Stderr, "claudeplugin:", err)
		os.Exit(1)
	}
}
