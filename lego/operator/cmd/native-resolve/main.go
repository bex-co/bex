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

// Command native-resolve is the resolve-native-runtime phase of a native
// build (w8/m51), as a first-party entrypoint of the bex image: it picks the
// toolchain line the service asked for (PYTHON_VERSION, .nvmrc, Gemfile …),
// rewrites the generated Dockerfile's FROM to that line's reviewed pin, and
// narrates the choice into the build log. An unsupported request exits with
// build.ExitTenantError, so the build fails on it by name instead of building
// on the default line.
//
// Environment: BEX_NATIVE_RUNTIME, BEX_NATIVE_SOURCE_DIR (the service's root
// directory in the checkout). It reads /native/render-env and rewrites
// /native/Dockerfile, both written by the preparer.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/bex-co/bex/lego/operator/internal/build"
)

func main() {
	os.Exit(run(os.Getenv("BEX_NATIVE_RUNTIME"), os.Getenv("BEX_NATIVE_SOURCE_DIR"),
		"/native/render-env", "/native/Dockerfile", os.Stdout, os.Stderr))
}

func run(runtime, sourceDir, bundle, dockerfile string, stdout, stderr io.Writer) int {
	choice, err := build.ResolveNativeBuild(runtime, bundle, dockerfile, sourceDir)
	var unsupported *build.UnsupportedVersionError
	switch {
	case errors.As(err, &unsupported):
		_, _ = fmt.Fprintln(stderr, unsupported.Error())
		return build.ExitTenantError
	case err != nil:
		_, _ = fmt.Fprintf(stderr, "native-resolve: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, choice.Narration())
	return 0
}
