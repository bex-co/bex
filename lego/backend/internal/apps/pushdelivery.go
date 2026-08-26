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
	"context"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// pushdelivery.go answers a question `autoDeploy` cannot: the stored boolean is
// the user's on/off SETTING, while this is whether a push can be DELIVERED to
// bex at all for this service's specific repo (w6/m99). Reporting only the
// setting made every surface — dashboard hint, REST, GraphQL, MCP — claim "a
// push to the tracked branch redeploys automatically via the GitHub app" for a
// github.com repo the workspace's GitHub App installation does not cover, where
// GitHub never sends a push event and no deploy can ever fire.
//
// A bex extension with no Render counterpart: Render can only create a service
// from a repo its connection already covers, so the state this field describes
// is unreachable there (docs/ADR026-github-integration.md, ADR018 parity notes).
const (
	// PushDeliveryGitHubApp: this repo belongs to the workspace's GitHub App
	// installation, so a push delivers an app-signed webhook with no per-repo
	// setup — the mechanism the "via the GitHub app" copy describes.
	PushDeliveryGitHubApp = "github_app"
	// PushDeliveryManualWebhook: the service builds from a repo the GitHub App
	// cannot deliver for (a non-github.com host, no connection for the repo's
	// account, or a github.com repo outside the installation's grant). A push
	// redeploys only through a manually configured webhook.
	PushDeliveryManualWebhook = "manual_webhook"
	// PushDeliveryNone: no repo at all (image-backed) — push-to-deploy does not
	// apply. Not "no": there is no push to deliver.
	PushDeliveryNone = "none"
	// PushDeliveryUnknown: the grant could not be determined (a GitHub failure).
	// Reported honestly rather than guessed in either direction — a wrong
	// "github_app" is the bug this file exists to fix, and a wrong
	// "manual_webhook" would send a correctly-connected user off to build a
	// webhook they don't need. Never fails the read.
	PushDeliveryUnknown = "unknown"
)

const (
	// pushDeliveryTTL bounds how stale a cached grant answer may be. The check
	// costs GitHub round-trips (mint an installation token, ask whether it can
	// see the repo), and the dashboard's service-detail page reads the service
	// repeatedly, so the answer is memoized rather than re-asked per read. A
	// minute of staleness is a display concern only: the live check still runs
	// on every deploy trigger (mintCloneSecret), which is what actually decides
	// whether a private clone gets a token.
	pushDeliveryTTL = time.Minute
	// pushDeliveryUnknownTTL keeps a GitHub outage from costing every read its
	// full timeout, without pinning "unknown" for a whole minute after recovery.
	pushDeliveryUnknownTTL = 5 * time.Second
	// pushDeliveryTimeout bounds one grant lookup. A slow GitHub must not hold a
	// service read open; the answer degrades to unknown instead.
	pushDeliveryTimeout = 3 * time.Second
)

// pushDeliveryMemo returns the per-(workspace, repo) answer cache, built on
// first use so every existing Service literal (tests included) keeps working
// without construction ceremony. core.TTLCache is the module's shared
// positive-cache primitive — same expiry-on-read and CacheMax sweep discipline
// the auth gate and OpenFGA checker use, and it is paired with a
// singleflight.Group here for the same reason they pair it: without coalescing,
// every concurrent reader of one cold key fires its own GitHub round-trip.
func (s *Service) pushDeliveryMemo() *core.TTLCache[string] {
	s.pushDeliveryOnce.Do(func() { s.pushDelivery = core.NewTTLCache[string]() })
	return s.pushDelivery
}

// pushDeliveryMethod reports how a git push to this App's tracked branch can
// reach bex. It is computed on READ, never snapshotted at create time: a
// workspace's installation grant is edited on GitHub's side at any moment, and
// a stale snapshot would only make this milestone's bug harder to see.
//
// The grant test is CloneTokenSource.RepoGranted — literally the call
// mintCloneSecret makes on every deploy trigger, with the token dropped — not a
// second grant-check code path free to drift from what a deploy really does.
func (s *Service) pushDeliveryMethod(ctx context.Context, a *appv1alpha1.App) string {
	repo := a.Spec.Repo
	switch {
	case repo == "":
		return PushDeliveryNone
	case s.GitHub == nil:
		// The GitHub App isn't wired on this install, so it can never be the
		// delivery mechanism — but the shared manual webhook still redeploys, so
		// this is manual_webhook, not "not applicable". Matches what the
		// dashboard already (correctly) shows for a workspace with no connection.
		return PushDeliveryManualWebhook
	}
	workspaceID := s.AppWorkspace(ctx, a)
	// "\n" cannot occur in a workspace id, so the two fields can't run together
	// into one key two different pairs share.
	key := workspaceID + "\n" + repo
	memo := s.pushDeliveryMemo()
	if method, ok := memo.Get(key); ok {
		return method
	}
	// Coalesce concurrent misses for the same workspace+repo into one lookup —
	// the TTLCache+singleflight pairing the auth gate and OpenFGA checker use.
	// Waiters share the leader's answer, including an "unknown" the leader
	// reached because ITS caller went away; that self-corrects in
	// pushDeliveryUnknownTTL and is cheaper than every waiter re-asking GitHub.
	answer, _, _ := s.pushDeliveryFlight.Do(key, func() (any, error) {
		lookup, cancel := context.WithTimeout(ctx, pushDeliveryTimeout)
		defer cancel()
		granted, err := s.GitHub.RepoGranted(lookup, workspaceID, repo)
		method, ttl := PushDeliveryManualWebhook, pushDeliveryTTL
		switch {
		case err != nil:
			method, ttl = PushDeliveryUnknown, pushDeliveryUnknownTTL
		case granted:
			method = PushDeliveryGitHubApp
		}
		memo.Put(key, method, time.Now().Add(ttl))
		return method, nil
	})
	method, ok := answer.(string)
	if !ok {
		return PushDeliveryUnknown
	}
	return method
}
