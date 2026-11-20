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

package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w1/m526 (ADR078 §3b): the dashboard's single "Connect GitHub account" entry
// starts the OAuth claim in chooser mode, and the callback always hands the
// proved set to the account chooser — never binding on its own, never failing
// just because nothing is installed yet.

const m526Dashboard = "https://dash.bex.co"

// chooserSvc is a claim-ready service whose callback redirects to a dashboard,
// so a test can follow the browser through the REST surface.
func chooserSvc(claimable []Installation) (*Service, *fakeVerifier) {
	svc, fv := claimSvc(claimable)
	svc.DashboardURL = m526Dashboard
	return svc, fv
}

// startClaimState runs POST /v1/git/claim with query and returns the signed
// state the claim URL carries.
func startClaimState(t *testing.T, m *http.ServeMux, query string) string {
	t.Helper()
	rec := do(t, m, "POST", "/v1/git/claim"+query)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /v1/git/claim%s => %d: %s", query, rec.Code, rec.Body.String())
	}
	var claim Claim
	if err := json.Unmarshal(rec.Body.Bytes(), &claim); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(claim.ClaimURL)
	if err != nil {
		t.Fatal(err)
	}
	state := u.Query().Get("state")
	if state == "" {
		t.Fatalf("claim URL %q carries no state", claim.ClaimURL)
	}
	return state
}

// redirectParam returns the one dashboard query parameter the callback set.
func redirectParam(t *testing.T, location, key string) string {
	t.Helper()
	u, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}
	if got := u.Scheme + "://" + u.Host + u.Path + "#" + u.Fragment; got != m526Dashboard+"/settings#integrations" {
		t.Fatalf("callback redirect = %q, want the dashboard settings page", location)
	}
	return u.Query().Get(key)
}

// TestChooserModeDefersEveryProvedSetToTheChooser: zero, one and three proved
// candidates all land on the chooser, nothing is bound, and each candidate is
// marked connected exactly when THIS workspace already holds it.
func TestChooserModeDefersEveryProvedSetToTheChooser(t *testing.T) {
	cases := []struct {
		name      string
		claimable []Installation
		// bound seeds existing bindings: installation -> workspace.
		bound         map[int64]string
		wantConnected map[int64]bool
	}{
		{name: "zero", claimable: nil, wantConnected: map[int64]bool{}},
		{
			name:          "one",
			claimable:     []Installation{{ID: 42, AccountLogin: "puncsky", AccountType: "User"}},
			wantConnected: map[int64]bool{42: false},
		},
		{
			name: "three",
			claimable: []Installation{
				{ID: 1, AccountLogin: "puncsky", AccountType: "User"},
				{ID: 2, AccountLogin: "bex-co", AccountType: "Organization"},
				{ID: 3, AccountLogin: "blockeden", AccountType: "Organization"},
			},
			// 2 is bound here; 3 only in another workspace, which does not count.
			bound:         map[int64]string{2: core.DefaultTenant, 3: "tea-other"},
			wantConnected: map[int64]bool{1: false, 2: true, 3: false},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := chooserSvc(tc.claimable)
			st := svc.Store.(*fakeStore)
			for inst, ws := range tc.bound {
				st.conns = append(st.conns, store.GitConnection{WorkspaceID: ws, InstallationID: inst, AccountLogin: "seeded"})
			}
			before := len(st.conns)
			m := mux(svc)

			state := startClaimState(t, m, "?chooser=true")
			rec := do(t, m, "GET", "/v1/git/callback?code=oauth-code&state="+url.QueryEscape(state))
			if rec.Code != http.StatusFound {
				t.Fatalf("callback => %d, want 302: %s", rec.Code, rec.Body.String())
			}
			selID := redirectParam(t, rec.Header().Get("Location"), "git_claim_selection")
			if selID == "" {
				t.Fatalf("callback redirect %q has no git_claim_selection", rec.Header().Get("Location"))
			}
			if len(st.conns) != before {
				t.Fatalf("chooser mode bound something on its own: %+v", st.conns)
			}

			rec = do(t, m, "GET", "/v1/git/claim/selections/"+selID)
			if rec.Code != http.StatusOK {
				t.Fatalf("read selection => %d: %s", rec.Code, rec.Body.String())
			}
			var sel ClaimSelection
			if err := json.Unmarshal(rec.Body.Bytes(), &sel); err != nil {
				t.Fatal(err)
			}
			got := map[int64]bool{}
			for _, c := range sel.Candidates {
				got[c.InstallationID] = c.Connected
			}
			if !reflect.DeepEqual(got, tc.wantConnected) {
				t.Fatalf("candidates (installation -> connected) = %v, want %v", got, tc.wantConnected)
			}
			// The wire shape is an array even when empty, so the chooser can tell
			// "nothing installed" from "gone".
			if !strings.Contains(rec.Body.String(), `"candidates":[`) {
				t.Fatalf("candidates must serialize as an array: %s", rec.Body.String())
			}
		})
	}
}

// TestClaimWithoutChooserIsUnchanged: the option is opt-in. A plain claimGit
// still binds a sole candidate and redirects with no selection.
func TestClaimWithoutChooserIsUnchanged(t *testing.T) {
	svc, _ := chooserSvc([]Installation{{ID: 42, AccountLogin: "puncsky", AccountType: "User"}})
	m := mux(svc)

	state := startClaimState(t, m, "")
	rec := do(t, m, "GET", "/v1/git/callback?code=oauth-code&state="+url.QueryEscape(state))
	if rec.Code != http.StatusFound {
		t.Fatalf("callback => %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != m526Dashboard+"/settings#integrations" {
		t.Fatalf("plain claim redirect = %q, want a bare /settings", loc)
	}
	if got, ok := svc.Store.(*fakeStore).firstFor(core.DefaultTenant); !ok || got.InstallationID != 42 {
		t.Fatalf("plain claim did not bind its sole candidate: %+v", svc.Store.(*fakeStore).conns)
	}
}

// TestClaimRESTRefusesMalformedChooser: a chooser value that is not a boolean
// is a 400 and mints nothing, never a silent "off".
func TestClaimRESTRefusesMalformedChooser(t *testing.T) {
	svc, _ := chooserSvc(nil)
	rec := do(t, mux(svc), "POST", "/v1/git/claim?chooser=maybe")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("chooser=maybe => %d, want 400", rec.Code)
	}
	if n := len(svc.Store.(*fakeStore).txns); n != 0 {
		t.Fatalf("a refused start minted %d transactions", n)
	}
}

// TestChooserModeKeepsEveryProof: chooser mode only changes where a proved set
// goes. A replayed callback, a foreign bex subject, a foreign workspace and an
// out-of-set installation id still bind nothing, and choosing an account this
// workspace already holds is an idempotent success.
func TestChooserModeKeepsEveryProof(t *testing.T) {
	claimable := []Installation{
		{ID: 1, AccountLogin: "puncsky", AccountType: "User"},
		{ID: 2, AccountLogin: "bex-co", AccountType: "Organization"},
	}
	// pending runs a chooser-mode claim to its selection.
	pending := func(t *testing.T) (*Service, *http.ServeMux, string, string) {
		t.Helper()
		svc, _ := chooserSvc(claimable)
		m := mux(svc)
		state := startClaimState(t, m, "?chooser=true")
		rec := do(t, m, "GET", "/v1/git/callback?code=oauth-code&state="+url.QueryEscape(state))
		return svc, m, state, redirectParam(t, rec.Header().Get("Location"), "git_claim_selection")
	}

	t.Run("replayed callback", func(t *testing.T) {
		svc, m, state, _ := pending(t)
		selections := len(svc.Store.(*fakeStore).selections)
		rec := do(t, m, "GET", "/v1/git/callback?code=oauth-code&state="+url.QueryEscape(state))
		if got := redirectParam(t, rec.Header().Get("Location"), "git_error"); got != "connect_failed" {
			t.Fatalf("replay => git_error %q, want connect_failed", got)
		}
		if len(svc.Store.(*fakeStore).selections) != selections {
			t.Fatal("a replayed callback offered a second selection")
		}
	})
	t.Run("foreign subject at the callback", func(t *testing.T) {
		svc, _ := chooserSvc(claimable)
		m := mux(svc)
		state := startClaimState(t, m, "?chooser=true")
		rec := doAs(t, m, "GET", "/v1/git/callback?code=oauth-code&state="+url.QueryEscape(state), "someone-else")
		if got := redirectParam(t, rec.Header().Get("Location"), "git_error"); got != "connect_failed" {
			t.Fatalf("foreign subject => git_error %q, want connect_failed", got)
		}
		if n := len(svc.Store.(*fakeStore).selections); n != 0 {
			t.Fatalf("a foreign subject was offered %d selections", n)
		}
	})
	t.Run("foreign subject at the selection", func(t *testing.T) {
		svc, m, _, selID := pending(t)
		if rec := doAs(t, m, "POST", "/v1/git/claim/selections/"+selID+"?installationId=1", "someone-else"); rec.Code != http.StatusForbidden {
			t.Fatalf("foreign select => %d, want 403", rec.Code)
		}
		if n := len(svc.Store.(*fakeStore).conns); n != 0 {
			t.Fatalf("a foreign select bound %d connections", n)
		}
	})
	t.Run("foreign workspace", func(t *testing.T) {
		svc, _ := chooserSvc(claimable)
		svc.Workspace = coretest.Workspaces([]string{"tea-a", "tea-elsewhere"})
		m := mux(svc)
		state := startClaimState(t, m, "?chooser=true&ownerId=tea-a")
		rec := do(t, m, "GET", "/v1/git/callback?code=oauth-code&state="+url.QueryEscape(state))
		selID := redirectParam(t, rec.Header().Get("Location"), "git_claim_selection")
		if rec := do(t, m, "POST", "/v1/git/claim/selections/"+selID+"?installationId=1&ownerId=tea-elsewhere"); rec.Code != http.StatusForbidden {
			t.Fatalf("select in another workspace => %d, want 403", rec.Code)
		}
		if n := len(svc.Store.(*fakeStore).conns); n != 0 {
			t.Fatalf("a select in another workspace bound %d connections", n)
		}
		// The rightful workspace can still complete it: the refusal spent nothing.
		if rec := do(t, m, "POST", "/v1/git/claim/selections/"+selID+"?installationId=1&ownerId=tea-a"); rec.Code != http.StatusOK {
			t.Fatalf("select in its own workspace => %d: %s", rec.Code, rec.Body.String())
		}
		if got := svc.Store.(*fakeStore).workspacesFor(1); !reflect.DeepEqual(got, []string{"tea-a"}) {
			t.Fatalf("installation 1 bound in %v, want [tea-a]", got)
		}
	})
	t.Run("out-of-set installation", func(t *testing.T) {
		svc, m, _, selID := pending(t)
		if rec := do(t, m, "POST", "/v1/git/claim/selections/"+selID+"?installationId=999"); rec.Code != http.StatusBadRequest {
			t.Fatalf("out-of-set select => %d, want 400", rec.Code)
		}
		if n := len(svc.Store.(*fakeStore).conns); n != 0 {
			t.Fatalf("an out-of-set select bound %d connections", n)
		}
	})
	t.Run("choosing binds once, a connected account idempotently", func(t *testing.T) {
		svc, m, _, selID := pending(t)
		st := svc.Store.(*fakeStore)
		st.conns = append(st.conns, store.GitConnection{WorkspaceID: core.DefaultTenant, InstallationID: 2, AccountLogin: "bex-co"})
		if rec := do(t, m, "POST", "/v1/git/claim/selections/"+selID+"?installationId=2"); rec.Code != http.StatusOK {
			t.Fatalf("select connected account => %d: %s", rec.Code, rec.Body.String())
		}
		if n, _ := st.CountGitConnections(context.Background(), core.DefaultTenant); n != 1 {
			t.Fatalf("choosing a connected account duplicated it: %d rows", n)
		}
		if rec := do(t, m, "POST", "/v1/git/claim/selections/"+selID+"?installationId=1"); rec.Code != http.StatusForbidden {
			t.Fatalf("second select on a spent selection => %d, want 403", rec.Code)
		}
		if n, _ := st.CountGitConnections(context.Background(), core.DefaultTenant); n != 1 {
			t.Fatalf("a spent selection bound again: %d rows", n)
		}
	})
}

// TestChooserSelectionUnrecordedIsNotAmbiguous: when a chooser-mode set of one
// cannot be persisted, the callback must not report "several accounts".
func TestChooserSelectionUnrecordedIsNotAmbiguous(t *testing.T) {
	svc, _ := chooserSvc([]Installation{{ID: 42, AccountLogin: "puncsky", AccountType: "User"}})
	fake := svc.Store.(*fakeStore)
	nonce := seedConnectTxn(t, svc, core.DefaultTenant, testCallerSubject)
	txn := fake.txns[nonce]
	txn.Chooser = true
	fake.txns[nonce] = txn
	svc.Store = &failingSelectionStore{fakeStore: fake}

	_, err := svc.claimFromCallback(context.Background(), nonce, testCallerSubject, "c")
	if err == nil || errors.Is(err, errAmbiguousClaim) || errors.Is(err, errNoClaimableInstallation) {
		t.Fatalf("unrecorded chooser selection err = %v, want a plain failure", err)
	}
	if callbackConnectErrorCode(err) != "connect_failed" {
		t.Fatalf("unrecorded chooser selection code = %q, want connect_failed", callbackConnectErrorCode(err))
	}
}

type failingSelectionStore struct{ *fakeStore }

func (f *failingSelectionStore) CreateGitHubClaimSelection(context.Context, store.GitHubClaimSelection) error {
	return errors.New("db down")
}

// TestGraphQLClaimGitChooserAndConnected: GraphQL takes the same chooser
// option REST does and returns the same connected mark.
func TestGraphQLClaimGitChooserAndConnected(t *testing.T) {
	svc, _ := chooserSvc([]Installation{
		{ID: 1, AccountLogin: "puncsky", AccountType: "User"},
		{ID: 2, AccountLogin: "bex-co", AccountType: "Organization"},
	})
	st := svc.Store.(*fakeStore)
	st.conns = append(st.conns, store.GitConnection{WorkspaceID: core.DefaultTenant, InstallationID: 2, AccountLogin: "bex-co"})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: testCallerCtx(), RequestString: `mutation { claimGit(chooser: true) { claimUrl } }`})
	if len(res.Errors) != 0 {
		t.Fatalf("claimGit errors: %v", res.Errors)
	}
	var nonce string
	for n, txn := range st.txns {
		if !txn.Chooser {
			t.Fatalf("claimGit(chooser: true) minted a transaction without chooser: %+v", txn)
		}
		nonce = n
	}
	_, err = svc.claimFromCallback(context.Background(), nonce, testCallerSubject, "c")
	var sel *claimSelectionRequiredError
	if !errors.As(err, &sel) {
		t.Fatalf("callback err = %v, want a pending selection", err)
	}
	res = graphql.Do(graphql.Params{
		Schema: schema, Context: testCallerCtx(),
		RequestString: `{ gitClaimSelection(id: "` + sel.SelectionID + `") { candidates { installationId accountLogin connected } } }`,
	})
	if len(res.Errors) != 0 {
		t.Fatalf("gitClaimSelection errors: %v", res.Errors)
	}
	candidates := res.Data.(map[string]any)["gitClaimSelection"].(map[string]any)["candidates"].([]any)
	got := map[string]any{}
	for _, c := range candidates {
		row := c.(map[string]any)
		got[row["accountLogin"].(string)] = row["connected"]
	}
	if want := map[string]any{"puncsky": false, "bex-co": true}; !reflect.DeepEqual(got, want) {
		t.Fatalf("GraphQL candidates connected = %v, want %v", got, want)
	}
}

// TestClaimSelectionShapesMatchREST: the chooser's objects expose exactly the
// REST JSON fields on GraphQL (the one-core rule TestGraphQLFieldsMatchRESTShape
// applies to connections and repos).
func TestClaimSelectionShapesMatchREST(t *testing.T) {
	if a, b := jsonFields(ClaimCandidate{}), gqlFields(gitClaimCandidateGQLType); !reflect.DeepEqual(a, b) {
		t.Errorf("ClaimCandidate field drift: REST=%v GraphQL=%v", a, b)
	}
	if a, b := jsonFields(ClaimSelection{}), gqlFields(gitClaimSelectionGQLType); !reflect.DeepEqual(a, b) {
		t.Errorf("ClaimSelection field drift: REST=%v GraphQL=%v", a, b)
	}
}

// TestCallbackSetupActionIsNeverAClaim (ADR078 §3a step 2, w1/m526): a redirect
// carrying setup_action is GitHub's setup-URL redirect. Without an
// installation_id it binds nothing and never reaches the claim resolver — even
// for a user who administers exactly one installation, the case that would have
// bound an unrelated account silently.
func TestCallbackSetupActionIsNeverAClaim(t *testing.T) {
	sole := []Installation{{ID: 42, AccountLogin: "puncsky", AccountType: "User"}}

	t.Run("request with state spends the nonce and binds nothing", func(t *testing.T) {
		svc, fv := chooserSvc(sole)
		m := mux(svc)
		state := startClaimState(t, m, "")
		rec := do(t, m, "GET", "/v1/git/callback?setup_action=request&code=oauth-code&state="+url.QueryEscape(state))
		if got := redirectParam(t, rec.Header().Get("Location"), "git_error"); got != "install_requested" {
			t.Fatalf("setup_action=request => git_error %q, want install_requested", got)
		}
		st := svc.Store.(*fakeStore)
		if len(st.conns) != 0 || len(st.selections) != 0 {
			t.Fatalf("setup_action=request bound %+v / offered %+v", st.conns, st.selections)
		}
		if len(st.txns) != 0 {
			t.Fatal("setup_action=request must spend the nonce")
		}
		if fv.gotClaimCode != "" {
			t.Fatal("setup_action=request reached the claim resolver")
		}
		// The spent nonce cannot then complete as a claim.
		rec = do(t, m, "GET", "/v1/git/callback?code=oauth-code&state="+url.QueryEscape(state))
		if got := redirectParam(t, rec.Header().Get("Location"), "git_error"); got != "connect_failed" {
			t.Fatalf("claim after a request => git_error %q, want connect_failed", got)
		}
		if len(st.conns) != 0 {
			t.Fatal("a spent nonce bound an installation")
		}
	})
	t.Run("request without state still reads as a request", func(t *testing.T) {
		svc, _ := chooserSvc(sole)
		rec := do(t, mux(svc), "GET", "/v1/git/callback?setup_action=request")
		if got := redirectParam(t, rec.Header().Get("Location"), "git_error"); got != "install_requested" {
			t.Fatalf("stateless request => git_error %q, want install_requested", got)
		}
	})
	t.Run("another setup_action without an installation is invalid", func(t *testing.T) {
		svc, fv := chooserSvc(sole)
		m := mux(svc)
		state := startClaimState(t, m, "")
		rec := do(t, m, "GET", "/v1/git/callback?setup_action=update&code=oauth-code&state="+url.QueryEscape(state))
		if got := redirectParam(t, rec.Header().Get("Location"), "git_error"); got != "invalid_installation" {
			t.Fatalf("setup_action=update without id => git_error %q, want invalid_installation", got)
		}
		if len(svc.Store.(*fakeStore).conns) != 0 || fv.gotClaimCode != "" {
			t.Fatal("a setup redirect without an installation reached the claim branch")
		}
	})
	t.Run("install with an installation binds as before", func(t *testing.T) {
		svc, fv := chooserSvc(nil)
		fv.ok = true
		m := mux(svc)
		rec := do(t, m, "GET", callbackPath(t, svc, "42", core.DefaultTenant)+"&setup_action=install")
		if loc := rec.Header().Get("Location"); loc != m526Dashboard+"/settings#integrations" {
			t.Fatalf("install callback redirect = %q, want a bare /settings", loc)
		}
		if got, ok := svc.Store.(*fakeStore).firstFor(core.DefaultTenant); !ok || got.InstallationID != 42 {
			t.Fatalf("install callback did not bind 42: %+v", svc.Store.(*fakeStore).conns)
		}
	})
	t.Run("an OAuth return without setup_action is still a claim", func(t *testing.T) {
		svc, _ := chooserSvc(sole)
		m := mux(svc)
		state := startClaimState(t, m, "")
		do(t, m, "GET", "/v1/git/callback?code=oauth-code&state="+url.QueryEscape(state))
		if got, ok := svc.Store.(*fakeStore).firstFor(core.DefaultTenant); !ok || got.InstallationID != 42 {
			t.Fatalf("claim return did not bind its sole candidate: %+v", svc.Store.(*fakeStore).conns)
		}
	})
	t.Run("JSON mode answers a request with its message", func(t *testing.T) {
		svc, _ := claimSvc(sole)
		rec := do(t, mux(svc), "GET", "/v1/git/callback?setup_action=request")
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "sent to the organization's owners") {
			t.Fatalf("JSON-mode request => %d %s", rec.Code, rec.Body.String())
		}
	})
}
