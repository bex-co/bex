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
	"encoding/json"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// patchServiceRequest is the subset of Render's PATCH /v1/services/{id} body
// bex honors: a plan change nested under serviceDetails, a root-directory change,
// and Render's top-level autoDeploy toggle. PATCH is partial — fields this bex
// doesn't support (name, ...) are accepted and silently ignored rather than
// rejected, a safe superset; omitting a field leaves it unchanged.
type patchServiceRequest struct {
	ServiceDetails *struct {
		Plan string `json:"plan"`
		// IdleTTLSeconds is a bex extra (Render has no idle-timeout field) — the
		// free-tier auto-sleep window. A pointer so "absent" (leave unchanged) is
		// distinct from an explicit 0 (restore the controller default).
		IdleTTLSeconds *int32 `json:"idleTTLSeconds"`
	} `json:"serviceDetails"`
	// RootDir is a pointer so "absent" (leave unchanged) is distinct from an
	// explicit "" (restore the repo root) — Render's Root Directory setting,
	// the Settings → Build & Deploy save flow (w5/m13).
	RootDir *string `json:"rootDir"`
	// AutoDeploy is Render's "yes"/"no" (or bool-ish) toggle for push-to-deploy
	// (spec.autoDeploy). "" => absent (leave unchanged); parseYesNo maps the rest
	// to a tri-state so the Settings → Build & Deploy toggle can flip it (w2/m9).
	AutoDeploy string `json:"autoDeploy"`
}

// scaleRequest is Render's POST /v1/services/{id}/scale body: the desired
// running instance count. numInstances < 1 or > 100 is core.ErrBadRequest.
type scaleRequest struct {
	NumInstances int32 `json:"numInstances"`
}

// createServiceRequest is the POST /v1/services body — shaped to Render's create
// schema (verified against its public API: top-level name/repo/branch/image
// object/envVars, plan + numInstances + healthCheckPath nested under
// serviceDetails, type used to pick private_service). bex reads the Render
// fields it can honor and adds a few extensions; the Render fields it can't yet
// honor (ownerId, region, autoDeploy, runtime build/start commands) are ignored,
// a safe superset. One of repo/image is required.
type createServiceRequest struct {
	// Render fields.
	Type           string             `json:"type"`     // web_service (default) | private_service | background_worker | cron_job
	Schedule       string             `json:"schedule"` // cron expression, required when type is cron_job
	Command        string             `json:"command"`  // overrides the image's entrypoint for a cron_job; empty runs its own command
	Name           string             `json:"name"`
	Repo           string             `json:"repo"`
	Image          *imageRef          `json:"image"` // prebuilt image: Render nests the path in an object
	Branch         string             `json:"branch"`
	AutoDeploy     string             `json:"autoDeploy"` // Render's "yes"|"no"; "" => default
	EnvVars        []keyValue         `json:"envVars"`
	ServiceDetails *serviceDetailsReq `json:"serviceDetails"`
	// bex extensions (no Render create-body equivalent): the build strategy, the
	// listen port (Render auto-detects it; bex's App CR needs it explicitly),
	// custom domains in one call, and a top-level plan convenience.
	Builder string `json:"builder"`
	// RootDir scopes build-from-git to a subdirectory of Repo, mirroring
	// Render's Root Directory setting (monorepo support). Empty is the repo root.
	RootDir string   `json:"rootDir"`
	Port    int32    `json:"port"`
	Plan    string   `json:"plan"`
	Domains []string `json:"domains"`
}

type keyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// imageRef is Render's prebuilt-image object: the image path lives under
// `image.imagePath`, not a bare top-level string (ownerId/registry credential
// are Render-registry concerns bex doesn't model, so they're ignored).
type imageRef struct {
	ImagePath string `json:"imagePath"`
}

// serviceDetailsReq is where Render nests plan, numInstances and healthCheckPath
// on create — the same location PATCH and GET report them. schedule is Render's
// cronJobDetails.schedule (accepted here or at the top level, top level wins).
type serviceDetailsReq struct {
	Plan            string `json:"plan"`
	NumInstances    int32  `json:"numInstances"`
	HealthCheckPath string `json:"healthCheckPath"`
	Schedule        string `json:"schedule"`
	Command         string `json:"command"`
}

// toCreateRequest folds the Render-nested and bex top-level fields into the
// neutral CreateRequest. serviceDetails is Render's canonical location for
// plan/numInstances/healthCheckPath; the top-level plan is a bex convenience
// fallback. type:private_service maps to the in-cluster-only flag.
func (r createServiceRequest) toCreateRequest() CreateRequest {
	plan, health, schedule, command := r.Plan, "", r.Schedule, r.Command
	var replicas int32
	if r.ServiceDetails != nil {
		if r.ServiceDetails.Plan != "" {
			plan = r.ServiceDetails.Plan
		}
		health = r.ServiceDetails.HealthCheckPath
		replicas = r.ServiceDetails.NumInstances
		if schedule == "" {
			schedule = r.ServiceDetails.Schedule // top-level schedule wins over the nested one
		}
		if command == "" {
			command = r.ServiceDetails.Command // top-level command wins over the nested one
		}
	}
	image := ""
	if r.Image != nil {
		image = r.Image.ImagePath
	}
	var env []appv1alpha1.EnvVar
	for _, e := range r.EnvVars {
		env = append(env, appv1alpha1.EnvVar{Name: e.Key, Value: e.Value})
	}
	return CreateRequest{
		Name:            r.Name,
		Type:            r.Type,
		Schedule:        schedule,
		Command:         command,
		Repo:            r.Repo,
		Image:           image,
		Branch:          r.Branch,
		Builder:         r.Builder,
		RootDir:         r.RootDir,
		Port:            r.Port,
		Replicas:        replicas,
		Plan:            plan,
		HealthCheckPath: health,
		Env:             env,
		Hosts:           r.Domains,
		AutoDeploy:      parseYesNo(r.AutoDeploy),
	}
}

// parseYesNo maps Render's autoDeploy enum ("yes"/"no", or the bool-ish
// "true"/"false") to a tri-state *bool; "" => nil (use the platform default).
func parseYesNo(s string) *bool {
	switch s {
	case "yes", "true":
		t := true
		return &t
	case "no", "false":
		f := false
		return &f
	default:
		return nil
	}
}

// RegisterREST mounts the App-lifecycle routes — Render-public-API compatible.
// Paths, the {service, cursor} list envelope, the string suspended enum, and the
// verb status codes (suspend/resume 202, restart 200) all match Render's OpenAPI
// spec. Served at both /v1/services (Render's noun) and /v1/apps (bex's noun);
// it holds no logic beyond routing + Render serialization.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	list := func(w http.ResponseWriter, r *http.Request) {
		apps, err := s.List(r.Context(), r.URL.Query().Get("ownerId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toServiceList(apps)) // [{service, cursor}, ...]
	}
	get := func(w http.ResponseWriter, r *http.Request) {
		app, err := s.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderService(app))
	}
	// verb maps a Service action to a handler with a Render-accurate status code.
	verb := func(status int, fn func(context.Context, string) (AppView, error)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			app, err := fn(r.Context(), r.PathValue("id"))
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			core.WriteJSON(w, status, toRenderService(app))
		}
	}
	// patch handles PATCH /v1/services/{id} — a plan change (serviceDetails.plan),
	// an idle-timeout change (serviceDetails.idleTTLSeconds), and/or a root
	// directory change (rootDir); an unknown plan or a rootDir on an image-backed
	// App is core.ErrBadRequest => 400.
	patch := func(w http.ResponseWriter, r *http.Request) {
		var req patchServiceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		id := r.PathValue("id")
		var plan string
		var idleTTL *int32
		if req.ServiceDetails != nil {
			plan, idleTTL = req.ServiceDetails.Plan, req.ServiceDetails.IdleTTLSeconds
		}
		autoDeploy := parseYesNo(req.AutoDeploy) // nil => not provided (don't change)
		if plan == "" && idleTTL == nil && req.RootDir == nil && autoDeploy == nil {
			get(w, r) // no supported field present => read-only no-op
			return
		}
		// Apply the supported fields in turn; the no-op guard above guarantees at
		// least one runs, so app is always set by the time we serialize.
		var app AppView
		var err error
		if plan != "" {
			if app, err = s.SetPlan(r.Context(), id, plan); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		if idleTTL != nil {
			if app, err = s.SetIdleTTL(r.Context(), id, *idleTTL); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		if req.RootDir != nil {
			if app, err = s.SetRootDir(r.Context(), id, *req.RootDir); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		if autoDeploy != nil {
			if app, err = s.SetAutoDeploy(r.Context(), id, *autoDeploy); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		core.WriteJSON(w, http.StatusOK, toRenderService(app))
	}
	// scale handles POST /v1/services/{id}/scale — sets the running instance
	// count (numInstances); out-of-range is core.ErrBadRequest => 400.
	scale := func(w http.ResponseWriter, r *http.Request) {
		var req scaleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		app, err := s.Scale(r.Context(), r.PathValue("id"), req.NumInstances)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusAccepted, toRenderService(app)) // Render: scale => 202
	}

	// create handles POST /v1/services — create-or-update a service from a
	// Render-shaped body; deploy-from-chat rides this with a repo (no bespoke
	// deploy endpoint). Render returns 201 Created on success.
	create := func(w http.ResponseWriter, r *http.Request) {
		var req createServiceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		app, err := s.Create(r.Context(), req.toCreateRequest())
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, toRenderService(app)) // Render: create => 201
	}

	// deleteSvc handles DELETE /v1/services/{id} — remove the service and let the
	// operator's ownerRefs cascade its derived resources. Render returns 204 No
	// Content with an empty body; unknown id => core.ErrNotFound => 404.
	deleteSvc := func(w http.ResponseWriter, r *http.Request) {
		if err := s.Delete(r.Context(), r.PathValue("id")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent) // Render: delete => 204, empty body
	}

	// runCron handles the cron run trigger (Render's POST /cron-jobs/{id}/runs):
	// bump spec.runAt so the operator materializes a one-off Job. Render returns a
	// cronJobRun; bex returns the updated service (its status.runs gains the run
	// once the operator reconciles). 201 Created, matching create-style verbs.
	runCron := func(w http.ResponseWriter, r *http.Request) {
		app, err := s.TriggerCronRun(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, toRenderService(app))
	}
	// Render's canonical path is /cron-jobs/{id}/runs; bex also accepts it under the
	// /v1/services and /v1/apps nouns (registered in the base loop below).
	mux.HandleFunc("POST /v1/cron-jobs/{id}/runs", runCron)

	// Custom-domains sub-resource (Render-compatible).
	listDomains := func(w http.ResponseWriter, r *http.Request) {
		domains, err := s.ListDomains(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toCustomDomainList(domains))
	}
	addDomain := func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		d, err := s.AddDomain(r.Context(), r.PathValue("id"), req.Name)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, toRenderCustomDomain(d))
	}
	getDomain := func(w http.ResponseWriter, r *http.Request) {
		d, err := s.GetDomain(r.Context(), r.PathValue("id"), r.PathValue("name"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderCustomDomain(d))
	}
	deleteDomain := func(w http.ResponseWriter, r *http.Request) {
		if err := s.DeleteDomain(r.Context(), r.PathValue("id"), r.PathValue("name")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
	// verifyDomain re-checks DNS/cert state now (Render's POST …/verify) and returns
	// the fresh domain. 200 OK — bex verification is automatic, so this is a re-read.
	verifyDomain := func(w http.ResponseWriter, r *http.Request) {
		d, err := s.VerifyDomain(r.Context(), r.PathValue("id"), r.PathValue("name"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderCustomDomain(d))
	}

	// Autoscaling sub-resource (Render-compatible).
	// GET   …/autoscaling — current config (bex extension; Render has no GET)
	// PUT   …/autoscaling — upsert autoscaling (Render: PUT, 200)
	// DELETE …/autoscaling — disable autoscaling (Render: DELETE, 204)
	getAutoscaling := func(w http.ResponseWriter, r *http.Request) {
		av, err := s.GetAutoscaling(r.Context(), r.PathValue("id"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, av)
	}
	putAutoscaling := func(w http.ResponseWriter, r *http.Request) {
		var req SetAutoscalingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		av, err := s.SetAutoscaling(r.Context(), r.PathValue("id"), req)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, av)
	}
	deleteAutoscaling := func(w http.ResponseWriter, r *http.Request) {
		if err := s.DeleteAutoscaling(r.Context(), r.PathValue("id")); err != nil {
			core.WriteErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}

	// Register the same handlers under Render's /v1/services and bex's /v1/apps.
	for _, base := range []string{"/v1/services", "/v1/apps"} {
		mux.HandleFunc("GET "+base, list)
		mux.HandleFunc("POST "+base, create) // Render: create => 201
		mux.HandleFunc("GET "+base+"/{id}", get)
		mux.HandleFunc("PATCH "+base+"/{id}", patch)
		mux.HandleFunc("DELETE "+base+"/{id}", deleteSvc) // Render: delete => 204
		mux.HandleFunc("POST "+base+"/{id}/suspend", verb(http.StatusAccepted, s.Suspend))
		mux.HandleFunc("POST "+base+"/{id}/resume", verb(http.StatusAccepted, s.Resume))
		mux.HandleFunc("POST "+base+"/{id}/restart", verb(http.StatusOK, s.Restart)) // Render: restart => 200
		mux.HandleFunc("POST "+base+"/{id}/scale", scale)                            // Render: scale => 202
		mux.HandleFunc("POST "+base+"/{id}/runs", runCron)                           // cron run trigger (bex noun)
		mux.HandleFunc("GET "+base+"/{id}/autoscaling", getAutoscaling)
		mux.HandleFunc("PUT "+base+"/{id}/autoscaling", putAutoscaling)
		mux.HandleFunc("DELETE "+base+"/{id}/autoscaling", deleteAutoscaling)
		mux.HandleFunc("GET "+base+"/{id}/custom-domains", listDomains)
		mux.HandleFunc("POST "+base+"/{id}/custom-domains", addDomain)
		mux.HandleFunc("GET "+base+"/{id}/custom-domains/{name}", getDomain)
		mux.HandleFunc("DELETE "+base+"/{id}/custom-domains/{name}", deleteDomain)
		mux.HandleFunc("POST "+base+"/{id}/custom-domains/{name}/verify", verifyDomain)
	}
}
