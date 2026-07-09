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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// --- Create (the Render-shaped create surface deploy rides) ---

func TestCreateWritesAppCR(t *testing.T) {
	svc, cl := newService(nil) // empty cluster

	v, err := svc.Create(context.Background(), CreateRequest{
		Name: "web", Image: "nginx:1", Port: 8080,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if v.Name != "web" {
		t.Errorf("view name = %q, want web", v.Name)
	}
	a := getApp(t, cl, "web")
	if a.Spec.Image != "nginx:1" || a.Spec.Port != 8080 {
		t.Errorf("spec = %+v, want image nginx:1 port 8080", a.Spec)
	}
	if !a.Spec.Expose {
		t.Error("a web service must be exposed at the platform hostname")
	}
	if a.Spec.Replicas != 1 || a.Spec.Tier != "free" {
		t.Errorf("defaults not applied: replicas=%d tier=%q", a.Spec.Replicas, a.Spec.Tier)
	}
}

func TestCreateRepoDefaultsBranchMain(t *testing.T) {
	svc, cl := newService(nil)
	if _, err := svc.Create(context.Background(), CreateRequest{Name: "app", Repo: "https://github.com/x/y"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if b := getApp(t, cl, "app").Spec.Branch; b != "main" {
		t.Errorf("branch = %q, want main", b)
	}
}

func TestCreateIsUpsertRedeploy(t *testing.T) {
	// An existing App + a repeat Create for the same name updates it (redeploy),
	// bumping restartedAt so a repo-backed App rebuilds — never a duplicate.
	existing := sampleApp("web")
	existing.Spec.Repo = "https://github.com/x/y"
	existing.Spec.Image = ""
	svc, cl := newService(nil, existing)

	if _, err := svc.Create(context.Background(), CreateRequest{
		Name: "web", Repo: "https://github.com/x/y", Plan: "standard",
	}); err != nil {
		t.Fatalf("Create (update): %v", err)
	}
	a := getApp(t, cl, "web")
	if a.Spec.RestartedAt == "" {
		t.Error("a repeat create must bump restartedAt to force a redeploy")
	}
	if a.Spec.Tier != "standard" {
		t.Errorf("update should apply the new plan: tier=%q", a.Spec.Tier)
	}
}

func TestCreateUpdatePreservesEnvFromSecret(t *testing.T) {
	// The secrets feature owns spec.envFromSecret; a create update must not wipe it.
	existing := sampleApp("web")
	existing.Spec.EnvFromSecret = "web-env"
	svc, cl := newService(nil, existing)

	if _, err := svc.Create(context.Background(), CreateRequest{Name: "web", Image: "nginx:2"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.EnvFromSecret; got != "web-env" {
		t.Errorf("envFromSecret = %q, want web-env (must survive an update)", got)
	}
}

func TestCreateValidation(t *testing.T) {
	svc, _ := newService(nil)
	cases := []struct {
		name string
		req  CreateRequest
	}{
		{"bad name", CreateRequest{Name: "Not A Label", Image: "x"}},
		{"no repo or image", CreateRequest{Name: "web"}},
		{"unknown plan", CreateRequest{Name: "web", Image: "x", Plan: "gold"}},
		{"port too high", CreateRequest{Name: "web", Image: "x", Port: 70000}},
		{"replicas too high", CreateRequest{Name: "web", Image: "x", Replicas: 1000}},
	}
	for _, c := range cases {
		if _, err := svc.Create(context.Background(), c.req); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("%s: want ErrBadRequest, got %v", c.name, err)
		}
	}
}

func TestCreateAutoDeployDefaults(t *testing.T) {
	svc, cl := newService(nil)
	// repo-backed => autoDeploy on by default (push-to-deploy works out of the box).
	if _, err := svc.Create(context.Background(), CreateRequest{Name: "repoapp", Repo: "https://github.com/x/y"}); err != nil {
		t.Fatalf("Create repo: %v", err)
	}
	if !getApp(t, cl, "repoapp").Spec.AutoDeploy {
		t.Error("a repo-backed create should default autoDeploy on")
	}
	// image-backed => off (nothing to rebuild on push).
	if _, err := svc.Create(context.Background(), CreateRequest{Name: "imgapp", Image: "nginx:1"}); err != nil {
		t.Fatalf("Create image: %v", err)
	}
	if getApp(t, cl, "imgapp").Spec.AutoDeploy {
		t.Error("an image-backed create should default autoDeploy off")
	}
	// explicit opt-out wins.
	off := false
	if _, err := svc.Create(context.Background(), CreateRequest{Name: "manualapp", Repo: "https://github.com/x/y", AutoDeploy: &off}); err != nil {
		t.Fatalf("Create manual: %v", err)
	}
	if getApp(t, cl, "manualapp").Spec.AutoDeploy {
		t.Error("explicit autoDeploy:false must win over the repo default")
	}
}

func TestWebhookSkipsAutoDeployOff(t *testing.T) {
	const secret = "s3cr3t"
	off := repoApp("off", "https://github.com/bex/hello", "main")
	off.Spec.AutoDeploy = false // opted out of push-to-deploy
	svc, cl := newService(nil, off)
	h := &GitWebhook{Svc: svc, Secret: secret}

	body := pushBody(t, "https://github.com/bex/hello", "refs/heads/main")
	req := httptest.NewRequest("POST", "/v1/webhooks/git", strings.NewReader(string(body)))
	req.Header.Set("X-Hub-Signature-256", sign(secret, body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("=> 200, got %d", rec.Code)
	}
	if getApp(t, cl, "off").Spec.RestartedAt != "" {
		t.Error("an autoDeploy:false App must not redeploy on push")
	}
}

func TestCreateAcceptsRenderPlanSpelling(t *testing.T) {
	svc, cl := newService(nil)
	if _, err := svc.Create(context.Background(), CreateRequest{Name: "web", Image: "x", Plan: "pro_plus"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Tier; got != "pro-plus" {
		t.Errorf("tier = %q, want pro-plus (Render spelling normalized)", got)
	}
}

// --- Deploy (bex.yml -> Create) ---

const sampleManifest = `
apps:
  - name: hello
    repo: https://github.com/bex/hello
    branch: main
    port: 3000
    tier: starter
    healthCheckPath: /healthz
    envVars:
      - key: FOO
        value: bar
    domains:
      - hello.example.com
`

func TestDeployMapsManifest(t *testing.T) {
	svc, cl := newService(nil)
	v, err := svc.Deploy(context.Background(), DeployRequest{Manifest: sampleManifest})
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if v.Name != "hello" {
		t.Fatalf("name = %q, want hello", v.Name)
	}
	a := getApp(t, cl, "hello")
	if a.Spec.Repo != "https://github.com/bex/hello" || a.Spec.Tier != "starter" {
		t.Errorf("spec = %+v", a.Spec)
	}
	if a.Spec.HealthCheckPath != "/healthz" {
		t.Errorf("healthCheckPath = %q", a.Spec.HealthCheckPath)
	}
	if a.Spec.Host != "hello.example.com" {
		t.Errorf("host = %q, want hello.example.com", a.Spec.Host)
	}
	if len(a.Spec.Env) != 1 || a.Spec.Env[0].Name != "FOO" || a.Spec.Env[0].Value != "bar" {
		t.Errorf("env = %+v, want [FOO=bar]", a.Spec.Env)
	}
}

func TestDeployRepoOverrideWins(t *testing.T) {
	svc, cl := newService(nil)
	if _, err := svc.Deploy(context.Background(), DeployRequest{
		Repo: "https://github.com/other/repo", Manifest: sampleManifest,
	}); err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if r := getApp(t, cl, "hello").Spec.Repo; r != "https://github.com/other/repo" {
		t.Errorf("repo = %q, want the override", r)
	}
}

func TestDeployRejectsEmptyAndPrivateWithDomains(t *testing.T) {
	svc, _ := newService(nil)
	if _, err := svc.Deploy(context.Background(), DeployRequest{Manifest: "apps: []"}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("empty manifest => ErrBadRequest, got %v", err)
	}
	priv := "apps:\n  - name: p\n    image: x\n    type: private\n    domains: [a.example.com]\n"
	if _, err := svc.Deploy(context.Background(), DeployRequest{Manifest: priv}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("private+domains => ErrBadRequest, got %v", err)
	}
}

// --- REST create fragment ---

func TestRESTCreateService(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// Render's create shape: image is an object {imagePath}; plan and numInstances
	// nest under serviceDetails; envVars is [{key,value}].
	body := `{"name":"web","image":{"imagePath":"nginx:1"},"serviceDetails":{"plan":"standard","numInstances":3},"envVars":[{"key":"K","value":"V"}]}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => 201, got %d: %s", rec.Code, rec.Body)
	}
	a := getApp(t, cl, "web")
	if a.Spec.Image != "nginx:1" || a.Spec.Tier != "standard" {
		t.Errorf("spec = %+v", a.Spec)
	}
	if a.Spec.Replicas != 3 {
		t.Errorf("serviceDetails.numInstances not honored: replicas = %d, want 3", a.Spec.Replicas)
	}
	if len(a.Spec.Env) != 1 || a.Spec.Env[0].Name != "K" {
		t.Errorf("env = %+v, want [K=V]", a.Spec.Env)
	}
}

// TestRESTCreatePrivateService maps Render's type:private_service to in-cluster
// only (no platform-hostname expose).
func TestRESTCreatePrivateService(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"type":"private_service","name":"worker","image":{"imagePath":"nginx:1"}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => 201, got %d: %s", rec.Code, rec.Body)
	}
	if getApp(t, cl, "worker").Spec.Expose {
		t.Error("a private_service must not be exposed at the platform hostname")
	}
}

// --- HMAC git webhook ---

func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func pushBody(t *testing.T, repo, ref string) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"ref":        ref,
		"repository": map[string]string{"clone_url": repo},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func repoApp(name, repo, branch string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Spec.Image = ""
	a.Spec.Repo = repo
	a.Spec.Branch = branch
	a.Spec.AutoDeploy = true // push-to-deploy enabled (the webhook gates on this)
	return a
}

func TestWebhookValidSignatureRedeploys(t *testing.T) {
	const secret = "s3cr3t"
	svc, cl := newService(nil, repoApp("web", "https://github.com/bex/hello.git", "main"))
	h := &GitWebhook{Svc: svc, Secret: secret}

	body := pushBody(t, "https://github.com/bex/hello", "refs/heads/main")
	req := httptest.NewRequest("POST", "/v1/webhooks/git", strings.NewReader(string(body)))
	req.Header.Set("X-Hub-Signature-256", sign(secret, body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("valid push => 200, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.RestartedAt; got == "" {
		t.Error("a valid push must redeploy (bump restartedAt)")
	}
	var out struct {
		Redeployed []string `json:"redeployed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Redeployed) != 1 || out.Redeployed[0] != "web" {
		t.Errorf("response = %s, want redeployed [web]", rec.Body)
	}
}

func TestWebhookInvalidSignatureRejected(t *testing.T) {
	const secret = "s3cr3t"
	svc, cl := newService(nil, repoApp("web", "https://github.com/bex/hello", "main"))
	h := &GitWebhook{Svc: svc, Secret: secret}
	body := pushBody(t, "https://github.com/bex/hello", "refs/heads/main")

	for _, tc := range []struct{ name, sig string }{
		{"absent", ""},
		{"wrong secret", sign("nope", body)},
		{"garbage", "sha256=zzzz"},
	} {
		req := httptest.NewRequest("POST", "/v1/webhooks/git", strings.NewReader(string(body)))
		if tc.sig != "" {
			req.Header.Set("X-Hub-Signature-256", tc.sig)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s => 401, got %d", tc.name, rec.Code)
		}
	}
	if getApp(t, cl, "web").Spec.RestartedAt != "" {
		t.Error("a rejected push must not redeploy")
	}
}

func TestWebhookNoSecretUnavailable(t *testing.T) {
	svc, _ := newService(nil, repoApp("web", "https://github.com/bex/hello", "main"))
	h := &GitWebhook{Svc: svc, Secret: ""}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/webhooks/git", strings.NewReader("{}")))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("no secret => 503, got %d", rec.Code)
	}
}

func TestWebhookSkipsNonMatchingRepoOrBranch(t *testing.T) {
	const secret = "s3cr3t"
	svc, cl := newService(nil,
		repoApp("web", "https://github.com/bex/hello", "main"),
		repoApp("other", "https://github.com/bex/other", "main"),
		repoApp("dev", "https://github.com/bex/hello", "develop"),
	)
	h := &GitWebhook{Svc: svc, Secret: secret}
	body := pushBody(t, "https://github.com/bex/hello", "refs/heads/main")
	req := httptest.NewRequest("POST", "/v1/webhooks/git", strings.NewReader(string(body)))
	req.Header.Set("X-Hub-Signature-256", sign(secret, body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("=> 200, got %d: %s", rec.Code, rec.Body)
	}
	// only web matches (same repo + branch main); other/dev untouched.
	if getApp(t, cl, "web").Spec.RestartedAt == "" {
		t.Error("web should have redeployed")
	}
	if getApp(t, cl, "other").Spec.RestartedAt != "" {
		t.Error("a different repo must not redeploy")
	}
	if getApp(t, cl, "dev").Spec.RestartedAt != "" {
		t.Error("a different branch must not redeploy")
	}
}

func TestCanonicalRepoMatchesURLForms(t *testing.T) {
	ev := pushEvent{}
	ev.Repository.CloneURL = "https://github.com/bex/hello.git"
	ev.Repository.SSHURL = "git@github.com:bex/hello.git"
	for _, spec := range []string{
		"https://github.com/bex/hello",
		"https://github.com/bex/hello.git",
		"git@github.com:bex/hello.git",
		"ssh://git@github.com/bex/hello",
	} {
		if !repoMatches(spec, ev) {
			t.Errorf("spec.repo %q should match the pushed repo", spec)
		}
	}
	if repoMatches("https://github.com/bex/nope", ev) {
		t.Error("a different repo must not match")
	}
}
