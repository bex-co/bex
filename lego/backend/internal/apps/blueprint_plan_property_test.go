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
	"math/rand"
	"reflect"
	"strconv"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	"service-domain-singular": `services:
  - name: api
    type: web
    runtime: image
    image: {url: nginx:1.27}
    domain: api.example.test
`,
	"service-scaling-only": `services:
  - name: api
    type: web
    runtime: image
    image: {url: worker:1}
    scaling:
      minInstances: 2
      maxInstances: 8
      targetCPUPercent: 70
`,
	"service-hook-only": `services:
  - name: api
    type: web
    runtime: image
    image: {url: nginx:1.27}
    initialDeployHook: npm run seed
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
	want, err := blueprintReadSpec(ctx, withCurrentSettings(svc.req, app))
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

// mergeBlueprintEnv keeps the file's zero-value convention — a projected list
// is nil, never an empty non-nil slice — even when the merge drops every
// current variable. A sync before w8/302 could write a nameless variable, and
// the merge drops those; without the canonical nil the apply stores an empty
// non-nil list, reports a change on the next sync too, and only settles on the
// third pass. The same empty-vs-nil disagreement is the whole w4/m124 family.
func TestMergeBlueprintEnvCanonicalNil(t *testing.T) {
	t.Parallel()
	if got := mergeBlueprintEnv([]appv1alpha1.EnvVar{{Name: ""}}, nil); got != nil {
		t.Fatalf("mergeBlueprintEnv(nameless, none) = %#v, want nil", got)
	}

	live := appv1alpha1.AppSpec{Env: []appv1alpha1.EnvVar{{Name: ""}}}
	dst := *live.DeepCopy()
	changed, err := ApplyBlueprintServiceSpec(&dst, appv1alpha1.AppSpec{}, map[string]BlueprintField{"envVars": {}})
	if err != nil || !changed || dst.Env != nil {
		t.Fatalf("first apply = changed %v err %v env %#v, want a change to nil", changed, err, dst.Env)
	}
	again := *dst.DeepCopy()
	changedAgain, err := ApplyBlueprintServiceSpec(&again, appv1alpha1.AppSpec{}, map[string]BlueprintField{"envVars": {}})
	if err != nil || changedAgain || !reflect.DeepEqual(again, dst) {
		t.Fatalf("second apply = changed %v err %v, want a no-op: %#v", changedAgain, err, again)
	}
}

// blueprintDatabaseAgreement applies a database want to live and compares the
// result with the field list the plan reports: the same two invariants the
// service property tests pin (apply is idempotent; the probes agree with the
// whole apply), for the appliers that previously carried no such coverage. A
// refusal is a plan error, never an update, so error cases assert the refusal
// writes nothing and skip the probes, as the plan does.
func blueprintDatabaseAgreement(t *testing.T, live *appv1alpha1.Database, want appv1alpha1.DatabaseSpec, fields map[string]BlueprintField) {
	t.Helper()
	probe := live.DeepCopy()
	changed, err := ApplyBlueprintDatabaseSpec(probe, want, fields, blueprintTestNow)
	if err != nil {
		if !reflect.DeepEqual(live.Spec, probe.Spec) || !reflect.DeepEqual(live.Annotations, probe.Annotations) {
			t.Fatalf("a refused apply mutated the database: %#v", probe)
		}
		return
	}

	again := probe.DeepCopy()
	changedAgain, err := ApplyBlueprintDatabaseSpec(again, want, fields, blueprintTestNow)
	if err != nil {
		t.Fatalf("second apply refused after a successful first: %v", err)
	}
	if changedAgain || !reflect.DeepEqual(again.Spec, probe.Spec) {
		t.Fatalf("apply is not idempotent: second changed=%v\n first: %#v\nsecond: %#v", changedAgain, probe.Spec, again.Spec)
	}

	var probed []string
	for name := range fields {
		one := live.DeepCopy()
		moved, err := ApplyBlueprintDatabaseSpec(one, want, oneField(fields, name), blueprintTestNow)
		if err != nil {
			t.Fatalf("isolated probe of %q refused: %v", name, err)
		}
		if moved {
			probed = append(probed, name)
		}
	}
	switch {
	case changed && len(probed) == 0:
		t.Fatalf("apply changes the spec but no probed field reports it:\n before: %#v\n  after: %#v", live.Spec, probe.Spec)
	case !changed && len(probed) > 0:
		t.Fatalf("apply is a no-op but the plan reports %v", probed)
	}
}

func blueprintDatabaseAgreementLive() *appv1alpha1.Database {
	return &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: "default"},
		Spec: appv1alpha1.DatabaseSpec{
			Name:            "orders",
			DatabaseName:    "orders_db",
			DatabaseUser:    "orders_user",
			Plan:            "basic-256mb",
			Version:         "16",
			StorageGB:       10,
			DiskAutoscaling: true,
			Pooler:          true,
			Public:          true,
			IPAllowList:     []appv1alpha1.IPAllowEntry{{CIDR: "10.0.0.0/8"}},
		},
	}
}

func TestApplyBlueprintDatabaseSpecIdempotentAndAgrees(t *testing.T) {
	t.Parallel()
	full := map[string]BlueprintField{
		"databaseName": {}, "user": {}, "plan": {}, "postgresMajorVersion": {},
		"diskSizeGB": {}, "storageAutoscalingEnabled": {}, "connectionPool": {},
		"ipAllowList": {}, "readReplicas": {}, "highAvailability": {},
	}
	t.Run("identical spec is a no-op", func(t *testing.T) {
		t.Parallel()
		live := blueprintDatabaseAgreementLive()
		blueprintDatabaseAgreement(t, live, *live.Spec.DeepCopy(), full)
	})
	t.Run("a storage growth applies, stamps, and settles", func(t *testing.T) {
		t.Parallel()
		live := blueprintDatabaseAgreementLive()
		blueprintDatabaseAgreement(t, live, appv1alpha1.DatabaseSpec{StorageGB: 15}, map[string]BlueprintField{"diskSizeGB": {}})
	})
	t.Run("disabling features applies narrowly", func(t *testing.T) {
		t.Parallel()
		live := blueprintDatabaseAgreementLive()
		blueprintDatabaseAgreement(t, live, appv1alpha1.DatabaseSpec{}, map[string]BlueprintField{
			"storageAutoscalingEnabled": {}, "connectionPool": {}, "ipAllowList": {},
		})
	})
	t.Run("a refused increase writes nothing", func(t *testing.T) {
		t.Parallel()
		live := blueprintDatabaseAgreementLive()
		live.Annotations = map[string]string{
			appv1alpha1.AnnotationStorageIncreasedAt: blueprintTestNow.Add(-time.Hour).Format(time.RFC3339),
		}
		blueprintDatabaseAgreement(t, live, appv1alpha1.DatabaseSpec{StorageGB: 15}, map[string]BlueprintField{"diskSizeGB": {}})
	})
}

// A database's name and owner role are immutable identities: the stored value,
// or, for a database created without one, the default the operator derived
// from its resource id. The plan must refuse exactly what applyDatabase
// refuses, so the truth table below crosses the stored value (named, unnamed)
// with the declaration (the stored identity, the derived default, another
// name): only another name is refused, and an accepted declaration never
// rewrites the spec. Until this was pinned the plan judged only a non-empty
// stored value, so a database created without a databaseName planned an
// update for any declared name and the apply then refused it.
func TestApplyBlueprintDatabaseSpecIdentityTruthTable(t *testing.T) {
	t.Parallel()
	const resourceID = "dpg-abc"
	derivedName := appv1alpha1.DefaultPostgresDatabaseName(resourceID)
	derivedUser := derivedName + "_user"
	for _, tc := range []struct {
		name               string
		storedName, stored string
		declaredName       string
		declaredUser       string
		refused            string
	}{
		{name: "named: same identity", storedName: "orders_db", stored: "orders_user", declaredName: "orders_db", declaredUser: "orders_user"},
		{name: "named: another name", storedName: "orders_db", stored: "orders_user", declaredName: "other", declaredUser: "orders_user", refused: "databaseName"},
		{name: "named: another user", storedName: "orders_db", stored: "orders_user", declaredName: "orders_db", declaredUser: "other", refused: "user"},
		{name: "named: the derived default is another name", storedName: "orders_db", stored: "orders_user", declaredName: derivedName, declaredUser: "orders_user", refused: "databaseName"},
		{name: "unnamed: the derived identity", declaredName: derivedName, declaredUser: derivedUser},
		{name: "unnamed: another name", declaredName: "other", declaredUser: derivedUser, refused: "databaseName"},
		{name: "unnamed: another user", declaredName: derivedName, declaredUser: "other", refused: "user"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			live := &appv1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{Name: resourceID, Namespace: "default"},
				Spec:       appv1alpha1.DatabaseSpec{Name: "orders", Plan: "basic-256mb", DatabaseName: tc.storedName, DatabaseUser: tc.stored},
			}
			// applyDatabase's own guard, which the plan must agree with.
			applyRefuses := tc.declaredName != live.Spec.EffectiveDatabaseName(live.Name) ||
				tc.declaredUser != live.Spec.EffectiveDatabaseUser(live.Name)
			if applyRefuses != (tc.refused != "") {
				t.Fatalf("model error: applyDatabase refuses=%v, table expects %q", applyRefuses, tc.refused)
			}
			want := appv1alpha1.DatabaseSpec{DatabaseName: tc.declaredName, DatabaseUser: tc.declaredUser}
			fields := map[string]BlueprintField{"databaseName": {}, "user": {}}
			probe := live.DeepCopy()
			changed, err := ApplyBlueprintDatabaseSpec(probe, want, fields, blueprintTestNow)
			if tc.refused != "" {
				var conflict *BlueprintFieldConflictError
				if !errors.As(err, &conflict) || conflict.Path != tc.refused {
					t.Fatalf("want a %s conflict, got changed=%v err=%v", tc.refused, changed, err)
				}
				return
			}
			if err != nil || changed || !reflect.DeepEqual(probe.Spec, live.Spec) {
				t.Fatalf("declaring the database's own identity: changed=%v err=%v spec=%#v", changed, err, probe.Spec)
			}
			blueprintDatabaseAgreement(t, live, want, fields)
		})
	}
}

// blueprintKeyValueAgreement is the key-value twin of blueprintDatabaseAgreement.
func blueprintKeyValueAgreement(t *testing.T, live appv1alpha1.KeyValueSpec, want appv1alpha1.KeyValueSpec, fields map[string]BlueprintField) {
	t.Helper()
	probe := *live.DeepCopy()
	changed := ApplyBlueprintKeyValueSpec(&probe, want, fields)

	again := *probe.DeepCopy()
	if changedAgain := ApplyBlueprintKeyValueSpec(&again, want, fields); changedAgain || !reflect.DeepEqual(again, probe) {
		t.Fatalf("apply is not idempotent: second changed=%v\n first: %#v\nsecond: %#v", changedAgain, probe, again)
	}

	var probed []string
	for name := range fields {
		one := *live.DeepCopy()
		if ApplyBlueprintKeyValueSpec(&one, want, oneField(fields, name)) {
			probed = append(probed, name)
		}
	}
	switch {
	case changed && len(probed) == 0:
		t.Fatalf("apply changes the spec but no probed field reports it:\n before: %#v\n  after: %#v", live, probe)
	case !changed && len(probed) > 0:
		t.Fatalf("apply is a no-op but the plan reports %v", probed)
	}
}

func TestApplyBlueprintKeyValueSpecIdempotentAndAgrees(t *testing.T) {
	t.Parallel()
	live := appv1alpha1.KeyValueSpec{
		Plan:            "starter",
		MaxmemoryPolicy: "allkeys-lru",
		PersistenceMode: "journal-snapshot",
		Public:          true,
		IPAllowList:     []appv1alpha1.IPAllowEntry{{CIDR: "192.0.2.0/24"}},
	}
	full := map[string]BlueprintField{
		"plan": {}, "ipAllowList": {}, "maxmemoryPolicy": {}, "persistenceMode": {},
	}
	t.Run("identical spec is a no-op", func(t *testing.T) {
		t.Parallel()
		blueprintKeyValueAgreement(t, live, *live.DeepCopy(), full)
	})
	t.Run("declared changes apply and settle", func(t *testing.T) {
		t.Parallel()
		blueprintKeyValueAgreement(t, live, appv1alpha1.KeyValueSpec{
			Plan: "standard", MaxmemoryPolicy: "volatile-lru", PersistenceMode: "off",
		}, map[string]BlueprintField{"plan": {}, "maxmemoryPolicy": {}, "persistenceMode": {}})
	})
	t.Run("a declared empty allow list disables access", func(t *testing.T) {
		t.Parallel()
		blueprintKeyValueAgreement(t, live, appv1alpha1.KeyValueSpec{}, map[string]BlueprintField{"ipAllowList": {}})
	})
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
