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
	"fmt"
	"reflect"
	"slices"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// BlueprintOmission controls what an existing resource does when a Blueprint
// does not declare a field. It is deliberately data, rather than a collection
// of zero-value checks: the compiler keeps field presence in BlueprintIR and
// the apply path can therefore distinguish absent, null, false, and empty.
type BlueprintOmission string

const (
	BlueprintPreserveOnOmission BlueprintOmission = "preserve"
	BlueprintClearOnOmission    BlueprintOmission = "clear"
)

// BlueprintServiceFieldPolicy states ownership for a Render service field.
// The policies below are intentionally limited to fields represented by
// AppSpec. Unsupported source vocabulary is rejected by the capability gate,
// before it can reach this layer.
type BlueprintServiceFieldPolicy struct {
	Name     string
	Omission BlueprintOmission
}

// BlueprintServiceFieldPolicies is the single service sync policy table. The
// exceptional buildFilter behavior follows Render's Blueprint sync semantics:
// removing the block clears filters, whereas most omitted fields retain their
// current value on an existing service.
var BlueprintServiceFieldPolicies = []BlueprintServiceFieldPolicy{
	{Name: "type", Omission: BlueprintPreserveOnOmission},
	{Name: "runtime", Omission: BlueprintPreserveOnOmission},
	{Name: "schedule", Omission: BlueprintPreserveOnOmission},
	{Name: "repo", Omission: BlueprintPreserveOnOmission},
	{Name: "image", Omission: BlueprintPreserveOnOmission},
	{Name: "branch", Omission: BlueprintPreserveOnOmission},
	{Name: "builder", Omission: BlueprintPreserveOnOmission},
	{Name: "rootDir", Omission: BlueprintPreserveOnOmission},
	{Name: "buildFilter", Omission: BlueprintClearOnOmission},
	{Name: "buildCommand", Omission: BlueprintPreserveOnOmission},
	{Name: "startCommand", Omission: BlueprintPreserveOnOmission},
	{Name: "dockerCommand", Omission: BlueprintPreserveOnOmission},
	{Name: "dockerfilePath", Omission: BlueprintPreserveOnOmission},
	{Name: "dockerContext", Omission: BlueprintPreserveOnOmission},
	{Name: "registryCredential", Omission: BlueprintPreserveOnOmission},
	{Name: "numInstances", Omission: BlueprintPreserveOnOmission},
	{Name: "scaling", Omission: BlueprintPreserveOnOmission},
	{Name: "plan", Omission: BlueprintPreserveOnOmission},
	{Name: "healthCheckPath", Omission: BlueprintPreserveOnOmission},
	{Name: "maxShutdownDelaySeconds", Omission: BlueprintPreserveOnOmission},
	{Name: "envVars", Omission: BlueprintPreserveOnOmission},
	{Name: "autoDeploy", Omission: BlueprintPreserveOnOmission},
	{Name: "autoDeployTrigger", Omission: BlueprintPreserveOnOmission},
	{Name: "ipAllowList", Omission: BlueprintPreserveOnOmission},
	{Name: "domains", Omission: BlueprintPreserveOnOmission},
	{Name: "domain", Omission: BlueprintPreserveOnOmission},
	{Name: "staticPublishPath", Omission: BlueprintPreserveOnOmission},
	{Name: "routes", Omission: BlueprintPreserveOnOmission},
	{Name: "headers", Omission: BlueprintPreserveOnOmission},
	{Name: "renderSubdomainPolicy", Omission: BlueprintPreserveOnOmission},
	{Name: "maintenanceMode", Omission: BlueprintPreserveOnOmission},
	{Name: "preDeployCommand", Omission: BlueprintPreserveOnOmission},
	{Name: "initialDeployHook", Omission: BlueprintPreserveOnOmission},
}

func blueprintServiceOmission(name string) BlueprintOmission {
	for _, policy := range BlueprintServiceFieldPolicies {
		if policy.Name == name {
			return policy.Omission
		}
	}
	return BlueprintPreserveOnOmission
}

// blueprintServiceFieldAppliers maps declared Blueprint fields to the
// presence-gated spec copy each performs. Grouped names apply once when any
// member is declared (domains|domain, autoDeploy|autoDeployTrigger). Fields
// whose omission or interplay carries meaning (buildFilter, scaling and
// numInstances) stay inline in ApplyBlueprintServiceSpec.
var blueprintServiceFieldAppliers = []struct {
	names []string
	apply func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec)
}{
	{names: []string{"type"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) { dst.Type = want.Type }},
	{names: []string{"runtime"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) { dst.Runtime = want.Runtime }},
	{names: []string{"schedule"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) { dst.Schedule = want.Schedule }},
	{names: []string{"repo"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) { dst.Repo = want.Repo }},
	{names: []string{"image"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) {
		dst.Image = want.Image
		// image.creds rides the image field's presence: a resolved credential
		// rebinding applies with it, while a creds-less image change leaves an
		// existing binding untouched (preserve-on-omission, like the field).
		if want.RegistryCredentialID != nil {
			dst.RegistryCredentialID = clonePtr(want.RegistryCredentialID)
		}
	}},
	{names: []string{"branch"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) { dst.Branch = want.Branch }},
	{names: []string{"builder"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) { dst.Builder = want.Builder }},
	{names: []string{"rootDir"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) { dst.RootDir = want.RootDir }},
	{names: []string{"buildCommand"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) { dst.BuildCommand = want.BuildCommand }},
	{names: []string{"startCommand"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) { dst.StartCommand = want.StartCommand }},
	{names: []string{"dockerCommand"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) { dst.StartCommand = want.StartCommand }},
	{names: []string{"dockerfilePath"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) { dst.DockerfilePath = want.DockerfilePath }},
	{names: []string{"dockerContext"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) { dst.DockerContext = want.DockerContext }},
	{names: []string{"registryCredential"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) {
		dst.RegistryCredentialID = clonePtr(want.RegistryCredentialID)
	}},
	{names: []string{"plan"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) { dst.Tier = want.Tier }},
	{names: []string{"healthCheckPath"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) { dst.HealthCheckPath = want.HealthCheckPath }},
	{names: []string{"maxShutdownDelaySeconds"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) {
		dst.MaxShutdownDelaySeconds = clonePtr(want.MaxShutdownDelaySeconds)
	}},
	{names: []string{"autoDeploy", "autoDeployTrigger"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) {
		dst.AutoDeploy = want.AutoDeploy
	}},
	{names: []string{"ipAllowList"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) {
		dst.SetIPAllowListEntries(want.EffectiveIPAllowListEntries())
	}},
	{names: []string{"domains", "domain"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) {
		dst.Host = want.Host
		// Keep the zero value canonical. Kubernetes drops an empty slice when it
		// round-trips the CR, so preserving [] here would make an identical
		// Blueprint look changed on every re-apply.
		dst.Hosts = nil
		if len(want.Hosts) > 0 {
			dst.Hosts = slices.Clone(want.Hosts)
		}
	}},
	{names: []string{"staticPublishPath"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) { dst.PublishPath = want.PublishPath }},
	{names: []string{"routes"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) { dst.Routes = slices.Clone(want.Routes) }},
	{names: []string{"headers"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) { dst.Headers = slices.Clone(want.Headers) }},
	{names: []string{"renderSubdomainPolicy"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) {
		dst.SubdomainPolicy = want.SubdomainPolicy
	}},
	{names: []string{"maintenanceMode"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) {
		if want.MaintenanceMode == nil {
			dst.MaintenanceMode = nil
			return
		}
		dst.MaintenanceMode = want.MaintenanceMode.DeepCopy()
	}},
	{names: []string{"preDeployCommand"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) { dst.PreDeployCommand = want.PreDeployCommand }},
	{names: []string{"initialDeployHook"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) { dst.PreDeployCommand = want.PreDeployCommand }},
	{names: []string{"envVars"}, apply: func(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) {
		dst.Env = mergeBlueprintEnv(dst.Env, want.Env)
	}},
}

func anyPresent(fields map[string]BlueprintField, names ...string) bool {
	for _, name := range names {
		if _, ok := fields[name]; ok {
			return true
		}
	}
	return false
}

// ApplyBlueprintServiceSpec applies a normalized service only for source fields
// that the Blueprint actually declared. It is for existing resources; creation
// still uses specFromCreate, where product defaults are appropriate. The return
// value reports whether the resulting desired spec differs from the original.
//
// buildFilter is Render's documented exception: omission clears an existing
// filter. An explicit envVars block only upserts values the Blueprint owns;
// dashboard/API vars not named in the Blueprint remain intact.
func ApplyBlueprintServiceSpec(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec, fields map[string]BlueprintField) bool {
	before := *dst.DeepCopy()
	present := func(name string) bool {
		_, ok := fields[name]
		return ok
	}

	for _, field := range blueprintServiceFieldAppliers {
		if anyPresent(fields, field.names...) {
			field.apply(dst, want)
		}
	}

	if present("buildFilter") {
		if want.BuildFilter == nil {
			dst.BuildFilter = nil
		} else {
			dst.BuildFilter = want.BuildFilter.DeepCopy()
		}
	} else if blueprintServiceOmission("buildFilter") == BlueprintClearOnOmission {
		dst.BuildFilter = nil
	}

	// A declared scaling block owns autoscaling. Conversely, an explicit manual
	// count returns the service to manual scaling if no autoscaling block was
	// declared. Omission of both preserves the currently active scaling mode.
	if present("scaling") {
		if want.Autoscaling == nil {
			dst.Autoscaling = nil
		} else {
			dst.Autoscaling = want.Autoscaling.DeepCopy()
		}
	} else if present("numInstances") {
		dst.Autoscaling = nil
		dst.Replicas = want.Replicas
	}

	return !reflect.DeepEqual(before, *dst)
}

// mergeBlueprintEnv replaces variables owned by the declared Blueprint while
// retaining undeclared mutable values. It has stable ordering: surviving
// values retain their order and newly declared names append in manifest order.
func mergeBlueprintEnv(current, declared []appv1alpha1.EnvVar) []appv1alpha1.EnvVar {
	byName := make(map[string]appv1alpha1.EnvVar, len(declared))
	for _, variable := range declared {
		byName[variable.Name] = variable
	}
	merged := make([]appv1alpha1.EnvVar, 0, len(current)+len(declared))
	for _, variable := range current {
		if replacement, ok := byName[variable.Name]; ok {
			merged = append(merged, replacement)
			delete(byName, variable.Name)
			continue
		}
		merged = append(merged, variable)
	}
	for _, variable := range declared {
		if replacement, ok := byName[variable.Name]; ok {
			merged = append(merged, replacement)
			delete(byName, variable.Name)
		}
	}
	return merged
}

// BlueprintFieldConflictError identifies a semantically valid Blueprint field
// that cannot be applied to the resource's current state (for example a
// shrink-only disk request). Adapters turn it into their native structured
// validation response without losing the source field path.
type BlueprintFieldConflictError struct {
	Path    string
	Message string
}

func (e *BlueprintFieldConflictError) Error() string {
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// ApplyBlueprintDatabaseSpec applies only declared database fields. Its
// immutability and grow-only checks happen before the caller mutates a CR, so a
// plan cannot report a change that the operator would later ignore.
func ApplyBlueprintDatabaseSpec(dst *appv1alpha1.DatabaseSpec, want appv1alpha1.DatabaseSpec, fields map[string]BlueprintField) (bool, error) {
	before := *dst.DeepCopy()
	present := func(name string) bool { _, ok := fields[name]; return ok }
	if present("databaseName") {
		if dst.DatabaseName != "" && want.DatabaseName != dst.DatabaseName {
			return false, &BlueprintFieldConflictError{Path: "databaseName", Message: "is immutable after creation"}
		}
		dst.DatabaseName = want.DatabaseName
	}
	if present("user") {
		if dst.DatabaseUser != "" && want.DatabaseUser != dst.DatabaseUser {
			return false, &BlueprintFieldConflictError{Path: "user", Message: "is immutable after creation"}
		}
		dst.DatabaseUser = want.DatabaseUser
	}
	if present("plan") {
		dst.Plan = want.Plan
	}
	if present("postgresMajorVersion") {
		dst.Version = want.Version
	}
	if present("diskSizeGB") {
		if dst.StorageGB > 0 && want.StorageGB < dst.StorageGB {
			return false, &BlueprintFieldConflictError{Path: "diskSizeGB", Message: "cannot shrink existing storage"}
		}
		dst.StorageGB = want.StorageGB
	}
	if present("storageAutoscalingEnabled") {
		dst.DiskAutoscaling = want.DiskAutoscaling
	}
	if present("connectionPool") {
		dst.Pooler = want.Pooler
	}
	if present("ipAllowList") {
		dst.IPAllowList = slices.Clone(want.IPAllowList)
	}
	if present("readReplicas") {
		dst.ReadReplicas = slices.Clone(want.ReadReplicas)
	}
	if present("highAvailability") {
		dst.HighAvailability = want.HighAvailability
	}
	return !reflect.DeepEqual(before, *dst), nil
}

// ApplyBlueprintKeyValueSpec is the equivalent presence-aware updater for a
// Key Value resource. The schema owns no disk/version fields for this service;
// all current Blueprint-owned knobs are listed explicitly here.
func ApplyBlueprintKeyValueSpec(dst *appv1alpha1.KeyValueSpec, want appv1alpha1.KeyValueSpec, fields map[string]BlueprintField) bool {
	before := *dst.DeepCopy()
	present := func(name string) bool { _, ok := fields[name]; return ok }
	if present("plan") {
		dst.Plan = want.Plan
	}
	if present("ipAllowList") {
		dst.IPAllowList = slices.Clone(want.IPAllowList)
	}
	if present("maxmemoryPolicy") {
		dst.MaxmemoryPolicy = want.MaxmemoryPolicy
	}
	if present("persistenceMode") {
		dst.PersistenceMode = want.PersistenceMode
	}
	return !reflect.DeepEqual(before, *dst)
}
