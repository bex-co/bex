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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// serviceRefsManifest: a web consumer with host, hostport and port references
// to a private service. The pserv's kind decides how its port is known: a
// configured image service has its effective port at once, a Git-built Docker
// one only after its first build (w6/m148).
func serviceRefsManifest(pserv string) string {
	return `
services:
  - name: web
    type: web
    runtime: image
    image: {url: 'web:1'}
    envVars:
      - key: API_HOST
        fromService: {name: api, type: pserv, property: host}
      - key: API_HOSTPORT
        fromService: {name: api, type: pserv, property: hostport}
      - key: API_PORT
        fromService: {name: api, type: pserv, property: port}
  - name: api
    type: pserv
` + pserv
}

func envValue(app *appv1alpha1.App, key string) (string, bool) {
	for _, e := range app.Spec.Env {
		if e.Name == key {
			return e.Value, true
		}
	}
	return "", false
}

// The live bug w6/m148 fixes: a port edited after creation (w4/m121) was
// ignored by the next Blueprint sync, whose consumer kept <slug>:3000.
func TestBlueprintPortReferencesFollowAnEditedTargetPort(t *testing.T) {
	ctx := context.Background()
	service, client := newService(nil)
	manifest := serviceRefsManifest("    runtime: image\n    image: {url: 'api:1'}\n")
	if _, err := service.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
		t.Fatal(err)
	}
	api := getApp(t, client, "api")
	slug := api.Spec.PlatformSubdomain(api.Name)
	web := getApp(t, client, "web")
	for key, want := range map[string]string{"API_HOST": slug, "API_HOSTPORT": slug + ":10000", "API_PORT": "10000"} {
		if got, _ := envValue(web, key); got != want {
			t.Fatalf("first apply %s = %q, want %q", key, got, want)
		}
	}
	if _, err := service.SetPort(ctx, "api", 8080); err != nil {
		t.Fatal(err)
	}
	if _, err := service.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
		t.Fatal(err)
	}
	web = getApp(t, client, "web")
	for key, want := range map[string]string{"API_HOST": slug, "API_HOSTPORT": slug + ":8080", "API_PORT": "8080"} {
		if got, _ := envValue(web, key); got != want {
			t.Fatalf("after SetPort + resync %s = %q, want %q", key, got, want)
		}
	}
	refs, err := serviceRefsOf(web)
	want := []serviceRefRecord{{Key: "API_HOSTPORT", Target: api.Name, Property: serviceRefPropertyHostPort}, {Key: "API_PORT", Target: api.Name, Property: serviceRefPropertyPort}}
	if err != nil || !reflect.DeepEqual(refs, want) {
		t.Fatalf("recorded refs = %+v (%v), want %+v", refs, err, want)
	}
}

// ADR089 port decision 5: a service's own port never depends on its
// consumers' environment, so two services that reference each other's
// hostport need no cycle rule. Both resolve on first apply and each follows
// the other's port change.
func TestMutualPortReferencesResolveWithoutACycleRule(t *testing.T) {
	ctx := context.Background()
	service, client := newService(nil)
	manifest := `
services:
  - name: left
    type: pserv
    runtime: image
    image: {url: 'left:1'}
    envVars:
      - key: PEER
        fromService: {name: right, type: pserv, property: hostport}
  - name: right
    type: pserv
    runtime: image
    image: {url: 'right:1'}
    envVars:
      - key: PEER
        fromService: {name: left, type: pserv, property: hostport}
`
	if _, err := service.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("mutual references refused: %v", err)
	}
	peers := func() (string, string) {
		left, _ := envValue(getApp(t, client, "left"), "PEER")
		right, _ := envValue(getApp(t, client, "right"), "PEER")
		return left, right
	}
	leftApp, rightApp := getApp(t, client, "left"), getApp(t, client, "right")
	leftSlug, rightSlug := leftApp.Spec.PlatformSubdomain(leftApp.Name), rightApp.Spec.PlatformSubdomain(rightApp.Name)
	if left, right := peers(); left != rightSlug+":10000" || right != leftSlug+":10000" {
		t.Fatalf("first apply: left.PEER=%q right.PEER=%q", left, right)
	}
	if _, err := service.SetPort(ctx, "right", 8080); err != nil {
		t.Fatal(err)
	}
	if _, err := service.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
		t.Fatal(err)
	}
	if left, right := peers(); left != rightSlug+":8080" || right != leftSlug+":10000" {
		t.Fatalf("after right's SetPort: left.PEER=%q right.PEER=%q", left, right)
	}
}

// A Git-built Docker private service learns its port from its image after the
// first build, so a first apply has nothing true to write: the port-carrying
// variables are omitted — never 3000 — and recorded for the propagator. A
// second, unchanged sync writes nothing at all.
func TestBlueprintPortReferencesToAnImagePortTargetNeverGuess(t *testing.T) {
	ctx := context.Background()
	service, client := newService(nil)
	manifest := serviceRefsManifest("    runtime: docker\n    repo: https://github.com/example/api\n")
	if _, err := service.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
		t.Fatal(err)
	}
	api := getApp(t, client, "api")
	if api.Spec.PortMode != appv1alpha1.PortModeImageV1 || api.ServicePort() != 0 {
		t.Fatalf("target is not a pending image-port service: mode=%q port=%d", api.Spec.PortMode, api.ServicePort())
	}
	web := getApp(t, client, "web")
	if got, _ := envValue(web, "API_HOST"); got != api.Spec.PlatformSubdomain(api.Name) {
		t.Fatalf("API_HOST = %q, want the slug", got)
	}
	for _, key := range []string{"API_HOSTPORT", "API_PORT"} {
		if got, ok := envValue(web, key); ok {
			t.Fatalf("%s = %q was written before the target's port was known", key, got)
		}
	}
	for _, e := range web.Spec.Env {
		if strings.Contains(e.Value, "3000") {
			t.Fatalf("a guessed 3000 reached %s", e.Name)
		}
	}
	if raw := web.Annotations[serviceRefsAnnotation]; !strings.Contains(raw, `"key":"API_HOSTPORT"`) || !strings.Contains(raw, `"key":"API_PORT"`) || strings.Contains(raw, "API_HOST\"") {
		t.Fatalf("recorded refs = %s, want the two port-carrying references only", raw)
	}
	before := web.ResourceVersion
	if _, err := service.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
		t.Fatal(err)
	}
	if after := getApp(t, client, "web"); after.ResourceVersion != before {
		t.Fatalf("an unchanged sync rewrote the consumer: %s -> %s", before, after.ResourceVersion)
	}
}

// Dropping the references from the Blueprint clears the record, so the
// propagator stops rewriting variables the file no longer owns.
func TestBlueprintServiceRefsRecordFollowsTheFile(t *testing.T) {
	ctx := context.Background()
	service, client := newService(nil)
	if _, err := service.DeployStack(ctx, DeployRequest{Manifest: serviceRefsManifest("    runtime: image\n    image: {url: 'api:1'}\n")}); err != nil {
		t.Fatal(err)
	}
	if getApp(t, client, "web").Annotations[serviceRefsAnnotation] == "" {
		t.Fatal("references were not recorded")
	}
	without := `
services:
  - name: web
    type: web
    runtime: image
    image: {url: 'web:1'}
  - name: api
    type: pserv
    runtime: image
    image: {url: 'api:1'}
`
	if _, err := service.DeployStack(ctx, DeployRequest{Manifest: without}); err != nil {
		t.Fatal(err)
	}
	if raw, ok := getApp(t, client, "web").Annotations[serviceRefsAnnotation]; ok {
		t.Fatalf("record kept after the file dropped its references: %s", raw)
	}
}

// propagatorFixture is a store-managed consumer recording hostport and port
// references to target, plus the propagator over a recording store.
func propagatorFixture(t *testing.T, target *appv1alpha1.App, env ...appv1alpha1.EnvVar) (*ServiceRefPropagator, client.Client, *recordingStore) {
	t.Helper()
	consumer := sampleApp("web")
	consumer.Labels = map[string]string{store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: "srv-web", serviceRefsLabel: "true"}
	consumer.Annotations = map[string]string{serviceRefsAnnotation: encodeServiceRefs([]serviceRefRecord{
		{Key: "API_HOSTPORT", Target: target.Name, Property: serviceRefPropertyHostPort},
		{Key: "API_PORT", Target: target.Name, Property: serviceRefPropertyPort},
	})}
	consumer.Spec.Env = env
	st := &recordingStore{}
	svc, cl := newService(st, consumer, target)
	return &ServiceRefPropagator{Svc: svc}, cl, st
}

func imageTarget(name string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Spec.Type, a.Spec.PortMode, a.Spec.Repo, a.Spec.Image = appv1alpha1.TypePrivateService, appv1alpha1.PortModeImageV1, "https://github.com/example/api", ""
	return a
}

// A pending image-port target writes nothing; its first discovered network
// fills both variables in one patch that opens one config_change deploy.
func TestServiceRefPropagatorWaitsForDiscoveryThenWritesOnce(t *testing.T) {
	ctx := context.Background()
	target := imageTarget("api")
	p, cl, st := propagatorFixture(t, target)
	if err := p.tick(ctx); err != nil {
		t.Fatal(err)
	}
	if web := getApp(t, cl, "web"); len(web.Spec.Env) != 0 || len(st.deployCalls) != 0 {
		t.Fatalf("pending target wrote %+v / %d deploys", web.Spec.Env, len(st.deployCalls))
	}
	discovered := getApp(t, cl, "api")
	discovered.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "i", Revision: "r1", PrimaryPort: 8123, Ports: []int32{8123, 9000}}
	if err := cl.Update(ctx, discovered); err != nil {
		t.Fatal(err)
	}
	for range 2 { // a second tick finds nothing to do
		if err := p.tick(ctx); err != nil {
			t.Fatal(err)
		}
	}
	web := getApp(t, cl, "web")
	if hp, _ := envValue(web, "API_HOSTPORT"); hp != "api:8123" {
		t.Fatalf("API_HOSTPORT = %q, want api:8123", hp)
	}
	if port, _ := envValue(web, "API_PORT"); port != "8123" {
		t.Fatalf("API_PORT = %q, want 8123", port)
	}
	if len(st.deployCalls) != 1 || st.deployCalls[0].Trigger != store.TriggerConfigChange {
		t.Fatalf("deploys = %+v, want exactly one config_change", st.deployCalls)
	}
}

// A configured target's port edit reaches the consumer without a Blueprint
// sync, as one config_change deploy; other variables keep their order.
func TestServiceRefPropagatorFollowsSetPort(t *testing.T) {
	ctx := context.Background()
	target := sampleApp("api")
	target.Spec.Type = appv1alpha1.TypePrivateService
	p, cl, st := propagatorFixture(t, target,
		appv1alpha1.EnvVar{Name: "API_HOSTPORT", Value: "api:10000"},
		appv1alpha1.EnvVar{Name: "APP_ENV", Value: "prod"},
		appv1alpha1.EnvVar{Name: "API_PORT", Value: "10000"})
	if err := p.tick(ctx); err != nil || len(st.deployCalls) != 0 {
		t.Fatalf("an unchanged target patched the consumer: %v / %d deploys", err, len(st.deployCalls))
	}
	edited := getApp(t, cl, "api")
	edited.Spec.Port = 8080
	if err := cl.Update(ctx, edited); err != nil {
		t.Fatal(err)
	}
	if err := p.tick(ctx); err != nil {
		t.Fatal(err)
	}
	want := []appv1alpha1.EnvVar{{Name: "API_HOSTPORT", Value: "api:8080"}, {Name: "APP_ENV", Value: "prod"}, {Name: "API_PORT", Value: "8080"}}
	if got := getApp(t, cl, "web").Spec.Env; !reflect.DeepEqual(got, want) {
		t.Fatalf("env = %+v, want %+v", got, want)
	}
	if len(st.deployCalls) != 1 {
		t.Fatalf("deploys = %d, want 1", len(st.deployCalls))
	}
}

// During a port-set replacement the consumer keeps the serving primary; only
// promotion (ServingNetwork moving) changes it (ADR089 D4).
func TestServiceRefPropagatorIgnoresAnUnpromotedCandidate(t *testing.T) {
	ctx := context.Background()
	target := imageTarget("api")
	target.Status.ServingNetwork = &appv1alpha1.ImageNetworkStatus{Image: "a", Revision: "r1", PrimaryPort: 8123, Ports: []int32{8123}}
	target.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "b", Revision: "r2", PrimaryPort: 7000, Ports: []int32{7000}}
	p, cl, st := propagatorFixture(t, target,
		appv1alpha1.EnvVar{Name: "API_HOSTPORT", Value: "api:8123"}, appv1alpha1.EnvVar{Name: "API_PORT", Value: "8123"})
	if err := p.tick(ctx); err != nil || len(st.deployCalls) != 0 {
		t.Fatalf("a candidate's port leaked before promotion: %v / %d deploys", err, len(st.deployCalls))
	}
	promoted := getApp(t, cl, "api")
	promoted.Status.ServingNetwork = promoted.Status.ImageNetwork.DeepCopy()
	if err := cl.Update(ctx, promoted); err != nil {
		t.Fatal(err)
	}
	if err := p.tick(ctx); err != nil {
		t.Fatal(err)
	}
	if hp, _ := envValue(getApp(t, cl, "web"), "API_HOSTPORT"); hp != "api:7000" {
		t.Fatalf("after promotion API_HOSTPORT = %q, want api:7000", hp)
	}
}

// Two replicas racing on the same change open at most one deploy: the loser's
// optimistic lock fails, it re-reads, and finds nothing left to write.
func TestServiceRefPropagatorRaceOpensOneDeploy(t *testing.T) {
	ctx := context.Background()
	target := sampleApp("api")
	target.Spec.Type, target.Spec.Port = appv1alpha1.TypePrivateService, 8080
	p, cl, st := propagatorFixture(t, target)
	stale := getApp(t, cl, "web") // both replicas read this version
	lookup := func(string) *appv1alpha1.App { return getApp(t, cl, "api") }
	if wrote, err := p.Svc.propagateServiceRefs(ctx, getApp(t, cl, "web"), lookup); err != nil || !wrote {
		t.Fatalf("first replica: wrote=%t err=%v", wrote, err)
	}
	if wrote, err := p.Svc.propagateServiceRefs(ctx, stale, lookup); err != nil || wrote {
		t.Fatalf("second replica on a stale read: wrote=%t err=%v, want a no-op", wrote, err)
	}
	if len(st.deployCalls) != 1 {
		t.Fatalf("deploys = %d, want 1", len(st.deployCalls))
	}
}

// A vanished target keeps the consumer's last value; nothing is guessed.
func TestServiceRefPropagatorKeepsTheLastValueForAMissingTarget(t *testing.T) {
	ctx := context.Background()
	target := sampleApp("api")
	target.Spec.Type = appv1alpha1.TypePrivateService
	p, cl, st := propagatorFixture(t, target, appv1alpha1.EnvVar{Name: "API_HOSTPORT", Value: "api:9999"})
	if err := cl.Delete(ctx, getApp(t, cl, "api")); err != nil {
		t.Fatal(err)
	}
	if err := p.tick(ctx); err != nil {
		t.Fatal(err)
	}
	if hp, _ := envValue(getApp(t, cl, "web"), "API_HOSTPORT"); hp != "api:9999" || len(st.deployCalls) != 0 {
		t.Fatalf("missing target rewrote the consumer: %q / %d deploys", hp, len(st.deployCalls))
	}
}
