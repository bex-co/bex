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

package deploys

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The rollback comparison (w1/m646, docs/render-artifacts/rollback-comparison.md)
// lists what a rollback to a deploy changes against the live release.

// Secret values the fixture records. None may ever reach a response.
var comparisonSecretValues = []string{"old-value", "new-value", "gone-value", "added-value", "same-value",
	"file-one", "file-two", "file-same", "file-gone", "file-new"}

type comparisonRelease struct {
	spec  appv1alpha1.ReleaseRecordSpec
	tier  string
	env   map[string]string
	files map[string]string
	// adopted is the spec the release was adopted with, when the record keeps one.
	adopted *appv1alpha1.AppSpec
}

// releaseObjects is generation's record and its own env and file copies, as
// the operator writes them (ADR004 § Per-release configuration snapshots).
func releaseObjects(app string, generation int64, r comparisonRelease) []client.Object {
	spec, _ := json.Marshal(r.spec)
	data := map[string][]byte{appv1alpha1.ReleaseRecordSpecKey: spec}
	if r.adopted != nil {
		data[appv1alpha1.ReleaseRecordAdoptedSpecKey], _ = json.Marshal(r.adopted)
	}
	if r.tier != "" {
		cpu, mem := tierResources(r.tier)
		tmpl, _ := json.Marshal(corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name:      appv1alpha1.ReleaseRecordAppContainer,
			Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(cpu), corev1.ResourceMemory: resource.MustParse(mem)}},
		}}}})
		data[appv1alpha1.ReleaseRecordPodTemplateKey] = tmpl
	}
	objs := []client.Object{&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.ReleaseRecordName(app, generation), Namespace: "default"}, Data: data}}
	copyOf := func(source string, values map[string]string) {
		d := map[string][]byte{}
		for k, v := range values {
			d[k] = []byte(v)
		}
		objs = append(objs, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.AppReleaseSnapshotName(app, source, generation), Namespace: "default"}, Data: d})
	}
	if r.spec.EnvFromSecret != "" {
		copyOf(r.spec.EnvFromSecret, r.env)
	}
	if r.files != nil {
		copyOf(app+"-files", r.files)
	}
	return objs
}

func tierResources(tier string) (cpu, mem string) {
	switch tier {
	case "starter":
		return "500m", "512Mi"
	case "pro":
		return "2", "4Gi"
	}
	panic(tier)
}

type comparisonFixture struct {
	svc            *Service
	target, live   store.Deploy
	groupA, groupB string
}

// newComparisonFixture: D1 (generation 1) is the target and D2 (generation 2)
// serves. Between them the start command, health check path and instance
// count changed, env vars and secret files were added, deleted and changed,
// group A was unlinked and group B linked, a group D1 linked was deleted, and
// the plan went from pro down to starter.
func newComparisonFixture(t *testing.T) comparisonFixture {
	t.Helper()
	ctx := context.Background()
	ds := newFakeStore()
	d1, _ := ds.CreateDeploy(ctx, "srv-1", "create", "", 1, store.CommitInfo{}, "")
	if _, err := ds.CloseDeploy(ctx, d1.ID, store.DeployLive, "web@sha256:1"); err != nil {
		t.Fatal(err)
	}
	d2, _ := ds.CreateDeploy(ctx, "srv-1", "config_change", "", 2, store.CommitInfo{}, "")
	if _, err := ds.CloseDeploy(ctx, d2.ID, store.DeployLive, "web@sha256:2"); err != nil {
		t.Fatal(err)
	}
	groupA, groupB, gone := ids.New(ids.EnvGroup), ids.New(ids.EnvGroup), ids.New(ids.EnvGroup)
	app := sampleApp("web", "srv-1")
	app.Spec.Image = ""
	app.Spec.Runtime = "node"
	app.Spec.Tier = "starter"
	app.Labels[core.LabelTenant] = "tea-a"
	objs := []client.Object{app}
	objs = append(objs, releaseObjects("web", 1, comparisonRelease{
		spec: appv1alpha1.ReleaseRecordSpec{Version: 1, StartCommand: "npm start -- --v1", HealthCheckPath: new("/old"), Replicas: new(int32(1)),
			EnvFromSecret: "web-env", EnvFromSecrets: []string{groupA + "-env", gone + "-env"}, FilesFromSecrets: []string{groupA + "-files", "web-files"},
			Runtime: &appv1alpha1.ReleaseRuntimeSettings{PreDeployCommand: "npm run migrate"}},
		tier:  "pro",
		env:   map[string]string{"KEEP": "old-value", "GONE": "gone-value", "SAME": "same-value"},
		files: map[string]string{"a.txt": "file-one", "b.txt": "file-same", "gone.txt": "file-gone"},
	})...)
	objs = append(objs, releaseObjects("web", 2, comparisonRelease{
		spec: appv1alpha1.ReleaseRecordSpec{Version: 1, StartCommand: "npm start -- --v2", HealthCheckPath: new("/healthz"), Replicas: new(int32(3)),
			EnvFromSecret: "web-env", EnvFromSecrets: []string{groupB + "-env"}, FilesFromSecrets: []string{"web-files"},
			Runtime: &appv1alpha1.ReleaseRuntimeSettings{PreDeployCommand: "npm run migrate"}},
		tier:  "starter",
		env:   map[string]string{"KEEP": "new-value", "ADDED": "added-value", "SAME": "same-value"},
		files: map[string]string{"a.txt": "file-two", "b.txt": "file-same", "new.txt": "file-new"},
	})...)
	svc, _ := newService(ds, objs...)
	svc.EnvGroupIDsByName = func(ctx context.Context) (map[string]string, error) {
		if ws, _ := core.WorkspaceFrom(ctx); ws != "tea-a" {
			t.Errorf("groups read in workspace %q, want the service's tea-a", ws)
		}
		return map[string]string{"group-a": groupA, "group-b": groupB}, nil
	}
	return comparisonFixture{svc: svc, target: d1, live: d2, groupA: groupA, groupB: groupB}
}

func TestRollbackComparisonListsWhatChangedSinceTheDeploy(t *testing.T) {
	f := newComparisonFixture(t)
	got, err := f.svc.RollbackComparison(context.Background(), "web", f.target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentLiveDeploy == nil || got.CurrentLiveDeploy.ID != f.live.ID || got.Deploy.ID != f.target.ID {
		t.Fatalf("compared %+v against %+v, want %s against %s", got.Deploy, got.CurrentLiveDeploy, f.target.ID, f.live.ID)
	}
	live, old := got.CurrentLiveDeploy.Snapshot, got.Deploy.Snapshot
	if live == nil || old == nil {
		t.Fatalf("snapshots live=%v target=%v, want both", live, old)
	}
	none := []string{}
	wantLive := ServiceConfigSnapshot{RunCommand: "npm start -- --v2", PreDeployCommand: "npm run migrate", HealthCheckPath: "/healthz",
		Instances: new(int32(3)), Plan: "starter", EnvGroups: []SnapshotEnvGroup{{ID: f.groupB, Name: "group-b"}},
		EnvVarChanges: EnvVarChanges{none, none, none, none, none, none}}
	if !reflect.DeepEqual(*live, wantLive) {
		t.Errorf("live snapshot = %+v\nwant %+v", *live, wantLive)
	}
	wantOld := ServiceConfigSnapshot{RunCommand: "npm start -- --v1", PreDeployCommand: "npm run migrate", HealthCheckPath: "/old",
		Instances: new(int32(1)), Plan: "pro", EnvGroups: []SnapshotEnvGroup{{ID: f.groupA, Name: "group-a"}}, DeletedEnvGroupCount: 1,
		EnvVarChanges: EnvVarChanges{
			CreatedVarKeys: []string{"ADDED"}, DeletedVarKeys: []string{"GONE"}, UpdatedVarKeys: []string{"KEEP"},
			CreatedFileKeys: []string{"new.txt"}, DeletedFileKeys: []string{"gone.txt"}, UpdatedFileKeys: []string{"a.txt"},
		}}
	if !reflect.DeepEqual(*old, wantOld) {
		t.Errorf("target snapshot = %+v\nwant %+v", *old, wantOld)
	}
	if !got.CurrentPlanLower {
		t.Error("the saved starter plan is below the pro plan the deploy ran on, but no plan warning")
	}
}

// The GraphQL answer carries Render's field names, names only.
func TestRollbackComparisonGraphQLNamesKeysAndNeverValues(t *testing.T) {
	f := newComparisonFixture(t)
	res := graphql.Do(graphql.Params{Schema: testSchema(t, f.svc), Context: context.Background(), RequestString: `{
		rollbackComparison(serviceId: "web", deployId: "` + f.target.ID + `") {
			currentPlanLower
			currentLiveDeploy { id serviceConfigSnapshot { runCommand dockerCommand instances plan envGroups { id name } } }
			deploy { id serviceConfigSnapshot {
				runCommand preDeployCommand dockerCommand healthCheckPath instances plan deletedEnvGroupCount
				envGroups { id name }
				envVarChanges { createdVarKeys deletedVarKeys updatedVarKeys createdFileKeys deletedFileKeys updatedFileKeys }
			} }
		}
	}`})
	if len(res.Errors) > 0 {
		t.Fatal(res.Errors)
	}
	raw, _ := json.Marshal(res.Data)
	for _, value := range comparisonSecretValues {
		if strings.Contains(string(raw), value) {
			t.Errorf("response carries the value %q: %s", value, raw)
		}
	}
	var out struct {
		RollbackComparison struct {
			CurrentPlanLower  bool
			CurrentLiveDeploy struct {
				ServiceConfigSnapshot map[string]any
			}
			Deploy struct {
				ServiceConfigSnapshot struct {
					DockerCommand        *string
					Plan                 string
					DeletedEnvGroupCount int
					EnvVarChanges        map[string][]string
				}
			}
		}
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	old := out.RollbackComparison.Deploy.ServiceConfigSnapshot
	if old.DockerCommand != nil || old.Plan != "pro" || old.DeletedEnvGroupCount != 1 {
		t.Errorf("target snapshot: dockerCommand %v plan %q deleted groups %d, want null, pro, 1", old.DockerCommand, old.Plan, old.DeletedEnvGroupCount)
	}
	if got := old.EnvVarChanges["updatedFileKeys"]; !reflect.DeepEqual(got, []string{"a.txt"}) {
		t.Errorf("updatedFileKeys = %v, want [a.txt]", got)
	}
	if got := out.RollbackComparison.CurrentLiveDeploy.ServiceConfigSnapshot["dockerCommand"]; got != nil {
		t.Errorf("live dockerCommand = %v, want null for a native runtime", got)
	}
	if !out.RollbackComparison.CurrentPlanLower {
		t.Error("currentPlanLower = false")
	}
}

// Rolling back to a deploy whose configuration the live release still runs
// changes nothing: every list is empty, so the page says so.
func TestRollbackComparisonOfAnUnchangedDeployIsEmpty(t *testing.T) {
	ctx := context.Background()
	ds := newFakeStore()
	for gen := int64(1); gen <= 2; gen++ {
		d, _ := ds.CreateDeploy(ctx, "srv-1", "manual", "", gen, store.CommitInfo{}, "")
		if _, err := ds.CloseDeploy(ctx, d.ID, store.DeployLive, "web@sha256:x"); err != nil {
			t.Fatal(err)
		}
	}
	same := comparisonRelease{
		spec: appv1alpha1.ReleaseRecordSpec{Version: 1, StartCommand: "./run", HealthCheckPath: new(""), Replicas: new(int32(0)), EnvFromSecret: "web-env"},
		tier: "starter", env: map[string]string{"KEY": "same-value"},
	}
	app := sampleApp("web", "srv-1")
	app.Spec.Tier = "starter"
	objs := append([]client.Object{app}, releaseObjects("web", 1, same)...)
	svc, _ := newService(ds, append(objs, releaseObjects("web", 2, same)...)...)
	list, _ := ds.ListDeploys(ctx, "srv-1", store.DeployFilter{})
	got, err := svc.RollbackComparison(ctx, "web", list[len(list)-1].ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Deploy.Snapshot == nil || got.CurrentLiveDeploy == nil || got.CurrentLiveDeploy.Snapshot == nil {
		t.Fatal("missing snapshot")
	}
	if !reflect.DeepEqual(*got.Deploy.Snapshot, *got.CurrentLiveDeploy.Snapshot) || got.CurrentPlanLower {
		t.Errorf("unchanged deploy compared as %+v against %+v (plan warning %v)", *got.Deploy.Snapshot, *got.CurrentLiveDeploy.Snapshot, got.CurrentPlanLower)
	}
	// An image-backed service's start command is its Docker Command, and a
	// zero replica count is the default single instance.
	if s := got.Deploy.Snapshot; s.DockerCommand != "./run" || s.RunCommand != "" || s.Instances == nil || *s.Instances != 1 {
		t.Errorf("image service snapshot = %+v, want dockerCommand ./run and 1 instance", *s)
	}
}

// A side whose record is gone has no snapshot, and the page shows Render's
// "Unable to load configuration…" for either side rather than an empty diff.
func TestRollbackComparisonWithoutARecordHasNoSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name   string
		remove func(f comparisonFixture) string
		live   bool
	}{
		{"target record reclaimed", func(comparisonFixture) string { return appv1alpha1.ReleaseRecordName("web", 1) }, true},
		{"target env copy reclaimed", func(comparisonFixture) string { return appv1alpha1.ReleaseSnapshotName("web-env", 1) }, true},
		{"target files copy reclaimed", func(comparisonFixture) string { return appv1alpha1.ReleaseSnapshotName("web-files", 1) }, true},
		{"live record missing", func(comparisonFixture) string { return appv1alpha1.ReleaseRecordName("web", 2) }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newComparisonFixture(t)
			if err := f.svc.Client.Delete(context.Background(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: tc.remove(f)}}); err != nil {
				t.Fatal(err)
			}
			got, err := f.svc.RollbackComparison(context.Background(), "web", f.target.ID)
			if err != nil {
				t.Fatal(err)
			}
			if (got.CurrentLiveDeploy.Snapshot != nil) != tc.live || (got.Deploy.Snapshot != nil) == tc.live {
				t.Fatalf("live snapshot %v, target snapshot %v; want only the %s side", got.CurrentLiveDeploy.Snapshot != nil, got.Deploy.Snapshot != nil, map[bool]string{true: "live", false: "target"}[tc.live])
			}
			if tc.live && got.CurrentPlanLower {
				t.Error("a plan warning without the target's record")
			}
		})
	}
}

// A record from before version 1 cannot say which env sources it read.
func TestRollbackComparisonIgnoresLegacyRecords(t *testing.T) {
	f := newComparisonFixture(t)
	legacy, _ := json.Marshal(appv1alpha1.ReleaseRecordSpec{StartCommand: "npm start -- --v1"})
	rec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: appv1alpha1.ReleaseRecordName("web", 1)}, Data: map[string][]byte{appv1alpha1.ReleaseRecordSpecKey: legacy}}
	if err := f.svc.Client.Update(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.RollbackComparison(context.Background(), "web", f.target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Deploy.Snapshot != nil {
		t.Fatalf("legacy record compared as %+v", *got.Deploy.Snapshot)
	}
}

// Two services linking one group each compare their own env and file copies:
// a group's copies are scoped by the reading service (AppReleaseSnapshotName),
// and only the service's own sources are diffed.
func TestRollbackComparisonReadsOnlyTheServicesOwnCopies(t *testing.T) {
	ctx := context.Background()
	ds := newFakeStore()
	group := ids.New(ids.EnvGroup)
	var objs []client.Object
	targets := map[string]string{}
	for _, svc := range []struct{ name, id, oldValue, newValue string }{
		{"api", "srv-a", "same-value", "same-value"},
		{"web", "srv-w", "old-value", "new-value"},
	} {
		for gen := int64(1); gen <= 2; gen++ {
			d, _ := ds.CreateDeploy(ctx, svc.id, "manual", "", gen, store.CommitInfo{}, "")
			if _, err := ds.CloseDeploy(ctx, d.ID, store.DeployLive, svc.name+"@sha256:x"); err != nil {
				t.Fatal(err)
			}
			if gen == 1 {
				targets[svc.name] = d.ID
			}
			value := map[int64]string{1: svc.oldValue, 2: svc.newValue}[gen]
			objs = append(objs, releaseObjects(svc.name, gen, comparisonRelease{
				spec: appv1alpha1.ReleaseRecordSpec{Version: 1, EnvFromSecret: svc.name + "-env", EnvFromSecrets: []string{group + "-env"}},
				env:  map[string]string{"SHARED": value},
			})...)
			// The group's own per-service copies differ at every generation;
			// a group's values are not part of the comparison.
			objs = append(objs, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: appv1alpha1.AppReleaseSnapshotName(svc.name, group+"-env", gen)},
				Data: map[string][]byte{"GROUP_KEY": []byte(value + string(rune('0'+gen)))}})
		}
		objs = append(objs, sampleApp(svc.name, svc.id))
	}
	service, _ := newService(ds, objs...)
	for name, want := range map[string][]string{"api": nil, "web": {"SHARED"}} {
		got, err := service.RollbackComparison(ctx, name, targets[name])
		if err != nil {
			t.Fatal(err)
		}
		if c := got.Deploy.Snapshot.EnvVarChanges; !slices.Equal(c.UpdatedVarKeys, want) || len(c.CreatedVarKeys)+len(c.DeletedVarKeys) != 0 {
			t.Errorf("%s: changes %+v, want only %v updated", name, c, want)
		}
	}
}

// A cron job runs a command with no instances, health check or pre-deploy step.
func TestRollbackComparisonFollowsTheServiceType(t *testing.T) {
	f := newComparisonFixture(t)
	app := getApp(t, f.svc.Client, "web")
	app.Spec.Type = appv1alpha1.TypeCronJob
	app.Spec.Runtime = "docker"
	if err := f.svc.Client.Update(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	cron, _ := json.Marshal(appv1alpha1.ReleaseRecordSpec{Version: 1, StartCommand: "ignored", Command: new("./report --daily"),
		HealthCheckPath: new("/old"), Replicas: new(int32(1)), EnvFromSecret: "web-env", FilesFromSecrets: []string{"web-files"},
		Runtime: &appv1alpha1.ReleaseRuntimeSettings{PreDeployCommand: "npm run migrate"}})
	rec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: appv1alpha1.ReleaseRecordName("web", 1)}, Data: map[string][]byte{appv1alpha1.ReleaseRecordSpecKey: cron}}
	if err := f.svc.Client.Update(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.RollbackComparison(context.Background(), "web", f.target.ID)
	if err != nil {
		t.Fatal(err)
	}
	s := got.Deploy.Snapshot
	if s == nil || s.DockerCommand != "./report --daily" || s.RunCommand != "" || s.PreDeployCommand != "" || s.HealthCheckPath != "" || s.Instances != nil {
		t.Fatalf("cron snapshot = %+v, want only dockerCommand ./report --daily", s)
	}
}

// A static site compares the environment its builds read (w1/m660): the env
// var, secret file and environment group lists a service's comparison has,
// names only, and none of a service's commands, health check, instance count
// or plan, which a static site does not have.
func TestRollbackComparisonOfAStaticSiteComparesItsBuildEnvironment(t *testing.T) {
	f := newComparisonFixture(t)
	app := getApp(t, f.svc.Client, "web")
	app.Spec.Type = appv1alpha1.TypeStaticSite
	app.Spec.Runtime, app.Spec.Tier = "", ""
	if err := f.svc.Client.Update(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.RollbackComparison(context.Background(), "web", f.target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentLiveDeploy == nil || got.CurrentLiveDeploy.Snapshot == nil || got.Deploy.Snapshot == nil {
		t.Fatalf("static site compared as %+v / %+v, want both snapshots", got.CurrentLiveDeploy, got.Deploy)
	}
	none := []string{}
	wantLive := ServiceConfigSnapshot{EnvGroups: []SnapshotEnvGroup{{ID: f.groupB, Name: "group-b"}},
		EnvVarChanges: EnvVarChanges{none, none, none, none, none, none}}
	if !reflect.DeepEqual(*got.CurrentLiveDeploy.Snapshot, wantLive) {
		t.Errorf("live snapshot = %+v\nwant %+v", *got.CurrentLiveDeploy.Snapshot, wantLive)
	}
	wantOld := ServiceConfigSnapshot{EnvGroups: []SnapshotEnvGroup{{ID: f.groupA, Name: "group-a"}}, DeletedEnvGroupCount: 1,
		EnvVarChanges: EnvVarChanges{
			CreatedVarKeys: []string{"ADDED"}, DeletedVarKeys: []string{"GONE"}, UpdatedVarKeys: []string{"KEEP"},
			CreatedFileKeys: []string{"new.txt"}, DeletedFileKeys: []string{"gone.txt"}, UpdatedFileKeys: []string{"a.txt"},
		}}
	if !reflect.DeepEqual(*got.Deploy.Snapshot, wantOld) {
		t.Errorf("target snapshot = %+v\nwant %+v", *got.Deploy.Snapshot, wantOld)
	}
	if got.CurrentPlanLower {
		t.Error("a static site has no plan to warn about")
	}

	res := graphql.Do(graphql.Params{Schema: testSchema(t, f.svc), Context: context.Background(), RequestString: `{
		rollbackComparison(serviceId: "web", deployId: "` + f.target.ID + `") {
			deploy { serviceConfigSnapshot {
				runCommand preDeployCommand dockerCommand healthCheckPath instances plan deletedEnvGroupCount
				envGroups { id name }
				envVarChanges { createdVarKeys deletedVarKeys updatedVarKeys createdFileKeys deletedFileKeys updatedFileKeys }
			} }
		}
	}`})
	if len(res.Errors) > 0 {
		t.Fatal(res.Errors)
	}
	raw, _ := json.Marshal(res.Data)
	for _, value := range comparisonSecretValues {
		if strings.Contains(string(raw), value) {
			t.Errorf("response carries the value %q: %s", value, raw)
		}
	}
	for _, field := range []string{"runCommand", "preDeployCommand", "dockerCommand", "healthCheckPath", "instances", "plan"} {
		if !strings.Contains(string(raw), `"`+field+`":null`) {
			t.Errorf("static snapshot's %s is not null: %s", field, raw)
		}
	}
}

// A static Rollback or Restart copies an earlier release's retained
// publication, so it serves files that release's build produced. Its side of
// the comparison is that build's environment, read through the copy its
// adopted spec selected, not the sources saved when the copy was requested
// (w1/m660). Here D1 built with old-value and D2 with new-value; D3 rolled
// back to D1 while new-value stayed saved, and D4 restarted D3.
func TestRollbackComparisonOfAStaticCopyReadsTheBuildItCopied(t *testing.T) {
	ctx := context.Background()
	newFixture := func(t *testing.T) (*Service, []store.Deploy) {
		t.Helper()
		ds := newFakeStore()
		var deploys []store.Deploy
		for gen, trigger := range []string{"create", "manual", "rollback", "restart"} {
			d, _ := ds.CreateDeploy(ctx, "srv-1", trigger, "", int64(gen+1), store.CommitInfo{}, "")
			if _, err := ds.CloseDeploy(ctx, d.ID, store.DeployLive, ""); err != nil {
				t.Fatal(err)
			}
			deploys = append(deploys, d)
		}
		app := sampleApp("web", "srv-1")
		app.Spec = appv1alpha1.AppSpec{Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/bex-co/site", PublishPath: "dist"}
		app.Status.ActiveRevision, app.Status.ReleaseGeneration = "rev-4", 4
		copyOf := func(gen int64, prefix string) *appv1alpha1.AppSpec {
			spec := app.Spec
			spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: gen, StaticPrefix: prefix}
			return &spec
		}
		built := func(value string, adopted *appv1alpha1.AppSpec) comparisonRelease {
			return comparisonRelease{spec: appv1alpha1.ReleaseRecordSpec{Version: 1, EnvFromSecret: "web-env"},
				env: map[string]string{"KEY": value}, adopted: adopted}
		}
		objs := []client.Object{app}
		objs = append(objs, releaseObjects("web", 1, built("old-value", nil))...)
		// A selection the saved spec still carried from an earlier release is
		// not this release's: D2 built.
		objs = append(objs, releaseObjects("web", 2, built("new-value", copyOf(1, "web/rev-1/")))...)
		objs = append(objs, releaseObjects("web", 3, built("new-value", copyOf(3, "web/rev-1/")))...)
		objs = append(objs, releaseObjects("web", 4, built("new-value", copyOf(4, "tea-a/web/rev-3/")))...)
		svc, _ := newService(ds, objs...)
		return svc, deploys
	}
	updated := func(t *testing.T, svc *Service, target store.Deploy) []string {
		t.Helper()
		got, err := svc.RollbackComparison(ctx, "web", target.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.CurrentLiveDeploy == nil || got.CurrentLiveDeploy.Snapshot == nil || got.Deploy.Snapshot == nil {
			t.Fatalf("compared %+v against %+v, want both snapshots", got.Deploy, got.CurrentLiveDeploy)
		}
		return got.Deploy.Snapshot.EnvVarChanges.UpdatedVarKeys
	}

	svc, deploys := newFixture(t)
	if got := updated(t, svc, deploys[0]); len(got) != 0 {
		t.Errorf("Rollback on D1, whose build the live copy serves: updated %v, want nothing", got)
	}
	if got := updated(t, svc, deploys[1]); !slices.Equal(got, []string{"KEY"}) {
		t.Errorf("Rollback on D2: updated %v, want [KEY]: live serves D1's build", got)
	}
	if got := updated(t, svc, deploys[2]); len(got) != 0 {
		t.Errorf("Rollback on D3, a copy of the build live serves: updated %v, want nothing", got)
	}

	t.Run("the copied build's record reclaimed", func(t *testing.T) {
		svc, deploys := newFixture(t)
		if err := svc.Client.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: appv1alpha1.ReleaseRecordName("web", 1)}}); err != nil {
			t.Fatal(err)
		}
		got, err := svc.RollbackComparison(ctx, "web", deploys[1].ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.CurrentLiveDeploy == nil || got.CurrentLiveDeploy.Snapshot != nil || got.Deploy.Snapshot == nil {
			t.Fatalf("live %+v, target %+v: want the live side unavailable, not the copy's saved sources", got.CurrentLiveDeploy, got.Deploy.Snapshot)
		}
	})
}

// staticBuildGeneration follows a copy only to the earlier release its prefix
// names; a prefix naming no earlier release has no build to compare.
func TestStaticBuildGenerationRefusesAPrefixNamingNoEarlierRelease(t *testing.T) {
	ctx := context.Background()
	app := sampleApp("web", "srv-1")
	app.Spec = appv1alpha1.AppSpec{Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/bex-co/site"}
	var objs []client.Object
	for gen, prefix := range map[int64]string{2: "web/rev-2/", 3: "web/latest/"} {
		spec := app.Spec
		spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: gen, StaticPrefix: prefix}
		objs = append(objs, releaseObjects("web", gen, comparisonRelease{spec: appv1alpha1.ReleaseRecordSpec{Version: 1}, adopted: &spec})...)
	}
	svc, _ := newService(newFakeStore(), append(objs, app)...)
	for _, gen := range []int64{2, 3} {
		if got, err := svc.staticBuildGeneration(ctx, app, gen); err != nil || got != 0 {
			t.Errorf("generation %d resolved to (%d, %v), want 0", gen, got, err)
		}
	}
	if got, err := svc.staticBuildGeneration(ctx, app, 7); err != nil || got != 7 {
		t.Errorf("a release with no record resolved to (%d, %v), want itself", got, err)
	}
}

// The read takes the deploy's read relation and the Environment page's key access.
func TestRollbackComparisonAuthorization(t *testing.T) {
	f := newComparisonFixture(t)
	refused := errors.New("environment keys refused")
	f.svc.EnvironmentKeys = func(context.Context, string) error { return refused }
	if _, err := f.svc.RollbackComparison(context.Background(), "web", f.target.ID); !errors.Is(err, refused) {
		t.Fatalf("without environment key access: %v, want %v", err, refused)
	}
	f.svc.EnvironmentKeys = nil
	f.svc.Authz = deniedDeployHookChecker{}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-x", Method: "session"})
	if _, err := f.svc.RollbackComparison(ctx, "web", f.target.ID); err == nil {
		t.Fatal("a caller without can_view_activity read the comparison")
	}
	f.svc.Authz = nil
	if _, err := f.svc.RollbackComparison(context.Background(), "web", "dep-missing"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("unknown deploy: %v, want not found", err)
	}
}

// A rollback keeps the saved instance count when it moved on since the
// target release: the operator restores the record's count only while the
// saved count still matches the record's SavedReplicas
// (TestSelectedReplicaCountAllowsLaterOperationalScale), so the comparison
// must preview the saved count, not the record's. Here D1 ran on 1 instance,
// the service was scaled to 4 (D2), and a rollback to D1 keeps 4.
func TestRollbackComparisonTargetInstancesFollowLaterScale(t *testing.T) {
	ctx := context.Background()
	ds := newFakeStore()
	d1, _ := ds.CreateDeploy(ctx, "srv-1", "create", "", 1, store.CommitInfo{}, "")
	if _, err := ds.CloseDeploy(ctx, d1.ID, store.DeployLive, "web@sha256:1"); err != nil {
		t.Fatal(err)
	}
	d2, _ := ds.CreateDeploy(ctx, "srv-1", "config_change", "", 2, store.CommitInfo{}, "")
	if _, err := ds.CloseDeploy(ctx, d2.ID, store.DeployLive, "web@sha256:2"); err != nil {
		t.Fatal(err)
	}
	same := map[string]string{"KEEP": "same-value"}
	var objs []client.Object
	objs = append(objs, releaseObjects("web", 1, comparisonRelease{
		spec: appv1alpha1.ReleaseRecordSpec{Version: 1, StartCommand: "./run", HealthCheckPath: new("/healthz"),
			Replicas: new(int32(1)), SavedReplicas: new(int32(1)), EnvFromSecret: "web-env"},
		env: same,
	})...)
	objs = append(objs, releaseObjects("web", 2, comparisonRelease{
		spec: appv1alpha1.ReleaseRecordSpec{Version: 1, StartCommand: "./run", HealthCheckPath: new("/healthz"),
			Replicas: new(int32(4)), SavedReplicas: new(int32(4)), EnvFromSecret: "web-env"},
		env: same,
	})...)
	app := sampleApp("web", "srv-1")
	app.Spec.Image = ""
	app.Spec.Runtime = "node"
	app.Spec.Tier = "starter"
	app.Spec.Replicas = 4
	objs = append(objs, app)
	svc, _ := newService(ds, objs...)
	got, err := svc.RollbackComparison(ctx, "web", d1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentLiveDeploy == nil || got.CurrentLiveDeploy.Snapshot == nil || got.Deploy.Snapshot == nil {
		t.Fatalf("live=%v target=%v, want both snapshots", got.CurrentLiveDeploy, got.Deploy.Snapshot)
	}
	if n := *got.Deploy.Snapshot.Instances; n != 4 {
		t.Errorf("target instances = %d, want 4: the rollback keeps the saved count", n)
	}
	if n := *got.CurrentLiveDeploy.Snapshot.Instances; n != 4 {
		t.Errorf("live instances = %d, want 4", n)
	}
	if !reflect.DeepEqual(*got.Deploy.Snapshot, *got.CurrentLiveDeploy.Snapshot) {
		t.Errorf("target compared as %+v against live %+v, want no differences", *got.Deploy.Snapshot, *got.CurrentLiveDeploy.Snapshot)
	}
}

// The live side shows the count live runs: when live is itself a selected
// release (a rollback serving) that was explicitly scaled, it runs the
// saved count, not its record's. A later rollback keeps that count too, so
// both sides read it and the Instances row stays empty.
func TestRollbackComparisonSelectedLiveShowsTheRunningCount(t *testing.T) {
	ctx := context.Background()
	ds := newFakeStore()
	d1, _ := ds.CreateDeploy(ctx, "srv-1", "create", "", 1, store.CommitInfo{}, "")
	if _, err := ds.CloseDeploy(ctx, d1.ID, store.DeployLive, "web@sha256:1"); err != nil {
		t.Fatal(err)
	}
	d2, _ := ds.CreateDeploy(ctx, "srv-1", "rollback", "", 2, store.CommitInfo{}, "")
	if _, err := ds.CloseDeploy(ctx, d2.ID, store.DeployLive, "web@sha256:2"); err != nil {
		t.Fatal(err)
	}
	same := map[string]string{"KEEP": "same-value"}
	var objs []client.Object
	for gen := int64(1); gen <= 2; gen++ {
		objs = append(objs, releaseObjects("web", gen, comparisonRelease{
			spec: appv1alpha1.ReleaseRecordSpec{Version: 1, StartCommand: "./run", HealthCheckPath: new("/healthz"),
				Replicas: new(int32(2)), SavedReplicas: new(int32(2)), EnvFromSecret: "web-env"},
			env: same,
		})...)
	}
	app := sampleApp("web", "srv-1")
	app.Spec.Image = ""
	app.Spec.Runtime = "node"
	app.Spec.Tier = "starter"
	app.Spec.Replicas = 6
	app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 2, SourceGeneration: 1, Image: "web@sha256:2"}
	app.Annotations = map[string]string{appv1alpha1.AnnotationReleaseConfigScaled: strconv.FormatInt(2, 10)}
	app.Status.ReleaseGeneration = 2
	app.Status.SelectedReleaseGeneration = 2
	objs = append(objs, app)
	svc, _ := newService(ds, objs...)
	got, err := svc.RollbackComparison(ctx, "web", d1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentLiveDeploy == nil || got.CurrentLiveDeploy.Snapshot == nil || got.Deploy.Snapshot == nil {
		t.Fatalf("live=%v target=%v, want both snapshots", got.CurrentLiveDeploy, got.Deploy.Snapshot)
	}
	if n := *got.CurrentLiveDeploy.Snapshot.Instances; n != 6 {
		t.Errorf("live instances = %d, want 6: the scaled selection runs the saved count", n)
	}
	if n := *got.Deploy.Snapshot.Instances; n != 6 {
		t.Errorf("target instances = %d, want 6: the rollback keeps the saved count", n)
	}
}

// previewInstanceCount mirrors the operator's selection guard cell for cell:
// the record's count runs unless an explicit scale stamped this release or
// the saved count moved on since the record was written, and a side that
// never materializes through the guard keeps the record's count.
func TestPreviewInstanceCountMirrorsTheSelectionGuard(t *testing.T) {
	app := func(saved int32, scaled string) *appv1alpha1.App {
		a := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Replicas: saved}}
		if scaled != "" {
			a.Annotations = map[string]string{appv1alpha1.AnnotationReleaseConfigScaled: scaled}
		}
		return a
	}
	record := func(count int32, savedAt *int32) appv1alpha1.ReleaseRecordSpec {
		return appv1alpha1.ReleaseRecordSpec{Replicas: new(count), SavedReplicas: savedAt}
	}
	for _, tc := range []struct {
		name    string
		a       *appv1alpha1.App
		spec    appv1alpha1.ReleaseRecordSpec
		preview previewInstances
		want    int32
	}{
		{"an unselected side keeps the record", app(4, ""), record(1, new(int32(1))), previewInstances{}, 1},
		{"an unselected side ignores a later saved scale", app(4, ""), record(1, new(int32(4))), previewInstances{}, 1},
		{"a selection keeps the record while saved matches", app(1, ""), record(1, new(int32(1))), previewInstances{selected: true}, 1},
		{"a selection follows a later saved scale", app(4, ""), record(1, new(int32(1))), previewInstances{selected: true}, 4},
		{"a selection follows an explicit scale", app(4, "2"), record(1, new(int32(1))), previewInstances{selected: true, scaledRelease: 2}, 4},
		{"a scale stamp on another release is ignored", app(4, "3"), record(1, new(int32(4))), previewInstances{selected: true, scaledRelease: 2}, 1},
		{"a legacy record without saved replicas keeps the record", app(4, ""), record(1, nil), previewInstances{selected: true}, 1},
		{"a zero record is the one-instance default", app(0, ""), record(0, nil), previewInstances{selected: true}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := previewInstanceCount(tc.a, tc.spec, tc.preview); got != tc.want {
				t.Errorf("previewInstanceCount = %d, want %d", got, tc.want)
			}
		})
	}
}
