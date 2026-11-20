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

package deploybudgets

import (
	"testing"
	"time"
)

// TestGateOutlastsMechanism is the drift guard no per-module test can be:
// the one assertion that reads BOTH sides of every mechanism/observer pair.
// A test that compares one side's constant to a literal copied into its own
// file guards nothing — either side moves together with its own test and the
// other stays green. Here the canonical values meet, so narrowing a gate to
// (or past) its mechanism fails here whatever module the edit lands in.
func TestGateOutlastsMechanism(t *testing.T) {
	pairs := []struct {
		name        string
		mechanism   string
		gate        string
		mechanismVs time.Duration
		gateVs      time.Duration
	}{
		{"rollout", "RolloutBudget", "DeployGateTimeout", RolloutBudget, DeployGateTimeout},
		{"pre-deploy", "PreDeployBudget", "PreDeployGateTimeout", PreDeployBudget, PreDeployGateTimeout},
		{"build", "BuildBudget", "BuildGateTimeout", BuildBudget, BuildGateTimeout},
	}
	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			if p.gateVs <= p.mechanismVs {
				t.Fatalf("%s = %v must be strictly longer than %s = %v, or the generic "+
					"gate message races the mechanism's own verdict",
					p.gate, p.gateVs, p.mechanism, p.mechanismVs)
			}
		})
	}
}
