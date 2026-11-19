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

package agentsession

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/twofactor"
)

// orgSettings: tea-org's own settings are off and its org requires what org
// says; every other workspace requires nothing.
type orgSettings struct{ org core.LoginOrganization }

func (s orgSettings) LoginRequirements(_ context.Context, workspaceID string) (core.LoginRequirements, error) {
	if workspaceID != "tea-org" {
		return core.LoginRequirements{}, nil
	}
	return core.LoginRequirements{}.WithOrganization(s.org.ID, s.org.Name, s.org.TwoFactor, s.org.GoogleLogin), nil
}

func (s orgSettings) OrganizationLoginRequirements(context.Context, string) (core.LoginRequirements, error) {
	return core.LoginRequirements{}, nil
}

type orgEnrollment map[string]bool

func (e orgEnrollment) MFAEnrolled(_ context.Context, subject string) (bool, bool, error) {
	enrolled, human := e[subject]
	return enrolled, human, nil
}

type orgGoogle map[string]bool

func (g orgGoogle) GoogleAccountMatches(_ context.Context, subject string) (bool, bool, error) {
	matches, human := g[subject]
	return matches, human, nil
}

// w7/m466: the SSH gateway's verb answers an org workspace whose own setting
// is off with the org's requirement, through the real gate: an unenrolled
// member is refused with the 2FA code, an SSH key not created in a Google
// sign-in with the Google code, and a member who meets it is allowed. Since
// w7/090 a refusal names the org, so the gateway's banner can.
func TestTwoFactorVerbAnswersAnOrganizationsRequirement(t *testing.T) {
	acme := func(code string) core.LoginRefusal {
		return core.LoginRefusal{Code: code, OrganizationID: "org-1", OrganizationName: "Acme"}
	}
	for name, tc := range map[string]struct {
		org         core.LoginOrganization
		subject     string
		signIn      core.SignIn
		wantRefusal core.LoginRefusal
	}{
		"org 2FA, unenrolled":         {org: core.LoginOrganization{ID: "org-1", Name: "Acme", TwoFactor: true}, subject: "unenrolled", wantRefusal: acme(core.CodeWorkspaceTwoFactorRequired)},
		"org 2FA, enrolled":           {org: core.LoginOrganization{ID: "org-1", Name: "Acme", TwoFactor: true}, subject: "enrolled"},
		"org Google, password key":    {org: core.LoginOrganization{ID: "org-1", Name: "Acme", GoogleLogin: true}, subject: "enrolled", signIn: core.SignInPassword, wantRefusal: acme(core.CodeWorkspaceGoogleLoginRequired)},
		"org Google, Google-made key": {org: core.LoginOrganization{ID: "org-1", Name: "Acme", GoogleLogin: true}, subject: "enrolled", signIn: core.SignInGoogle},
	} {
		t.Run(name, func(t *testing.T) {
			gate := twofactor.New(twofactor.Config{
				Settings:   orgSettings{org: tc.org},
				Enrollment: orgEnrollment{"enrolled": true, "unenrolled": false},
				Google:     orgGoogle{"enrolled": true, "unenrolled": true},
			})
			srv := httptest.NewServer(twoFactorHandler(gate))
			t.Cleanup(srv.Close)
			client := &TwoFactorClient{URL: srv.URL, Secret: []byte("exec-secret")}
			required, refusal, err := client.Ask(context.Background(), "tea-org", tc.subject, "ssh", tc.signIn)
			if err != nil || !required || refusal != tc.wantRefusal {
				t.Fatalf("Ask = (required %v, refusal %+v, %v), want (true, %+v)", required, refusal, err, tc.wantRefusal)
			}
			if required, refusal, err := client.Ask(context.Background(), "tea-alone", tc.subject, "ssh", tc.signIn); err != nil || required || refusal != (core.LoginRefusal{}) {
				t.Fatalf("a standalone workspace = (%v, %+v, %v), want nothing required", required, refusal, err)
			}
		})
	}
}
