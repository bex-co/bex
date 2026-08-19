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
	"testing"

	"github.com/bex-co/bex/lego/operator/internal/controller"
)

// TestRuntimeModeDefaultsToKubernetes pins the default an unset BEX_RUNTIME
// resolves to. It used to be opensandbox — an untested host runtime production
// never runs — and because AppReconciler skips convergeRegistryCredentials
// whenever Mode is not kubernetes, a forgotten variable silently disabled
// per-App registry credentials and routed deploys down that path instead of
// failing loudly.
func TestRuntimeModeDefaultsToKubernetes(t *testing.T) {
	t.Setenv("BEX_RUNTIME", "")
	if got := runtimeMode(); got != controller.ModeKubernetes {
		t.Fatalf("unset BEX_RUNTIME resolved to %q, want %q", got, controller.ModeKubernetes)
	}
}

// TestRuntimeModeHonorsExplicitSetting keeps the variable meaningful: the new
// default must not become a hard-coding that ignores an operator's choice.
func TestRuntimeModeHonorsExplicitSetting(t *testing.T) {
	for _, mode := range []string{controller.ModeOpenSandbox, controller.ModeKubernetes} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("BEX_RUNTIME", mode)
			if got := runtimeMode(); got != mode {
				t.Fatalf("BEX_RUNTIME=%q resolved to %q", mode, got)
			}
		})
	}
}
