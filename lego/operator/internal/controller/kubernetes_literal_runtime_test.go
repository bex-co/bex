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
	"os/exec"
	"regexp"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/execution/executiontest"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestKubernetesLiteralCommandsReachRuntimeAcrossExecutionModes(t *testing.T) {
	const script = `printf 'PID=%s\nLITERAL=%s\nSUB=%s\nENV=%s\n' "$$" 'a$$b $(PORT)' "$(printf shell)" "$MARK"`
	const mark = `pa$$wo$(HOME)rd`
	app := projectionApp(func(a *appv1alpha1.App) {
		a.Spec.Image = "example:1"
		a.Spec.Repo = ""
		a.Spec.Runtime = "image"
		a.Spec.StartCommand = script
		a.Spec.Env = []appv1alpha1.EnvVar{{Name: "HOME", Value: "/platform"}, {Name: "MARK", Value: mark}}
	})
	runtimeEnv := map[string]string{}
	processEnv := make([]string, 0, len(app.Spec.Env)+1)
	for _, e := range appEnv(app, 10000) {
		value := executiontest.Expand(e.Value, runtimeEnv)
		runtimeEnv[e.Name] = value
		processEnv = append(processEnv, e.Name+"="+value)
	}
	if runtimeEnv["MARK"] != mark {
		t.Fatalf("literal env value changed: %q", runtimeEnv["MARK"])
	}
	native := app.DeepCopy()
	native.Spec.Repo = "https://github.com/example/app"
	native.Spec.Image = ""
	native.Spec.Builder = build.BuilderNative
	commands := map[string][]string{
		"deployment":                appContainer(app, deploymentParams{image: app.Spec.Image, port: 10000}, "").Command,
		"scheduled and manual cron": (&AppReconciler{}).cronPodSpec(app, app.Spec.Image, 10000, nil, launchUnknown, "").Spec.Containers[0].Command,
		"Docker predeploy":          preDeployArgv(app, app.Spec.Image, script),
		"native predeploy":          preDeployArgv(native, "native:1", script),
	}
	expected := regexp.MustCompile(`(?s)^PID=[0-9]+\nLITERAL=a\$\$b \$\(PORT\)\nSUB=shell\nENV=pa\$\$wo\$\(HOME\)rd\n$`)
	for name, command := range commands {
		t.Run(name, func(t *testing.T) {
			if len(command) == 0 {
				t.Fatal("command not projected")
			}
			actual := make([]string, len(command))
			for i, arg := range command {
				actual[i] = executiontest.Expand(arg, runtimeEnv)
			}
			cmd := exec.Command(actual[0], actual[1:]...)
			cmd.Env = processEnv
			output, err := cmd.CombinedOutput()
			if err != nil || !expected.Match(output) {
				t.Fatalf("runtime output=%q err=%v argv=%q", output, err, actual)
			}
		})
	}
	// Secret references stay references: kubelet reads their bytes rather than
	// expanding them as an authored EnvVar.Value.
	app.Spec.Env = []appv1alpha1.EnvVar{{Name: "REF", ValueFrom: &appv1alpha1.EnvVarSource{SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "secret", Key: "value"}}}}
	env := appEnv(app, 10000)
	if len(env) != 2 || env[0].Value != "" || env[0].ValueFrom == nil || !strings.EqualFold(env[0].ValueFrom.SecretKeyRef.Name, "secret") {
		t.Fatalf("secret reference changed: %+v", env)
	}
}

// TestPreLiteralNativeArtifactRebuildsOnRedeploy pins the w8/206 rebuild gate.
// A native image built before Kubernetes literals were protected baked a
// collapsed `$$` into its RUN output and CMD. Nothing in the saved spec changed
// when the operator was fixed, so its artifact fingerprint still matches. A
// redeploy on the same commit with the same commands must still build a fresh
// artifact under a fresh build revision: every deploy verb stamps
// spec.restartedAt (an artifact input) and a new release generation. An
// operational edit (a scale) keeps reusing the served image, as before.
func TestPreLiteralNativeArtifactRebuildsOnRedeploy(t *testing.T) {
	const commit = "180e352cc63d9c887ce4b1bffe84c6d5def07cd5"
	served := appv1alpha1.AppSpec{
		Repo: "https://github.com/render-examples/express-hello-world", Branch: "main", BuildCommit: commit,
		Runtime: "node", Builder: build.BuilderNative, Port: 10000, Replicas: 1,
		BuildCommand: `printf "BUILDDD=[a$$b]\n"; yarn`,
		StartCommand: `echo "pid=[$$] dd=[a$$b]"; exec node app.js`,
		RestartedAt:  "2026-10-26T11:26:00Z",
	}
	identity := desiredAppReleaseIdentity(served)
	staleImage := "registry.bex/tea-x/app:" + appv1alpha1.BuildRevision(2)
	servedApp := func() *appv1alpha1.App {
		return &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "tea-x", UID: "app-uid", Generation: 2,
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "2"}},
			Spec: served,
			Status: appv1alpha1.AppStatus{
				Phase: appv1alpha1.PhaseRunning, Image: staleImage, ArtifactImage: staleImage,
				ArtifactFingerprint: identity.artifact, ReleaseFingerprint: identity.release,
				ReleaseArtifactFingerprint: identity.artifact,
				ReleaseGeneration:          2, ActiveRevision: "rev-2", ObservedGeneration: 2,
			},
		}
	}

	// deploys.triggerFetched: a new release generation and restart stamp; the
	// same commit re-resolves to the same BuildCommit.
	redeploy := servedApp()
	redeploy.Generation = 3
	redeploy.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "3"
	redeploy.Spec.RestartedAt = "2026-10-29T08:00:00Z"
	decision := prepareAppReleaseDecision(redeploy, nil)
	if !decision.artifactChanged || !decision.releaseChanged {
		t.Fatalf("redeploy decision = %+v, want a new artifact and release", decision)
	}
	if image, ok := reusableArtifactImage(redeploy, decision); ok || image != "" {
		t.Fatalf("redeploy reused %q, want a rebuild of the pre-literal artifact", image)
	}
	if got := releaseBuildRevision(redeploy); got != appv1alpha1.BuildRevision(3) {
		t.Fatalf("build revision = %q, want a fresh revision so neither the old Job nor the old tag is reused", got)
	}

	scaled := servedApp()
	scaled.Generation = 3
	scaled.Spec.Replicas = 2
	decision = prepareAppReleaseDecision(scaled, nil)
	if image, ok := reusableArtifactImage(scaled, decision); decision.artifactChanged || !ok || image != staleImage {
		t.Fatalf("scale decision = %+v reused %q (%v), want the served image", decision, image, ok)
	}
}
