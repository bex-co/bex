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

// Package deploybudgets is the one home for the six timeouts that bound a
// deploy, three mechanism budgets the operator enforces and the three
// observer gates bex-api watches them with. Both modules import it, so the
// two sides of each pair can never drift apart again: before, each side
// pinned the other's number as a literal copied into its own test file, and
// either side could move together with its own test while the other stayed
// green. That already happened once (pre-m80 the rollout's real budget was 3
// minutes, the accidental minimum of three unrelated choices, none of them
// Render's), so TestGateOutlastsMechanism pins the relationship here, on the
// canonical values, in one assertion.
package deploybudgets

import "time"

// Mechanism budgets: Render's contracts, enforced by the operator.
const (
	// RolloutBudget is how long a new pod may take to first report healthy:
	// Render cancels the deploy past 15 minutes and keeps routing to the
	// existing instances. The operator spends it as the startupProbe budget
	// and Deployment.spec.progressDeadlineSeconds (one number, so neither
	// can silently become the tighter).
	RolloutBudget = 15 * time.Minute
	// PreDeployBudget bounds a single pre-deploy Job's wall-clock: Render's
	// 30-minute pre-deploy limit.
	PreDeployBudget = 30 * time.Minute
	// BuildBudget bounds a single build Job's wall-clock: Render's 120-minute
	// build timeout.
	BuildBudget = 120 * time.Minute
)

// Observer gates: bex-api's control-plane policy. Each watches its mechanism
// from another module and is deliberately LONGER than it, so the mechanism's
// own specific verdict lands before the generic gate message. Equal would be
// a race: the vague "did not become healthy within the health-gate window"
// could beat the Deployment's own ProgressDeadlineExceeded and report the
// message w7/m79 worked to eliminate. Deriving each gate from its mechanism
// plus an explicit margin keeps the values distinct but unlinkable only by a
// deliberate edit here, next to the test that pins the relationship.
const (
	// DeployGateTimeout observes the rollout the way the two gates below
	// observe their Jobs, with the same rule (a control-plane budget must be
	// strictly longer than the mechanism it watches) and a 3-minute margin.
	DeployGateTimeout = RolloutBudget + 3*time.Minute
	// PreDeployGateTimeout observes the pre-deploy Job with a 5-minute margin.
	PreDeployGateTimeout = PreDeployBudget + 5*time.Minute
	// BuildGateTimeout observes the build Job with a 5-minute margin.
	BuildGateTimeout = BuildBudget + 5*time.Minute
)
