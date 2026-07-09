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
// bex honors: a plan change nested under serviceDetails, matching where GET
// reports it back. PATCH is partial — fields this bex doesn't support (name,
// autoDeploy, ...) are accepted and silently ignored rather than rejected, a
// safe superset like the rest of the Render surface; omitting serviceDetails
// or plan leaves the App's plan unchanged.
type patchServiceRequest struct {
	ServiceDetails *struct {
		Plan string `json:"plan"`
	} `json:"serviceDetails"`
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
	Type           string             `json:"type"` // web_service (default) | private_service
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
	Builder string   `json:"builder"`
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
// on create — the same location PATCH and GET report them.
type serviceDetailsReq struct {
	Plan            string `json:"plan"`
	NumInstances    int32  `json:"numInstances"`
	HealthCheckPath string `json:"healthCheckPath"`
}

// toCreateRequest folds the Render-nested and bex top-level fields into the
// neutral CreateRequest. serviceDetails is Render's canonical location for
// plan/numInstances/healthCheckPath; the top-level plan is a bex convenience
// fallback. type:private_service maps to the in-cluster-only flag.
func (r createServiceRequest) toCreateRequest() CreateRequest {
	plan, health := r.Plan, ""
	var replicas int32
	if r.ServiceDetails != nil {
		if r.ServiceDetails.Plan != "" {
			plan = r.ServiceDetails.Plan
		}
		health = r.ServiceDetails.HealthCheckPath
		replicas = r.ServiceDetails.NumInstances
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
		Repo:            r.Repo,
		Image:           image,
		Branch:          r.Branch,
		Builder:         r.Builder,
		Port:            r.Port,
		Replicas:        replicas,
		Plan:            plan,
		HealthCheckPath: health,
		Env:             env,
		Hosts:           r.Domains,
		Private:         r.Type == "private_service",
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
		apps, err := s.List(r.Context())
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
	// patch handles PATCH /v1/services/{id} — today, only a plan change
	// (serviceDetails.plan); an unknown plan is core.ErrBadRequest => 400.
	patch := func(w http.ResponseWriter, r *http.Request) {
		var req patchServiceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		if req.ServiceDetails == nil || req.ServiceDetails.Plan == "" {
			get(w, r) // no supported field present => read-only no-op
			return
		}
		app, err := s.SetPlan(r.Context(), r.PathValue("id"), req.ServiceDetails.Plan)
		if err != nil {
			core.WriteErr(w, err)
			return
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

	// Register the same handlers under Render's /v1/services and bex's /v1/apps.
	for _, base := range []string{"/v1/services", "/v1/apps"} {
		mux.HandleFunc("GET "+base, list)
		mux.HandleFunc("POST "+base, create) // Render: create => 201
		mux.HandleFunc("GET "+base+"/{id}", get)
		mux.HandleFunc("PATCH "+base+"/{id}", patch)
		mux.HandleFunc("POST "+base+"/{id}/suspend", verb(http.StatusAccepted, s.Suspend))
		mux.HandleFunc("POST "+base+"/{id}/resume", verb(http.StatusAccepted, s.Resume))
		mux.HandleFunc("POST "+base+"/{id}/restart", verb(http.StatusOK, s.Restart)) // Render: restart => 200
		mux.HandleFunc("POST "+base+"/{id}/scale", scale)                            // Render: scale => 202
		mux.HandleFunc("GET "+base+"/{id}/custom-domains", listDomains)
		mux.HandleFunc("POST "+base+"/{id}/custom-domains", addDomain)
		mux.HandleFunc("GET "+base+"/{id}/custom-domains/{name}", getDomain)
		mux.HandleFunc("DELETE "+base+"/{id}/custom-domains/{name}", deleteDomain)
	}
}
