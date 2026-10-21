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

// Package renderenv is Render's default environment variables
// (render.com/docs/environment-variables) as bex computes them for an App
// (w5/m148; ADR018). The operator injects them; bex-api resolves a Blueprint
// fromService.envVarKey that copies one from a sibling. Both read this one
// definition, so the two can never disagree.
package renderenv

import (
	"cmp"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's documented variable names.
const (
	Render           = "RENDER"
	IsPullRequest    = "IS_PULL_REQUEST"
	CPUCount         = "RENDER_CPU_COUNT"
	DiscoveryService = "RENDER_DISCOVERY_SERVICE"
	ExternalHostname = "RENDER_EXTERNAL_HOSTNAME"
	ExternalURL      = "RENDER_EXTERNAL_URL"
	GitBranch        = "RENDER_GIT_BRANCH"
	GitCommit        = "RENDER_GIT_COMMIT"
	GitRepoSlug      = "RENDER_GIT_REPO_SLUG"
	InstanceID       = "RENDER_INSTANCE_ID"
	ServiceID        = "RENDER_SERVICE_ID"
	ServiceName      = "RENDER_SERVICE_NAME"
	ServiceType      = "RENDER_SERVICE_TYPE"
	WebConcurrency   = "RENDER_WEB_CONCURRENCY"
)

// LabelAppID is the label carrying a store-managed App's id: the same value
// as the operator's labelAppID and bex-api's core.LabelAppID, neither of
// which types can import.
const LabelAppID = "bex.co/app-id"

// Var is one variable and its value.
type Var struct{ Name, Value string }

// names is every documented variable.
var names = map[string]bool{
	Render: true, IsPullRequest: true, CPUCount: true, DiscoveryService: true,
	ExternalHostname: true, ExternalURL: true, GitBranch: true, GitCommit: true,
	GitRepoSlug: true, InstanceID: true, ServiceID: true, ServiceName: true,
	ServiceType: true, WebConcurrency: true,
}

// IsPlatformName reports a documented name the platform owns. Any other
// RENDER_-prefixed name is an ordinary user variable.
func IsPlatformName(name string) bool { return names[name] }

// ServiceTypeOf is Render's short type name: web, pserv, cron, worker or static.
func ServiceTypeOf(t string) string {
	switch t {
	case appv1alpha1.TypePrivateService:
		return "pserv"
	case appv1alpha1.TypeCronJob:
		return "cron"
	case appv1alpha1.TypeBackgroundWorker:
		return "worker"
	case appv1alpha1.TypeStaticSite:
		return "static"
	}
	return "web"
}

// RepoSlug is Render's "$username/$reponame" for a git URL in any form
// (https, ssh, scp), keeping the repository's own case; "" when the URL does
// not carry an owner and a repository.
func RepoSlug(repo string) string {
	s := strings.TrimSpace(repo)
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if at := strings.LastIndex(s, "@"); at >= 0 {
		s = s[at+1:]
	}
	s = strings.TrimSuffix(strings.TrimRight(strings.ReplaceAll(s, ":", "/"), "/"), ".git")
	parts := strings.Split(s, "/")
	if len(parts) < 3 || parts[len(parts)-2] == "" || parts[len(parts)-1] == "" {
		return ""
	}
	return parts[len(parts)-2] + "/" + parts[len(parts)-1]
}

// PlanCPUCount is max(1, ceil(plan CPUs)): Render's RENDER_CPU_COUNT and its
// recommended web concurrency. A tier without a CPU count (unset,
// best-effort) gets 1, like Render's smallest plans.
func PlanCPUCount(tier string) int {
	cpu, _, _, ok := tiers.Compute.Resources(tier)
	if !ok {
		return 1
	}
	q := resource.MustParse(cpu)
	return max(1, int(q.Value())) // Value rounds up: 500m is 1
}

const discoverySuffix = "-discovery"

// HasDiscoveryService reports the types RENDER_DISCOVERY_SERVICE resolves for:
// the long-running, scalable ones.
func HasDiscoveryService(app *appv1alpha1.App) bool {
	return app.Spec.InternallyAddressable() || app.Spec.Type == appv1alpha1.TypeBackgroundWorker
}

// DiscoveryServiceName is Render's "<service>-discovery" for app's slug, kept
// within a DNS label.
func DiscoveryServiceName(app *appv1alpha1.App) string {
	slug := app.Spec.PlatformSubdomain(app.Name)
	if len(slug)+len(discoverySuffix) <= 63 {
		return slug + discoverySuffix
	}
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte(slug)))[:8]
	return slug[:63-len(discoverySuffix)-1-len(sum)] + "-" + sum + discoverySuffix
}

// Static is every variable whose value follows from the App and the platform
// base domain alone, in a stable order. It leaves out the per-instance
// RENDER_INSTANCE_ID and the per-image RENDER_GIT_COMMIT.
func Static(app *appv1alpha1.App, baseDomain string) []Var {
	slug := app.Spec.PlatformSubdomain(app.Name)
	cpus := strconv.Itoa(PlanCPUCount(app.Spec.Tier))
	vars := []Var{
		{Render, "true"},
		{IsPullRequest, "false"},
		{CPUCount, cpus},
		{ServiceID, cmp.Or(app.Labels[LabelAppID], app.Name)},
		{ServiceName, slug},
		{ServiceType, ServiceTypeOf(app.Spec.Type)},
	}
	if HasDiscoveryService(app) {
		vars = append(vars, Var{DiscoveryService, DiscoveryServiceName(app)})
	}
	// Only when the platform host actually routes: the same Expose and
	// subdomain-policy rule as AppSpec.EffectiveHosts.
	if app.Spec.PubliclyRoutable() && app.Spec.Expose && baseDomain != "" &&
		app.Spec.SubdomainPolicy != appv1alpha1.SubdomainPolicyDisabled {
		host := slug + "." + baseDomain
		vars = append(vars, Var{ExternalHostname, host}, Var{ExternalURL, "https://" + host})
	}
	if app.Spec.Repo != "" {
		vars = append(vars, Var{GitBranch, cmp.Or(app.Spec.Branch, appv1alpha1.DefaultBranch)})
		if s := RepoSlug(app.Spec.Repo); s != "" {
			vars = append(vars, Var{GitRepoSlug, s})
		}
	}
	if app.Spec.InternallyAddressable() {
		vars = append(vars, Var{WebConcurrency, cpus})
	}
	return vars
}

// Value is one Static variable of app, and whether app has it.
func Value(app *appv1alpha1.App, name, baseDomain string) (string, bool) {
	for _, v := range Static(app, baseDomain) {
		if v.Name == name {
			return v.Value, true
		}
	}
	return "", false
}
