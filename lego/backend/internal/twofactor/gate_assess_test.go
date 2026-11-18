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

package twofactor

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// gate_assess_test.go pins the assessment behind the enable confirmation
// (docs/ADR012-auth.md §9c, w7/m471): who a requirement would block, decided
// by Check's own rules for a member who is not making a request, an API key
// and an SSH key. Every row runs through both the assessment and Check, so a
// rule changed in one and not the other fails here.

// assessFixture holds one workspace per requirement and five people:
// enrolled or not, with a linked Google account matching their email or not,
// and a platform subject Kratos does not know.
type assessFixture struct {
	gate       *Gate
	enrollment *fakeEnrollment
	google     *fakeGoogle
	keys       *fakeKeys
	clock      *clock
	reg        *prometheus.Registry
}

var assessWorkspaces = map[string]core.LoginRequirements{
	"tea-2fa":    {TwoFactor: true},
	"tea-google": {GoogleLogin: true},
	"tea-both":   {TwoFactor: true, GoogleLogin: true},
}

func newAssessFixture() *assessFixture {
	f := &assessFixture{
		enrollment: newEnrollment("enrolled-google", "enrolled-nogoogle", "unenrolled-google", "unenrolled-nogoogle"),
		google: &fakeGoogle{
			matching: map[string]bool{"enrolled-google": true, "unenrolled-google": true},
			humans:   map[string]bool{"enrolled-google": true, "enrolled-nogoogle": true, "unenrolled-google": true, "unenrolled-nogoogle": true},
		},
		keys:  &fakeKeys{creators: map[string]string{}, signIns: map[string]core.SignIn{}, keys: map[string]bool{}},
		clock: &clock{t: time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)},
		reg:   prometheus.NewRegistry(),
	}
	f.enrollment.enroll("enrolled-google")
	f.enrollment.enroll("enrolled-nogoogle")
	settings := &fakeSettings{on: map[string]bool{}, google: map[string]bool{}}
	for ws, r := range assessWorkspaces {
		settings.on[ws], settings.google[ws] = r.TwoFactor, r.GoogleLogin
	}
	f.gate = New(Config{
		Settings: settings, Enrollment: f.enrollment, Google: f.google, Keys: f.keys,
		Now: f.clock.now, Metrics: NewMetrics(f.reg),
	})
	return f
}

// refusals sums every refusal the gate has counted.
func (f *assessFixture) refusals(t *testing.T) float64 {
	t.Helper()
	families, err := f.reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var n float64
	for _, fam := range families {
		for _, m := range fam.GetMetric() {
			n += m.GetCounter().GetValue()
		}
	}
	return n
}

// checkAllows is Check's own answer for id in the workspace that requires r's
// requirement.
func (f *assessFixture) checkAllows(t *testing.T, ws string, id core.Identity) bool {
	t.Helper()
	err := f.gate.Check(context.Background(), ws, id)
	if err != nil && !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("Check(%s, %+v) = %v, want an allow or a refusal", ws, id, err)
	}
	return err == nil
}

// want is a row's answer per requirement: 2FA, Google login, both.
type want [3]bool

var assessOrder = []string{"tea-2fa", "tea-google", "tea-both"}

func TestAssessMemberAgreesWithCheck(t *testing.T) {
	f := newAssessFixture()
	ctx := context.Background()
	rows := map[string]want{
		"enrolled-google":     {true, true, true},
		"enrolled-nogoogle":   {true, false, false},
		"unenrolled-google":   {false, true, false},
		"unenrolled-nogoogle": {false, false, false},
		// Kratos does not know it: a platform principal, exempt from both.
		"platform": {true, true, true},
	}
	for subject, w := range rows {
		for i, ws := range assessOrder {
			got, err := f.gate.AssessMember(ctx, assessWorkspaces[ws], subject)
			if err != nil {
				t.Fatalf("AssessMember(%s, %s) = %v", ws, subject, err)
			}
			if got != w[i] {
				t.Errorf("AssessMember(%s, %s) = %v, want %v", ws, subject, got, w[i])
			}
			// The same member as Check sees them without a sign-in of their own:
			// background work acting as the member, a gateway ticket.
			if check := f.checkAllows(t, ws, core.Identity{Subject: subject, Method: core.MethodSession}); check != got {
				t.Errorf("%s in %s: Check allows=%v but AssessMember=%v", subject, ws, check, got)
			}
		}
	}
	if n := f.refusals(t); n == 0 {
		t.Fatal("Check refused nobody: the fixture is broken")
	}
}

func TestAssessAPIKeyAgreesWithCheck(t *testing.T) {
	type key struct {
		creator string
		signIn  core.SignIn
	}
	rows := map[key]want{
		{"enrolled-google", core.SignInUnrecorded}:   {true, true, true}, // unstamped: Google login exempts it
		{"enrolled-google", core.SignInGoogle}:       {true, true, true},
		{"enrolled-google", core.SignInPassword}:     {true, false, false},
		{"enrolled-nogoogle", core.SignInGoogle}:     {true, false, false}, // the creator's account no longer matches
		{"unenrolled-google", core.SignInGoogle}:     {false, true, false},
		{"unenrolled-google", core.SignInUnrecorded}: {false, true, false},
		// No recorded creator: never meets 2FA; unstamped, Google login exempts it.
		{"", core.SignInUnrecorded}: {false, true, false},
		{"", core.SignInGoogle}:     {false, false, false},
		// A creator Kratos no longer knows (a deleted account) is no exemption.
		{"deleted", core.SignInUnrecorded}: {false, true, false},
		{"deleted", core.SignInGoogle}:     {false, false, false},
	}
	f := newAssessFixture()
	ctx := context.Background()
	n := 0
	for k, w := range rows {
		n++
		clientID := fmt.Sprintf("key-%d", n)
		if k.creator == "" {
			f.keys.keys[clientID] = true
		} else {
			f.keys.creators[clientID] = k.creator
		}
		f.keys.signIns[clientID] = k.signIn
		for i, ws := range assessOrder {
			got, err := f.gate.AssessAPIKey(ctx, assessWorkspaces[ws], k.creator, k.signIn)
			if err != nil {
				t.Fatalf("AssessAPIKey(%s, %+v) = %v", ws, k, err)
			}
			if got != w[i] {
				t.Errorf("AssessAPIKey(%s, %+v) = %v, want %v", ws, k, got, w[i])
			}
			if check := f.checkAllows(t, ws, apiKey(clientID)); check != got {
				t.Errorf("key %+v in %s: Check allows=%v but AssessAPIKey=%v", k, ws, check, got)
			}
		}
	}
}

func TestAssessSSHKeyAgreesWithCheck(t *testing.T) {
	type key struct {
		subject string
		signIn  core.SignIn
	}
	rows := map[key]want{
		{"enrolled-google", core.SignInUnrecorded}:     {true, true, true},
		{"enrolled-google", core.SignInGoogle}:         {true, true, true},
		{"enrolled-google", core.SignInPassword}:       {true, false, false},
		{"enrolled-nogoogle", core.SignInGoogle}:       {true, false, false},
		{"unenrolled-google", core.SignInGoogle}:       {false, true, false},
		{"unenrolled-nogoogle", core.SignInUnrecorded}: {false, true, false},
		// A platform subject is exempt from 2FA, but a stamped key needs a
		// matching Google account it cannot have.
		{"platform", core.SignInUnrecorded}: {true, true, true},
		{"platform", core.SignInGoogle}:     {true, false, false},
	}
	f := newAssessFixture()
	ctx := context.Background()
	for k, w := range rows {
		for i, ws := range assessOrder {
			got, err := f.gate.AssessSSHKey(ctx, assessWorkspaces[ws], k.subject, k.signIn)
			if err != nil {
				t.Fatalf("AssessSSHKey(%s, %+v) = %v", ws, k, err)
			}
			if got != w[i] {
				t.Errorf("AssessSSHKey(%s, %+v) = %v, want %v", ws, k, got, w[i])
			}
			// The gateway's identity for a native SSH key carries its stamp.
			id := core.Identity{Subject: k.subject, Method: core.MethodSSH, SignIn: k.signIn}
			if check := f.checkAllows(t, ws, id); check != got {
				t.Errorf("SSH key %+v in %s: Check allows=%v but AssessSSHKey=%v", k, ws, check, got)
			}
		}
	}
}

// An assessment refuses nobody, so it never moves the refusal counter; the
// lock-out guard does not either.
func TestAnAssessmentCountsNoRefusal(t *testing.T) {
	f := newAssessFixture()
	ctx := context.Background()
	both := assessWorkspaces["tea-both"]
	if ok, err := f.gate.AssessMember(ctx, both, "unenrolled-nogoogle"); ok || err != nil {
		t.Fatalf("AssessMember = (%v, %v), want (false, nil)", ok, err)
	}
	if ok, err := f.gate.AssessAPIKey(ctx, both, "", core.SignInPassword); ok || err != nil {
		t.Fatalf("AssessAPIKey = (%v, %v), want (false, nil)", ok, err)
	}
	if ok, err := f.gate.AssessSSHKey(ctx, both, "unenrolled-google", core.SignInPassword); ok || err != nil {
		t.Fatalf("AssessSSHKey = (%v, %v), want (false, nil)", ok, err)
	}
	if ok, err := f.gate.SignedInWithGoogle(ctx, signedIn("unenrolled-google", core.SignInPassword)); ok || err != nil {
		t.Fatalf("SignedInWithGoogle = (%v, %v), want (false, nil)", ok, err)
	}
	f.enrollment.err = errors.New("kratos down")
	if _, err := f.gate.AssessMember(ctx, assessWorkspaces["tea-2fa"], "enrolled-nogoogle"); !errors.Is(err, core.ErrAuthzUnavailable) {
		t.Fatalf("AssessMember with Kratos down = %v, want ErrAuthzUnavailable", err)
	}
	if n := f.refusals(t); n != 0 {
		t.Fatalf("refusals counted = %v, want none: nobody was refused", n)
	}
}

// A lookup that cannot answer is an error, never "meets" and never "would be
// blocked", so the confirmation can say it could not check. Requiring nothing
// asks nobody.
func TestAnAssessmentThatCannotAnswerIsAnError(t *testing.T) {
	ctx := context.Background()
	f := newAssessFixture()
	f.enrollment.err = errors.New("kratos down")
	f.google.err = errors.New("kratos down")
	for _, ws := range assessOrder {
		r := assessWorkspaces[ws]
		if ok, err := f.gate.AssessMember(ctx, r, "enrolled-google"); ok || !errors.Is(err, core.ErrAuthzUnavailable) {
			t.Errorf("AssessMember(%s) = (%v, %v), want ErrAuthzUnavailable", ws, ok, err)
		}
		if ok, err := f.gate.AssessAPIKey(ctx, r, "enrolled-google", core.SignInGoogle); ok || !errors.Is(err, core.ErrAuthzUnavailable) {
			t.Errorf("AssessAPIKey(%s) = (%v, %v), want ErrAuthzUnavailable", ws, ok, err)
		}
		if ok, err := f.gate.AssessSSHKey(ctx, r, "enrolled-google", core.SignInGoogle); ok || !errors.Is(err, core.ErrAuthzUnavailable) {
			t.Errorf("AssessSSHKey(%s) = (%v, %v), want ErrAuthzUnavailable", ws, ok, err)
		}
	}
	if ok, err := f.gate.AssessMember(ctx, core.LoginRequirements{}, "unenrolled-nogoogle"); !ok || err != nil {
		t.Fatalf("AssessMember requiring nothing = (%v, %v), want (true, nil) with no lookup", ok, err)
	}

	// No identity lookup wired at all is the same outage.
	bare := New(Config{Settings: &fakeSettings{}})
	if _, err := bare.AssessMember(ctx, assessWorkspaces["tea-both"], "enrolled-google"); !errors.Is(err, core.ErrAuthzUnavailable) {
		t.Fatalf("AssessMember with no lookup wired = %v, want ErrAuthzUnavailable", err)
	}
}

// The assessment reads the gate's own caches: a member Check has just looked
// up costs the confirmation no second Kratos call.
func TestAnAssessmentReadsTheGatesCaches(t *testing.T) {
	f := newAssessFixture()
	ctx := context.Background()
	_ = f.gate.Check(ctx, "tea-both", core.Identity{Subject: "enrolled-google", Method: core.MethodSession})
	enrollments, accounts := f.enrollment.total(), f.google.calls
	if _, err := f.gate.AssessMember(ctx, assessWorkspaces["tea-both"], "enrolled-google"); err != nil {
		t.Fatal(err)
	}
	if f.enrollment.total() != enrollments || f.google.calls != accounts {
		t.Fatalf("lookups after the assessment = (%d, %d), want the cached (%d, %d)", f.enrollment.total(), f.google.calls, enrollments, accounts)
	}
}
