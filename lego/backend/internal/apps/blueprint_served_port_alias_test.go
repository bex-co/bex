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
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/302: every sync plans and then applies one parsed stack. The plan's
// target spec shares its env array with that stack, and dropServedPortEnv
// deleted a PORT restating the served port in place, so the apply read the
// shifted array with a zeroed tail and wrote a variable with no name. The
// operator copies spec.Env into the container, and Kubernetes refuses a
// Deployment whose env has an empty name.

func TestReSyncOfAServedPortWritesNoNamelessVariable(t *testing.T) {
	const web = `
services:
  - name: web
    type: web
    runtime: docker
    repo: https://github.com/bex/stack
    envVars:
      - {key: PORT, value: "10000"}
      - {key: APP_ENV, value: prod}`
	cases := map[string]struct{ first, second string }{
		"the same file again": {first: web, second: web},
		// The apply's own drop ran on the request a forward host reference
		// then re-applied in its second pass.
		"a file adding a host reference to a service the sync creates": {
			first:  web,
			second: web + "\n      - {key: API_HOST, fromService: {name: api, type: web, property: host}}\n  - {name: api, type: web, runtime: image, image: {url: nginx:1.27}}",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, cl := newService(nil)
			ctx := context.Background()
			for _, manifest := range []string{tc.first, tc.second} {
				if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
					t.Fatalf("DeployStack: %v", err)
				}
			}
			env := getApp(t, cl, "web").Spec.Env
			if hasEnv(env, "") {
				t.Fatalf("web env = %+v, want no variable without a name", env)
			}
			if got := findEnv(t, env, "PORT").Value; got != "10000" {
				t.Errorf("PORT = %q, want the declared 10000 kept", got)
			}
		})
	}
}

// A service an earlier sync left with a nameless variable: the next sync plans
// its removal as an envVars change and drops it.
func TestSyncDropsANamelessVariableAnEarlierSyncWrote(t *testing.T) {
	const manifest = `
services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1.27}
    envVars:
      - {key: APP_ENV, value: prod}`
	svc, cl := newService(nil)
	ctx := context.Background()
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	damaged := getApp(t, cl, "web")
	damaged.Spec.Env = append(damaged.Spec.Env, appv1alpha1.EnvVar{})
	if err := cl.Update(ctx, damaged); err != nil {
		t.Fatal(err)
	}
	web := planAction(t, planActionsForTest(ctx, t, svc, manifest), "web")
	if web.Operation != BlueprintPlanUpdate || len(web.ChangedFields) != 1 || web.ChangedFields[0].Path != "envVars" {
		t.Fatalf("web plan = %+v, want the nameless variable's removal as an envVars update", web)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if env := getApp(t, cl, "web").Spec.Env; len(env) != 1 || env[0].Name != "APP_ENV" {
		t.Fatalf("web env = %+v, want APP_ENV alone", env)
	}
}
