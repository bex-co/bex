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

package workspaces

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"slices"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// login_requirement_impact.go answers who turning a login requirement on
// would block, before an admin or an Org Owner confirms it
// (docs/ADR012-auth.md §9c, "The enable confirmation", w7/m471): each member
// who would lose access and each API key and SSH key that would stop working,
// as Render's SSO enforcement lists "Each listed member will lose access" and
// "Each listed API key will be invalidated". Every answer is the enforcement
// gate's own (twofactor.Gate's assessment over its caches), never a second
// copy of its rules.

// The requirements a confirmation can be asked about, GraphQL's
// LoginRequirement enum.
const (
	LoginRequirementTwoFactor   = "TWO_FACTOR"
	LoginRequirementGoogleLogin = "GOOGLE_LOGIN"
)

// LoginAssessor is the enforcement gate's assessment of a member and their
// keys outside a request, and the workspace's current requirements through the
// gate's own cache. *twofactor.Gate satisfies it. An error means a lookup
// could not answer.
type LoginAssessor interface {
	Required(ctx context.Context, workspaceID string) (core.LoginRequirements, error)
	AssessMember(ctx context.Context, required core.LoginRequirements, subject string) (bool, error)
	AssessAPIKey(ctx context.Context, required core.LoginRequirements, creator string, signIn core.SignIn) (bool, error)
	AssessSSHKey(ctx context.Context, required core.LoginRequirements, subject string, signIn core.SignIn) (bool, error)
}

// LoginImpactKeys reads the credentials a requirement holds: the API keys
// bound to some workspaces and some people's SSH keys. The composition root
// adapts the Hydra client list, the key bindings and the ssh_keys table onto
// it.
type LoginImpactKeys interface {
	BoundAPIKeys(ctx context.Context, workspaceIDs []string) ([]BoundAPIKey, error)
	SSHKeysOf(ctx context.Context, subjects []string) ([]store.SSHKey, error)
}

// BoundAPIKey is an API key bound to a workspace, with the stamps the gate
// reads: who created it and how they were signed in.
type BoundAPIKey struct {
	ID            string
	Name          string
	WorkspaceID   string
	CreatedBy     string
	CreatedSignIn core.SignIn
}

// LoginRequirementImpact is who a requirement would block: members who lose
// access or keep it with keys that stop working, then the keys no member
// created. Empty lists mean nobody would be blocked; a read that cannot tell
// fails instead.
type LoginRequirementImpact struct {
	Members             []LoginImpactMember `json:"members"`
	UnattributedAPIKeys []LoginImpactAPIKey `json:"unattributedApiKeys"`
}

// LoginImpactMember is one person the requirement affects. LosesAccess false
// means only the listed keys stop working.
type LoginImpactMember struct {
	UserID      string              `json:"userId"`
	Email       string              `json:"email"`
	Name        string              `json:"name"`
	LosesAccess bool                `json:"losesAccess"`
	APIKeys     []LoginImpactAPIKey `json:"apiKeys"`
	SSHKeys     []LoginImpactSSHKey `json:"sshKeys"`
}

// LoginImpactAPIKey is an API key that would stop working. WorkspaceName is
// the workspace it is bound to, filled on an org's read only.
type LoginImpactAPIKey struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	WorkspaceName string `json:"workspaceName"`
}

// LoginImpactSSHKey is an SSH key that would stop working.
type LoginImpactSSHKey struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
}

// LoginRequirementImpact answers who turning requirement on in workspaceID
// would block, assessed against requirement ORed with what the workspace
// already requires (its own setting and its org's): its members, the API
// keys bound to it and its members' SSH keys, which the gate blocks there
// too. Admin only (can_manage on the exact workspace), as the setter is. A
// read: denials are recorded, an allowed call is not. The default workspace,
// which the gate never enforces, answers empty. A lookup that cannot answer
// (Kratos, Hydra, the store) fails the read with core.ErrAuthzUnavailable,
// never an empty list.
func (s *Service) LoginRequirementImpact(ctx context.Context, workspaceID, requirement string) (LoginRequirementImpact, error) {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	id, err := s.AuthorizeWorkspace(ctx, core.RelCanManage, workspaceID)
	if err != nil {
		return LoginRequirementImpact{}, err
	}
	adds, err := parseLoginRequirement(requirement)
	if err != nil {
		return LoginRequirementImpact{}, err
	}
	if s.Store == nil {
		return LoginRequirementImpact{}, core.ErrWorkspacesUnavailable
	}
	if id == core.DefaultTenant {
		return LoginRequirementImpact{Members: []LoginImpactMember{}, UnattributedAPIKeys: []LoginImpactAPIKey{}}, nil
	}
	if s.LoginAssessor == nil || s.LoginImpactKeys == nil {
		return LoginRequirementImpact{}, core.ErrAuthzUnavailable
	}
	current, err := s.LoginAssessor.Required(ctx, id)
	if err != nil {
		return LoginRequirementImpact{}, err
	}
	members, err := s.Store.ListTenantMembers(ctx, id)
	if err != nil {
		return LoginRequirementImpact{}, impactUnavailable("listing the members", err)
	}
	people := make([]string, 0, len(members))
	for _, m := range members {
		people = append(people, m.Subject)
	}
	return s.assessLoginImpact(ctx, orLogin(current, adds), people, []string{id}, nil)
}

// OrganizationLoginRequirementImpact answers who turning requirement on for
// orgID would block across the org (ADR012 §9c, "Who is held to it"): every
// person in an org workspace, Guests included, and the org's own members,
// each listed once; the API keys bound to any org workspace, each naming its
// workspace; and those people's SSH keys. It assesses against requirement
// ORed with the org's own settings. Owner only, as the setter is; otherwise
// LoginRequirementImpact's rules.
func (s *Service) OrganizationLoginRequirementImpact(ctx context.Context, orgID, requirement string) (LoginRequirementImpact, error) {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	if err := s.AuthorizeOn(ctx, core.RelCanManage, core.OrganizationObject(orgID)); err != nil {
		return LoginRequirementImpact{}, err
	}
	adds, err := parseLoginRequirement(requirement)
	if err != nil {
		return LoginRequirementImpact{}, err
	}
	orgs, ok := s.organizations()
	if !ok {
		return LoginRequirementImpact{}, core.ErrWorkspacesUnavailable
	}
	o, err := requireOwner(ctx, orgs, orgID)
	if err != nil {
		return LoginRequirementImpact{}, err
	}
	if s.LoginAssessor == nil || s.LoginImpactKeys == nil {
		return LoginRequirementImpact{}, core.ErrAuthzUnavailable
	}
	owners, err := orgs.ListOrganizationMembers(ctx, orgID)
	if err != nil {
		return LoginRequirementImpact{}, impactUnavailable("listing the org's members", err)
	}
	workspaces, err := orgs.ListOrganizationWorkspaces(ctx, orgID)
	if err != nil {
		return LoginRequirementImpact{}, impactUnavailable("listing the org's workspaces", err)
	}
	var people []string
	seen := map[string]bool{}
	add := func(subject string) {
		if !seen[subject] {
			seen[subject] = true
			people = append(people, subject)
		}
	}
	ids := make([]string, 0, len(workspaces))
	names := make(map[string]string, len(workspaces))
	for _, w := range workspaces {
		ids = append(ids, w.ID)
		names[w.ID] = w.Label()
	}
	members, err := orgs.ListMembersOfTenants(ctx, ids)
	if err != nil {
		return LoginRequirementImpact{}, impactUnavailable("listing the workspaces' members", err)
	}
	for _, m := range owners {
		add(m.Subject)
	}
	for _, m := range members {
		add(m.Subject)
	}
	required := orLogin(core.LoginRequirements{TwoFactor: o.RequireTwoFactor, GoogleLogin: o.RequireGoogleLogin}, adds)
	return s.assessLoginImpact(ctx, required, people, ids, names)
}

// parseLoginRequirement maps the wire enum onto the requirement it adds.
func parseLoginRequirement(requirement string) (core.LoginRequirements, error) {
	switch requirement {
	case LoginRequirementTwoFactor:
		return core.LoginRequirements{TwoFactor: true}, nil
	case LoginRequirementGoogleLogin:
		return core.LoginRequirements{GoogleLogin: true}, nil
	}
	return core.LoginRequirements{}, fmt.Errorf("%w: requirement: must be %s or %s", core.ErrBadRequest, LoginRequirementTwoFactor, LoginRequirementGoogleLogin)
}

func orLogin(a, b core.LoginRequirements) core.LoginRequirements {
	a.TwoFactor = a.TwoFactor || b.TwoFactor
	a.GoogleLogin = a.GoogleLogin || b.GoogleLogin
	return a
}

// impactUnavailable is a lookup the read cannot do without: logged, and
// answered as "could not check".
func impactUnavailable(what string, err error) error {
	log.Printf("workspaces: login requirement impact: %s: %v", what, err)
	return core.ErrAuthzUnavailable
}

// impactLookupLimit bounds how many gate lookups one read runs at once:
// enough for an org of hundreds, few enough not to crowd Kratos.
const impactLookupLimit = 8

// assessLoginImpact assesses people (in the order given, each once), their SSH
// keys and the API keys bound to workspaceIDs against required. A key whose
// creator is not among people is listed as unattributed. workspaceNames, when
// set, names each API key's workspace.
func (s *Service) assessLoginImpact(ctx context.Context, required core.LoginRequirements, people, workspaceIDs []string, workspaceNames map[string]string) (LoginRequirementImpact, error) {
	apiKeys, err := s.LoginImpactKeys.BoundAPIKeys(ctx, workspaceIDs)
	if err != nil {
		return LoginRequirementImpact{}, impactUnavailable("listing the API keys", err)
	}
	sshKeys, err := s.LoginImpactKeys.SSHKeysOf(ctx, people)
	if err != nil {
		return LoginRequirementImpact{}, impactUnavailable("listing the SSH keys", err)
	}

	var (
		mu           sync.Mutex
		losesAccess  = map[string]bool{}
		blockedAPI   = map[string][]LoginImpactAPIKey{}
		blockedSSH   = map[string][]LoginImpactSSHKey{}
		unattributed []LoginImpactAPIKey
	)
	isPerson := make(map[string]bool, len(people))
	for _, p := range people {
		isPerson[p] = true
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(impactLookupLimit)
	for _, p := range people {
		g.Go(func() error {
			ok, err := s.LoginAssessor.AssessMember(gctx, required, p)
			if err == nil && !ok {
				mu.Lock()
				losesAccess[p] = true
				mu.Unlock()
			}
			return err
		})
	}
	for _, k := range sshKeys {
		g.Go(func() error {
			ok, err := s.LoginAssessor.AssessSSHKey(gctx, required, k.Subject, k.CreatedSignIn)
			if err == nil && !ok {
				mu.Lock()
				blockedSSH[k.Subject] = append(blockedSSH[k.Subject], LoginImpactSSHKey{ID: k.ID, Name: k.Name, Fingerprint: k.Fingerprint})
				mu.Unlock()
			}
			return err
		})
	}
	for _, k := range apiKeys {
		g.Go(func() error {
			ok, err := s.LoginAssessor.AssessAPIKey(gctx, required, k.CreatedBy, k.CreatedSignIn)
			if err == nil && !ok {
				v := LoginImpactAPIKey{ID: k.ID, Name: k.Name, WorkspaceName: workspaceNames[k.WorkspaceID]}
				mu.Lock()
				if isPerson[k.CreatedBy] {
					blockedAPI[k.CreatedBy] = append(blockedAPI[k.CreatedBy], v)
				} else {
					unattributed = append(unattributed, v)
				}
				mu.Unlock()
			}
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return LoginRequirementImpact{}, err // the gate's own ErrAuthzUnavailable, already logged
	}

	listed := slices.DeleteFunc(slices.Clone(people), func(p string) bool {
		return !losesAccess[p] && len(blockedAPI[p]) == 0 && len(blockedSSH[p]) == 0
	})
	out := LoginRequirementImpact{Members: make([]LoginImpactMember, 0, len(listed)), UnattributedAPIKeys: sortedKeys(unattributed)}
	if len(listed) == 0 {
		return out, nil
	}
	userIDs, err := s.Store.OwnerIDsForSubjects(ctx, listed)
	if err != nil {
		return LoginRequirementImpact{}, impactUnavailable("reading user ids", err)
	}
	var attrs map[string]IdentityAttrs
	if s.Identities != nil {
		attrs = s.Identities.LookupMany(ctx, listed)
	}
	for _, p := range listed {
		ssh := append([]LoginImpactSSHKey{}, blockedSSH[p]...)
		slices.SortFunc(ssh, func(a, b LoginImpactSSHKey) int { return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.ID, b.ID)) })
		out.Members = append(out.Members, LoginImpactMember{
			UserID: userIDs[p], Email: attrs[p].Email, Name: attrs[p].Name,
			LosesAccess: losesAccess[p],
			APIKeys:     sortedKeys(blockedAPI[p]),
			SSHKeys:     ssh,
		})
	}
	return out, nil
}

// sortedKeys orders keys by workspace, then name, so concurrent assessment
// answers the same list every time; never nil.
func sortedKeys(keys []LoginImpactAPIKey) []LoginImpactAPIKey {
	out := append([]LoginImpactAPIKey{}, keys...)
	slices.SortFunc(out, func(a, b LoginImpactAPIKey) int {
		return cmp.Or(cmp.Compare(a.WorkspaceName, b.WorkspaceName), cmp.Compare(a.Name, b.Name), cmp.Compare(a.ID, b.ID))
	})
	return out
}
