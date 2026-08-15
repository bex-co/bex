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
	"reflect"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestCreateOwnedSpecFieldParity is a drift guard over the three functions
// that each enumerate the create-owned AppSpec field set by hand, with no
// shared source of truth: specFromCreate's assembly, applyCreateToSpec
// (service.go), and ApplyBlueprintServiceSpec (blueprint_plan.go). A field
// added to one enumeration but not the others half-applies on the other
// paths — silently, because nothing compared the sets until now.
//
// The guard probes each function's ACTUAL field set (build maximal valid
// inputs, run the function, record which AppSpec fields end up non-zero) and
// requires the sets to be equal modulo an explicit, commented exceptions
// list. A future field added to one enumeration but not the others therefore
// fails here loudly, naming the field.
func TestCreateOwnedSpecFieldParity(t *testing.T) {
	fromCreate := specFromCreateFieldSet(t)

	assertFieldSetsMatch(t, "specFromCreate", fromCreate,
		"applyCreateToSpec", applyCreateToSpecFieldSet(t),
		map[string]string{
			// Observed live drift (recorded, deliberately NOT fixed by the
			// refactor that added this guard): applyCreateToSpec copies
			// want.NotificationsToSend, but specFromCreate never sets it —
			// CreateRequest has no notificationsToSend field. On the
			// applyCreate upsert path (deploy.go applyCreateWithFields with
			// fields == nil) the copy therefore always writes "" over an
			// existing spec.notificationsToSend configured via the
			// notification-policy verb, and makes such an App permanently
			// "changed" to the idempotency probes.
			"NotificationsToSend": "copied by applyCreateToSpec, never produced by specFromCreate",
			// specFromCreate binds Render's registryCredentialId at create
			// time; applyCreateToSpec leaves an existing binding untouched on
			// re-apply (the registry-credential feature owns later changes).
			"RegistryCredentialID": "set at create only, preserved on the upsert path",
			// specFromCreate materializes a Blueprint scaling: block at create
			// time; applyCreateToSpec leaves spec.autoscaling alone (later
			// changes belong to SetAutoscaling / the Blueprint scaling policy).
			"Autoscaling": "set at create only, preserved on the upsert path",
		})

	assertFieldSetsMatch(t, "specFromCreate", fromCreate,
		"ApplyBlueprintServiceSpec", applyBlueprintServiceSpecFieldSet(t),
		map[string]string{
			// A cron's spec.command: specFromCreate projects req.Command, but
			// no Blueprint field policy writes dst.Command (render.yaml's
			// dockerCommand/startCommand land on spec.startCommand instead).
			"Command": "projected at create, never written by the Blueprint apply",
			// render.yaml has no port field; create defaults spec.port, the
			// Blueprint apply preserves whatever the service already has.
			"Port": "create-time default only; not a Blueprint field",
			// notifyOnFail is not part of the Blueprint schema; re-apply
			// preserves the per-service setting.
			"NotifyOnFail": "not a Blueprint field",
			// Expose derives from the service type at create; the Blueprint
			// apply never recomputes it (type is immutable on the upsert path).
			"Expose": "derived from type at create, never recomputed on apply",
			// Same asymmetry as the applyCreateToSpec list above.
			"RegistryCredentialID": "set at create only, preserved on the upsert path",
		})
}

// specFromCreateFieldSet is the union of AppSpec fields specFromCreate
// produces across a maximal set of VALID create requests (type-conditional
// fields — cron schedule/command, static publishPath/routes/headers, image vs
// repo source — cannot all ride one request).
func specFromCreateFieldSet(t *testing.T) map[string]bool {
	t.Helper()
	requests := createProbeRequests()
	assertProbeRequestsCoverEveryCreateField(t, requests)
	set := map[string]bool{}
	for _, req := range requests {
		spec, err := specFromCreate(req)
		if err != nil {
			t.Fatalf("specFromCreate(%s): %v", req.Name, err)
		}
		for field := range nonZeroSpecFields(spec) {
			set[field] = true
		}
	}
	return set
}

// createProbeRequests returns valid create requests whose union sets every
// CreateRequest field to a distinctive non-zero value (enforced by
// assertProbeRequestsCoverEveryCreateField, so a future CreateRequest field
// must be added here and its spec projection — if any — becomes visible to
// the parity assertion).
func createProbeRequests() []CreateRequest {
	shutdown := int32(45)
	cpu := int32(70)
	autoDeploy := true
	return []CreateRequest{
		{
			// A repo-backed web service carries the widest field surface one
			// valid request can. OwnerID/EnvironmentID/SecretFiles/
			// InitialDeployHook/DryRun are consumed by Create/applyCreate, not
			// specFromCreate; they are set here only to satisfy the coverage
			// guard with zero effect on the produced spec.
			OwnerID:                 "tea-probe",
			EnvironmentID:           "env-probe",
			EnvironmentSpecified:    true,
			Name:                    "probe-web",
			Type:                    appv1alpha1.TypeWebService,
			Repo:                    "https://github.com/acme/web",
			RegistryCredentialID:    strp("rc-probe"),
			Branch:                  "release",
			Runtime:                 "node",
			BuildCommand:            "npm run build",
			StartCommand:            "node server.js",
			RootDir:                 "apps/web",
			BuildFilter:             &BuildFilterView{Paths: []string{"apps/web/**"}},
			DockerfilePath:          "docker/Dockerfile",
			Port:                    8080,
			Replicas:                2,
			Plan:                    "starter",
			HealthCheckPath:         "/healthz",
			MaxShutdownDelaySeconds: &shutdown,
			Env:                     []appv1alpha1.EnvVar{{Name: "PROBE", Value: "1"}},
			SecretFiles:             []core.SecretFile{{ID: "sf-probe", Name: "probe.txt", Content: "x"}},
			Hosts:                   []string{"www.probe.example.com", "probe.example.com"},
			AutoDeploy:              &autoDeploy,
			NotifyOnFail:            "notify",
			SubdomainPolicy:         appv1alpha1.SubdomainPolicyEnabled,
			PreDeployCommand:        "bin/migrate",
			InitialDeployHook:       "bin/seed",
			IPAllowList:             []core.IPAllowListEntry{{CIDRBlock: "203.0.113.0/24", Description: "office"}},
			MaintenanceMode:         &MaintenanceModeView{Enabled: true, URI: "https://status.example.com/maintenance"},
			Autoscaling:             &SetAutoscalingRequest{MinInstances: 1, MaxInstances: 3, TargetCPUPercent: &cpu},
			DryRun:                  true,
		},
		// Prebuilt image (repo and every build-from-git field must be absent).
		{Name: "probe-image", Image: "registry.example.com/acme/api:1.2.3"},
		// Cron: schedule + command are the only type-conditional projections.
		{
			Name:     "probe-cron",
			Type:     appv1alpha1.TypeCronJob,
			Image:    "registry.example.com/acme/job:1",
			Schedule: "*/5 * * * *",
			Command:  "bin/report",
		},
		// Static site: publishPath/routes/headers, plus an explicit builder
		// (the web probe's runtime forbids a non-auto builder).
		{
			Name:        "probe-static",
			Type:        appv1alpha1.TypeStaticSite,
			Repo:        "https://github.com/acme/site",
			Builder:     "dockerfile",
			PublishPath: "dist",
			Routes:      []StaticRouteView{{Type: "redirect", Source: "/old", Destination: "/new"}},
			Headers:     []StaticHeaderView{{Path: "/", Name: "X-Probe", Value: "1"}},
		},
	}
}

// assertProbeRequestsCoverEveryCreateField keeps the probe honest: every
// CreateRequest field must be non-zero in at least one probe request, so a
// new create input cannot silently escape the parity guard.
func assertProbeRequestsCoverEveryCreateField(t *testing.T, requests []CreateRequest) {
	t.Helper()
	typ := reflect.TypeOf(CreateRequest{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		covered := false
		for _, req := range requests {
			if !reflect.ValueOf(req).Field(i).IsZero() {
				covered = true
				break
			}
		}
		if !covered {
			t.Errorf("createProbeRequests sets no non-zero value for CreateRequest.%s — add it to a probe request so its spec projection (if any) is guarded", field.Name)
		}
	}
}

// applyCreateToSpecFieldSet records which AppSpec fields applyCreateToSpec
// propagates from a fully populated desired spec onto an empty existing one.
func applyCreateToSpecFieldSet(t *testing.T) map[string]bool {
	t.Helper()
	var dst appv1alpha1.AppSpec
	applyCreateToSpec(&dst, populatedAppSpec(t))
	return nonZeroSpecFields(dst)
}

// applyBlueprintServiceSpecFieldSet records which AppSpec fields
// ApplyBlueprintServiceSpec can write, with every Blueprint field policy
// declared present. numInstances only applies when no scaling block is
// declared, so the manual-replicas branch is probed separately and unioned.
func applyBlueprintServiceSpecFieldSet(t *testing.T) map[string]bool {
	t.Helper()
	want := populatedAppSpec(t)
	allFields := map[string]BlueprintField{}
	for _, policy := range BlueprintServiceFieldPolicies {
		allFields[policy.Name] = BlueprintField{}
	}
	var withScaling appv1alpha1.AppSpec
	ApplyBlueprintServiceSpec(&withScaling, want, allFields)
	withoutScaling := map[string]BlueprintField{}
	for name := range allFields {
		if name != "scaling" {
			withoutScaling[name] = BlueprintField{}
		}
	}
	var manual appv1alpha1.AppSpec
	ApplyBlueprintServiceSpec(&manual, want, withoutScaling)
	set := nonZeroSpecFields(withScaling)
	for field := range nonZeroSpecFields(manual) {
		set[field] = true
	}
	return set
}

// populatedAppSpec returns an AppSpec with EVERY exported field set to a
// distinctive non-zero value, so a copy function's output set equals exactly
// the fields it propagates.
func populatedAppSpec(t *testing.T) appv1alpha1.AppSpec {
	t.Helper()
	var spec appv1alpha1.AppSpec
	populateValue(t, reflect.ValueOf(&spec).Elem())
	v := reflect.ValueOf(spec)
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		if field.PkgPath != "" {
			continue
		}
		if v.Field(i).IsZero() {
			t.Fatalf("populatedAppSpec left AppSpec.%s zero — extend populateValue for kind %s", field.Name, v.Field(i).Kind())
		}
	}
	return spec
}

// populateValue fills v with a non-zero value, recursing through pointers,
// structs, slices, and maps.
func populateValue(t *testing.T, v reflect.Value) {
	t.Helper()
	switch v.Kind() {
	case reflect.String:
		v.SetString("probe")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(7)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(7)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(7)
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		populateValue(t, v.Elem())
	case reflect.Slice:
		elem := reflect.New(v.Type().Elem()).Elem()
		populateValue(t, elem)
		v.Set(reflect.Append(reflect.MakeSlice(v.Type(), 0, 1), elem))
	case reflect.Map:
		key := reflect.New(v.Type().Key()).Elem()
		populateValue(t, key)
		value := reflect.New(v.Type().Elem()).Elem()
		populateValue(t, value)
		m := reflect.MakeMap(v.Type())
		m.SetMapIndex(key, value)
		v.Set(m)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).PkgPath != "" {
				continue
			}
			populateValue(t, v.Field(i))
		}
	default:
		t.Fatalf("populateValue: unsupported kind %s (%s)", v.Kind(), v.Type())
	}
}

// nonZeroSpecFields returns the names of spec's exported fields holding a
// non-zero value.
func nonZeroSpecFields(spec appv1alpha1.AppSpec) map[string]bool {
	out := map[string]bool{}
	v := reflect.ValueOf(spec)
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		if field.PkgPath != "" {
			continue
		}
		if !v.Field(i).IsZero() {
			out[field.Name] = true
		}
	}
	return out
}

// assertFieldSetsMatch fails unless the two field-name sets are equal modulo
// the explicit exceptions: every difference must be a listed exception, and
// every exception must actually be a difference (a stale excuse fails too, so
// the list can only ever shrink truthfully).
func assertFieldSetsMatch(t *testing.T, aName string, a map[string]bool, bName string, b map[string]bool, exceptions map[string]string) {
	t.Helper()
	all := map[string]bool{}
	for field := range a {
		all[field] = true
	}
	for field := range b {
		all[field] = true
	}
	differs := map[string]bool{}
	for field := range all {
		if a[field] == b[field] {
			continue
		}
		differs[field] = true
		if _, excused := exceptions[field]; !excused {
			t.Errorf("%s and %s disagree on AppSpec.%s (%s sets it: %v, %s sets it: %v) — propagate it on both paths or add it to this guard's exceptions with a reason",
				aName, bName, field, aName, a[field], bName, b[field])
		}
	}
	for field, reason := range exceptions {
		if !differs[field] {
			t.Errorf("exception for AppSpec.%s (%q) is stale — %s and %s now agree; remove it", field, reason, aName, bName)
		}
	}
}
