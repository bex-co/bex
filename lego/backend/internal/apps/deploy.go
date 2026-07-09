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
	"fmt"

	"sigs.k8s.io/yaml"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// deploy.go is the deploy-from-chat mapper (pillar 4): it turns a repo + a
// render.yaml-shaped bex.yml into a CreateRequest and rides Core.Create, so
// "deploy this" is the same create verb every other client uses — no bespoke
// deploy endpoint (t001 amended 2026-07-08). The field mapping mirrors
// scripts/app-apply.sh, the reference bex.yml → App CR projection.

// DeployRequest is the deploy-from-chat input: a git repo plus its bex.yml
// (render.yaml-shaped manifest). Repo/Branch, when set, override the manifest —
// an agent that already knows the checkout it is deploying need not duplicate it
// in the file.
type DeployRequest struct {
	Repo     string
	Branch   string
	Manifest string
}

// bexManifest is the render.yaml-shaped bex.yml: a list of apps. Deploy-from-chat
// deploys the first entry (a repo maps to one service); the field names match
// scripts/app-apply.sh so a manifest that applies with the script deploys here.
type bexManifest struct {
	Apps []bexApp `json:"apps"`
}

type bexApp struct {
	Name            string      `json:"name"`
	Repo            string      `json:"repo"`
	Image           string      `json:"image"`
	Branch          string      `json:"branch"`
	Builder         string      `json:"builder"`
	Port            int32       `json:"port"`
	Replicas        int32       `json:"replicas"`
	Tier            string      `json:"tier"`
	HealthCheckPath string      `json:"healthCheckPath"`
	Type            string      `json:"type"` // "web" (default, public) | "private"
	Domains         []string    `json:"domains"`
	EnvVars         []bexEnvVar `json:"envVars"`
}

type bexEnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Deploy maps a repo + bex.yml onto Create — one agent call takes code to a
// live URL through the Render-shaped create surface. Repeating it for the same
// service redeploys (Create's update-in-place), not a duplicate.
func (s *Service) Deploy(ctx context.Context, req DeployRequest) (AppView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return AppView{}, err
	}
	cr, err := createFromManifest(req)
	if err != nil {
		return AppView{}, err
	}
	return s.create(ctx, cr)
}

// createFromManifest parses the bex.yml and projects its first app (with the
// request's repo/branch overrides applied) onto a CreateRequest.
func createFromManifest(req DeployRequest) (CreateRequest, error) {
	var m bexManifest
	if err := yaml.Unmarshal([]byte(req.Manifest), &m); err != nil {
		return CreateRequest{}, fmt.Errorf("%w: bex.yml is not valid YAML: %v", core.ErrBadRequest, err)
	}
	if len(m.Apps) == 0 {
		return CreateRequest{}, fmt.Errorf("%w: bex.yml must define at least one app under apps[]", core.ErrBadRequest)
	}
	a := m.Apps[0]

	repo := a.Repo
	if req.Repo != "" {
		repo = req.Repo // the explicit deploy target wins over the manifest
	}
	branch := a.Branch
	if req.Branch != "" {
		branch = req.Branch
	}

	// type decides exposure, render.yaml-style: web (default) is public;
	// private is in-cluster only and cannot carry domains (no ingress).
	private := a.Type == "private"
	if private && len(a.Domains) > 0 {
		return CreateRequest{}, fmt.Errorf("%w: a private service has no ingress and cannot list domains", core.ErrBadRequest)
	}

	env := make([]appv1alpha1.EnvVar, 0, len(a.EnvVars))
	for _, e := range a.EnvVars {
		env = append(env, appv1alpha1.EnvVar{Name: e.Key, Value: e.Value})
	}
	if len(env) == 0 {
		env = nil
	}

	return CreateRequest{
		Name:            a.Name,
		Repo:            repo,
		Image:           a.Image,
		Branch:          branch,
		Builder:         a.Builder,
		Port:            a.Port,
		Replicas:        a.Replicas,
		Plan:            a.Tier,
		HealthCheckPath: a.HealthCheckPath,
		Env:             env,
		Hosts:           a.Domains,
		Private:         private,
	}, nil
}
