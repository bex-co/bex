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
	"fmt"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// addressRefTargets are one declaration per service type: web/private have a
// network address, worker/cron/static do not.
var addressRefTargets = map[string]string{
	"web":    `{name: target, type: web, runtime: image, image: {url: "t:1"}}`,
	"pserv":  `{name: target, type: pserv, runtime: image, image: {url: "t:1"}}`,
	"worker": `{name: target, type: worker, runtime: image, image: {url: "t:1"}}`,
	"cron":   `{name: target, type: cron, runtime: image, image: {url: "t:1"}, schedule: "0 0 1 1 *"}`,
	"static": `{name: target, type: web, runtime: static, repo: "https://github.com/o/r", staticPublishPath: "."}`,
}

// addressRefManifest declares the caller before its target, so every case is
// a forward reference.
func addressRefManifest(targetType, refType string, ref string) string {
	return fmt.Sprintf(`
services:
  - name: caller
    type: web
    runtime: image
    image: {url: "c:1"}
    envVars:
      - {key: UPSTREAM, fromService: {name: target, type: %s, %s}}
  - %s
`, refType, ref, addressRefTargets[targetType])
}

// TestBlueprintAddressRefsRequireAnAddressableTarget: host, port and hostport
// all name a sibling's network address, which only web/private services have.
// port used to resolve to "0" and hostport to "<slug>:0" for a worker, cron
// job or static site (w8/081). A static site's host is accepted (w8/303).
func TestBlueprintAddressRefsRequireAnAddressableTarget(t *testing.T) {
	svc, _ := newService(nil)
	refTypes := map[string]string{"worker": "worker", "cron": "cron", "static": "web"}
	for _, target := range []string{"worker", "cron", "static"} {
		refType := refTypes[target]
		for _, property := range []string{"host", "port", "hostport"} {
			if target == "static" && property == "host" {
				continue // the site's public hostname (w8/303)
			}
			manifest := addressRefManifest(target, refType, "property: "+property)
			t.Run(target+"/"+property, func(t *testing.T) {
				_, err := parseStack(DeployRequest{Manifest: manifest})
				if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), `envVars["UPSTREAM"]`) || !strings.Contains(err.Error(), "no network address") {
					t.Fatalf("parse = %v, want the addressability refusal naming envVars[\"UPSTREAM\"]", err)
				}
				v, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
				if err != nil || v.Valid || len(v.Errors) != 1 {
					t.Fatalf("validate = %+v, err %v; want one refusal", v, err)
				}
				if e := v.Errors[0]; e.Path == nil || *e.Path != "services[0].envVars[0]" || e.Line == nil || *e.Line != 8 {
					t.Fatalf("refusal located at path %v line %v, want the caller's envVars[0] on line 8", e.Path, e.Line)
				}
			})
		}
		// Copying a plain env var from the same target stays supported.
		t.Run(target+"/envVarKey", func(t *testing.T) {
			manifest := strings.Replace(addressRefManifest(target, refType, "envVarKey: MODE"), addressRefTargets[target],
				strings.Replace(addressRefTargets[target], "{name: target,", "{name: target, envVars: [{key: MODE, value: x}],", 1), 1)
			if _, err := parseStack(DeployRequest{Manifest: manifest}); err != nil {
				t.Fatalf("envVarKey copy from %s: %v", target, err)
			}
		})
	}
	for _, target := range []string{"web", "pserv"} {
		for _, property := range []string{"host", "port", "hostport"} {
			t.Run(target+"/"+property, func(t *testing.T) {
				if _, err := parseStack(DeployRequest{Manifest: addressRefManifest(target, target, "property: "+property)}); err != nil {
					t.Fatalf("%s %s: %v", target, property, err)
				}
			})
		}
	}
}

// w8/303: Render's own movie-context-provider template takes its static
// widget site's host and builds the site's public URL from it. The reference
// resolves to the site's public hostname, whether the file declares the site
// or it already exists in the workspace.
func TestBlueprintStaticSiteHostIsItsPublicHostname(t *testing.T) {
	ctx := context.Background()
	t.Run("declared", func(t *testing.T) {
		svc, cl := newBaseDomainService("onbex.test", "")
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: addressRefManifest("static", "web", "property: host")}); err != nil {
			t.Fatalf("DeployStack: %v", err)
		}
		if got := findEnv(t, getApp(t, cl, "caller").Spec.Env, "UPSTREAM").Value; got != "target.onbex.test" {
			t.Errorf("UPSTREAM = %q, want the static site's public hostname target.onbex.test", got)
		}
	})
	manifest := `
services:
  - name: caller
    type: web
    runtime: image
    image: {url: "c:1"}
    envVars:
      - {key: UPSTREAM, fromService: {name: widget, type: web, property: host}}
`
	existing := func(policy string) *appv1alpha1.App {
		return &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "widget", Namespace: "default"},
			Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeStaticSite, Expose: true, Repo: "https://github.com/o/r",
				Subdomain: "widget-x7k2", SubdomainPolicy: policy},
		}
	}
	t.Run("existing", func(t *testing.T) {
		svc, cl := newBaseDomainService("onbex.test", "", existing(""))
		if v, err := svc.ValidateBlueprint(ctx, "", manifest, ""); err != nil || !v.Valid {
			t.Fatalf("validate = %+v, %v; want valid", v, err)
		}
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
			t.Fatalf("DeployStack: %v", err)
		}
		if got := findEnv(t, getApp(t, cl, "caller").Spec.Env, "UPSTREAM").Value; got != "widget-x7k2.onbex.test" {
			t.Errorf("UPSTREAM = %q, want the existing site's public hostname widget-x7k2.onbex.test", got)
		}
	})
	// A site with its platform subdomain disabled has no public hostname of
	// its own: the variable is left out, and the service is still written.
	t.Run("no platform host", func(t *testing.T) {
		svc, cl := newBaseDomainService("onbex.test", "", existing(appv1alpha1.SubdomainPolicyDisabled))
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
			t.Fatalf("DeployStack: %v", err)
		}
		if env := getApp(t, cl, "caller").Spec.Env; hasEnv(env, "UPSTREAM") {
			t.Errorf("caller env = %+v, want UPSTREAM left out", env)
		}
	})
}

// The refusal lands in parse, so a direct deploy writes nothing.
func TestDeployStackRefusesUnaddressablePortRefBeforeWriting(t *testing.T) {
	svc, cl := newService(nil)
	_, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: addressRefManifest("worker", "worker", "property: port")})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("DeployStack = %v, want the addressability refusal", err)
	}
	var apps appv1alpha1.AppList
	if err := cl.List(context.Background(), &apps); err != nil || len(apps.Items) != 0 {
		t.Fatalf("apps after refusal = %d (err %v), want none", len(apps.Items), err)
	}
}
