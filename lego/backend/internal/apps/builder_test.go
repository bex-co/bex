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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestRenderNativeBuildContractAcrossCreateSurfaces(t *testing.T) {
	t.Run("rest", func(t *testing.T) {
		svc, cl := newService(nil)
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(`{
			"type":"web_service",
			"name":"web",
			"repo":"https://github.com/x/web",
			"serviceDetails":{
				"runtime":"node",
				"envSpecificDetails":{"buildCommand":"npm ci","startCommand":"npm start"}
			}
		}`)))
		if rec.Code != http.StatusCreated {
			t.Fatalf("create => %d: %s", rec.Code, rec.Body)
		}
		app := getApp(t, cl, "web")
		if app.Spec.Builder != "native" || app.Spec.Runtime != "node" || app.Spec.BuildCommand != "npm ci" || app.Spec.StartCommand != "npm start" {
			t.Fatalf("spec = %+v", app.Spec)
		}
		var out struct {
			Builder        string         `json:"builder"`
			ServiceDetails map[string]any `json:"serviceDetails"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Builder != "" || out.ServiceDetails["runtime"] != "node" {
			t.Fatalf("response = %s", rec.Body)
		}
	})

	t.Run("graphql", func(t *testing.T) {
		svc, cl := newService(nil)
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
		})
		if err != nil {
			t.Fatal(err)
		}
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation { createService(name:"web", repo:"https://github.com/x/web", runtime:"go", buildCommand:"go build -o app .", startCommand:"./app") { runtime buildCommand startCommand builder } }`})
		if len(res.Errors) > 0 {
			t.Fatal(res.Errors)
		}
		app := getApp(t, cl, "web")
		if app.Spec.Builder != "native" || app.Spec.Runtime != "go" {
			t.Fatalf("spec = %+v", app.Spec)
		}
	})

	t.Run("mcp", func(t *testing.T) {
		req := createWebServiceArgs{
			Name: "web", Repo: "https://github.com/x/web", Runtime: "python",
			BuildCommand: "pip install -r requirements.txt", StartCommand: "gunicorn app:app",
		}.toCreateRequest()
		if req.Runtime != "python" || req.BuildCommand == "" || req.StartCommand == "" {
			t.Fatalf("request = %+v", req)
		}
	})
}

func TestExplicitBuildpackRemainsBexExtension(t *testing.T) {
	svc, cl := newService(nil)
	if _, err := svc.Create(context.Background(), CreateRequest{Name: "web", Repo: "https://github.com/x/web", Builder: "buildpack"}); err != nil {
		t.Fatal(err)
	}
	if got := getApp(t, cl, "web").Spec.Builder; got != "buildpack" {
		t.Fatalf("spec.builder = %q", got)
	}
}

func TestRenderDockerDetailsMapToDockerfileBuild(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(`{
		"type":"web_service",
		"name":"web",
		"repo":"https://github.com/x/web",
		"serviceDetails":{
			"runtime":"docker",
			"envSpecificDetails":{
				"dockerCommand":"bin/server",
				"dockerContext":"services/web",
				"dockerfilePath":"docker/Dockerfile.prod"
			}
		}
	}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", rec.Code, rec.Body)
	}
	app := getApp(t, cl, "web")
	if app.Spec.Builder != "dockerfile" || app.Spec.Runtime != "docker" ||
		app.Spec.RootDir != "services/web" || app.Spec.DockerfilePath != "docker/Dockerfile.prod" ||
		app.Spec.StartCommand != "bin/server" {
		t.Fatalf("spec = %+v", app.Spec)
	}
}

func TestCreateRejectsUnknownBuilder(t *testing.T) {
	svc, _ := newService(nil)
	_, err := svc.Create(context.Background(), CreateRequest{Name: "web", Repo: "https://github.com/x/web", Builder: "magic"})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("error = %v, want bad request", err)
	}
}
