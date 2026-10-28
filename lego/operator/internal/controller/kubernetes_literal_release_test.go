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

package controller

import (
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/bex-co/bex/lego/operator/internal/execution/executiontest"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestRuntimePlatformLiteralsKeepBuildInputsRaw(t *testing.T) {
	const branch = "release/$$/$(PORT)"
	app := projectionApp(func(a *appv1alpha1.App) { a.Spec.Repo = "https://github.com/acme/web"; a.Spec.Branch = branch })
	platform := platformEnv(app, platformEnvInputs{})
	original := append([]corev1.EnvVar(nil), platform...)
	runtime := withPlatformEnv([]corev1.EnvVar{{Name: portEnvName, Value: "10000"}}, platform)
	if got := executiontest.Expand(envMap(runtime)["RENDER_GIT_BRANCH"], map[string]string{"PORT": "injected"}); got != branch {
		t.Fatalf("runtime branch=%q, want raw%q", got, branch)
	}
	if !reflect.DeepEqual(platform, original) {
		t.Fatal("runtime escaping changed reusable platform inputs")
	}
	if got := envMap(withBuildPlatformEnv(nil, app, ""))["RENDER_GIT_BRANCH"]; got != branch {
		t.Fatalf("build branch=%q, want unescaped%q", got, branch)
	}
	for _, value := range runtime {
		if value.Name == "RENDER_INSTANCE_ID" && (value.ValueFrom == nil || value.ValueFrom.FieldRef == nil || value.ValueFrom.FieldRef.FieldPath != "metadata.name") {
			t.Fatalf("field reference became a literal: %+v", value)
		}
	}
}

func TestDollarCommandsKeepCurrentAndHistoricalLaunchClassification(t *testing.T) {
	app := projectionApp(func(a *appv1alpha1.App) { a.Spec.Repo = "https://github.com/acme/native"; a.Spec.Runtime = "node" })
	for _, command := range []string{`printf 'a$$b'`, `printf "a$$b $(PORT)"`, `sh -c 'printf "a$$b $(PORT)"'`} {
		for name, argv := range map[string][]string{"protected": shellCommand(command), "raw parser": rawShellCommand(command), "pre164legacy": legacyShellCommand(command)} {
			t.Run(command+"/"+name, func(t *testing.T) {
				rec := runtimeConfigRecord{spec: appv1alpha1.ReleaseRecordSpec{Version: 1, StartCommand: command}, template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: appContainerName, Image: "historic-image", Command: argv}}}}}
				if got := rec.launch(app); got != launchShell {
					t.Fatalf("historical command=%v classified%q, want shell", argv, got)
				}
				projected := appContainer(rec.projectRuntimeApp(app, 1), deploymentParams{image: "historic-image", port: 10000, launch: rec.launch(app)}, "")
				raw := rawShellCommand(command)
				if len(projected.Command) != len(raw) {
					t.Fatalf("projected argv=%v, want%v", projected.Command, raw)
				}
				for i, arg := range projected.Command {
					if got := executiontest.Expand(arg, map[string]string{"PORT": "injected"}); got != raw[i] {
						t.Fatalf("argument%d=%q, want%q", i, got, raw[i])
					}
				}
			})
		}
	}
}

func TestLiteralReleaseRecordKeepsAuthoredValuesThroughRepeatedProjection(t *testing.T) {
	const command = `printf "a$$b $(PORT)"`
	const literal = `env$$$(PORT)`
	app := projectionApp(func(a *appv1alpha1.App) { a.Spec.Repo = "https://github.com/acme/native"; a.Spec.Runtime = "node" })
	rec := &runtimeConfigRecord{spec: appv1alpha1.ReleaseRecordSpec{Version: 1, StartCommand: command, Env: []appv1alpha1.EnvVar{{Name: "MARK", Value: literal}}}, template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: appContainerName, Image: "historic-image", Command: shellCommand(command)}}}}}
	r := m202Reconciler(t, app, 1, rec)
	for range 2 {
		loaded, err := r.readRuntimeConfigRecord(context.Background(), app, 1)
		if err != nil {
			t.Fatal(err)
		}
		if loaded.spec.StartCommand != command || len(loaded.spec.Env) != 1 || loaded.spec.Env[0].Value != literal {
			t.Fatalf("record contains wire escaping: %+v", loaded.spec)
		}
		effective := loaded.projectRuntimeApp(app, 1)
		projected := appContainer(effective, deploymentParams{image: "historic-image", port: 10000, launch: loaded.launch(app)}, "")
		if got := executiontest.Expand(envMap(projected.Env)["MARK"], map[string]string{"PORT": "injected"}); got != literal {
			t.Fatalf("materialized env=%q, want%q", got, literal)
		}
		if err := r.writeRuntimeConfigRecord(context.Background(), app, 1, &runtimeConfigRecord{spec: releaseRecordSpec(effective), template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{projected}}}}); err != nil {
			t.Fatal(err)
		}
	}
}
