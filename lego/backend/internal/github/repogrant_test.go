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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestRepoGrantedNeverDivergesFromCloneToken is w6/m99's anti-drift guard. The
// product now REPORTS push-deliverability from RepoGranted while a deploy still
// ACTS on cloneToken; if those two ever answered differently, the dashboard
// would be lying again — the exact failure this milestone exists to close. Both
// delegate to repoGrant, and this pins that they agree over every branch that
// decides the verdict, URL normalization included.
func TestRepoGrantedNeverDivergesFromCloneToken(t *testing.T) {
	st := newFakeStore()
	st.conns = append(st.conns, store.GitConnection{WorkspaceID: "default", InstallationID: 7, AccountLogin: "octo"})
	granted := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{token: "ghs_fresh", repoOK: true}, Store: st}
	ungranted := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{token: "x", repoOK: false}, Store: st}
	off := &Service{Base: &core.Base{Namespace: "default"}}
	noConn := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{token: "x", repoOK: true}, Store: newFakeStore()}
	failCheck := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{token: "x", repoErr: &APIError{Status: 500}}, Store: st}

	for _, c := range []struct {
		name    string
		svc     *Service
		repo    string
		want    bool
		wantErr bool
	}{
		// URL normalization: every spelling of the SAME granted repo must land
		// on the same verdict, so a service stored with a `.git` suffix or the
		// scp form isn't told its pushes need a manual webhook.
		{name: "granted-https", svc: granted, repo: "https://github.com/octo/app", want: true},
		{name: "granted-dot-git", svc: granted, repo: "https://github.com/octo/app.git", want: true},
		{name: "granted-trailing-slash", svc: granted, repo: "https://github.com/octo/app/", want: true},
		{name: "granted-scp-form", svc: granted, repo: "git@github.com:octo/app.git", want: true},
		{name: "granted-mixed-case-owner", svc: granted, repo: "https://github.com/Octo/App", want: true},
		// The live-found case: a github.com repo under a connected account that
		// the installation does not grant.
		{name: "ungranted-repo", svc: ungranted, repo: "https://github.com/octo/app", want: false},
		// Owner outside every connection, another git host, GitHub off.
		{name: "unconnected-owner", svc: granted, repo: "https://github.com/stranger/repo", want: false},
		{name: "other-git-host", svc: granted, repo: "https://gitlab.com/octo/app", want: false},
		{name: "github-off", svc: off, repo: "https://github.com/octo/app", want: false},
		{name: "no-connection-at-all", svc: noConn, repo: "https://github.com/octo/app", want: false},
		// A GitHub failure must surface as an error on BOTH, so the read path
		// can report "unknown" instead of a false negative.
		{name: "grant-check-failure", svc: failCheck, repo: "https://github.com/octo/app", wantErr: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			_, tokenOK, tokenErr := c.svc.cloneToken(ctx, "default", c.repo)
			readOK, readErr := c.svc.DeployTokenSource().RepoGranted(ctx, "default", c.repo)
			if (tokenErr != nil) != (readErr != nil) {
				t.Fatalf("cloneToken err=%v but RepoGranted err=%v — the two paths disagree", tokenErr, readErr)
			}
			if c.wantErr {
				if readErr == nil {
					t.Fatalf("RepoGranted(%q) = %v, want an error", c.repo, readOK)
				}
				return
			}
			if readErr != nil {
				t.Fatalf("RepoGranted(%q): %v", c.repo, readErr)
			}
			if readOK != tokenOK {
				t.Errorf("RepoGranted(%q) = %v but cloneToken ok = %v", c.repo, readOK, tokenOK)
			}
			if readOK != c.want {
				t.Errorf("RepoGranted(%q) = %v, want %v", c.repo, readOK, c.want)
			}
		})
	}
}

// TestRepoGrantedUnionsEveryInstallation covers the milestone README's
// "Unverified this run" item: a workspace may hold several GitHub App
// installations (ADR078 §4), and deliverability must be answered against the
// union of their grants — resolving each repo through ITS OWN owner's
// connection, never only the first/active one.
func TestRepoGrantedUnionsEveryInstallation(t *testing.T) {
	fc := &fakeClient{repoOK: true, tokenByInst: map[int64]string{7: "tok-octo", 9: "tok-personal"}}
	svc := multiConnSvc(t, fc)
	src := svc.DeployTokenSource()
	ctx := context.Background()

	for _, repo := range []string{"https://github.com/octo/app", "https://github.com/personal/site"} {
		granted, err := src.RepoGranted(ctx, core.DefaultTenant, repo)
		if err != nil || !granted {
			t.Errorf("RepoGranted(%q) = %v,%v; want true,nil — each installation's grant counts", repo, granted, err)
		}
	}
	// An owner none of the installations covers stays false, so the union is a
	// union and not an "any connection exists" shortcut — the workspace-level
	// check that produced this milestone's bug in the first place.
	if granted, err := src.RepoGranted(ctx, core.DefaultTenant, "https://github.com/stranger/repo"); err != nil || granted {
		t.Errorf("RepoGranted(stranger) = %v,%v; want false,nil", granted, err)
	}
}
