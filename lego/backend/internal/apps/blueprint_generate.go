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

// blueprint_generate.go: Render's "Generate Blueprint" for bex (w8/m22 +
// w4/040) — serialize selected live resources back into a render.yaml the
// platform's own validator accepts. The emit set is exactly what the ADR049
// capability registry marks translated/equivalent; secret VALUES never appear
// (env vars backed by secrets emit `sync: false` name-only, or the
// fromDatabase/fromService/fromGroup reference form when the wiring is
// derivable and the target is in the same selection; selected env groups emit
// root envVarGroups with generateValue keys). Every generated manifest is
// self-checked through the real compiler+parser before it is returned.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// EnvNameSource lists a service's mutable-store env var NAMES — never values
// (*secrets.Service satisfies it, w8/m22). nil ⇒ mutable-store vars are
// omitted from generated manifests.
type EnvNameSource interface {
	ListEnvVarNames(ctx context.Context, service string) ([]string, error)
}

// EnvGroupExportSource returns an env group's display name and env-var key
// names for Blueprint generation (w4/040) — never values. *envgroups.Service
// satisfies it. nil ⇒ env-group selection / fromGroup linkage are omitted.
type EnvGroupExportSource interface {
	ExportEnvGroup(ctx context.Context, gid string) (name string, keys []string, err error)
}

// GenerateBlueprintRequest selects the resources to export.
type GenerateBlueprintRequest struct {
	OwnerID     string
	ServiceIDs  []string
	PostgresIDs []string
	KeyValueIDs []string
	EnvGroupIDs []string
}

// GenerateBlueprintResult is the generated manifest plus the filename the
// dashboard offers for download.
type GenerateBlueprintResult struct {
	Manifest string `json:"manifest"`
	Filename string `json:"filename"`
}

// blueprintPlanSpelling maps a bex compute tier id to render.yaml's plan enum
// spelling (multi-word plans use spaces there: "pro plus").
func blueprintPlanSpelling(tier string) string {
	return strings.ReplaceAll(tier, "-", " ")
}

// blueprintTypeSpelling reverses normalizeType to render.yaml's short forms.
var blueprintTypeSpelling = map[string]string{
	appv1alpha1.TypeWebService:       "web",
	appv1alpha1.TypePrivateService:   "pserv",
	appv1alpha1.TypeBackgroundWorker: "worker",
	appv1alpha1.TypeCronJob:          "cron",
	appv1alpha1.TypeStaticSite:       "web", // + runtime: static
}

// reverse property maps for link derivation (the forward maps live in
// deploy.go's resolve* helpers).
var dbSecretKeyProperty = map[string]string{
	"uri": "connectionString", "host": "host", "port": "port",
	"username": "user", "password": "password", "dbname": "database",
}

var kvSecretKeyProperty = map[string]string{
	"uri": "connectionString", "host": "host", "port": "port", "password": "password",
}

// GenerateBlueprint exports the selected resources as a render.yaml manifest.
// Requires can_view_sensitive on each resource (env wiring is sensitive
// metadata, though no secret value is ever read or emitted).
func (s *Service) GenerateBlueprint(ctx context.Context, req GenerateBlueprintRequest) (GenerateBlueprintResult, error) {
	if req.OwnerID != "" {
		ctx = core.WithWorkspace(ctx, req.OwnerID)
	}
	// Workspace-level gate first (the guard sweep's contract); each selected
	// resource is then authorized against ITS OWN workspace below. The
	// two-gate shape means a caller must hold can_view_sensitive in their
	// resolved workspace AND on each resource — acceptable here because the
	// dashboard scopes a selection to one ownerId.
	if err := s.Authorize(ctx, core.RelCanViewSensitive); err != nil {
		return GenerateBlueprintResult{}, err
	}
	if len(req.ServiceIDs)+len(req.PostgresIDs)+len(req.KeyValueIDs)+len(req.EnvGroupIDs) == 0 {
		return GenerateBlueprintResult{}, fmt.Errorf("%w: select at least one resource to generate a Blueprint from", core.ErrBadRequest)
	}

	var databases []*appv1alpha1.Database
	dbDisplayByID := map[string]string{} // CR name (dpg-…) → display name
	for _, id := range req.PostgresIDs {
		d, err := s.AuthorizeDatabase(ctx, core.RelCanViewSensitive, id)
		if err != nil {
			return GenerateBlueprintResult{}, err
		}
		databases = append(databases, d)
		dbDisplayByID[d.Name] = d.Spec.Name
	}
	var keyValues []*appv1alpha1.KeyValue
	kvDisplayByID := map[string]string{}
	for _, id := range req.KeyValueIDs {
		kv, err := s.AuthorizeKeyValue(ctx, core.RelCanViewSensitive, id)
		if err != nil {
			return GenerateBlueprintResult{}, err
		}
		keyValues = append(keyValues, kv)
		kvDisplayByID[kv.Name] = kv.Spec.Name
	}

	// Selected env groups: explicit EnvGroupIDs only (same contract as
	// postgresIds/keyValueIds — never auto-include links from services).
	selectedGroups := map[string]exportedEnvGroup{} // gid → export
	groupCache := map[string]exportedEnvGroup{}     // gid → export (selected + unselected resolve)
	if s.EnvGroupExport != nil {
		for _, gid := range req.EnvGroupIDs {
			eg, err := s.resolveEnvGroupExport(ctx, gid, groupCache)
			if err != nil {
				return GenerateBlueprintResult{}, err
			}
			selectedGroups[gid] = eg
		}
	} else if len(req.EnvGroupIDs) > 0 {
		return GenerateBlueprintResult{}, fmt.Errorf("%w: environment groups are unavailable", core.ErrSecretsUnavailable)
	}

	var services []map[string]any
	for _, id := range req.ServiceIDs {
		a, err := s.AuthorizeApp(ctx, core.RelCanViewSensitive, id)
		if err != nil {
			return GenerateBlueprintResult{}, err
		}
		entry, err := s.generateServiceEntry(ctx, a, dbDisplayByID, kvDisplayByID, selectedGroups, groupCache)
		if err != nil {
			return GenerateBlueprintResult{}, err
		}
		services = append(services, entry)
	}
	for _, kv := range keyValues {
		services = append(services, generateKeyValueEntry(kv))
	}

	doc := map[string]any{}
	if len(services) > 0 {
		doc["services"] = services
	}
	if len(databases) > 0 {
		var out []map[string]any
		for _, d := range databases {
			out = append(out, generateDatabaseEntry(d))
		}
		doc["databases"] = out
	}
	if len(req.EnvGroupIDs) > 0 {
		var out []map[string]any
		for _, gid := range req.EnvGroupIDs {
			out = append(out, generateEnvGroupEntry(selectedGroups[gid]))
		}
		doc["envVarGroups"] = out
	}

	raw, err := yaml.Marshal(doc)
	if err != nil {
		return GenerateBlueprintResult{}, fmt.Errorf("encoding generated Blueprint: %w", err)
	}
	manifest := string(raw)

	// Self-check: a generated manifest the platform's own validator rejects
	// is a generator bug, never a user error — fail loudly.
	source, ir, problems := CompileBlueprintIR(manifest)
	if len(problems) > 0 {
		return GenerateBlueprintResult{}, fmt.Errorf("generated Blueprint failed self-validation (%s); this is a bex bug", problems[0].Message)
	}
	if _, err := parseCompiledStack(blueprintParseOverrides{}, source, ir); err != nil {
		return GenerateBlueprintResult{}, fmt.Errorf("generated Blueprint failed self-validation (%v); this is a bex bug", err)
	}
	return GenerateBlueprintResult{Manifest: manifest, Filename: CanonicalBlueprintFilename}, nil
}

// exportedEnvGroup is the Blueprint-facing projection of one env group.
type exportedEnvGroup struct {
	name string
	keys []string
}

func (s *Service) resolveEnvGroupExport(ctx context.Context, gid string, cache map[string]exportedEnvGroup) (exportedEnvGroup, error) {
	if eg, ok := cache[gid]; ok {
		return eg, nil
	}
	name, keys, err := s.EnvGroupExport.ExportEnvGroup(ctx, gid)
	if err != nil {
		return exportedEnvGroup{}, err
	}
	eg := exportedEnvGroup{name: name, keys: keys}
	cache[gid] = eg
	return eg, nil
}

func generateEnvGroupEntry(eg exportedEnvGroup) map[string]any {
	vars := make([]map[string]any, 0, len(eg.keys))
	for _, key := range eg.keys {
		// generateValue (not empty literals): re-apply to an existing group
		// keeps live values; a fresh workspace mints secrets once. Emitting
		// empty value: would wipe on ApplyEnvGroup.
		vars = append(vars, map[string]any{"key": key, "generateValue": true})
	}
	return map[string]any{"name": eg.name, "envVars": vars}
}

func (s *Service) generateServiceEntry(ctx context.Context, a *appv1alpha1.App, dbDisplayByID, kvDisplayByID map[string]string, selectedGroups map[string]exportedEnvGroup, groupCache map[string]exportedEnvGroup) (map[string]any, error) {
	svcType := effectiveType(a.Spec.Type)
	entry := map[string]any{
		// The manifest-facing PUBLIC name, never the tenant-prefixed CR object
		// name: a store-managed App's a.Name is CRName(tenant, name), which
		// overruns ValidAppName's 30-char cap (so the create boundary
		// validateBlueprint runs would reject the file this exporter tells the
		// user to commit) and writes the workspace's tenant id into that repo.
		// appServiceName reads LabelServiceName, falling back to a.Name only for
		// the legacy hand-applied App that has no such label (its object name IS
		// the public name). Datastore entries already emit Spec.Name; this
		// aligns services with them. (w6/m114)
		"name": appServiceName(a),
		"type": blueprintTypeSpelling[svcType],
	}
	static := svcType == appv1alpha1.TypeStaticSite

	switch {
	case a.Spec.Image != "":
		entry["runtime"] = "image"
		entry["image"] = map[string]any{"url": a.Spec.Image}
	case static:
		entry["runtime"] = "static"
	case a.Spec.Runtime != "":
		entry["runtime"] = a.Spec.Runtime
	}
	if a.Spec.Repo != "" {
		entry["repo"] = a.Spec.Repo
		if a.Spec.Branch != "" && a.Spec.Branch != appv1alpha1.DefaultBranch {
			entry["branch"] = a.Spec.Branch
		}
	}
	if a.Spec.RootDir != "" {
		entry["rootDir"] = a.Spec.RootDir
	}
	if a.Spec.DockerfilePath != "" {
		entry["dockerfilePath"] = a.Spec.DockerfilePath
	}
	if a.Spec.DockerContext != "" {
		entry["dockerContext"] = a.Spec.DockerContext
	}
	if a.Spec.BuildCommand != "" {
		entry["buildCommand"] = a.Spec.BuildCommand
	}
	if a.Spec.StartCommand != "" {
		if a.Spec.Runtime == "docker" {
			entry["dockerCommand"] = a.Spec.StartCommand
		} else if !static {
			entry["startCommand"] = a.Spec.StartCommand
		}
	}
	if !static && a.Spec.Tier != "" && a.Spec.Tier != tiers.Compute.Default().ID {
		entry["plan"] = blueprintPlanSpelling(a.Spec.Tier)
	}
	if svcType == appv1alpha1.TypeCronJob {
		entry["schedule"] = a.Spec.Schedule
		// A cron's command override lives in Spec.Command (the PATCH path),
		// which render.yaml spells startCommand (or dockerCommand for docker).
		if a.Spec.Command != "" {
			if a.Spec.Runtime == "docker" {
				entry["dockerCommand"] = a.Spec.Command
			} else {
				entry["startCommand"] = a.Spec.Command
			}
		}
	}
	if a.Spec.HealthCheckPath != "" && svcType == appv1alpha1.TypeWebService {
		entry["healthCheckPath"] = a.Spec.HealthCheckPath
	}
	if static && a.Spec.PublishPath != "" {
		entry["staticPublishPath"] = a.Spec.PublishPath
	}
	switch {
	case a.Spec.Autoscaling != nil && a.Spec.Autoscaling.Enabled &&
		svcType != appv1alpha1.TypeBackgroundWorker:
		scaling := map[string]any{
			"minInstances": a.Spec.Autoscaling.MinReplicas,
			"maxInstances": a.Spec.Autoscaling.MaxReplicas,
		}
		if a.Spec.Autoscaling.TargetCPUPercent != nil {
			scaling["targetCPUPercent"] = *a.Spec.Autoscaling.TargetCPUPercent
		}
		if a.Spec.Autoscaling.TargetMemoryPercent != nil {
			scaling["targetMemoryPercent"] = *a.Spec.Autoscaling.TargetMemoryPercent
		}
		entry["scaling"] = scaling
	case a.Spec.Autoscaling != nil && a.Spec.Autoscaling.Enabled:
		// render.yaml rejects scaling on background workers; the closest
		// truthful export is the autoscaler's upper bound as a fixed count.
		entry["numInstances"] = a.Spec.Autoscaling.MaxReplicas
	case a.Spec.Replicas > 1:
		entry["numInstances"] = a.Spec.Replicas
	}
	// The primary custom domain lives in Spec.Host; Spec.Hosts carries only
	// the additional ones — emit the union or a single-domain service loses
	// its domain entirely.
	if domains := appDomains(a); len(domains) > 0 {
		entry["domains"] = domains
	}
	if !a.Spec.AutoDeploy && a.Spec.Repo != "" {
		entry["autoDeployTrigger"] = "off"
	}
	if a.Spec.PreDeployCommand != "" && !static && svcType != appv1alpha1.TypeCronJob {
		entry["preDeployCommand"] = a.Spec.PreDeployCommand
	}

	envVars, err := s.generateEnvVars(ctx, a, dbDisplayByID, kvDisplayByID, selectedGroups, groupCache)
	if err != nil {
		return nil, err
	}
	if len(envVars) > 0 {
		entry["envVars"] = envVars
	}
	return entry, nil
}

// generateEnvVars classifies an App's env wiring for export: literals keep
// their value; a SecretKeyRef into a SELECTED datastore's connection Secret
// becomes the reference form; every other secret-backed var — including the
// mutable-store vars whose names the EnvNames seam lists — emits
// `sync: false` name-only. Linked env groups (Spec.EnvFromSecrets) emit
// fromGroup when selected, else degrade each group key to sync:false (same
// dangling-free rule as unselected datastores). No secret value is ever read.
func (s *Service) generateEnvVars(ctx context.Context, a *appv1alpha1.App, dbDisplayByID, kvDisplayByID map[string]string, selectedGroups map[string]exportedEnvGroup, groupCache map[string]exportedEnvGroup) ([]map[string]any, error) {
	var out []map[string]any
	seen := map[string]bool{}
	for _, env := range a.Spec.Env {
		if env.Name == "" || seen[env.Name] {
			continue
		}
		seen[env.Name] = true
		if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
			out = append(out, map[string]any{"key": env.Name, "value": env.Value})
			continue
		}
		ref := env.ValueFrom.SecretKeyRef
		if entry, ok := datastoreReference(env.Name, ref, dbDisplayByID, kvDisplayByID); ok {
			out = append(out, entry)
			continue
		}
		// Secret-backed with no portable reference form: name only.
		out = append(out, map[string]any{"key": env.Name, "sync": false})
	}
	if s.EnvNames != nil {
		if names, err := s.EnvNames.ListEnvVarNames(ctx, a.Name); err == nil {
			for _, name := range names {
				if name == "" || seen[name] {
					continue
				}
				seen[name] = true
				out = append(out, map[string]any{"key": name, "sync": false})
			}
		}
	}
	if s.EnvGroupExport != nil {
		for _, secret := range a.Spec.EnvFromSecrets {
			gid, ok := envGroupIDFromSecret(secret)
			if !ok {
				continue
			}
			if eg, selected := selectedGroups[gid]; selected {
				out = append(out, map[string]any{"fromGroup": eg.name})
				continue
			}
			// Unselected linked group: expand keys as sync:false (no dangling
			// fromGroup). Authz denial fails closed; missing group is skipped.
			eg, err := s.resolveEnvGroupExport(ctx, gid, groupCache)
			if err != nil {
				if errors.Is(err, core.ErrNotFound) {
					continue
				}
				return nil, err
			}
			for _, key := range eg.keys {
				if key == "" || seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, map[string]any{"key": key, "sync": false})
			}
		}
	}
	return out, nil
}

// envGroupIDFromSecret reverses envgroups.envSecretName (<gid>-env).
func envGroupIDFromSecret(secret string) (string, bool) {
	if !strings.HasSuffix(secret, "-env") {
		return "", false
	}
	gid := strings.TrimSuffix(secret, "-env")
	if !strings.HasPrefix(gid, "evg-") {
		return "", false
	}
	return gid, true
}

// datastoreReference reverses deploy.go's resolveDatabaseRef/resolveKeyValueRef
// secret wiring back to the render.yaml reference form — only when the target
// datastore is part of the same selection, so the generated file parses.
func datastoreReference(key string, ref *appv1alpha1.SecretKeySelector, dbDisplayByID, kvDisplayByID map[string]string) (map[string]any, bool) {
	if display, ok := kvDisplayByID[ref.Name]; ok {
		property, known := kvSecretKeyProperty[ref.Key]
		if !known {
			return nil, false
		}
		return map[string]any{
			"key":         key,
			"fromService": map[string]any{"name": display, "type": "keyvalue", "property": property},
		}, true
	}
	secretName := ref.Name
	pooler := strings.HasSuffix(secretName, "-pooler-app")
	crName := strings.TrimSuffix(strings.TrimSuffix(secretName, "-pooler-app"), "-app")
	display, ok := dbDisplayByID[crName]
	if !ok {
		return nil, false
	}
	property, known := dbSecretKeyProperty[ref.Key]
	if !known {
		return nil, false
	}
	if pooler && property == "connectionString" {
		property = "connectionPoolString"
	}
	return map[string]any{
		"key":          key,
		"fromDatabase": map[string]any{"name": display, "property": property},
	}, true
}

// appDomains is the primary custom domain plus the additional ones.
func appDomains(a *appv1alpha1.App) []string {
	var out []string
	if a.Spec.Host != "" {
		out = append(out, a.Spec.Host)
	}
	return append(out, a.Spec.Hosts...)
}

func generateDatabaseEntry(d *appv1alpha1.Database) map[string]any {
	entry := map[string]any{"name": d.Spec.Name}
	if d.Spec.Plan != "" {
		entry["plan"] = d.Spec.Plan
	}
	if d.Spec.StorageGB > 0 {
		if t, ok := tiers.Postgres.ByID(d.Spec.Plan); !ok || d.Spec.StorageGB > t.StorageGB {
			entry["diskSizeGB"] = d.Spec.StorageGB
		}
	}
	if d.Spec.Version != "" {
		entry["postgresMajorVersion"] = d.Spec.Version
	}
	if d.Spec.HighAvailability {
		entry["highAvailability"] = map[string]any{"enabled": true}
	}
	if len(d.Spec.ReadReplicas) > 0 {
		var replicas []map[string]any
		for _, r := range d.Spec.ReadReplicas {
			replicas = append(replicas, map[string]any{"name": r.Name})
		}
		entry["readReplicas"] = replicas
	}
	if d.Spec.DatabaseName != "" {
		entry["databaseName"] = d.Spec.DatabaseName
	}
	if d.Spec.DatabaseUser != "" {
		entry["user"] = d.Spec.DatabaseUser
	}
	if len(d.Spec.IPAllowList) > 0 {
		entry["ipAllowList"] = allowListEntries(d.Spec.IPAllowList)
	}
	return entry
}

func generateKeyValueEntry(kv *appv1alpha1.KeyValue) map[string]any {
	entry := map[string]any{
		"name": kv.Spec.Name,
		"type": "keyvalue",
		// ipAllowList is required by the schema for key value instances; an
		// empty list is the explicit internal-only shape.
		"ipAllowList": allowListEntries(kv.Spec.IPAllowList),
	}
	if kv.Spec.Plan != "" && kv.Spec.Plan != tiers.Valkey.Default().ID {
		entry["plan"] = kv.Spec.Plan
	}
	if kv.Spec.MaxmemoryPolicy != "" {
		entry["maxmemoryPolicy"] = kv.Spec.MaxmemoryPolicy
	}
	if kv.Spec.PersistenceMode != "" {
		entry["persistenceMode"] = kv.Spec.PersistenceMode
	}
	return entry
}

func allowListEntries(entries []appv1alpha1.IPAllowEntry) []map[string]any {
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		entry := map[string]any{"source": e.CIDR}
		if e.Description != "" {
			entry["description"] = e.Description
		}
		out = append(out, entry)
	}
	return out
}
