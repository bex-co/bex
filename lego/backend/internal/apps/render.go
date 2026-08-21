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
	"cmp"
	"context"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type envVarInput struct {
	Key   string `json:"key" jsonschema:"the environment variable name"`
	Value string `json:"value" jsonschema:"the literal value"`
}

type secretFileInput struct {
	Name    string `json:"name" jsonschema:"file name beneath /etc/secrets"`
	Content string `json:"content" jsonschema:"secret file contents"`
}

func toEnvVars(in []envVarInput) []appv1alpha1.EnvVar {
	var env []appv1alpha1.EnvVar
	for _, e := range in {
		env = append(env, appv1alpha1.EnvVar{Name: e.Key, Value: e.Value})
	}
	return env
}

func toSecretFiles(in []secretFileInput) []core.SecretFile {
	var files []core.SecretFile
	for _, f := range in {
		files = append(files, core.SecretFile{Name: f.Name, Content: f.Content})
	}
	return files
}

// render.go maps AppView onto Render's public-API "service" shape, verified
// against Render's OpenAPI spec (render-public-api-1.json). A client written for
// Render sees the field names/enums/envelopes it expects; bex-only facts (phase,
// revision) are extra fields Render clients ignore — a superset, never a break.

// renderWebService is Render's default serviceType — reported for an App with no
// explicit spec.type set (view() already defaults empty => web_service).
const renderWebService = appv1alpha1.TypeWebService

// renderPrivateService is Render's serviceType for a private-network-only
// service — the one type whose serviceDetails omits `url` (captured
// docs/render-artifacts/service-addresses.md §3).
const renderPrivateService = appv1alpha1.TypePrivateService

// renderService mirrors components.schemas.service (the fields bex has a real
// equivalent for) plus bex-native extras.
type renderService struct {
	ID                 string                           `json:"id"` // stable Render-shaped srv- id; legacy hand-applied CRs fall back to their name
	Name               string                           `json:"name"`
	Slug               string                           `json:"slug"` // globally-unique platform-host segment (w4/m19/w4/m20)
	DisplayName        string                           `json:"displayName"`
	Type               string                           `json:"type"` // serviceType enum: web_service | private_service | background_worker | cron_job
	Suspended          string                           `json:"suspended"`
	DashboardURL       string                           `json:"dashboardUrl,omitempty"`
	CreatedAt          string                           `json:"createdAt,omitempty"`
	UpdatedAt          string                           `json:"updatedAt,omitempty"`
	Owner              *renderOwner                     `json:"owner,omitempty"` // safe Render superset; official Service clients still use ownerId
	ServiceDetails     map[string]any                   `json:"serviceDetails,omitempty"`
	ImagePath          string                           `json:"imagePath,omitempty"`
	RegistryCredential *renderRegistryCredentialSummary `json:"registryCredential,omitempty"`
	// RegistryCredentialID is a bex convenience extension. Render reports the
	// canonical {id,name} summary above; retaining the id keeps GraphQL/MCP and
	// existing bex clients symmetric.
	RegistryCredentialID string   `json:"registryCredentialId,omitempty"`
	Suspenders           []string `json:"suspenders"` // who suspended it: ["user"] while suspended (bex's only suspend path), [] otherwise (w4/014)

	// OwnerID is Render's workspace-scoping field (w6/m2/t004) — omitted for
	// Apps the control-plane projector never labeled (see AppView.OwnerID).
	OwnerID string `json:"ownerId,omitempty"`
	// ProjectID/EnvironmentID are the create-time grouping forward pointers.
	// EnvironmentID is Render's field; ProjectID is a safe bex superset that
	// makes the automatic parent-project join observable without another list.
	ProjectID     string `json:"projectId,omitempty"`
	EnvironmentID string `json:"environmentId,omitempty"`

	// bex-native superset (ignored by Render clients).
	// PublicRoutingNotice explains why an exposed service has no public address
	// (w7/m79). Render has no equivalent — it always has a platform host to give
	// — so this is a bex extension rather than a parity gap.
	PublicRoutingNotice string   `json:"publicRoutingNotice,omitempty"`
	Phase               string   `json:"phase,omitempty"`
	Replicas            int32    `json:"replicas"`
	Revision            string   `json:"revision,omitempty"`
	URLs                []string `json:"urls,omitempty"`
	// Schedule/Command/Runs describe a cron_job (Render nests schedule/command
	// under cronJobDetails and exposes runs at /cron-jobs/{id}/runs); empty
	// otherwise.
	Schedule            string        `json:"schedule,omitempty"`
	Command             string        `json:"command,omitempty"`
	Runs                []CronRunView `json:"runs,omitempty"`
	LastSuccessfulRunAt string        `json:"lastSuccessfulRunAt,omitempty"`
	IdleTTLSeconds      int32         `json:"idleTTLSeconds"` // free-tier auto-sleep window (bex extension; 0 = default)
	// RootDir is the subdirectory of the repo this service builds from (Render's
	// Root Directory setting, monorepo support). Empty is the repo root.
	RootDir string `json:"rootDir,omitempty"`
	// BuildFilter is Render's Build Filters object ({paths, ignoredPaths}, verified
	// against Render's service schema as a top-level field); omitted when unset.
	BuildFilter *BuildFilterView `json:"buildFilter,omitempty"`
	// Repo/Branch are the build-from-git source, empty for an image-backed App.
	Repo   string `json:"repo,omitempty"`
	Branch string `json:"branch,omitempty"`
	// Autoscaling mirrors Render's autoscaling sub-object (minInstances /
	// maxInstances / targetCPUPercent / targetMemoryPercent); omitted when
	// autoscaling is not configured.
	Autoscaling *renderAutoscaling `json:"autoscaling,omitempty"`
	// AutoDeploy is Render's Auto-Deploy toggle: whether a signed git push
	// redeploys this service (spec.autoDeploy). Render's public API represents
	// this as the string enum "yes"|"no" (components.schemas.autoDeploy), not a
	// JSON boolean — a client generated from Render's OpenAPI spec (e.g. the
	// official CLI) fails to unmarshal a bool here.
	AutoDeploy string `json:"autoDeploy"`
	// AutoDeployTrigger is Render's newer representation of the same Auto-Deploy
	// toggle (components.schemas.autoDeployTrigger): "off"|"commit"|"checksPass".
	// bex maps its boolean spec.autoDeploy onto "commit"/"off" and never emits
	// "checksPass" (a documented divergence — bex has no CI-gated deploy). Emitted
	// alongside the legacy string enum so both Render API generations read it.
	AutoDeployTrigger string `json:"autoDeployTrigger"`
	// NotifyOnFail is Render's per-service deploy-failure notification override
	// (spec.notifyOnFail): default | notify | ignore. Required on Render's
	// service object (never omitted) — docs/render-artifacts/notify-on-fail.md.
	NotifyOnFail        string `json:"notifyOnFail"`
	NotificationsToSend string `json:"notificationsToSend"`
	// HealthCheckPath is the HTTP path the ReadinessProbe pings (w1/m23/t001);
	// empty means the default "/". Render's healthCheckPath field.
	HealthCheckPath string `json:"healthCheckPath,omitempty"`
	// RenderSubdomainPolicy is Render's renderSubdomainPolicy field
	// (enabled|disabled), controlling whether the platform .onbex.co subdomain
	// is active for this service (w7/m31). Always present and non-empty.
	RenderSubdomainPolicy string `json:"renderSubdomainPolicy"`
	// IPAllowList is Render's inbound IP allowlist for a web_service or
	// static_site: each entry is {cidrBlock, description}. Nil/omitted when
	// the allowlist is empty (open to all source IPs, Render's default).
	IPAllowList []ipAllowEntry `json:"ipAllowList,omitempty"`
}

type renderRegistryCredentialSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type renderOwner struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	Type  string `json:"type"`
}

// renderAutoscaling is Render's autoscaling sub-object shape (verified against
// Render's PUT /v1/services/{id}/autoscaling request/response contract).
type renderAutoscaling struct {
	Enabled             bool   `json:"enabled"`
	MinInstances        int32  `json:"minInstances"`
	MaxInstances        int32  `json:"maxInstances"`
	TargetCPUPercent    *int32 `json:"targetCPUPercent,omitempty"`
	TargetMemoryPercent *int32 `json:"targetMemoryPercent,omitempty"`
}

// serviceWithCursor is components.schemas.serviceWithCursor — the list-item
// envelope. Render's GET /v1/services returns an array of these.
type serviceWithCursor struct {
	Service renderService `json:"service"`
	Cursor  string        `json:"cursor"`
}

// serviceAndDeploy is components.schemas.serviceAndDeploy — the envelope
// Render's POST /v1/services (create) returns, verified against the
// render-oss/cli generated client (ServiceAndDeploy: {deployId?, service?}). A
// client that unwraps `.service` from the create response (the official CLI's
// ServiceRepo.CreateService does exactly this) breaks against a bare service
// object. DeployID is omitted when no control-plane store is active (no deploy
// row is minted without a store) — an honest subset, never a wrong value.
type serviceAndDeploy struct {
	Service  renderService `json:"service"`
	DeployID string        `json:"deployId,omitempty"`
}

// renderServiceInstance is the official CLI's serviceInstance wire shape.
type renderServiceInstance struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
}

// yesNoEnum renders Render's autoDeploy bool as its wire enum ("yes"/"no").
func yesNoEnum(autoDeploy bool) string {
	if autoDeploy {
		return "yes"
	}
	return "no"
}

// triggerEnum renders Render's autoDeployTrigger for bex's boolean autoDeploy:
// "commit" (a push redeploys) or "off". bex never emits "checksPass" — it has no
// CI-gated deploy trigger (documented divergence, docs/ADR018-render-parity.md).
func triggerEnum(autoDeploy bool) string {
	if autoDeploy {
		return "commit"
	}
	return "off"
}

func toRenderService(a AppView) renderService {
	return toRenderServiceWithMetadata(a, resourcemeta.Config{})
}

func toRenderServiceWithMetadata(a AppView, metadata resourcemeta.Config) renderService {
	publicID := a.ID
	if publicID == "" {
		publicID = a.Name // hand-applied/store-less compatibility
	}
	svcType := effectiveType(a.Type) // defensive; view() already defaults this
	region := metadata.PlatformRegion()
	if region == "" {
		region = a.Region
	}
	dashboardURL := metadata.DashboardURL(resourcemeta.ServiceDashboardRoute(svcType), publicID)
	if dashboardURL == "" {
		dashboardURL = a.DashboardURL
	}
	registryCredentialID := ""
	if a.RegistryCredentialID != nil {
		registryCredentialID = *a.RegistryCredentialID
	}
	return renderService{
		ID:                    publicID,
		Name:                  renderServiceName(a),
		Slug:                  a.Slug,
		DisplayName:           a.DisplayName,
		Type:                  svcType,
		Suspended:             core.SuspendedEnum(a.Suspended),
		DashboardURL:          dashboardURL,
		CreatedAt:             a.CreatedAt,
		UpdatedAt:             a.UpdatedAt,
		ServiceDetails:        renderServiceDetails(a, svcType, region),
		ImagePath:             a.SourceImage,
		RegistryCredentialID:  registryCredentialID,
		Suspenders:            suspenders(a.Suspended),
		OwnerID:               a.OwnerID,
		ProjectID:             a.ProjectID,
		EnvironmentID:         a.EnvironmentID,
		Phase:                 a.Phase,
		PublicRoutingNotice:   a.PublicRoutingNotice,
		Replicas:              a.Replicas,
		Revision:              a.Revision,
		URLs:                  a.URLs,
		Schedule:              a.Schedule,
		Command:               a.Command,
		Runs:                  a.Runs,
		LastSuccessfulRunAt:   a.LastSuccessfulRunAt,
		IdleTTLSeconds:        a.IdleTTLSeconds,
		RootDir:               a.RootDir,
		BuildFilter:           a.BuildFilter,
		Repo:                  a.Repo,
		Branch:                a.Branch,
		Autoscaling:           toRenderAutoscaling(a.Autoscaling),
		AutoDeploy:            yesNoEnum(a.AutoDeploy),
		AutoDeployTrigger:     triggerEnum(a.AutoDeploy),
		NotifyOnFail:          a.NotifyOnFail,
		NotificationsToSend:   a.NotificationsToSend,
		RenderSubdomainPolicy: a.RenderSubdomainPolicy,
		HealthCheckPath:       a.HealthCheckPath,
		IPAllowList:           cloneIPAllowEntries(a.IPAllowList),
	}
}

// renderServiceDetails builds Render's serviceDetails map. Most keys are plain
// omit-when-empty strings and travel through the table; the rest carry a
// type-specific condition or a nested shape Render specifies exactly.
func renderServiceDetails(a AppView, svcType, region string) map[string]any {
	details := map[string]any{
		"numInstances": int(a.Replicas),
	}
	for _, field := range []struct{ key, value string }{
		// internalAddress is a documented bex extension (ADR041 D4): Render's REST
		// has no internal-address field — its consumers derive `<slug>:<port>` from
		// slug + port — but agents and scripts benefit from the explicit, additive key.
		{"internalAddress", a.InternalAddress},
		{"plan", a.Plan}, // webServiceDetails.plan (render-public-api-1.json)
		{"sshAddress", a.SSHAddress},
		{"region", region},
		{"schedule", a.Schedule},                       // cronJobDetails.schedule (render-public-api-1.json)
		{"command", a.Command},                         // cronJobDetails.command (render-public-api-1.json)
		{"lastSuccessfulRunAt", a.LastSuccessfulRunAt}, // cronJobDetails.lastSuccessfulRunAt
		{"publishPath", a.PublishPath},                 // staticSiteDetails.publishPath (render-public-api-1.json)
		{"preDeployCommand", a.PreDeployCommand},       // webServiceDetails.preDeployCommand (w1/m33)
		{"initialDeployHook", a.InitialDeployHook},     // w2/m45: blueprint-only one-time first-deploy command
		{"healthCheckPath", a.HealthCheckPath},
		{"renderSubdomainPolicy", a.RenderSubdomainPolicy},
	} {
		if field.value != "" {
			details[field.key] = field.value
		}
	}
	// Render's serviceDetails carries url for web_service/static_site but
	// OMITS it for private_service (captured 2026-07-26,
	// docs/render-artifacts/service-addresses.md §3) — bex used to leak the
	// cluster-internal URL there; the address now travels in internalAddress.
	if a.URL != "" && svcType != renderPrivateService {
		details["url"] = a.URL // Render web_service exposes the live URL here
	}
	if a.Runtime != "" {
		details["runtime"] = a.Runtime
		details["env"] = a.Runtime // deprecated Render response field, still required by its schema
	}
	if specific, ok := envSpecificDetails(a, svcType); ok {
		details["envSpecificDetails"] = specific
	}
	if svcType == appv1alpha1.TypeStaticSite {
		details["buildCommand"] = a.BuildCommand
	}
	if a.MaxShutdownDelaySeconds > 0 {
		// Render places this on web/private/background-worker serviceDetails.
		// view() supplies the shared Render/Kubernetes default (30) when the CR
		// pointer is unset, so legacy services read back exactly like Render.
		details["maxShutdownDelaySeconds"] = a.MaxShutdownDelaySeconds
	}
	if svcType == renderWebService {
		// Render's maintenanceMode is required on webServiceDetails (never
		// omitted), unlike the mostly-optional fields above — docs/render-
		// artifacts/maintenance-mode.md.
		details["maintenanceMode"] = map[string]any{
			"enabled": a.MaintenanceMode.Enabled,
			"uri":     a.MaintenanceMode.URI,
		}
	}
	return details
}

// envSpecificDetails mirrors Render's runtime-keyed envSpecificDetails: the
// docker shape for image builds, the buildpack shape for every other declared
// runtime. Absent entirely when the service declares no runtime.
func envSpecificDetails(a AppView, svcType string) (map[string]any, bool) {
	if a.Runtime == "" {
		return nil, false
	}
	if a.Runtime == "docker" {
		docker := map[string]any{
			"dockerCommand":  a.StartCommand,
			"dockerContext":  cmp.Or(a.DockerContext, a.RootDir),
			"dockerfilePath": a.DockerfilePath,
		}
		if a.RegistryCredentialID != nil && *a.RegistryCredentialID != "" {
			docker["registryCredentialId"] = *a.RegistryCredentialID
		}
		if a.PreDeployCommand != "" {
			docker["preDeployCommand"] = a.PreDeployCommand
		}
		return docker, true
	}
	startCommand := a.StartCommand
	if svcType == appv1alpha1.TypeCronJob {
		startCommand = a.Command
	}
	return map[string]any{
		"buildCommand":     a.BuildCommand,
		"startCommand":     startCommand,
		"preDeployCommand": a.PreDeployCommand,
	}, true
}

func toRenderAutoscaling(as *AutoscalingView) *renderAutoscaling {
	if as == nil {
		return nil
	}
	return &renderAutoscaling{
		Enabled:             as.Enabled,
		MinInstances:        as.MinInstances,
		MaxInstances:        as.MaxInstances,
		TargetCPUPercent:    as.TargetCPUPercent,
		TargetMemoryPercent: as.TargetMemoryPercent,
	}
}

func ownerToRender(owner resourcemeta.Owner) *renderOwner {
	if !owner.Available() {
		return nil
	}
	return &renderOwner{ID: owner.ID, Name: owner.Name, Email: owner.Email, Type: owner.Type}
}

// renderService projects metadata shared by REST/GraphQL/MCP without adding
// REST-only nested owner structure.
func (s *Service) renderService(a AppView) renderService {
	return toRenderServiceWithMetadata(a, s.Metadata)
}

// restServices enriches a whole page through one owner batch lookup and one
// registry-credential batch lookup (each at most one query per page).
func (s *Service) restServices(ctx context.Context, apps []AppView) []renderService {
	ownerIDs := make([]string, 0, len(apps))
	for _, app := range apps {
		ownerIDs = append(ownerIDs, app.OwnerID)
	}
	owners := resourcemeta.ResolveOwners(ctx, s.Owners, ownerIDs)
	credNames := s.resolveCredentialNames(ctx, apps)
	out := make([]renderService, 0, len(apps))
	for _, app := range apps {
		rendered := s.renderService(app)
		if owner, ok := owners[app.OwnerID]; ok {
			rendered.Owner = ownerToRender(owner)
		}
		if app.RegistryCredentialID != nil && *app.RegistryCredentialID != "" {
			id := *app.RegistryCredentialID
			if name, ok := credNames[id]; ok {
				rendered.RegistryCredential = &renderRegistryCredentialSummary{ID: id, Name: name}
			}
		}
		out = append(out, rendered)
	}
	return out
}

// resolveCredentialNames collects the unique credential ids from apps and
// resolves them per owning workspace (one IN(…) batch per workspace, not per
// app). Grouping by each App's OwnerID keeps the lookup workspace-scoped — a
// credential id is only ever resolved against the workspace of the App that
// references it, so display enrichment can never reach across tenants. In
// practice a render is single-workspace, so this is one query.
func (s *Service) resolveCredentialNames(ctx context.Context, apps []AppView) map[string]string {
	if s.RegistryCreds == nil {
		return nil
	}
	seen := make(map[string]struct{})
	byWorkspace := make(map[string][]string)
	for _, app := range apps {
		if app.RegistryCredentialID == nil || *app.RegistryCredentialID == "" {
			continue
		}
		id := *app.RegistryCredentialID
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		byWorkspace[app.OwnerID] = append(byWorkspace[app.OwnerID], id)
	}
	if len(byWorkspace) == 0 {
		return nil
	}
	out := make(map[string]string)
	for workspaceID, ids := range byWorkspace {
		for id, name := range s.RegistryCreds.ResolveCredentialNames(ctx, workspaceID, ids) {
			out[id] = name
		}
	}
	return out
}

func (s *Service) restService(ctx context.Context, app AppView) renderService {
	return s.restServices(ctx, []AppView{app})[0]
}

// renderServiceName maps bex's immutable public name + mutable display label
// onto Render's contract, where service.name itself is mutable. The typed id
// remains stable and continues to address the App CR through LabelAppID.
func renderServiceName(a AppView) string {
	if a.DisplayName != "" {
		return a.DisplayName
	}
	return a.Name
}

// renderCronJobRun mirrors Render's cronJobRun schema. triggeredBy/canceledBy
// are omitted because Kubernetes Jobs do not retain the caller identity; an
// honest subset is preferable to fabricating an actor.
type renderCronJobRun struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	StartedAt   string `json:"startedAt,omitempty"`
	FinishedAt  string `json:"finishedAt,omitempty"`
	TriggeredBy string `json:"triggeredBy,omitempty"`
	CanceledBy  string `json:"canceledBy,omitempty"`
}

func toRenderCronJobRun(run CronRunView) renderCronJobRun {
	return renderCronJobRun{
		ID: run.ID, Status: run.Status,
		StartedAt: run.StartedAt, FinishedAt: run.FinishedAt,
	}
}

// cronJobRunWithCursor is the standard Render list-item envelope. Render's
// current public spec does not expose this historical list route; bex keeps the
// milestone's first-class list as a documented extension using Render's normal
// {resource,cursor} grammar.
type cronJobRunWithCursor struct {
	CronJobRun renderCronJobRun `json:"cronJobRun"`
	Cursor     string           `json:"cursor"`
}

func toCronJobRunList(runs []CronRunView) []cronJobRunWithCursor {
	out := make([]cronJobRunWithCursor, 0, len(runs))
	for _, run := range runs {
		out = append(out, cronJobRunWithCursor{CronJobRun: toRenderCronJobRun(run), Cursor: run.ID})
	}
	return out
}

// StaticRouteView / StaticHeaderView already carry Render's own wire shapes
// (route: type/source/destination, header: path/name/value), so the /routes and
// /headers endpoints serialize them directly. These two only normalize an empty
// list to `[]` — Render never returns `null` for either collection.
func toRenderRoutes(routes []StaticRouteView) []StaticRouteView {
	if routes == nil {
		return []StaticRouteView{}
	}
	return routes
}

func toRenderHeaders(headers []StaticHeaderView) []StaticHeaderView {
	if headers == nil {
		return []StaticHeaderView{}
	}
	return headers
}

// toRenderServices maps a slice of AppViews to bare Render service objects (no
// cursor envelope) — the shape the MCP list_services tool returns.
func toRenderServices(apps []AppView) []renderService {
	out := make([]renderService, 0, len(apps))
	for _, a := range apps {
		out = append(out, toRenderService(a))
	}
	return out
}

func (s *Service) restServiceList(ctx context.Context, apps []AppView) []serviceWithCursor {
	rendered := s.restServices(ctx, apps)
	out := make([]serviceWithCursor, 0, len(apps))
	for i, app := range apps {
		out = append(out, serviceWithCursor{Service: rendered[i], Cursor: app.Name})
	}
	return out
}
