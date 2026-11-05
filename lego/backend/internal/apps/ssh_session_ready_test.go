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

package apps

import (
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/506: the shell and SSH gate names which of four blockers refused the
// session, always as a 409.
func TestSSHSessionReadyNamesTheBlocker(t *testing.T) {
	ready := AppView{
		Type: appv1alpha1.TypeWebService, Plan: "starter",
		Phase: string(appv1alpha1.PhaseRunning), Revision: "rev-1", Image: "img:v1",
	}
	with := func(mutate func(*AppView)) AppView {
		v := ready
		mutate(&v)
		return v
	}
	for _, tc := range []struct {
		name string
		view AppView
		want string // "" = ready
	}{
		{"ready", ready, ""},
		{"suspended", with(func(v *AppView) { v.Suspended = true }), "suspended; resume it first"},
		{"free plan", with(func(v *AppView) { v.Plan = "free" }), "needs a paid instance type"},
		{"unknown plan", with(func(v *AppView) { v.Plan = "" }), "needs a paid instance type"},
		{"static site", with(func(v *AppView) { v.Type = appv1alpha1.TypeStaticSite; v.Plan = "" }), "web services, private services and background workers"},
		{"cron job", with(func(v *AppView) { v.Type = appv1alpha1.TypeCronJob }), "web services, private services and background workers"},
		{"not running", with(func(v *AppView) { v.Phase = string(appv1alpha1.PhaseDeploying) }), "no running instance yet"},
		{"no revision", with(func(v *AppView) { v.Revision = "" }), "no running instance yet"},
		{"no image", with(func(v *AppView) { v.Image = "" }), "no running instance yet"},
	} {
		err := sshSessionReady(tc.view)
		if tc.want == "" {
			if err != nil {
				t.Errorf("%s: err = %v, want ready", tc.name, err)
			}
			continue
		}
		if !errors.Is(err, core.ErrConflict) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want a 409 containing %q", tc.name, err, tc.want)
		}
		if strings.Contains(err.Error(), "not eligible and running") {
			t.Errorf("%s: err = %v still the one-sentence refusal", tc.name, err)
		}
	}
}
