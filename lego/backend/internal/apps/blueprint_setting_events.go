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
	"errors"
	"fmt"
	"reflect"
	"slices"

	"k8s.io/utils/ptr"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A Blueprint (render.yaml) sync, approval or deploy that changes an existing
// service's settings records each change as the setting's own setter does
// (w9/091 for the plan, w9/092 for the rest), by the apply's own actor: the
// member who deployed, ran a manual sync or approved a planned one, and none
// for an automatic sync. A managed App's row-owned settings (source, plan,
// instance count, disk, domains) are recorded when their rows change, in
// persistBlueprintIntent and replaceBlueprintDomains; everything else, and a
// store-less App's every setting, once the App's patch lands. A move to
// another project or environment records service_moved the same way, from
// applyEnvironmentChange (w9/093). A service the Blueprint creates records
// none of these. Maintenance mode records through
// ConfigureMaintenanceMode, and env-group links and env seeds through their
// own verbs, so none of them is recorded here.

// blueprintSourceChange is the source write a Blueprint apply makes.
type blueprintSourceChange int

const (
	blueprintSourceKept blueprintSourceChange = iota
	// blueprintSourceLinked links the service to another build source.
	blueprintSourceLinked
	// blueprintSourceSet changes its own repo, image, branch or registry
	// credential.
	blueprintSourceSet
)

// blueprintSourceChangeOf is the source write an apply from cur to want makes.
func blueprintSourceChangeOf(cur, want *appv1alpha1.AppSpec) blueprintSourceChange {
	switch {
	case want.BuildSource != nil && (cur.BuildSource == nil || cur.BuildSource.Name != want.BuildSource.Name):
		return blueprintSourceLinked
	case cur.Repo != want.Repo || cur.Image != want.Image || cur.Branch != want.Branch ||
		!sameStringPtr(cur.RegistryCredentialID, want.RegistryCredentialID):
		return blueprintSourceSet
	default:
		return blueprintSourceKept
	}
}

// replaceBlueprintDomains writes a Blueprint's domains onto a managed App's
// rows, its domain claims or its legacy domain rows. A claims replacement
// records each host it added to or removed from current, the claims the
// apply's probe read, as AddDomain and DeleteDomain record theirs; a retry
// finds the claims already replaced and records nothing. A store without
// claims (no production store) cannot say what its replacement changed and
// records nothing. A store-less App's domains are its spec, recorded once its
// patch lands.
func (s *Service) replaceBlueprintDomains(ctx context.Context, existing, final *appv1alpha1.App, current []store.Domain) error {
	appID := s.blueprintIntentRow(existing)
	if appID == "" {
		return nil
	}
	if claims, _, ok := s.managedDomainClaims(existing); ok {
		declarations := s.claimDeclarations(final)
		rows, err := claims.ReplaceDomainClaims(ctx, appID, declarations)
		if errors.Is(err, store.ErrConflict) {
			return errDomainInUse()
		}
		if err != nil {
			return fmt.Errorf("claim Blueprint domains: %w", err)
		}
		before := make([]string, 0, len(current))
		for _, claim := range current {
			before = append(before, claim.Host)
		}
		after := make([]string, 0, len(declarations))
		for _, declaration := range declarations {
			after = append(after, declaration.Host)
		}
		s.recordStackDomainChanges(ctx, existing, before, after)
		applyVerifiedDomainClaims(&final.Spec, rows)
		return nil
	}
	if err := s.Store.ReplaceDomains(ctx, appID, final.Spec.Host, final.Spec.Hosts); err != nil {
		if errors.Is(err, store.ErrConflict) {
			return errDomainInUse()
		}
		return fmt.Errorf("claim Blueprint domains: %w", err)
	}
	return nil
}

// customDomainHosts is spec's domains less a's own platform subdomain, which a
// Blueprint may restate but is no custom domain.
func (s *Service) customDomainHosts(a *appv1alpha1.App, spec *appv1alpha1.AppSpec) []string {
	own := s.ownPlatformHost(a)
	return slices.DeleteFunc(blueprintDomainList(*spec), func(host string) bool { return host == "" || host == own })
}

// recordStackDomainChanges records custom_domain_added for each host in after
// and not before, then custom_domain_removed for each host in before and not
// after.
func (s *Service) recordStackDomainChanges(ctx context.Context, a *appv1alpha1.App, before, after []string) {
	for _, host := range after {
		if !slices.Contains(before, host) {
			s.RecordDomainEffect(ctx, a, core.AuditVerbAddDomain, host)
		}
	}
	for _, host := range before {
		if !slices.Contains(after, host) {
			s.RecordDomainEffect(ctx, a, core.AuditVerbDeleteDomain, host)
		}
	}
}

// recordStackSettingChanges records, once a Blueprint apply's patch of a
// landed, each setting it changed from before to after that no row recorded
// already: for a store-less App also its source, plan, instance count and
// domains. A setting the apply's source or build switch cleared or replaced
// is that switch's, which records as source_changed (or, for a runtime, as
// the rebuild it starts), so it records no event of its own.
func (s *Service) recordStackSettingChanges(ctx context.Context, a *appv1alpha1.App, before, after *appv1alpha1.AppSpec) {
	ctx = s.WithResolvedWorkspace(ctx)
	if s.blueprintIntentRow(a) == "" {
		switch blueprintSourceChangeOf(before, after) {
		case blueprintSourceLinked:
			s.recordSourceChanged(ctx, a, "", "", "")
		case blueprintSourceSet:
			s.recordSourceChanged(ctx, a, before.Branch, after.Branch, "")
		}
		if before.Tier != after.Tier {
			s.recordStackPlanChange(ctx, a, before, after)
		}
		if before.Replicas != after.Replicas {
			s.RecordScaleEffect(ctx, a, before.Replicas, after.Replicas)
		}
		s.recordStackDomainChanges(ctx, a, s.customDomainHosts(a, before), s.customDomainHosts(a, after))
	}
	switch from, to := before.Autoscaling, after.Autoscaling; {
	case autoscalingOn(to):
		if autoscalingChanged(from, *to) {
			s.RecordAutoscalingChanged(ctx, a, from, to)
		}
	case autoscalingOn(from):
		s.RecordAutoscalingChanged(ctx, a, from, nil)
	}
	// Only a change of the auto-deploy fields themselves: a switch to an image
	// turns no auto-deploy off, as the source setter's does not.
	if before.AutoDeploy != after.AutoDeploy || before.AutoDeployTrigger != after.AutoDeployTrigger {
		s.recordAutoDeployChange(ctx, a, renderAutoDeployTrigger(*before), renderAutoDeployTrigger(*after))
	}
	switched := switchedSettings(before, after)
	var verbs []string
	for _, setting := range stackConfigSettings {
		if setting.changed(before, after) && setting.changed(switched, after) {
			verbs = append(verbs, setting.verb)
		}
	}
	s.RecordAppConfigChanges(ctx, a, verbs...)
}

// switchedSettings is before with only the apply's source or build switch to
// after made: a link to a build source, which takes over the build settings
// and sets the start command (LinkBuildSource records one source_changed for
// all of it); a switch to a prebuilt image, which drops a built start command
// (w8/m70); and a change of build strategy, which drops the Dockerfile
// settings the new build does not read. A setting after holds as the switch
// left it is the switch's.
func switchedSettings(before, after *appv1alpha1.AppSpec) *appv1alpha1.AppSpec {
	switched := before.DeepCopy()
	if blueprintSourceChangeOf(before, after) == blueprintSourceLinked {
		linkBuildSourceSpec(switched, *after.BuildSource)
		switched.StartCommand, switched.Command = after.StartCommand, after.Command
		return switched
	}
	if prebuiltImageSource(*after) && !prebuiltImageSource(*before) {
		switched.StartCommand = ""
	}
	if buildStrategy(*after) != buildStrategy(*before) {
		switched.DockerfilePath, switched.DockerContext = "", ""
	}
	return switched
}

// stackConfigSettings are the settings whose setter records its verb through
// RecordAppConfigChanged, each with whether an apply changed it.
var stackConfigSettings = []struct {
	verb    string
	changed func(before, after *appv1alpha1.AppSpec) bool
}{
	{core.AuditVerbSetRootDir, func(b, a *appv1alpha1.AppSpec) bool { return b.RootDir != a.RootDir }},
	{core.AuditVerbSetDockerfilePath, func(b, a *appv1alpha1.AppSpec) bool { return b.DockerfilePath != a.DockerfilePath }},
	// A kept context moves with the root directory, which SetRootDir records
	// as no context change, so only a context other than that move is one.
	{core.AuditVerbSetDockerContext, func(b, a *appv1alpha1.AppSpec) bool {
		return a.DockerContext != rebaseDockerContext(b.RootDir, a.RootDir, b.DockerContext)
	}},
	{core.AuditVerbSetCommands, func(b, a *appv1alpha1.AppSpec) bool {
		return b.BuildCommand != a.BuildCommand || b.StartCommand != a.StartCommand || b.Command != a.Command
	}},
	{core.AuditVerbSetPreDeployCommand, func(b, a *appv1alpha1.AppSpec) bool { return b.PreDeployCommand != a.PreDeployCommand }},
	{core.AuditVerbSetBuildFilter, func(b, a *appv1alpha1.AppSpec) bool { return !reflect.DeepEqual(b.BuildFilter, a.BuildFilter) }},
	{core.AuditVerbSetMaxShutdownDelay, func(b, a *appv1alpha1.AppSpec) bool {
		return !ptr.Equal(b.MaxShutdownDelaySeconds, a.MaxShutdownDelaySeconds)
	}},
	// An empty policy is the enabled default.
	{core.AuditVerbSetSubdomainPolicy, func(b, a *appv1alpha1.AppSpec) bool {
		return (b.SubdomainPolicy == appv1alpha1.SubdomainPolicyDisabled) != (a.SubdomainPolicy == appv1alpha1.SubdomainPolicyDisabled)
	}},
	{core.AuditVerbSetIPAllowList, func(b, a *appv1alpha1.AppSpec) bool {
		return !reflect.DeepEqual(b.EffectiveIPAllowListEntries(), a.EffectiveIPAllowListEntries())
	}},
	{core.AuditVerbSetPublishPath, func(b, a *appv1alpha1.AppSpec) bool { return b.PublishPath != a.PublishPath }},
	{core.AuditVerbSetRoutes, func(b, a *appv1alpha1.AppSpec) bool { return !sameList(b.Routes, a.Routes) }},
	{core.AuditVerbSetHeaders, func(b, a *appv1alpha1.AppSpec) bool { return !sameList(b.Headers, a.Headers) }},
}

// sameList reports whether a and b hold the same items, nil and empty alike.
func sameList[T any](a, b []T) bool {
	return (len(a) == 0 && len(b) == 0) || reflect.DeepEqual(a, b)
}
