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

	if present("type") {
		dst.Type = want.Type
	}
	if present("runtime") {
		dst.Runtime = want.Runtime
	}
	if present("schedule") {
		dst.Schedule = want.Schedule
	}
	if present("repo") {
		dst.Repo = want.Repo
	}
	if present("image") {
		dst.Image = want.Image
	}
	if present("branch") {
		dst.Branch = want.Branch
	}
	if present("builder") {
		dst.Builder = want.Builder
	}
	if present("rootDir") {
		dst.RootDir = want.RootDir
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
	if present("buildCommand") {
		dst.BuildCommand = want.BuildCommand
	}
	if present("startCommand") {
		dst.StartCommand = want.StartCommand
	}
	if present("dockerCommand") {
		dst.StartCommand = want.StartCommand
	}
	if present("dockerfilePath") {
		dst.DockerfilePath = want.DockerfilePath
	}
	if present("plan") {
		dst.Tier = want.Tier
	}
	if present("healthCheckPath") {
		dst.HealthCheckPath = want.HealthCheckPath
	}
	if present("maxShutdownDelaySeconds") {
		dst.MaxShutdownDelaySeconds = clonePtr(want.MaxShutdownDelaySeconds)
	}
	if present("autoDeploy") || present("autoDeployTrigger") {
		dst.AutoDeploy = want.AutoDeploy
	}
	if present("ipAllowList") {
		dst.SetIPAllowListEntries(want.EffectiveIPAllowListEntries())
	}
	if present("domains") || present("domain") {
		dst.Host = want.Host
		dst.Hosts = slices.Clone(want.Hosts)
	}
	if present("staticPublishPath") {
		dst.PublishPath = want.PublishPath
	}
	if present("routes") {
		dst.Routes = slices.Clone(want.Routes)
	}
	if present("headers") {
		dst.Headers = slices.Clone(want.Headers)
	}
	if present("renderSubdomainPolicy") {
		dst.SubdomainPolicy = want.SubdomainPolicy
	}
	if present("maintenanceMode") {
		if want.MaintenanceMode == nil {
			dst.MaintenanceMode = nil
		} else {
			dst.MaintenanceMode = want.MaintenanceMode.DeepCopy()
		}
	}
	if present("preDeployCommand") {
		dst.PreDeployCommand = want.PreDeployCommand
	}
	if present("initialDeployHook") {
		dst.PreDeployCommand = want.PreDeployCommand
	}
	if present("envVars") {
		dst.Env = mergeBlueprintEnv(dst.Env, want.Env)
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
