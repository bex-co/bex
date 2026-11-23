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
	"reflect"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func execGQL(t *testing.T, schema graphql.Schema, query string, vars map[string]any) *graphql.Result {
	t.Helper()
	return graphql.Do(graphql.Params{
		Schema:         schema,
		Context:        context.Background(),
		RequestString:  query,
		VariableValues: vars,
	})
}

func TestGraphQLUpdateSourceStaticSiteDeploysOnce(t *testing.T) {
	st := &recordingStore{}
	app := managedRepoApp("site")
	app.Spec.Type = appv1alpha1.TypeStaticSite
	app.Spec.Runtime, app.Spec.Builder = "", ""
	app.Spec.PublishPath = "dist"
	app.Spec.BuildCommand = "npm run build"
	svc, cl := newService(st, app)
	var deployed []string
	svc.TriggerSourceDeploy = func(_ context.Context, name string) (string, error) {
		deployed = append(deployed, name)
		return "dep-static-1", nil
	}
	schema := sourceSwapSchema(t, svc)

	res := execGQL(t, schema, `mutation {
		setRepo(
			id: "site",
			repo: "https://github.com/acme/docs",
			branch: "release",
			buildCommand: "",
			publishPath: "public",
			deploy: true
		) { repo branch buildCommand publishPath latestDeployId }
	}`, nil)
	if len(res.Errors) > 0 {
		t.Fatalf("setRepo static site errors: %v", res.Errors)
	}
	got := res.Data.(map[string]any)["setRepo"].(map[string]any)
	if got["latestDeployId"] != "dep-static-1" {
		t.Fatalf("latestDeployId = %v, want dep-static-1", got["latestDeployId"])
	}
	spec := getApp(t, cl, "site").Spec
	if spec.Repo != "https://github.com/acme/docs" || spec.Branch != "release" || spec.BuildCommand != "" || spec.PublishPath != "public" {
		t.Fatalf("unexpected static spec: %+v", spec)
	}
	if len(st.deployCalls) != 0 {
		t.Fatalf("ApplyServicePatch opened implicit deploys before TriggerSourceDeploy: %+v", st.deployCalls)
	}
	if len(deployed) != 1 || deployed[0] != "site" {
		t.Fatalf("TriggerSourceDeploy calls = %v, want [site]", deployed)
	}
}

func TestGraphQLUpdateSourceStaticSiteValidationRefusals(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{
			name:  "runtime refused on static site",
			query: `mutation { setRepo(id: "site", repo: "https://github.com/acme/docs", branch: "main", runtime: "node", publishPath: "dist", deploy: true) { id } }`,
			want:  "static site has no runtime",
		},
		{
			name:  "startCommand refused on static site",
			query: `mutation { setRepo(id: "site", repo: "https://github.com/acme/docs", branch: "main", startCommand: "npm start", publishPath: "dist", deploy: true) { id } }`,
			want:  "start command is not applicable",
		},
		{
			name:  "empty publishPath refused",
			query: `mutation { setRepo(id: "site", repo: "https://github.com/acme/docs", branch: "main", publishPath: "", deploy: true) { id } }`,
			want:  "publishPath",
		},
		{
			name:  "command refused on static site",
			query: `mutation { setRepo(id: "site", repo: "https://github.com/acme/docs", branch: "main", command: "echo hi", deploy: true) { id } }`,
			want:  "not a cron_job",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := managedRepoApp("site")
			app.Spec.Type = appv1alpha1.TypeStaticSite
			app.Spec.PublishPath = "dist"
			svc, cl := newService(&recordingStore{}, app)
			deployed := 0
			svc.TriggerSourceDeploy = func(context.Context, string) (string, error) {
				deployed++
				return "dep-unexpected", nil
			}
			before := getApp(t, cl, "site")
			res := execGQL(t, sourceSwapSchema(t, svc), tc.query, nil)
			if len(res.Errors) == 0 || !strings.Contains(res.Errors[0].Message, tc.want) {
				t.Fatalf("errors = %v, want substring %q", res.Errors, tc.want)
			}
			if deployed != 0 {
				t.Fatalf("rejected update triggered %d deploys", deployed)
			}
			if !reflect.DeepEqual(before.Spec, getApp(t, cl, "site").Spec) {
				t.Fatalf("rejected update mutated spec: got %+v, want %+v", getApp(t, cl, "site").Spec, before.Spec)
			}
		})
	}
}

func TestGraphQLUpdateSourceRuntimeAndCommandsAcrossServiceTypes(t *testing.T) {
	for _, svcType := range []string{
		appv1alpha1.TypeWebService,
		appv1alpha1.TypePrivateService,
		appv1alpha1.TypeBackgroundWorker,
	} {
		t.Run(svcType+"/native-to-docker-and-back", func(t *testing.T) {
			st := &recordingStore{}
			app := managedRepoApp("svc")
			app.Spec.Type = svcType
			app.Spec.Runtime, app.Spec.Builder = "node", "native"
			app.Spec.BuildCommand, app.Spec.StartCommand = "npm ci", "npm start"
			svc, cl := newService(st, app)
			deployed := 0
			svc.TriggerSourceDeploy = func(context.Context, string) (string, error) {
				deployed++
				return "dep-rt-1", nil
			}
			schema := sourceSwapSchema(t, svc)

			// Native → Docker drops stale native commands and deploys once.
			res := execGQL(t, schema, `mutation {
				setRepo(id: "svc", repo: "https://github.com/acme/docker-app", branch: "main", runtime: "docker", deploy: true) {
					repo runtime builder buildCommand startCommand latestDeployId
				}
			}`, nil)
			if len(res.Errors) > 0 {
				t.Fatalf("native->docker errors: %v", res.Errors)
			}
			spec := getApp(t, cl, "svc").Spec
			if spec.Runtime != "docker" || spec.Builder != "dockerfile" || spec.BuildCommand != "" || spec.StartCommand != "" {
				t.Fatalf("after native->docker: %+v", spec)
			}
			if deployed != 1 || len(st.deployCalls) != 0 {
				t.Fatalf("deploys: trigger=%d implicit=%d, want 1/0", deployed, len(st.deployCalls))
			}

			// Docker → Python native requires and persists both commands, deploys once.
			res = execGQL(t, schema, `mutation {
				setRepo(id: "svc", repo: "https://github.com/acme/py-app", branch: "v2", runtime: "python", buildCommand: "pip install -r requirements.txt", startCommand: "gunicorn app:app", deploy: true) {
					repo branch runtime builder buildCommand startCommand latestDeployId
				}
			}`, nil)
			if len(res.Errors) > 0 {
				t.Fatalf("docker->python errors: %v", res.Errors)
			}
			spec = getApp(t, cl, "svc").Spec
			if spec.Runtime != "python" || spec.Builder != "native" || spec.BuildCommand != "pip install -r requirements.txt" || spec.StartCommand != "gunicorn app:app" {
				t.Fatalf("after docker->python: %+v", spec)
			}
			if deployed != 2 || len(st.deployCalls) != 0 {
				t.Fatalf("deploys: trigger=%d implicit=%d, want 2/0", deployed, len(st.deployCalls))
			}

			// Git → Image deploys once.
			res = execGQL(t, schema, `mutation {
				setImage(id: "svc", image: "nginx:1.27-alpine", deploy: true) {
					imagePath repo latestDeployId
				}
			}`, nil)
			if len(res.Errors) > 0 {
				t.Fatalf("git->image errors: %v", res.Errors)
			}
			if got := res.Data.(map[string]any)["setImage"].(map[string]any)["latestDeployId"]; got != "dep-rt-1" {
				t.Fatalf("setImage latestDeployId = %v, want dep-rt-1", got)
			}
			if deployed != 3 || len(st.deployCalls) != 0 {
				t.Fatalf("deploys: trigger=%d implicit=%d, want 3/0", deployed, len(st.deployCalls))
			}
		})
	}
}

func TestGraphQLUpdateSourceCronJobTransitions(t *testing.T) {
	for _, argName := range []string{"startCommand", "command"} {
		t.Run("docker-to-native-via-"+argName, func(t *testing.T) {
			st := &recordingStore{}
			app := managedRepoApp("nightly")
			app.Spec.Type = appv1alpha1.TypeCronJob
			app.Spec.Schedule = "0 * * * *"
			app.Spec.Runtime, app.Spec.Builder = "docker", "dockerfile"
			app.Spec.Command = "/old-docker-cmd"
			svc, cl := newService(st, app)
			deployed := 0
			svc.TriggerSourceDeploy = func(context.Context, string) (string, error) {
				deployed++
				return "dep-cron-1", nil
			}
			schema := sourceSwapSchema(t, svc)

			query := `mutation {
				setRepo(id: "nightly", repo: "https://github.com/acme/jobs", branch: "main", runtime: "node", buildCommand: "npm ci", ` + argName + `: "node job.js", deploy: true) {
					runtime startCommand command latestDeployId
				}
			}`
			res := execGQL(t, schema, query, nil)
			if len(res.Errors) > 0 {
				t.Fatalf("cron docker->native errors: %v", res.Errors)
			}
			spec := getApp(t, cl, "nightly").Spec
			if spec.Runtime != "node" || spec.Builder != "native" || spec.BuildCommand != "npm ci" || spec.StartCommand != "node job.js" || spec.Command != "" {
				t.Fatalf("after cron docker->native: %+v", spec)
			}
			if deployed != 1 || len(st.deployCalls) != 0 {
				t.Fatalf("deploys: trigger=%d implicit=%d, want 1/0", deployed, len(st.deployCalls))
			}

			// Native cron → Docker cron clears native commands and deploys once.
			res = execGQL(t, schema, `mutation {
				setRepo(id: "nightly", repo: "https://github.com/acme/docker-jobs", branch: "main", runtime: "docker", deploy: true) {
					runtime startCommand command latestDeployId
				}
			}`, nil)
			if len(res.Errors) > 0 {
				t.Fatalf("cron native->docker errors: %v", res.Errors)
			}
			spec = getApp(t, cl, "nightly").Spec
			if spec.Runtime != "docker" || spec.Builder != "dockerfile" || spec.BuildCommand != "" || spec.StartCommand != "" {
				t.Fatalf("after cron native->docker: %+v", spec)
			}
			if deployed != 2 {
				t.Fatalf("deployed = %d, want 2", deployed)
			}
		})
	}
}

func TestGraphQLUpdateSourceSuspendedSavesWithoutDeploying(t *testing.T) {
	app := managedRepoApp("web")
	app.Spec.Suspended = true
	svc, cl := newService(&recordingStore{}, app)
	deployed := 0
	svc.TriggerSourceDeploy = func(context.Context, string) (string, error) {
		deployed++
		return "dep-should-not-happen", nil
	}
	schema := sourceSwapSchema(t, svc)

	res := execGQL(t, schema, `mutation {
		setRepo(id: "web", repo: "https://github.com/acme/two", branch: "main", deploy: true) {
			repo latestDeployId
		}
	}`, nil)
	if len(res.Errors) > 0 {
		t.Fatalf("suspended setRepo errors: %v", res.Errors)
	}
	got := res.Data.(map[string]any)["setRepo"].(map[string]any)
	if got["latestDeployId"] != nil {
		t.Fatalf("suspended latestDeployId = %v, want nil", got["latestDeployId"])
	}
	if getApp(t, cl, "web").Spec.Repo != "https://github.com/acme/two" {
		t.Fatal("suspended setRepo did not save the new repo")
	}

	res = execGQL(t, schema, `mutation {
		setImage(id: "web", image: "nginx:1.27-alpine", deploy: true) {
			imagePath latestDeployId
		}
	}`, nil)
	if len(res.Errors) > 0 {
		t.Fatalf("suspended setImage errors: %v", res.Errors)
	}
	if res.Data.(map[string]any)["setImage"].(map[string]any)["latestDeployId"] != nil {
		t.Fatal("suspended setImage returned a deploy id")
	}
	if deployed != 0 {
		t.Fatalf("suspended updates triggered %d deploys, want 0", deployed)
	}
}

func TestGraphQLUpdateSourceValidationRejectsConflictingOrInvalidFields(t *testing.T) {
	app := managedRepoApp("web")
	app.Spec.Runtime, app.Spec.Builder = "docker", "dockerfile"
	app.Spec.BuildCommand, app.Spec.StartCommand = "", ""
	svc, cl := newService(&recordingStore{}, app)
	deployed := 0
	svc.TriggerSourceDeploy = func(context.Context, string) (string, error) {
		deployed++
		return "dep-no", nil
	}
	before := getApp(t, cl, "web")
	schema := sourceSwapSchema(t, svc)

	for _, tc := range []struct {
		name, query, want string
	}{
		{"unknown runtime", `mutation { setRepo(id: "web", repo: "https://github.com/acme/a", runtime: "cobol", deploy: true) { id } }`, "unsupported runtime"},
		{"native missing commands", `mutation { setRepo(id: "web", repo: "https://github.com/acme/a", runtime: "node", deploy: true) { id } }`, "requires buildCommand and startCommand"},
		{"publishPath on web", `mutation { setRepo(id: "web", repo: "https://github.com/acme/a", publishPath: "dist", deploy: true) { id } }`, "static_site"},
		{"both startCommand and command", `mutation { setRepo(id: "web", repo: "https://github.com/acme/a", startCommand: "a", command: "b", deploy: true) { id } }`, "cannot set both startCommand and command"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := execGQL(t, schema, tc.query, nil)
			if len(res.Errors) == 0 || !strings.Contains(res.Errors[0].Message, tc.want) {
				t.Fatalf("errors = %v, want %q", res.Errors, tc.want)
			}
			if deployed != 0 {
				t.Fatalf("rejected update triggered %d deploys", deployed)
			}
			if !reflect.DeepEqual(before.Spec, getApp(t, cl, "web").Spec) {
				t.Fatal("rejected update mutated spec")
			}
		})
	}
}

func TestGraphQLLinkBuildSourceDeployFlag(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	cl := svc.Client
	web, err := svc.Create(ctx, CreateRequest{Name: "web", Type: appv1alpha1.TypeWebService, Image: "nginx:1"})
	if err != nil {
		t.Fatal(err)
	}
	unbuilt, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{
		Name: "unbuilt",
		Git:  &BuildSourceGit{RepoURL: "https://github.com/acme/mono", Branch: "main", Runtime: "node", BuildCommand: "npm ci"},
	})
	if err != nil {
		t.Fatal(err)
	}
	built, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{
		Name: "built",
		Git:  &BuildSourceGit{RepoURL: "https://github.com/acme/built", Branch: "main", Runtime: "docker"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Mark the built source's carrier as having a succeeded build artifact.
	carrier := svc.linkedBuildSource(ctx, built.OwnerID, built.ID)
	if carrier == nil {
		t.Fatalf("carrier for %s not found", built.ID)
	}
	carrier.Status.ActiveRevision = "rev-1"
	carrier.Status.Image = "registry.local/acme/built@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := cl.Update(ctx, carrier); err != nil {
		t.Fatal(err)
	}

	deployed := 0
	svc.TriggerSourceDeploy = func(context.Context, string) (string, error) {
		deployed++
		return "dep-bds-1", nil
	}
	schema := sourceSwapSchema(t, svc)
	exec := func(q string) *graphql.Result {
		return graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: q})
	}

	// Unbuilt git build source with deploy: true saves link cleanly and opens no deploy.
	res := exec(`mutation {
		linkBuildSource(serviceId: "` + web.ID + `", buildSourceId: "` + unbuilt.ID + `", startCommand: "node server.js", deploy: true) {
			buildSourceId latestDeployId
		}
	}`)
	if len(res.Errors) > 0 {
		t.Fatalf("unbuilt linkBuildSource errors: %v", res.Errors)
	}
	got := res.Data.(map[string]any)["linkBuildSource"].(map[string]any)
	if got["buildSourceId"] != unbuilt.ID || got["latestDeployId"] != nil || deployed != 0 {
		t.Fatalf("unbuilt link = %v deployed=%d, want buildSourceId=%s latestDeployId=nil deployed=0", got, deployed, unbuilt.ID)
	}

	// Dry run with deploy: true writes nothing and deploys nothing.
	res = exec(`mutation {
		linkBuildSource(serviceId: "` + web.ID + `", buildSourceId: "` + built.ID + `", dryRun: true, deploy: true) {
			buildSourceId latestDeployId
		}
	}`)
	if len(res.Errors) > 0 || deployed != 0 {
		t.Fatalf("dryRun link errors=%v deployed=%d", res.Errors, deployed)
	}

	// Built build source with deploy: true deploys once.
	res = exec(`mutation {
		linkBuildSource(serviceId: "` + web.ID + `", buildSourceId: "` + built.ID + `", deploy: true) {
			buildSourceId latestDeployId
		}
	}`)
	if len(res.Errors) > 0 {
		t.Fatalf("built linkBuildSource errors: %v", res.Errors)
	}
	got = res.Data.(map[string]any)["linkBuildSource"].(map[string]any)
	if got["buildSourceId"] != built.ID || got["latestDeployId"] != "dep-bds-1" || deployed != 1 {
		t.Fatalf("built link = %v deployed=%d, want buildSourceId=%s latestDeployId=dep-bds-1 deployed=1", got, deployed, built.ID)
	}
}
