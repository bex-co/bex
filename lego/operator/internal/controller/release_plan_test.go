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

package controller

import (
	"testing"
	"time"
)

// forEachReleaseFacts calls fn with every combination of release facts: each
// flag both ways, every verdict and template, awake replicas of 0 and 1, and
// for each clock (the progress deadline, the served release's unavailability)
// unset, recent, and past its bound.
func forEachReleaseFacts(now time.Time, fn func(releaseFacts)) {
	verdicts := []rolloutVerdict{verdictNone, verdictRecorded, verdictFailed}
	templates := []templateRev{templateOther, templateServed, templateCurrent}
	deadlines := []struct {
		exceeded bool
		at       time.Time
	}{{}, {true, now.Add(-rolloutVerdictGrace / 2)}, {true, now.Add(-rolloutVerdictGrace - time.Minute)}}
	unavailable := []time.Time{{}, now.Add(-servedWakeBudget / 2), now.Add(-servedWakeBudget - time.Minute)}
	const flags = 10
	for mask := range 1 << flags {
		bit := func(i int) bool { return mask&(1<<i) != 0 }
		base := releaseFacts{
			suspended: bit(0), idle: bit(1), newer: bit(2), servedRuntime: bit(3), wokeForRelease: bit(4),
			preDeployPassed: bit(5), preDeployFailed: bit(6), scaled: bit(7), ready: bit(8), restorable: bit(9),
		}
		for _, verdict := range verdicts {
			for _, template := range templates {
				for _, replicas := range []int32{0, 1} {
					for _, deadline := range deadlines {
						for _, since := range unavailable {
							f := base
							f.verdict, f.template, f.replicas = verdict, template, replicas
							f.deadlineExceeded, f.deadlineAt = deadline.exceeded, deadline.at
							f.unavailableSince = since
							fn(f)
						}
					}
				}
			}
		}
	}
}

var actNames = [...]string{
	actRoll: "actRoll", actWake: "actWake", actGate: "actGate", actHoldStep: "actHoldStep",
	actHoldVerdict: "actHoldVerdict", actHoldParked: "actHoldParked", actStartServed: "actStartServed",
}

// releaseInvariants are the rules the lifecycle fixes established, each a
// premise about a pass and what its plan must then be.
var releaseInvariants = []struct {
	name  string
	holds func(f releaseFacts, p releasePlan, now time.Time) bool
}{{
	// A parking pass never writes Deploying: the service is going to sleep, and
	// Deploying would flash before Hibernated (w6/m147 t003).
	name: "parked never deploys",
	holds: func(_ releaseFacts, p releasePlan, _ time.Time) bool {
		return !p.parked || !p.deploying() && p.act != actGate && p.act != actStartServed
	},
}, {
	// Parked, a newer release never reaches the Deployment: a wake would start it
	// alone, with no progress deadline to fail it (w1/m172). Only a served release
	// with no template to restore from rolls.
	name: "newer and parked keeps the served template",
	holds: func(f releaseFacts, p releasePlan, _ time.Time) bool {
		keep := f.newer && p.parked && f.servedRuntime && f.restorable
		return !keep || p.act != actRoll
	},
}, {
	// Awake, a newer release rolls over the served release's template only once a
	// served pod is ready or the served release overran its budget, so a wake
	// starts the release that can serve first (w1/m172, w5/m123).
	name: "release template only over a ready or overdue served release",
	holds: func(f releaseFacts, p releasePlan, now time.Time) bool {
		starting := f.newer && f.servedRuntime && !p.parked && f.template == templateServed && f.replicas > 0 && p.act == actRoll
		overdue := !f.unavailableSince.IsZero() && now.Sub(f.unavailableSince) > servedWakeBudget
		return !starting || f.ready || overdue
	},
}, {
	// An idle App whose newer release is rolling with no verdict yet is not idle
	// until the deadline plus the grace (w6/m147).
	name: "idle mid-rollout does not park before its verdict",
	holds: func(f releaseFacts, p releasePlan, now time.Time) bool {
		rolling := f.idle && !f.suspended && f.newer && f.verdict == verdictNone && f.template == templateCurrent && f.scaled &&
			(!f.deadlineExceeded || now.Sub(f.deadlineAt) < rolloutVerdictGrace)
		return !rolling || !p.parked
	},
}, {
	// A deploy wakes a sleeping service once per release, and never one that is
	// suspended or whose rollout or pre-deploy step already failed (w6/076).
	name: "wake only an idle service, once, for a release that has not failed",
	holds: func(f releaseFacts, p releasePlan, _ time.Time) bool {
		return p.act != actWake ||
			f.idle && !f.suspended && f.newer && !f.wokeForRelease && f.verdict != verdictFailed && !f.preDeployFailed
	},
}, {
	// While a release that served still runs, a newer release whose step has not
	// passed never reaches the template (w1/m156).
	name: "unpassed step never rolls over a served release",
	holds: func(f releaseFacts, p releasePlan, _ time.Time) bool {
		unpassed := f.servedRuntime && !f.preDeployPassed
		return !unpassed || p.act != actRoll
	},
}, {
	// The rules above also make the lifecycle move: each of these fails when a
	// fix is removed outright rather than weakened.
	//
	// A failed rollout over a served release goes back to the served template
	// and stays held there (w1/m172).
	name: "a failed rollout holds the served release",
	holds: func(f releaseFacts, p releasePlan, _ time.Time) bool {
		failed := f.servedRuntime && f.preDeployPassed && f.verdict == verdictFailed && f.restorable
		return !failed || p.act == actHoldVerdict
	},
}, {
	// A failed pre-deploy step is a verdict: it is not run again (w1/m156).
	name: "a failed step never runs again",
	holds: func(f releaseFacts, p releasePlan, _ time.Time) bool {
		return !f.preDeployFailed || p.act != actGate
	},
}, {
	// A deploy to a sleeping service wakes it (w6/076).
	name: "an idle service wakes for a newer release",
	holds: func(f releaseFacts, p releasePlan, _ time.Time) bool {
		due := p.autoHibernating && f.newer && !f.wokeForRelease && f.verdict != verdictFailed && !f.preDeployFailed
		return !due || p.act == actWake
	},
}, {
	// The deferred park is bounded by the deadline plus the grace (w6/m147).
	name: "an idle service parks once the grace has passed",
	holds: func(f releaseFacts, p releasePlan, now time.Time) bool {
		overdue := f.idle && f.deadlineExceeded && now.Sub(f.deadlineAt) >= rolloutVerdictGrace
		return !overdue || p.parked
	},
}, {
	// A served release that overran its budget does not hold the newer release
	// any longer (w5/m123).
	name: "an overdue served release lets the newer release roll",
	holds: func(f releaseFacts, p releasePlan, now time.Time) bool {
		overdue := f.newer && f.servedRuntime && !p.parked && f.preDeployPassed && f.verdict != verdictFailed &&
			!f.unavailableSince.IsZero() && now.Sub(f.unavailableSince) > servedWakeBudget
		return !overdue || p.act == actRoll
	},
}, {
	// Every hold keeps a runtime that serves the prior release.
	name: "holds need a served runtime",
	holds: func(f releaseFacts, p releasePlan, _ time.Time) bool {
		return p.act == actRoll || p.act == actWake || f.servedRuntime
	},
}}

// TestReleasePlanInvariants checks planRelease over every combination of facts
// against releaseInvariants, so a later change that breaks one fails here with
// the facts that break it (w5/m123).
func TestReleasePlanInvariants(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	for _, inv := range releaseInvariants {
		t.Run(inv.name, func(t *testing.T) {
			checked, broken := 0, 0
			var first releaseFacts
			var firstPlan releasePlan
			forEachReleaseFacts(now, func(f releaseFacts) {
				checked++
				if p := planRelease(f, now); !inv.holds(f, p, now) {
					if broken == 0 {
						first, firstPlan = f, p
					}
					broken++
				}
			})
			if broken > 0 {
				t.Fatalf("broken by %d of %d fact combinations, first %+v -> %s, parked %v",
					broken, checked, first, actNames[firstPlan.act], firstPlan.parked)
			}
		})
	}
}

// TestReleasePlanLearnsMidPass checks the two facts an executor learns after
// planning: a pre-deploy step that passes on this pass, and a served template
// that cannot be restored.
func TestReleasePlanLearnsMidPass(t *testing.T) {
	now := time.Now()
	// A newer release over a served release whose pod is ready.
	newer := releaseFacts{newer: true, servedRuntime: true, template: templateServed, ready: true, replicas: 1, restorable: true}

	p := planRelease(newer, now)
	if p.act != actGate {
		t.Fatalf("act = %s, want actGate for a step that has not passed", actNames[p.act])
	}
	p.stepPassed()
	if p.act != actRoll {
		t.Fatalf("after the step passed, act = %s, want the rollout over the ready served pod", actNames[p.act])
	}

	failed := newer
	failed.preDeployPassed, failed.verdict = true, verdictFailed
	p = planRelease(failed, now)
	if p.act != actHoldVerdict {
		t.Fatalf("act = %s, want actHoldVerdict over a failed rollout", actNames[p.act])
	}
	p.unrestorable()
	if p.act != actRoll {
		t.Fatalf("with nothing to restore, act = %s, want the rollout, which settles Failed", actNames[p.act])
	}

	parked := newer
	parked.preDeployPassed, parked.suspended = true, true
	p = planRelease(parked, now)
	if p.act != actHoldParked {
		t.Fatalf("act = %s, want actHoldParked for a newer release while suspended", actNames[p.act])
	}
	p.unrestorable()
	if p.act != actRoll || !p.parked {
		t.Fatalf("with nothing to restore, act = %s parked %v, want the rollout, still parked", actNames[p.act], p.parked)
	}
}
