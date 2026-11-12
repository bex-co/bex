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
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// applyVerbs are a Blueprint apply's own audit rows, which name no service.
var applyVerbs = map[string]bool{
	"apps.CreateBlueprint": true, "apps.CreatePlannedBlueprint": true, "apps.PlanBlueprint": true,
	"apps.SyncBlueprint": true, "apps.ApproveBlueprint": true, "apps.DeployStack": true,
}

// serviceRows names every audit row sink recorded but the apply's own, in
// order, each with its typed details and the caller it names.
func serviceRows(sink *captureAuditSink) []string {
	var out []string
	for _, ev := range sink.events {
		if applyVerbs[ev.Verb] {
			continue
		}
		row := ev.Verb
		switch ev.Verb {
		case core.AuditVerbScale:
			row += fmt.Sprintf(" %d->%d", ptr.Deref(ev.InstanceCountFrom, -1), ptr.Deref(ev.InstanceCountTo, -1))
		case core.AuditVerbSetAutoscaling, core.AuditVerbDeleteAutoscaling:
			from, to := "none", "off"
			if ev.AutoscalingMinFrom != nil {
				from = fmt.Sprintf("%d..%d", *ev.AutoscalingMinFrom, ptr.Deref(ev.AutoscalingMaxFrom, -1))
				if !ptr.Deref(ev.AutoscalingEnabledFrom, false) {
					from += "(off)"
				}
			}
			if ev.AutoscalingMinTo != nil {
				to = fmt.Sprintf("%d..%d cpu %d", *ev.AutoscalingMinTo, ptr.Deref(ev.AutoscalingMaxTo, -1), ptr.Deref(ev.AutoscalingCPUTo, 0))
			}
			row += " " + from + "->" + to
		case core.AuditVerbSetAutoDeploy:
			row += fmt.Sprintf(" enabled=%t %s", ptr.Deref(ev.AutoDeployEnabled, false), ptr.Deref(ev.AutoDeployTrigger, ""))
			if ev.AutoDeployReason != nil {
				row += " " + *ev.AutoDeployReason
			}
		case core.AuditVerbAddDomain, core.AuditVerbDeleteDomain:
			row += " " + ev.Domain
		case core.AuditVerbAddDisk, core.AuditVerbUpdateDisk:
			if ev.DiskID == "" {
				row += " without its disk"
			}
			if ev.DiskSizeFromGB != nil {
				row += fmt.Sprintf(" %dGB->", *ev.DiskSizeFromGB)
			} else {
				row += " "
			}
			row += fmt.Sprintf("%dGB", ptr.Deref(ev.DiskSizeGB, -1))
		case core.AuditVerbSetPlan:
			row += " " + ptr.Deref(ev.PlanFrom, "") + "->" + ptr.Deref(ev.PlanTo, "")
		case core.AuditVerbMaintenanceModeEnabled:
			row += fmt.Sprintf(" enabled=%t", ptr.Deref(ev.MaintenanceModeTo, false))
		case core.AuditVerbEnvironmentServiceMoved, core.AuditVerbProjectServiceMoved:
			row += " " + placementRow(ev.ProjectFrom, ev.EnvironmentFrom) + "->" + placementRow(ev.ProjectTo, ev.EnvironmentTo)
		}
		if ev.Outcome != core.AuditAllowed {
			row += " " + string(ev.Outcome)
		}
		by := ev.Caller
		if by == "" {
			by = "no caller"
		}
		out = append(out, row+" by "+by)
	}
	return out
}

// placementRow spells one side of a move as project/environment, "none" for
// no placement.
func placementRow(project, environment *string) string {
	if project == nil && environment == nil {
		return "none"
	}
	return ptr.Deref(project, "") + "/" + ptr.Deref(environment, "")
}

// requireServiceTargets fails unless every row sink recorded but the apply's
// own targets app's service.
func requireServiceTargets(t *testing.T, sink *captureAuditSink, app *appv1alpha1.App) {
	t.Helper()
	for _, ev := range sink.events {
		if !applyVerbs[ev.Verb] && ev.Target != core.ServiceTarget(core.AppPublicID(app)) {
			t.Fatalf("%s row targets %q, want the service", ev.Verb, ev.Target)
		}
	}
}

const (
	imageService = `services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1}
    plan: starter
`
	nodeService = `services:
  - name: web
    type: web
    runtime: node
    repo: https://github.com/acme/app
    branch: main
    plan: starter
    buildCommand: npm ci
    startCommand: npm start
`
	dockerService = `services:
  - name: web
    type: web
    runtime: docker
    repo: https://github.com/acme/app
    branch: main
    plan: starter
    rootDir: api
    dockerfilePath: ./Dockerfile
`
	staticService = `services:
  - name: web
    type: web
    runtime: static
    repo: https://github.com/acme/site
    branch: main
    buildCommand: npm run build
    staticPublishPath: dist
`
	cronService = `services:
  - name: web
    type: cron
    runtime: image
    image: {url: busybox:1}
    plan: starter
    schedule: "0 * * * *"
    dockerCommand: ./run
`
	threeToFive = "    scaling: {minInstances: 1, maxInstances: 3, targetCPUPercent: 70}\n"
)

// blueprintSettingCases change one Blueprint service field each, from base to
// manifest, and name the rows the sync records. TestEveryBlueprintFieldRecordsOrIsExcused
// reads their fields.
var blueprintSettingCases = []struct {
	field    string
	name     string
	base     string
	manifest string
	want     []string
}{
	{"numInstances", "instance count", imageService + "    numInstances: 1\n", imageService + "    numInstances: 3\n",
		[]string{"apps.Scale 1->3 by id-a"}},
	{"scaling", "autoscaling turned on", imageService, imageService + threeToFive,
		[]string{"apps.SetAutoscaling none->1..3 cpu 70 by id-a"}},
	{"scaling", "autoscaling settings changed", imageService + threeToFive,
		imageService + "    scaling: {minInstances: 2, maxInstances: 5, targetCPUPercent: 60}\n",
		[]string{"apps.SetAutoscaling 1..3->2..5 cpu 60 by id-a"}},
	{"numInstances", "autoscaling turned off by an instance count", imageService + threeToFive, imageService + "    numInstances: 2\n",
		[]string{"apps.Scale 1->2 by id-a", "apps.DeleteAutoscaling 1..3->off by id-a"}},
	{"plan", "plan", imageService, strings.Replace(imageService, "starter", "standard", 1),
		[]string{"apps.SetPlan starter->standard by id-a"}},
	{"image", "image", imageService, strings.Replace(imageService, "nginx:1", "nginx:2", 1),
		[]string{"apps.SetSourceAndRegistryCredential by id-a"}},
	{"repo", "repository", nodeService, strings.Replace(nodeService, "acme/app", "acme/other", 1),
		[]string{"apps.SetSourceAndRegistryCredential by id-a"}},
	// A switch to a prebuilt image drops the built start command with it; that
	// is the source change, as the source setter's switch is.
	{"image", "a repository switched to a prebuilt image", nodeService, imageService,
		[]string{"apps.SetSourceAndRegistryCredential by id-a"}},
	{"branch", "branch", nodeService, strings.Replace(nodeService, "branch: main", "branch: dev", 1),
		[]string{"apps.SetSourceAndRegistryCredential by id-a"}},
	{"rootDir", "root directory", nodeService, nodeService + "    rootDir: api\n",
		[]string{"apps.SetRootDir by id-a"}},
	{"buildCommand", "build command", nodeService, strings.Replace(nodeService, "npm ci", "npm install", 1),
		[]string{"apps.SetCommands by id-a"}},
	{"startCommand", "start command", nodeService, strings.Replace(nodeService, "npm start", "node server.js", 1),
		[]string{"apps.SetCommands by id-a"}},
	{"dockerCommand", "a cron job's command", cronService, strings.Replace(cronService, "./run", "./run --all", 1),
		[]string{"apps.SetCommands by id-a"}},
	{"autoDeploy", "auto-deploy turned off", nodeService, nodeService + "    autoDeploy: false\n",
		[]string{"apps.SetAutoDeploy enabled=false commit setting_change by id-a"}},
	{"autoDeploy", "auto-deploy turned on", nodeService + "    autoDeploy: false\n", nodeService + "    autoDeploy: true\n",
		[]string{"apps.SetAutoDeploy enabled=true commit by id-a"}},
	{"autoDeployTrigger", "auto-deploy trigger off", nodeService, nodeService + "    autoDeployTrigger: off\n",
		[]string{"apps.SetAutoDeploy enabled=false commit setting_change by id-a"}},
	{"buildFilter", "build filter added", nodeService, nodeService + "    buildFilter: {paths: [src/**]}\n",
		[]string{"apps.SetBuildFilter by id-a"}},
	{"buildFilter", "build filter removed", nodeService + "    buildFilter: {paths: [src/**]}\n", nodeService,
		[]string{"apps.SetBuildFilter by id-a"}},
	{"dockerfilePath", "Dockerfile path", dockerService, strings.Replace(dockerService, "./Dockerfile", "./Dockerfile.prod", 1),
		[]string{"apps.SetDockerfilePath by id-a"}},
	{"dockerContext", "Docker context", dockerService, dockerService + "    dockerContext: ./ctx\n",
		[]string{"apps.SetDockerContext by id-a"}},
	{"rootDir", "a root directory that moves the Docker context with it", dockerService + "    dockerContext: ./ctx\n",
		strings.Replace(dockerService, "rootDir: api", "rootDir: svc", 1) + "    dockerContext: ./ctx\n",
		[]string{"apps.SetRootDir by id-a"}},
	// A runtime switch drops the Dockerfile settings the new build does not
	// read; SetRuntime records no event, and neither does the drop.
	{"dockerfilePath", "a Dockerfile build switched to a native runtime", dockerService,
		strings.Replace(strings.Replace(dockerService, "runtime: docker", "runtime: node", 1), "    dockerfilePath: ./Dockerfile\n",
			"    buildCommand: npm ci\n    startCommand: npm start\n", 1),
		[]string{"apps.SetCommands by id-a"}},
	{"preDeployCommand", "pre-deploy command", imageService, imageService + "    preDeployCommand: ./migrate\n",
		[]string{"apps.SetPreDeployCommand by id-a"}},
	{"maxShutdownDelaySeconds", "shutdown delay", imageService, imageService + "    maxShutdownDelaySeconds: 60\n",
		[]string{"apps.SetMaxShutdownDelay by id-a"}},
	{"ipAllowList", "inbound IP rules", imageService, imageService + "    ipAllowList:\n      - {source: 10.0.0.0/8, description: office}\n",
		[]string{"apps.SetIPAllowList by id-a"}},
	{"renderSubdomainPolicy", "platform subdomain disabled", imageService + "    domains: [app.example.com]\n",
		imageService + "    domains: [app.example.com]\n    renderSubdomainPolicy: disabled\n",
		[]string{"apps.SetSubdomainPolicy by id-a"}},
	{"renderSubdomainPolicy", "platform subdomain restated as enabled", imageService, imageService + "    renderSubdomainPolicy: enabled\n", nil},
	{"domains", "domain added", imageService, imageService + "    domains: [app.example.com]\n",
		[]string{"apps.AddDomain app.example.com by id-a"}},
	{"domains", "domain replaced", imageService + "    domains: [a.example.com]\n", imageService + "    domains: [b.example.com]\n",
		[]string{"apps.AddDomain b.example.com by id-a", "apps.DeleteDomain a.example.com by id-a"}},
	{"domain", "the single-domain spelling", imageService, imageService + "    domain: app.example.com\n",
		[]string{"apps.AddDomain app.example.com by id-a"}},
	{"maintenanceMode", "maintenance mode", imageService, imageService + "    maintenanceMode: {enabled: true}\n",
		[]string{"apps.SetMaintenanceMode enabled=true by id-a"}},
	{"staticPublishPath", "publish directory", staticService, strings.Replace(staticService, "dist", "build", 1),
		[]string{"apps.SetPublishPath by id-a"}},
	{"routes", "redirect and rewrite rules", staticService, staticService + "    routes:\n      - {type: rewrite, source: /old, destination: /new}\n",
		[]string{"apps.SetRoutes by id-a"}},
	{"headers", "response headers", staticService, staticService + "    headers:\n      - {path: /*, name: X-Frame-Options, value: DENY}\n",
		[]string{"apps.SetHeaders by id-a"}},
	{"healthCheckPath", "health check path", imageService, imageService + "    healthCheckPath: /healthz\n", nil},
	{"schedule", "a cron job's schedule", cronService, strings.Replace(cronService, "0 * * * *", "30 * * * *", 1), nil},
}

// w9/092: a Blueprint sync that changes an existing service's setting records
// the event that setting's own setter records, once, by the member who ran
// the sync, and nothing else; a re-sync that changes nothing records nothing.
// The service's create records nothing either.
func TestBlueprintSyncSettingChangesRecordTheirSettersEvents(t *testing.T) {
	for _, tc := range blueprintSettingCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := ownershipService(t)
			svc.DomainTraffic = allowDomainTraffic{}
			sink := &captureAuditSink{}
			svc.Audit = sink
			ctx := ownershipCtx()
			svc.GitFetcher = fakeBlueprintFetcher{contents: tc.base, sha: "abc1234"}
			bp, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/a", Branch: "main"})
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if got := serviceRows(sink); len(got) != 0 {
				t.Fatalf("the Blueprint's create of the service recorded %v, want nothing", got)
			}

			sink.events = nil
			if _, err := svc.SyncBlueprint(ctx, bp.ID, "tea-a", tc.manifest, "", nil); err != nil {
				t.Fatalf("sync: %v", err)
			}
			if got := serviceRows(sink); !slices.Equal(got, tc.want) {
				t.Fatalf("sync recorded %v, want %v", got, tc.want)
			}
			requireServiceTargets(t, sink, getTenantApp(t, svc.Client, "tea-a", "web"))

			sink.events = nil
			if _, err := svc.SyncBlueprint(ctx, bp.ID, "tea-a", tc.manifest, "", nil); err != nil {
				t.Fatalf("re-sync: %v", err)
			}
			if got := serviceRows(sink); len(got) != 0 {
				t.Fatalf("a re-sync that changes nothing recorded %v", got)
			}
		})
	}
}

// blueprintFieldsWithoutAnEvent are the Blueprint service fields whose change
// records no event, each with why.
var blueprintFieldsWithoutAnEvent = map[string]string{
	"type":              "an existing service's type cannot change: the sync refuses it",
	"runtime":           "SetRuntime records no event: the rebuild it starts is the event, and so are the settings it drops",
	"builder":           "x-bex.builder is the runtime's other spelling",
	"schedule":          "SetCronJob has no event type yet",
	"healthCheckPath":   "SetHealthCheckPath has no event type yet",
	"envVars":           "a Blueprint-owned value no env-var verb can write; seeds and fromGroup links record through their own verbs",
	"initialDeployHook": "no setter: the hook runs once, on the first deploy",
}

// blueprintFieldsCoveredElsewhere are recorded fields a store-less sync cannot
// exercise, with the test that does.
var blueprintFieldsCoveredElsewhere = map[string]string{
	"disk":               "TestManagedBlueprintDiskChangesRecordDiskEvents",
	"buildSource":        "TestManagedBlueprintBuildSourceLinkRecordsOnlyTheSourceChange",
	"registryCredential": "TestStackSettingChangesCountARegistryCredentialAsASourceChange",
	blueprintGrouping:    "TestManagedBlueprintRegroupRecordsServiceMoved",
}

// blueprintGrouping stands for where a Blueprint declares a service: the
// project and environment it is nested under (projects:, or the legacy
// ungrouped: list), which no policy-table field names (w9/093).
const blueprintGrouping = "projects/environments grouping"

// Every Blueprint service field either records its setter's event, through a
// case above or a test named here, or sits on the excused list with why; an
// excused field's cases record nothing. A field added to the policy table
// fails here until it is one or the other.
func TestEveryBlueprintFieldRecordsOrIsExcused(t *testing.T) {
	recorded := map[string]bool{}
	for _, tc := range blueprintSettingCases {
		recorded[tc.field] = recorded[tc.field] || len(tc.want) != 0
		if _, excused := blueprintFieldsWithoutAnEvent[tc.field]; excused && len(tc.want) != 0 {
			t.Errorf("case %q of the excused field %s records %v", tc.name, tc.field, tc.want)
		}
	}
	for field := range blueprintFieldsCoveredElsewhere {
		recorded[field] = true
	}
	// Every field the policy table names, buildSource, which the apply reads
	// beside it (a link, not a preserved value), and the service's grouping.
	fields := []string{"buildSource", blueprintGrouping}
	for _, policy := range BlueprintServiceFieldPolicies {
		fields = append(fields, policy.Name)
	}
	known := map[string]bool{}
	for _, field := range fields {
		known[field] = true
		_, excused := blueprintFieldsWithoutAnEvent[field]
		switch {
		case excused && recorded[field]:
			t.Errorf("Blueprint field %s is excused but covered as recording an event", field)
		case !excused && !recorded[field]:
			t.Errorf("Blueprint field %s has no case recording its setter's event and no excuse", field)
		}
	}
	for field := range recorded {
		if !known[field] {
			t.Errorf("case field %s is no Blueprint service field", field)
		}
	}
}

// An automatic sync applies a push nobody ran by hand, so the setting changes
// it records name no caller, as its plan change does (w9/091).
func TestAutomaticBlueprintSyncSettingChangesNameNoCaller(t *testing.T) {
	svc, fs, b, git := orderingBlueprintFixture(t)
	git.byCommit[shaA] = nodeService + "    numInstances: 1\n"
	git.byCommit[shaB] = nodeService + "    numInstances: 2\n    autoDeploy: false\n    preDeployCommand: ./migrate\n"
	sink := &captureAuditSink{}
	svc.Audit = sink
	w := &BlueprintAutoSyncWorker{Svc: svc}
	git.headSHA = shaA
	w.process(context.Background(), claimOrderingIntent(t, fs, b, shaA, "create"))
	if got := serviceRows(sink); len(got) != 0 {
		t.Fatalf("the automatic create recorded %v, want nothing", got)
	}

	sink.events = nil
	git.headSHA = shaB
	w.process(context.Background(), claimOrderingIntent(t, fs, b, shaB, "settings"))
	want := []string{
		"apps.Scale 1->2 by no caller",
		"apps.SetAutoDeploy enabled=false commit setting_change by no caller",
		"apps.SetPreDeployCommand by no caller",
	}
	if got := serviceRows(sink); !slices.Equal(got, want) {
		t.Fatalf("automatic sync recorded %v, want %v", got, want)
	}
}

// The approver of a planned sync applies it, so its setting changes name the
// approver; planning them records nothing.
func TestBlueprintPlanApprovalSettingChangesNameTheApprover(t *testing.T) {
	svc := newBlueprintService(newFakeBlueprintStore(), fakeWorkspace{"user-a": "tea-a", "approver": "tea-a"})
	svc.GitFetcher = fakeBlueprintFetcher{contents: planManifest("standard")}
	sink := &captureAuditSink{}
	svc.Audit = sink
	creator := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})
	approver := core.WithIdentity(context.Background(), core.Identity{Subject: "approver", Method: "session"})
	res, err := svc.CreatePlannedBlueprint(creator, RenderCreateBlueprintRequest{
		Name: "stack", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"},
	})
	if err != nil {
		t.Fatal(err)
	}
	bpID := res.Blueprint.ID
	if _, err := svc.ApproveBlueprint(creator, bpID, "tea-a", nil); err != nil {
		t.Fatalf("approve the create: %v", err)
	}

	svc.GitFetcher = fakeBlueprintFetcher{contents: planManifest("standard") + threeToFive}
	if plan, err := svc.PlanBlueprint(creator, bpID, "tea-a"); err != nil || plan.PlanningOutcome != PlanningPlanned {
		t.Fatalf("plan = %+v err=%v, want a planned change", plan, err)
	}
	if got := serviceRows(sink); len(got) != 0 {
		t.Fatalf("creating and planning recorded %v, want nothing", got)
	}
	if _, err := svc.ApproveBlueprint(approver, bpID, "tea-a", nil); err != nil {
		t.Fatalf("approve the change: %v", err)
	}
	want := []string{"apps.SetAutoscaling none->1..3 cpu 70 by approver"}
	if got := serviceRows(sink); !slices.Equal(got, want) {
		t.Fatalf("approval recorded %v, want %v", got, want)
	}
}

// A create whose service names a sibling declared after it re-applies the new
// service once the sibling exists; that pass only adds the reference, so it
// records nothing, managed or not.
func TestBlueprintCreateWithAForwardReferenceRecordsNothing(t *testing.T) {
	const manifest = `services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1}
    plan: starter
    numInstances: 2
    preDeployCommand: ./migrate
    scaling: {minInstances: 1, maxInstances: 3, targetCPUPercent: 70}
    envVars:
      - key: API_HOST
        fromService: {name: api, type: pserv, property: host}
  - name: api
    type: pserv
    runtime: image
    image: {url: nginx:1}
    plan: starter
`
	for _, managed := range []bool{false, true} {
		svc, cl := newService(nil)
		if managed {
			svc, cl = newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, &recordingStore{})
		}
		sink := &captureAuditSink{}
		svc.Audit = sink
		ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
			t.Fatalf("managed=%t create: %v", managed, err)
		}
		var apps appv1alpha1.AppList
		if err := cl.List(ctx, &apps); err != nil || len(apps.Items) != 2 {
			t.Fatalf("managed=%t apps = %d err=%v, want both services", managed, len(apps.Items), err)
		}
		web := slices.IndexFunc(apps.Items, func(a appv1alpha1.App) bool { return core.AppPublicName(&a) == "web" })
		if web < 0 || !slices.ContainsFunc(apps.Items[web].Spec.Env, func(e appv1alpha1.EnvVar) bool { return e.Name == "API_HOST" }) {
			t.Fatalf("managed=%t web env lacks API_HOST: the second pass did not run", managed)
		}
		if got := serviceRows(sink); len(got) != 0 {
			t.Fatalf("managed=%t the create recorded %v, want nothing", managed, got)
		}
	}
}

// managedStackService is a store-backed, tenant-scoped Service whose
// DeployStack writes managed Apps, recording audit rows into sink.
func managedStackService(t *testing.T, rec IntentStore) (*Service, client.Client, *captureAuditSink, context.Context) {
	t.Helper()
	svc, base := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, rec)
	svc.DomainTraffic = allowDomainTraffic{}
	sink := &captureAuditSink{}
	svc.Audit = sink
	return svc, base, sink, core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})
}

// refusingSpecPatches is base with every App patch that writes the spec
// refused; a source change's pending-source annotation still lands.
func refusingSpecPatches(base client.Client) client.Client {
	return interceptor.NewClient(base.(client.WithWatch), interceptor.Funcs{
		Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if _, ok := obj.(*appv1alpha1.App); ok {
				if data, err := patch.Data(obj); err != nil || bytes.Contains(data, []byte(`"spec"`)) {
					return errors.New("the API server refused the patch")
				}
			}
			return cl.Patch(ctx, obj, patch, opts...)
		},
	})
}

// A managed App's disk row is the disk's source of truth: a Blueprint that
// adds a disk records disk_created with the row's id and size, one that grows
// it disk_updated with both sizes, and a re-deploy that keeps it nothing.
func TestManagedBlueprintDiskChangesRecordDiskEvents(t *testing.T) {
	rec := &recordingStore{}
	svc, base, sink, ctx := managedStackService(t, rec)
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: imageService}); err != nil {
		t.Fatalf("create: %v", err)
	}
	app := getTenantApp(t, base, "tea-a", "web")
	if svc.blueprintIntentRow(app) == "" {
		t.Fatal("want a managed App")
	}
	for _, step := range []struct {
		name, manifest string
		want           []string
	}{
		{"add", imageService + "    disk: {name: data, mountPath: /var/data, sizeGB: 10}\n", []string{"apps.AddDisk 10GB by id-a"}},
		{"grow", imageService + "    disk: {name: data, mountPath: /var/data, sizeGB: 25}\n", []string{"apps.UpdateDisk 10GB->25GB by id-a"}},
		{"remount", imageService + "    disk: {name: data, mountPath: /srv/data, sizeGB: 25}\n", []string{"apps.UpdateDisk 25GB->25GB by id-a"}},
		{"keep", imageService + "    disk: {name: data, mountPath: /srv/data, sizeGB: 25}\n", nil},
	} {
		sink.events = nil
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: step.manifest}); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		if got := serviceRows(sink); !slices.Equal(got, step.want) {
			t.Fatalf("%s recorded %v, want %v", step.name, got, step.want)
		}
		rows, _ := rec.ListDisks(ctx, "tea-a", managedAppID(app))
		for _, ev := range sink.events {
			if (ev.Verb == core.AuditVerbAddDisk || ev.Verb == core.AuditVerbUpdateDisk) && (len(rows) != 1 || ev.DiskID != rows[0].ID) {
				t.Fatalf("%s: %s names disk %q, want the row %+v", step.name, ev.Verb, ev.DiskID, rows)
			}
		}
		requireServiceTargets(t, sink, app)
	}
}

// A managed App's row-owned settings are recorded when their rows change, so
// an apply whose patch fails and its retry record each exactly once, whether
// or not the projector copied the rows onto the App in between; a setting only
// the App's patch carries is recorded once that patch lands.
func TestManagedStackSettingChangesAreRecordedOnceAcrossARetry(t *testing.T) {
	for _, resynced := range []bool{false, true} {
		t.Run(fmt.Sprintf("resynced=%t", resynced), func(t *testing.T) {
			rec := &recordingStore{}
			svc, base, sink, ctx := managedStackService(t, rec)
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: imageService + "    numInstances: 1\n"}); err != nil {
				t.Fatalf("create: %v", err)
			}
			changed := strings.Replace(strings.Replace(imageService, "nginx:1", "nginx:2", 1), "starter", "standard", 1) +
				"    numInstances: 3\n    preDeployCommand: ./migrate\n"

			svc.Client = refusingSpecPatches(base)
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: changed}); err == nil {
				t.Fatal("want the deploy to fail at its patch")
			}
			rows := []string{"apps.SetSourceAndRegistryCredential by id-a", "apps.SetPlan starter->standard by id-a", "apps.Scale 1->3 by id-a"}
			if got := serviceRows(sink); !slices.Equal(got, rows) {
				t.Fatalf("the failed apply recorded %v, want only the rows that changed %v", got, rows)
			}
			app := getTenantApp(t, base, "tea-a", "web")
			if app.Spec.Replicas != 1 || app.Spec.PreDeployCommand != "" {
				t.Fatalf("CR = %d instances, pre-deploy %q, want the failed patch to have left both", app.Spec.Replicas, app.Spec.PreDeployCommand)
			}
			if resynced { // the projector copies the rows onto the App
				app.Spec.Image, app.Spec.Tier, app.Spec.Replicas = "nginx:2", "standard", 3
				if err := base.Update(ctx, app); err != nil {
					t.Fatal(err)
				}
			}

			svc.Client = base
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: changed}); err != nil {
				t.Fatalf("retry: %v", err)
			}
			want := append(rows, "apps.SetPreDeployCommand by id-a")
			if got := serviceRows(sink); !slices.Equal(got, want) {
				t.Fatalf("the failed apply and its retry recorded %v, want %v", got, want)
			}
		})
	}
}

// branchFacts records the service event facts a Service inserts.
type branchFacts struct{ facts []store.ServiceEventFact }

func (b *branchFacts) InsertServiceEventFact(_ context.Context, fact store.ServiceEventFact) (bool, error) {
	b.facts = append(b.facts, fact)
	return true, nil
}

// A managed App's branch change records source_changed and the branch_changed
// fact, as the source setter does, once its source row changes: a retry after
// a failed patch records neither again.
func TestManagedBlueprintBranchChangeRecordsTheBranchFactOnce(t *testing.T) {
	rec := &recordingStore{}
	svc, base, sink, ctx := managedStackService(t, rec)
	facts := &branchFacts{}
	svc.EventFacts = facts
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: nodeService}); err != nil {
		t.Fatalf("create: %v", err)
	}
	sink.events = nil
	dev := strings.Replace(nodeService, "branch: main", "branch: dev", 1)
	svc.Client = refusingSpecPatches(base)
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: dev}); err == nil {
		t.Fatal("want the deploy to fail at its patch")
	}
	svc.Client = base
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: dev}); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if got, want := serviceRows(sink), []string{"apps.SetSourceAndRegistryCredential by id-a"}; !slices.Equal(got, want) {
		t.Fatalf("the deploy and its retry recorded %v, want %v", got, want)
	}
	app := getTenantApp(t, base, "tea-a", "web")
	if len(facts.facts) != 1 || facts.facts[0].Type != store.EventFactBranchChanged ||
		facts.facts[0].AppID != managedAppID(app) || facts.facts[0].BranchFrom != "main" || facts.facts[0].BranchTo != "dev" {
		t.Fatalf("facts = %+v, want one branch_changed main->dev for the service", facts.facts)
	}
}

// A managed App's Blueprint domains are its claims: an added and a dropped
// host record custom_domain_added and custom_domain_removed once the claims
// are replaced, pending or not, and a re-deploy records nothing.
func TestManagedBlueprintDomainChangesRecordDomainEvents(t *testing.T) {
	claims := newMemoryDomainClaimStore()
	svc, _, sink, ctx := managedStackService(t, claims)
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: imageService + "    domains: [a.example.com]\n"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if got := serviceRows(sink); len(got) != 0 {
		t.Fatalf("the create recorded %v, want nothing", got)
	}
	for _, step := range []struct {
		manifest string
		want     []string
	}{
		{imageService + "    domains: [a.example.com, b.example.com]\n", []string{"apps.AddDomain b.example.com by id-a"}},
		{imageService + "    domains: [b.example.com]\n", []string{"apps.DeleteDomain a.example.com by id-a"}},
		{imageService + "    domains: [b.example.com]\n", nil},
	} {
		sink.events = nil
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: step.manifest}); err != nil {
			t.Fatalf("deploy: %v", err)
		}
		if got := serviceRows(sink); !slices.Equal(got, step.want) {
			t.Fatalf("deploy recorded %v, want %v", got, step.want)
		}
	}
}

// A Blueprint that links an existing service to a build source records one
// source_changed, as LinkBuildSource does: the build settings the link takes
// over (root directory, build command, build filter) and the start command it
// sets are the link's, not changes of their own.
func TestManagedBlueprintBuildSourceLinkRecordsOnlyTheSourceChange(t *testing.T) {
	svc, st, ctx := newBuildSourceService(t)
	sink := &captureAuditSink{}
	svc.Audit = sink
	const own = `services:
  - type: web
    name: api
    runtime: node
    repo: https://github.com/acme/api
    branch: main
    rootDir: api
    buildCommand: npm ci
    startCommand: node old.js
    buildFilter: {paths: [api/**]}
`
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: own}); err != nil {
		t.Fatalf("own source: %v", err)
	}
	linked := buildSourceBlueprint[:strings.Index(buildSourceBlueprint, "  - type: web\n    name: site")]
	sink.events = nil
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: linked}); err != nil {
		t.Fatalf("link: %v", err)
	}
	if len(st.linked) != 1 {
		t.Fatalf("store links = %v, want one", st.linked)
	}
	app := getTenantApp(t, svc.Client, "tea-a", "api")
	var got []string
	for _, row := range serviceRows(sink) {
		if !strings.HasPrefix(row, "apps.CreateBuildSource") {
			got = append(got, row)
		}
	}
	if want := []string{"apps.SetSourceAndRegistryCredential by user-a"}; !slices.Equal(got, want) {
		t.Fatalf("the link recorded %v, want %v", got, want)
	}
	for _, ev := range sink.events {
		if ev.Verb == core.AuditVerbSetSourceAndRegistryCredential && ev.Target != core.ServiceTarget(core.AppPublicID(app)) {
			t.Fatalf("source_changed targets %q, want the linked service", ev.Target)
		}
	}
}

// A changed registry credential is a source change, as SetRegistryCredential's
// is: the store-less apply records source_changed for it and nothing else.
func TestStackSettingChangesCountARegistryCredentialAsASourceChange(t *testing.T) {
	app := sampleApp("web")
	svc, _ := newService(nil, app)
	sink := &captureAuditSink{}
	svc.Audit = sink
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})
	after := app.Spec.DeepCopy()
	after.RegistryCredentialID = ptr.To("rc-declared")
	svc.recordStackSettingChanges(ctx, app, &app.Spec, after)
	if got, want := serviceRows(sink), []string{"apps.SetSourceAndRegistryCredential by id-a"}; !slices.Equal(got, want) {
		t.Fatalf("recorded %v, want %v", got, want)
	}
}
