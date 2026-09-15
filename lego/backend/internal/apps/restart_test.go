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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w1/m148: REST POST .../restart and MCP restart_service restart on the
// running commit. They reach apps.Service.Restart, which used to clear
// spec.buildCommit "so the restart uses Branch HEAD".

func TestRestartKeepsTheBuildCommitPin(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Repo = "https://github.com/acme/web"
	app.Spec.BuildCommit = "f3284af44e2f00fbfe2b2f10f04ae5243e1e2bdf"
	cl := fakeClient(app)
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}

	if _, err := svc.Restart(context.Background(), "web"); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	got := getApp(t, cl, "web")
	if got.Spec.BuildCommit != app.Spec.BuildCommit {
		t.Errorf("spec.buildCommit = %q, want the pin %q kept", got.Spec.BuildCommit, app.Spec.BuildCommit)
	}
	if got.Spec.RestartedAt == "" {
		t.Error("restart must stamp spec.restartedAt")
	}
}

// With the deploys verb wired, REST and MCP run it — the one restart
// GraphQL restartServer runs — and never patch the App themselves.
func TestRestartRunsTheWiredDeployVerb(t *testing.T) {
	cl := fakeClient(sampleApp("web"))
	var restarted []string
	svc := &Service{
		Base: &core.Base{Client: cl, Namespace: "default"},
		RestartDeploy: func(_ context.Context, name string) error {
			restarted = append(restarted, name)
			return nil
		},
	}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services/web/restart", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST restart = %d %s, want 200", rec.Code, rec.Body)
	}
	if len(restarted) != 1 || restarted[0] != "web" {
		t.Errorf("deploy verb calls = %v, want [web]", restarted)
	}
	if got := getApp(t, cl, "web").Spec.RestartedAt; got != "" {
		t.Errorf("apps patched spec.restartedAt = %q itself; the deploy verb owns the restart", got)
	}

	refused := errors.New("no live deploy")
	svc.RestartDeploy = func(context.Context, string) error { return refused }
	if _, err := svc.Restart(context.Background(), "web"); !errors.Is(err, refused) {
		t.Errorf("Restart error = %v, want the deploy verb's refusal", err)
	}
}
