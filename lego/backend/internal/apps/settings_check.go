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
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// This file holds each patchable setting's POST-AUTHORIZATION CHECK, split out
// of the setter that used to inline it (w9/m166).
//
// The split exists so one refusal can be reached twice: by the setter itself,
// on the single-setting path GraphQL and the dashboard still use, and by
// ApplyServicePatch's preflight (see preflightServicePatch for why that pass
// exists).
//
// Every function here is READ-ONLY: it may fetch and it may compute the
// normalized value its caller will write, but it must never write. A setter
// calls its check and then writes the value the check returned, so the two
// paths cannot drift on either the rule or the normalization.

func checkHealthCheckPath(a *appv1alpha1.App, path string) (string, error) {
	if a.Spec.Type == appv1alpha1.TypeCronJob || a.Spec.Type == appv1alpha1.TypeBackgroundWorker {
		return "", fmt.Errorf("%w: health check path is not applicable to a %s", core.ErrBadRequest, a.Spec.Type)
	}
	trimmed := strings.TrimSpace(path)
	if trimmed != "" && !strings.HasPrefix(trimmed, "/") {
		return "", fmt.Errorf("%w: health check path must start with /", core.ErrBadRequest)
	}
	return trimmed, nil
}

func checkIdleTTL(seconds int32) error {
	if seconds < 0 || seconds > MaxIdleTTLSeconds {
		return fmt.Errorf("%w: idleTTLSeconds must be 0-%d", core.ErrBadRequest, MaxIdleTTLSeconds)
	}
	return nil
}

func (s *Service) checkRootDir(ctx context.Context, a *appv1alpha1.App, name, rootDir string) error {
	// What gets built is what runs, so this is the same identity change that
	// repointing the source is (w4/m126).
	if err := s.requireUnprotected(ctx, a, "redefine"); err != nil {
		return err
	}
	if err := requireRepoBacked(a, name, "root directory only applies to build-from-git"); err != nil {
		return err
	}
	if !store.ValidRootDir(rootDir) {
		return fmt.Errorf("%w: rootDirectory must be a relative path with no '..' components", core.ErrBadRequest)
	}
	return nil
}

func (s *Service) checkDockerfilePath(ctx context.Context, a *appv1alpha1.App, name, dockerfilePath string) (string, error) {
	// What gets built is what runs, so this is the same identity change that
	// repointing the source is (w4/m126).
	if err := s.requireUnprotected(ctx, a, "redefine"); err != nil {
		return "", err
	}
	if err := requireRepoBacked(a, name, "dockerfile path only applies to Dockerfile builds"); err != nil {
		return "", err
	}
	runtime := strings.ToLower(strings.TrimSpace(a.Spec.Runtime))
	builder := strings.ToLower(strings.TrimSpace(a.Spec.Builder))
	if (runtime != "" && runtime != "docker") || builder == "native" || builder == "buildpack" {
		return "", fmt.Errorf("%w: dockerfile path only applies to a Dockerfile-built service", core.ErrBadRequest)
	}
	dockerfilePath = strings.TrimSpace(dockerfilePath)
	if dockerfilePath != "" && !store.ValidRootDir(dockerfilePath) {
		return "", fmt.Errorf("%w: dockerfilePath must be a relative path with no '..' components", core.ErrBadRequest)
	}
	return dockerfilePath, nil
}

func checkBuildFilter(a *appv1alpha1.App, name string, filter *BuildFilterView) (*appv1alpha1.BuildFilterSpec, error) {
	if err := requireRepoBacked(a, name, "build filters only apply to build-from-git"); err != nil {
		return nil, err
	}
	return normalizeBuildFilter(filter)
}

func (s *Service) checkPreDeployCommand(ctx context.Context, a *appv1alpha1.App) error {
	// Same class as the build/start commands: attacker-chosen code the service
	// runs with its own identity, so a protected environment asks (w4/m126).
	if err := s.requireUnprotected(ctx, a, "redefine"); err != nil {
		return err
	}
	if a.Spec.Type == appv1alpha1.TypeCronJob || a.Spec.Type == appv1alpha1.TypeStaticSite {
		return fmt.Errorf("%w: a pre-deploy command does not apply to a %s", core.ErrBadRequest, a.Spec.Type)
	}
	return nil
}

func (s *Service) checkCommands(ctx context.Context, a *appv1alpha1.App, startCommand *string) error {
	// What gets built is what runs, so this is the same identity change that
	// repointing the source is (w4/m126).
	if err := s.requireUnprotected(ctx, a, "redefine"); err != nil {
		return err
	}
	if a.Spec.Type == appv1alpha1.TypeStaticSite && startCommand != nil {
		return fmt.Errorf("%w: start command is not applicable to a static_site", core.ErrBadRequest)
	}
	return nil
}

func checkPublishPath(a *appv1alpha1.App, name, publishPath string) error {
	if err := validatePublishPath(publishPath); err != nil {
		return err
	}
	return requireStaticSite(a, name)
}

func checkPort(a *appv1alpha1.App, port int32) error {
	if !a.Spec.InternallyAddressable() {
		return fmt.Errorf("%w: port only applies to a web_service or private_service; %q has no listening port",
			core.ErrBadRequest, effectiveType(a.Spec.Type))
	}
	if a.Spec.UsesImagePorts() && appv1alpha1.IsReservedImagePort(port) {
		return fmt.Errorf("%w: private image port %d is reserved", core.ErrBadRequest, port)
	}
	return validateServicePort(port)
}

func (s *Service) checkSubdomainPolicy(ctx context.Context, a *appv1alpha1.App, policy string) (string, error) {
	normalized, err := normalizeSubdomainPolicy(policy)
	if err != nil {
		return "", err
	}
	// renderSubdomainPolicy toggles the platform subdomain, which only a web
	// service or static site HAS. A private service, background worker or cron
	// job has no ingress, so there is no subdomain to enable or disable — refuse
	// with exactly that reason, not the misleading custom-domain guard below
	// (which then sends the caller into an add-domain call that itself 400s
	// because the type has no ingress) and not a silent success (w6/m130).
	if !a.Spec.PubliclyRoutable() {
		return "", fmt.Errorf("%w: renderSubdomainPolicy applies only to web services and static sites; a %s has no platform subdomain to toggle", core.ErrBadRequest, effectiveType(a.Spec.Type))
	}
	if normalized == appv1alpha1.SubdomainPolicyDisabled {
		if a.Spec.Host == "" && len(a.Spec.Hosts) == 0 {
			return "", s.noVerifiedDomainError(ctx, a)
		}
	}
	return normalized, nil
}

func (s *Service) checkCronJob(ctx context.Context, a *appv1alpha1.App, name string, schedule, command *string) error {
	// Changing what the cron runs is the identity change a protected
	// environment asks about (w4/m126); rescheduling when it runs is not.
	if command != nil {
		if err := s.requireUnprotected(ctx, a, "redefine"); err != nil {
			return err
		}
	}
	if schedule != nil {
		trimmed := strings.TrimSpace(*schedule)
		if trimmed == "" {
			return fmt.Errorf("%w: schedule is required", core.ErrBadRequest)
		}
		if !validCronSchedule(trimmed) {
			return fmt.Errorf("%w: schedule must be a valid 5-field cron expression (e.g. '0 * * * *')", core.ErrBadRequest)
		}
	}
	if a.Spec.Type != appv1alpha1.TypeCronJob {
		return fmt.Errorf("%w: service %q is not a cron_job", core.ErrBadRequest, name)
	}
	return nil
}

func checkAutoscaling(a *appv1alpha1.App, req SetAutoscalingRequest) (appv1alpha1.AutoscalingSpec, error) {
	if a.Spec.Disk != nil {
		return appv1alpha1.AutoscalingSpec{}, fmt.Errorf("%w: detach the disk before enabling autoscaling", core.ErrBadRequest)
	}
	return autoscalingSpec(req, a.Spec.Type, a.Spec.Tier)
}

// checkPlan resolves the requested Render plan to its tier and runs every
// policy gate SetPlan owes — type, disk, billing, maintenance and downgrade.
// It returns the resolved tier so the setter writes exactly what was checked.
func (s *Service) checkPlan(ctx context.Context, a *appv1alpha1.App, plan string) (string, error) {
	t, ok := tiers.Compute.ByRenderPlan(plan)
	if !ok {
		return "", fmt.Errorf("%w: plan must be one of %s", core.ErrBadRequest, strings.Join(tiers.Compute.RenderPlans(), "|"))
	}
	tier := t.ID
	// Paid-only types refuse a downgrade to free here, which is the SetPlan /
	// PreviewSetPlan half of the create-time rule in normalizeTierForType
	// (ADR030 §7; private services added by w1/111).
	if paidOnlyServiceType(a.Spec.Type) && !core.PaidPlan(tier) {
		return "", errFreePlanForType(a.Spec.Type)
	}
	if err := diskPlanError(a, tier); err != nil {
		return "", err
	}
	if err := s.RequirePlanBilling(ctx, a.Labels[core.LabelTenant], tier); err != nil {
		return "", err
	}
	if tier == "free" && a.Spec.MaintenanceMode != nil && a.Spec.MaintenanceMode.Enabled {
		return "", fmt.Errorf("%w: disable maintenance mode before changing to the free plan", core.ErrBadRequest)
	}
	if err := planDowngradeError(a, tier, plan); err != nil {
		return "", err
	}
	return tier, nil
}

// checkMaintenanceMode validates the two-key object and returns it normalized.
// The protected-environment guard applies only to turning maintenance ON —
// turning it off restores availability (like Resume), and a URI-only edit
// changes neither.
func (s *Service) checkMaintenanceMode(ctx context.Context, a *appv1alpha1.App, in MaintenanceModeView) (MaintenanceModeView, error) {
	in.URI = strings.TrimSpace(in.URI)
	if err := s.validateMaintenanceMode(ctx, a, in); err != nil {
		return MaintenanceModeView{}, err
	}
	current := maintenanceModeView(a.Spec.MaintenanceMode)
	if current == in {
		return in, nil
	}
	if in.Enabled && !current.Enabled {
		if err := s.requireUnprotected(ctx, a, "take offline"); err != nil {
			return MaintenanceModeView{}, err
		}
	}
	return in, nil
}

// probedSourceKey carries the exact source ApplyServicePatch's preflight
// already probed, so its apply pass does not repeat the two OUTBOUND calls
// checkSourcePatch makes — the registry-credential resolve and the GitHub
// repository check.
//
// Every other check in the table is local arithmetic, so running it in both
// passes costs nothing. These two are calls to someone else's API: repeating
// them would double the latency of every source change and double the load bex
// puts on that API, for an answer taken microseconds earlier in the same
// request.
//
// The marker carries the resolved source ITSELF rather than the service id, so
// the skip is gated on value equality: the probes are reused only for a source
// byte-identical to the one that was probed. An id would have had to be matched
// against the App, and the caller-supplied id, `metadata.name` and the two name
// labels are four different spellings of "which service" — guessing among them
// is how this skip silently stopped firing in review.
type probedSourceKey struct{}

func withProbedSource(ctx context.Context, probed sourceFields) context.Context {
	return context.WithValue(ctx, probedSourceKey{}, probed)
}

func sourceAlreadyProbed(ctx context.Context, next sourceFields) bool {
	probed, ok := ctx.Value(probedSourceKey{}).(sourceFields)
	return ok && probed.repo == next.repo && probed.image == next.image &&
		probed.branch == next.branch &&
		sameStringPtr(probed.registryCredentialID, next.registryCredentialID)
}

// checkSourcePatch folds patch onto a's current source and runs every gate
// SetSourceAndRegistryCredential owes before its first write: the owner match,
// the protected-environment guard, per-field validation, and — only when the
// resolved source actually moves — the registry-credential and repository
// reachability probes. It returns the resolved source and whether it changed,
// so the setter reuses the same answer rather than recomputing it.
func (s *Service) checkSourcePatch(ctx context.Context, a *appv1alpha1.App, patch sourcePatch) (sourceFields, bool, error) {
	if patch.ImageOwnerID != nil {
		ownerID := strings.TrimSpace(*patch.ImageOwnerID)
		if ownerID != "" && ownerID != a.Labels[core.LabelTenant] {
			return sourceFields{}, false, fmt.Errorf("%w: image.ownerId does not match the service owner", core.ErrBadRequest)
		}
	}
	// A protected environment guards what the service RUNS, not only whether it
	// runs (w4/m126). Placed before resolveSourcePatch's own no-op return, so a
	// caller cannot learn whether a change would have applied by watching which
	// error comes back.
	if verb := protectedSourceVerb(patch); verb != "" {
		if err := s.requireUnprotected(ctx, a, verb); err != nil {
			return sourceFields{}, false, err
		}
	}
	next, err := resolveSourcePatch(a, patch)
	if err != nil {
		return sourceFields{}, false, err
	}
	if next.repo == a.Spec.Repo &&
		next.image == a.Spec.Image &&
		next.branch == a.Spec.Branch &&
		sameStringPtr(next.registryCredentialID, a.Spec.RegistryCredentialID) {
		return next, false, nil
	}
	if sourceAlreadyProbed(ctx, next) {
		return next, true, nil
	}
	probe := a.DeepCopy()
	next.applyTo(probe)
	if err := s.validateExternalRegistryCredential(ctx, probe); err != nil {
		return sourceFields{}, false, err
	}
	if next.repo != "" && next.repo != a.Spec.Repo && s.GitHub != nil {
		if err := s.GitHub.ValidateRepo(ctx, s.AppWorkspace(ctx, a), next.repo); err != nil {
			return sourceFields{}, false, err
		}
	}
	return next, true, nil
}
