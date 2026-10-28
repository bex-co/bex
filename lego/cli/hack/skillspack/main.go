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

// Command skillspack regenerates internal/skills/pack from a render-oss/skills
// checkout at packgen.UpstreamCommit: go run ./hack/skillspack <checkout> <pack dir>.
package main

import (
	"fmt"
	"os"

	"github.com/bex-co/bex/lego/cli/internal/skills/packgen"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: skillspack <render-oss/skills checkout> <pack dir>")
		os.Exit(2)
	}
	if err := packgen.Generate(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "skillspack:", err)
		os.Exit(1)
	}
}
