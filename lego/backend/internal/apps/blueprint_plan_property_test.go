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
	"math/rand"
	"reflect"
	"strconv"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The Blueprint plan/apply cycle rests on two invariants, modelled here rather
// than inferred from any one fixture:
//
//  1. Apply is idempotent. Applying a manifest to the spec it just produced is
//     a no-op: a sync re-planned against its own result must report no change,
//     or every sync forever plans an update it need not make (the w4/m124
//     family).
//  2. The plan names what apply does. The per-field probe list
//     (blueprintServiceProbePaths) is empty exactly when the whole apply is a
//     no-op, and every reported path is one the apply really moves.
//
// The manifests below are valid render.yaml documents the compiler accepts,
// each declaring a distinct field family; the live specs are the shapes a real
// service reaches (zero value, a fully populated pre-Blueprint service, an
// image-backed one, a build-source-linked one). The omission paths the apply
// takes — buildFilter clearing, an image service taking the Blueprint's repo,
// a service the file unlinks from its build source — are exercised by pairing
// those live shapes with the files that trigger them.

var blueprintPlanPropertyManifests = map[string]string{
	"image-service": `services:
  - name: api
    type: web
    runtime: image
    image: {url: nginx:1.27}
    plan: starter
    numInstances: 2
    envVars:
      - key: MODE
        value: prod
`,
	"git-service": `services:
  - name: api
    type: web
    runtime: node
    repo: https://github.com/bex-co/api
    branch: main
    buildCommand: npm ci
    startCommand: npm start
    plan: standard
    healthCheckPath: /healthz
    maxShutdownDelaySeconds: 30
    autoDeploy: true
    autoDeployTrigger: commit
    preDeployCommand: npm run migrate
    initialDeployHook: npm run seed
`,
	"git-service-build-filter": `services:
  - name: api
    type: web
    runtime: node
    repo: https://github.com/bex-co/api
    branch: main
    buildCommand: npm ci
    startCommand: npm start
    buildFilter:
      paths: [src/**]
      ignoredPaths: [docs/**]
`,
	"docker-service": `services:
  - name: api
    type: web
    runtime: docker
    repo: https://github.com/bex-co/api
    branch: main
    rootDir: backend
    dockerfilePath: ./Dockerfile
    dockerContext: .
    dockerCommand: /app/server
`,
	"static-site": `services:
  - name: site
    type: web
    runtime: static
    repo: https://github.com/bex-co/site
    branch: main
    staticPublishPath: ./dist
    routes:
      - {type: rewrite, source: /a, destination: /b}
    headers:
      - {path: /a, name: X-A, value: b}
`,
	"cron-job": `services:
  - name: nightly
    type: cron
    runtime: image
    image: {url: busybox:1.36}
    schedule: '0 3 * * *'
    dockerCommand: /bin/true
`,
	"private-service": `services:
  - name: cache
    type: pserv
    runtime: image
    image: {url: redis:7}
`,
	"web-ip-allow-list": `services:
  - name: api
    type: web
    runtime: image
    image: {url: nginx:1.27}
    ipAllowList:
      - {source: 10.0.0.0/8, description: internal}
`,
	"autoscaling-web": `services:
  - name: api
    type: web
    runtime: image
    image: {url: worker:1}
    numInstances: 3
    scaling:
      minInstances: 2
      maxInstances: 8
      targetCPUPercent: 70
`,
	"service-disk": `services:
  - name: api
    type: web
    runtime: image
    image: {url: nginx:1.27}
    disk:
      name: data
      mountPath: /var/lib/data
      sizeGB: 20
`,
	"service-domains": `services:
  - name: api
    type: web
    runtime: image
    image: {url: nginx:1.27}
    domains:
      - api.example.test
      - alt.example.test
`,
	"service-one-domain": `services:
  - name: api
    type: web
    runtime: image
    image: {url: nginx:1.27}
    domains:
      - api.example.test
`,
	"docker-context-without-root-dir": `services:
  - name: api
    type: web
    runtime: docker
    repo: https://github.com/bex-co/api
    branch: main
    dockerContext: ./docker
`,
	"service-maintenance": `services:
  - name: api
    type: web
    runtime: image
    image: {url: nginx:1.27}
    maintenanceMode:
      enabled: true
      uri: https://status.example.test/maintenance
`,
}

// blueprintPlanPropertyLiveSpecs are the existing services a manifest can be
// re-planned against.
func blueprintPlanPropertyLiveSpecs() map[string]appv1alpha1.AppSpec {
	cpu := int32(55)
	return map[string]appv1alpha1.AppSpec{
		"zero": {Type: appv1alpha1.TypeWebService},
		"populated": {
			Type:              appv1alpha1.TypeWebService,
			Runtime:           "node",
			Repo:              "https://github.com/bex-co/legacy",
			Branch:            "release",
			Tier:              "pro",
			Replicas:          4,
			AutoDeploy:        true,
			AutoDeployTrigger: "commit",
			BuildCommand:      "make build",
			StartCommand:      "make run",
			RootDir:           "svc",
			DockerContext:     "svc/docker",
			DockerfilePath:    "./Dockerfile",
			HealthCheckPath:   "/healthz",
			PublishPath:       "./public",
			PreDeployCommand:  "make migrate",
			InitialDeployHook: "make seed",
			Host:              "old.example.test",
			Hosts:             []string{"also-old.example.test"},
			Env:               []appv1alpha1.EnvVar{{Name: "MODE", Value: "old"}, {Name: "DASHBOARD", Value: "keep"}},
			BuildFilter:       &appv1alpha1.BuildFilterSpec{Paths: []string{"old/**"}},
			Routes:            []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/a", Destination: "/b"}},
			Headers:           []appv1alpha1.StaticHeader{{Path: "/a", Name: "X-A", Value: "b"}},
			IPAllowList:       []string{"10.0.0.0/8"},
			RegistryCredentialID: func() *string {
				id := "rgc-legacy"
				return &id
			}(),
			Disk: &appv1alpha1.DiskSpec{
				Name: "data", MountPath: "/var/lib/data", SizeGB: 10,
			},
			Autoscaling: &appv1alpha1.AutoscalingSpec{
				Enabled: true, MinReplicas: 1, MaxReplicas: 6, TargetCPUPercent: &cpu,
			},
			MaintenanceMode: &appv1alpha1.MaintenanceModeSpec{Enabled: true, URI: "https://status.example.test/old"},
		},
		"image-backed": {
			Type:         appv1alpha1.TypeWebService,
			Runtime:      "image",
			Image:        "nginx:1.26",
			Branch:       "main",
			StartCommand: "old-start",
			Tier:         "starter",
			Host:         "img.example.test",
			RegistryCredentialID: func() *string {
				id := "rgc-image"
				return &id
			}(),
		},
		"build-source-linked": {
			Type:        appv1alpha1.TypeWebService,
			BuildSource: &appv1alpha1.BuildSourceLink{Name: "mono", Image: "mono:abc123"},
			Tier:        "starter",
		},
		"legacy-hosts-split": {
			Type:  appv1alpha1.TypeWebService,
			Hosts: []string{"api.example.test", "alt.example.test"},
		},
		"native-git": {
			Type:          appv1alpha1.TypeWebService,
			Repo:          "https://github.com/bex-co/api",
			Branch:        "main",
			Runtime:       "node",
			BuildCommand:  "npm ci",
			StartCommand:  "npm start",
			Tier:          "starter",
			RootDir:       "svc",
			DockerContext: "",
		},
	}
}

// blueprintPlanService reads a manifest's single service as the planner reads
// it.
func blueprintPlanService(t *testing.T, manifest string) parsedService {
	t.Helper()
	st := parseBlueprintStackForTest(t, manifest)
	if len(st.services) != 1 {
		t.Fatalf("%d services parsed, want 1", len(st.services))
	}
	return st.services[0]
}

// blueprintPlanWant projects a compiled service as the planner does:
// blueprintReadSpec over the declared request, the build source link the
// declared buildSource names, the sync's own hook reconciliation, and the
// served-PORT drop.
func blueprintPlanWant(t *testing.T, svc parsedService, app *appv1alpha1.App) appv1alpha1.AppSpec {
	t.Helper()
	ctx := context.WithValue(context.Background(), unresolvedRepoKey{}, true)
	want, err := blueprintReadSpec(ctx, withCurrentTier(svc.req, app))
	if err != nil {
		t.Fatalf("blueprintReadSpec: %v", err)
	}
	want.BuildSource = desiredBuildSourceLink(svc, nil)
	initialDeployHookState(svc.req, app, &want)
	dropServedPortEnv(&want, app)
	return want
}

// blueprintPlanAgreement applies a manifest to live and compares the result
// with the field list the plan reports.
func blueprintPlanAgreement(t *testing.T, live appv1alpha1.AppSpec, svc parsedService) {
	t.Helper()
	app := &appv1alpha1.App{Spec: live}
	want := blueprintPlanWant(t, svc, app)

	before := live.DeepCopy()
	dst := live.DeepCopy()
	changed, err := ApplyBlueprintServiceSpec(dst, want, svc.fields)
	if err != nil {
		// A refusal (a disk shrink, a disk/shape conflict) is a plan error,
		// never an update; the plan surfaces it as a conflict and the
		// per-field probes are not consulted.
		return
	}

	// Invariant 1: apply is idempotent.
	again := dst.DeepCopy()
	changedAgain, err := ApplyBlueprintServiceSpec(again, want, svc.fields)
	if err != nil {
		t.Fatalf("second apply refused after a successful first: %v", err)
	}
	if changedAgain || !reflect.DeepEqual(again, dst) {
		t.Fatalf("apply is not idempotent: second apply changed=%v\n first: %#v\nsecond: %#v", changedAgain, dst, again)
	}

	// Invariant 2a: the plan's per-field probes agree with the whole apply.
	probes := blueprintPlanFieldChanges(blueprintServiceProbePaths(svc.fields), func(name string) (bool, error) {
		probe := before.DeepCopy()
		return probeBlueprintServiceSpec(probe, want, svc.fields, name)
	})
	switch {
	case changed && len(probes) == 0:
		t.Fatalf("apply changes the spec but no probed field reports it:\n before: %#v\n  after: %#v", before, dst)
	case !changed && len(probes) > 0:
		t.Fatalf("apply is a no-op but the plan reports %v", probes)
	}

	// Invariant 2b: every field the plan reports really moved.
	if !changed {
		return
	}
	for _, change := range probes {
		probe := before.DeepCopy()
		if moved, err := probeBlueprintServiceSpec(probe, want, svc.fields, change.Path); err != nil || !moved {
			t.Fatalf("plan reports %q but its isolated apply moved=%v err=%v", change.Path, moved, err)
		}
	}
}

func TestBlueprintPlanAndApplyAgree(t *testing.T) {
	for name, manifest := range blueprintPlanPropertyManifests {
		svc := blueprintPlanService(t, manifest)
		for liveName, live := range blueprintPlanPropertyLiveSpecs() {
			t.Run(name+"/"+liveName, func(t *testing.T) {
				blueprintPlanAgreement(t, live, svc)
			})
		}
	}
}

// An entry that declares no source builds the Blueprint's own repository, which
// a sync's parse hands it (serviceSource's containing override). Applying that
// file to an image-backed service replaces its image, and to a build-source
// linked one removes the link: moves neither field's own applier makes, so the
// plan has to attribute them to the path the file dropped.
func TestBlueprintPlanAndApplyAgreeOnTheBlueprintRepo(t *testing.T) {
	const manifest = `services:
  - name: api
    type: web
    runtime: docker
`
	source, ir, problems := CompileBlueprintIR(manifest)
	if len(problems) > 0 {
		t.Fatalf("CompileBlueprintIR() problems = %+v", problems)
	}
	st, err := parseCompiledStack(blueprintParseOverrides{
		repo: "https://github.com/bex-co/mono", branch: "main", containing: true,
	}, source, ir)
	if err != nil {
		t.Fatalf("parseCompiledStack: %v", err)
	}
	if len(st.services) != 1 {
		t.Fatalf("%d services parsed, want 1", len(st.services))
	}
	svc := st.services[0]
	if _, declared := svc.fields["repo"]; declared {
		t.Fatal("the entry declares no repo; it inherits the Blueprint's")
	}
	for liveName, live := range blueprintPlanPropertyLiveSpecs() {
		t.Run(liveName, func(t *testing.T) { blueprintPlanAgreement(t, live, svc) })
	}
}

// blueprintPlanRandomLiveSpec builds a live service shape from a deterministic
// field pool, beside a manifest compiled for the same service type (the App
// CRD refuses a type change, so the model holds it fixed). Every other field
// is free: the live states a service reaches are the ones the appliers must
// plan correctly, not only the tidy ones a fixture names.
func blueprintPlanRandomLiveSpec(r *rand.Rand, svcType string) appv1alpha1.AppSpec {
	pick := func(values ...string) string {
		if r.Intn(3) == 0 {
			return ""
		}
		return values[r.Intn(len(values))]
	}
	int32Ptr := func(value int32) *int32 { return &value }
	stringPtr := func(value string) *string { return &value }
	spec := appv1alpha1.AppSpec{
		Type:                    svcType,
		Runtime:                 pick("node", "docker", "image", "static", "python"),
		Repo:                    pick("https://github.com/acme/app", "https://github.com/acme/other"),
		Image:                   pick("nginx:1.27", "redis:7"),
		Branch:                  pick("main", "release"),
		RootDir:                 pick("svc", "apps/api"),
		DockerContext:           pick("docker", "svc/docker"),
		DockerfilePath:          pick("./Dockerfile", "docker/Dockerfile"),
		BuildCommand:            pick("npm ci", "make build"),
		StartCommand:            pick("npm start", "make run"),
		Command:                 pick("npm run job", "/bin/true"),
		Builder:                 pick("native"),
		Tier:                    pick("starter", "standard", "pro", "free"),
		Replicas:                int32(r.Intn(5)),
		Schedule:                pick("* * * * *", "0 3 * * *"),
		AutoDeploy:              r.Intn(2) == 0,
		AutoDeployTrigger:       pick("commit", "checksPass"),
		Host:                    pick("a.example.test", "b.example.test"),
		Hosts:                   []string{"c.example.test", "d.example.test"},
		MaxShutdownDelaySeconds: clonePtr(int32Ptr(int32(r.Intn(120)))),
		RegistryCredentialID:    stringPtr("rgc-live"),
		PreDeployCommand:        pick("make migrate"),
		InitialDeployHook:       pick("make seed"),
		PublishPath:             pick("./dist"),
		SubdomainPolicy:         pick("disabled"),
		IPAllowList:             []string{"10.0.0.0/8", "192.168.0.0/16"},
		Routes:                  []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/a", Destination: "/b"}},
		Headers:                 []appv1alpha1.StaticHeader{{Path: "/a", Name: "X-A", Value: "b"}},
		BuildFilter:             &appv1alpha1.BuildFilterSpec{Paths: []string{"old/**"}},
		Disk:                    &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/lib/data", SizeGB: int32(10 + r.Intn(30))},
	}
	if r.Intn(2) == 0 {
		spec.Port = int32(10000 + r.Intn(3))
		spec.Env = []appv1alpha1.EnvVar{
			{Name: "MODE", Value: "old"},
			{Name: "DASHBOARD", Value: "keep"},
			{Name: "PORT", Value: strconv.Itoa(int(spec.Port))},
		}
	}
	if r.Intn(2) == 0 {
		cpu := int32(60)
		spec.Autoscaling = &appv1alpha1.AutoscalingSpec{Enabled: r.Intn(2) == 0, MinReplicas: 1, MaxReplicas: 4, TargetCPUPercent: &cpu}
	}
	if r.Intn(2) == 0 {
		spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: r.Intn(2) == 0, URI: "https://status.example.test/old"}
	}
	// A cron needs its schedule, and a prebuilt image excludes a repo.
	if svcType == appv1alpha1.TypeCronJob && spec.Schedule == "" {
		spec.Schedule = "* * * * *"
	}
	if spec.Image != "" && spec.Repo != "" {
		spec.Repo = ""
	}
	return spec
}

func TestBlueprintPlanAndApplyAgreeOnGeneratedLiveSpecs(t *testing.T) {
	r := rand.New(rand.NewSource(20261115))
	for name, manifest := range blueprintPlanPropertyManifests {
		svc := blueprintPlanService(t, manifest)
		svcType := blueprintPlanWant(t, svc, &appv1alpha1.App{}).Type
		t.Run(name, func(t *testing.T) {
			for i := 0; i < 60; i++ {
				blueprintPlanAgreement(t, blueprintPlanRandomLiveSpec(r, svcType), svc)
			}
		})
	}
}
