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

package deploys

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// PlatformRebuildPath is the fleet rebuild verb on bex-api's cluster-internal
// :8091 listener, behind the control-plane bearer (BEX_CP_TOKEN). It ships a
// re-pinned runtime (a toolchain security patch such as CVE-2026-111497) to
// services that are already running: a running container keeps the toolchain
// it was built on until something rebuilds it. Never mounted on :8090.
const PlatformRebuildPath = "/internal/platform-rebuild"

// rebuildTriggers open deploys that build the service from source. A restart,
// rollback or configuration rollout reuses an existing artifact, so it does
// not carry a re-pinned toolchain.
var rebuildTriggers = append([]string{store.TriggerCreate}, sourceOnlyTriggers...)

// platformRebuildCallTimeout bounds one service's trigger (GitHub token mint +
// commit resolve + CR patch) so one stuck call cannot use up the batch.
const platformRebuildCallTimeout = 30 * time.Second

// platformRebuildWriteTimeout covers the largest batch the CLI allows to wait
// for (its own 10-minute client timeout).
const platformRebuildWriteTimeout = 10 * time.Minute

// PlatformRebuildRequest selects one batch. Since is required: a service that
// has already been rebuilt from source after it (the moment the patched
// operator finished rolling out) is done, so re-running the same request
// continues where the last batch stopped instead of rebuilding twice.
type PlatformRebuildRequest struct {
	Since time.Time `json:"since"`
	// Apply opens the deploys; false only reports the plan.
	Apply bool `json:"apply"`
	// Limit bounds how many deploys one applied batch opens (build admission
	// queues the rest anyway, but a batch keeps the blast radius reviewable).
	Limit int `json:"limit"`
}

// PlatformRebuildItem is one service's line in the plan or result.
type PlatformRebuildItem struct {
	Workspace string `json:"workspace"`
	Service   string `json:"service"`
	Namespace string `json:"namespace"`
	Type      string `json:"type,omitempty"`
	Runtime   string `json:"runtime,omitempty"`
	Builder   string `json:"builder,omitempty"`
	// Deploy is the opened dep-… id (applied batches only).
	Deploy string `json:"deploy,omitempty"`
	Commit string `json:"commit,omitempty"`
	Error  string `json:"error,omitempty"`
}

// PlatformRebuildResult reports the batch. Planned lists the eligible
// services left after it; Done counts those already rebuilt since Since.
type PlatformRebuildResult struct {
	Applied   bool                  `json:"applied"`
	Done      int                   `json:"done"`
	Triggered []PlatformRebuildItem `json:"triggered"`
	Planned   []PlatformRebuildItem `json:"planned"`
	Failed    []PlatformRebuildItem `json:"failed"`
}

// platformRebuildEligible reports whether a's running containers were built on
// a bex-pinned toolchain that a rebuild would replace: a native runtime or
// buildpacks, never the repo's own Dockerfile (whose base image is the
// tenant's). A static site only serves files its build produced, so its
// toolchain never handles a request.
func platformRebuildEligible(a *appv1alpha1.App) bool {
	if a.Spec.Repo == "" || a.Spec.Suspended || a.DeletionTimestamp != nil || appStoreID(a) == "" {
		return false
	}
	return a.Spec.Type != appv1alpha1.TypeStaticSite && a.Spec.Runtime != "image" && !core.IsDockerfileBuild(a.Spec)
}

// PlatformRebuild rebuilds every eligible service at its live commit, at most
// req.Limit per call, as a deployed_by_render deploy. It is the platform's own
// action, so it carries no tenant authorization and no billing gate: the
// caller is the cluster-internal bearer, never a workspace member.
func (s *Service) PlatformRebuild(ctx context.Context, req PlatformRebuildRequest) (PlatformRebuildResult, error) {
	res := PlatformRebuildResult{Applied: req.Apply, Triggered: []PlatformRebuildItem{}, Planned: []PlatformRebuildItem{}, Failed: []PlatformRebuildItem{}}
	if s.Store == nil {
		return res, core.ErrDeploysUnavailable
	}
	if req.Since.IsZero() {
		return res, fmt.Errorf("%w: since is required (when the patched toolchain finished rolling out)", core.ErrBadRequest)
	}
	var list appv1alpha1.AppList
	if err := s.Client.List(ctx, &list, client.HasLabels{store.LabelAppID}); err != nil {
		return res, err
	}
	apps := list.Items
	sort.Slice(apps, func(i, j int) bool {
		if apps[i].Namespace != apps[j].Namespace {
			return apps[i].Namespace < apps[j].Namespace
		}
		return apps[i].Name < apps[j].Name
	})
	for i := range apps {
		a := &apps[i]
		if !platformRebuildEligible(a) {
			continue
		}
		item := PlatformRebuildItem{
			Workspace: a.Labels[core.LabelTenant], Service: core.AppPublicName(a), Namespace: a.Namespace,
			Type: a.Spec.Type, Runtime: a.Spec.Runtime, Builder: a.Spec.Builder,
		}
		rebuilt, err := s.rebuiltSince(ctx, a, req.Since)
		switch {
		case err != nil:
			item.Error = err.Error()
			res.Failed = append(res.Failed, item)
		case rebuilt:
			res.Done++
		case !req.Apply || (req.Limit > 0 && len(res.Triggered) >= req.Limit):
			res.Planned = append(res.Planned, item)
		default:
			if d, err := s.platformRebuildOne(ctx, item.Service, a); err != nil {
				item.Error = err.Error()
				res.Failed = append(res.Failed, item)
			} else {
				item.Deploy, item.Commit = d.ID, d.CommitID
				res.Triggered = append(res.Triggered, item)
			}
		}
	}
	return res, nil
}

// platformRebuildOne opens a's rebuild at its live commit.
func (s *Service) platformRebuildOne(ctx context.Context, service string, a *appv1alpha1.App) (DeployView, error) {
	ctx, cancel := context.WithTimeout(ctx, platformRebuildCallTimeout)
	defer cancel()
	commit, err := s.liveCommit(ctx, service, a)
	if err != nil {
		return DeployView{}, err
	}
	return s.triggerFetched(ctx, service, a, TriggerParams{CommitID: commit, restart: true}, store.TriggerDeployedByRender)
}

// rebuiltSince reports whether a has a deploy opened after since that builds
// from source and has not failed: live, or still on its way there.
func (s *Service) rebuiltSince(ctx context.Context, a *appv1alpha1.App, since time.Time) (bool, error) {
	statuses := append([]string{store.DeployLive}, store.OpenDeployStatuses()...)
	rows, err := s.Store.ListDeploys(ctx, appStoreID(a), store.DeployFilter{CreatedAfter: since, Statuses: statuses})
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(rows, func(d store.Deploy) bool { return slices.Contains(rebuildTriggers, d.Trigger) }), nil
}

// PlatformRebuildHandler serves POST PlatformRebuildPath. Token is the
// control-plane bearer; an empty token never authorizes. An applied batch runs
// past the :8091 server's 30s WriteTimeout, so the handler extends its own
// deadline rather than lose the report of deploys it already opened.
func PlatformRebuildHandler(s *Service, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Constant-time, like the control-plane API's own bearer (store.API).
		want := []byte("Bearer " + token)
		if token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			core.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(platformRebuildWriteTimeout))
		r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
		req, err := core.DecodeBody[PlatformRebuildRequest](r)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		res, err := s.PlatformRebuild(r.Context(), req)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, res)
	})
}
