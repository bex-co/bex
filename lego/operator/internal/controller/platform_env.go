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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/registry"
	"github.com/bex-co/bex/lego/types/renderenv"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// platformEnvInputs are the values that do not come from the App itself.
type platformEnvInputs struct {
	baseDomain string
	// gitCommit is the commit the running image was built from ("" when the
	// image carries none, e.g. a prebuilt image).
	gitCommit string
}

// platformEnv is Render's default set for app's container (w5/m148; ADR018):
// renderenv's static set, the pod name as RENDER_INSTANCE_ID, and the image's
// commit. As on Render, the documented names are platform values the operator
// owns the way it owns PORT; any other RENDER_ name is the user's.
func platformEnv(app *appv1alpha1.App, in platformEnvInputs) []corev1.EnvVar {
	static := renderenv.Static(app, in.baseDomain)
	env := make([]corev1.EnvVar, 0, len(static)+2)
	for _, v := range static {
		env = append(env, corev1.EnvVar{Name: v.Name, Value: v.Value})
	}
	env = append(env, corev1.EnvVar{Name: renderenv.InstanceID, ValueFrom: &corev1.EnvVarSource{
		FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.name"},
	}})
	if in.gitCommit != "" {
		env = append(env, corev1.EnvVar{Name: renderenv.GitCommit, Value: in.gitCommit})
	}
	return env
}

// withPlatformEnv drops any user entry the platform owns and adds the platform
// set just before PORT, which appEnv always appends last.
func withPlatformEnv(env, platform []corev1.EnvVar) []corev1.EnvVar {
	if len(env) == 0 {
		return env
	}
	out := make([]corev1.EnvVar, 0, len(env)+len(platform))
	for _, e := range env[:len(env)-1] {
		if !renderenv.IsPlatformName(e.Name) {
			out = append(out, e)
		}
	}
	for _, e := range platform {
		if e.ValueFrom == nil {
			// Git branch/repository values are user input too. Keep the build
			// inputs raw, and protect only this runtime EnvVar.Value carrier.
			e.Value = execution.KubernetesLiteral(e.Value)
		}
		out = append(out, e)
	}
	return append(out, env[len(env)-1])
}

// imageCommit is the commit image was built from, for RENDER_GIT_COMMIT. bex's
// builds stamp it as the image's OCI revision label, so it follows the image a
// release actually runs (held, rolled back or current). It is read once per
// image and cached in status; an image outside the App's own built repository
// (a prebuilt image) has none.
//
// Best effort: an environment variable never holds a rollout. A failed read
// is logged, not cached, and retried on the next pass; the variable then
// appears with the next pod template.
func (r *AppReconciler) imageCommit(ctx context.Context, app *appv1alpha1.App, image string) string {
	if cached := app.Status.ImageRevision; cached != nil && cached.Image == image {
		return cached.Commit
	}
	reference, ok := r.ownImageReference(app, image)
	if !ok {
		return ""
	}
	commit, err := r.readImageRevision(ctx, app, reference)
	if err != nil {
		logf.FromContext(ctx).Info("RENDER_GIT_COMMIT unavailable for now", "app", app.Name, "image", image, "error", err.Error())
		return ""
	}
	// A label a user's own Dockerfile set is kept only when it is an object id.
	commit = strings.ToLower(expectedGitObjectID(commit))
	app.Status.ImageRevision = &appv1alpha1.ImageRevisionStatus{Image: image, Commit: commit}
	return commit
}

func (r *AppReconciler) readImageRevision(ctx context.Context, app *appv1alpha1.App, reference string) (string, error) {
	authorization, ready, err := r.registryAuthHeader(ctx, app)
	if err != nil {
		return "", err
	}
	if !ready {
		return "", fmt.Errorf("image registry credential is not active yet")
	}
	return registry.ReadImageRevision(ctx, r.HTTPClient, r.Registry, appIdentity(app).Repo(), reference, authorization)
}

// withBuildPlatformEnv adds Render's build-time platform variables to a build's
// literal env, replacing any stale user entry the platform owns. A build has
// no pod instance and no recommended concurrency, and its commit is stamped in
// the build pod itself.
func withBuildPlatformEnv(env []corev1.EnvVar, app *appv1alpha1.App, baseDomain string) []corev1.EnvVar {
	platform := platformEnv(app, platformEnvInputs{baseDomain: baseDomain})
	out := make([]corev1.EnvVar, 0, len(env)+len(platform))
	for _, e := range env {
		if !renderenv.IsPlatformName(e.Name) {
			out = append(out, e)
		}
	}
	for _, e := range platform {
		if e.ValueFrom == nil && e.Name != renderenv.WebConcurrency {
			out = append(out, e)
		}
	}
	return out
}

// reconcileDiscoveryService converges the headless Service whose DNS name
// resolves to every running instance (Render's RENDER_DISCOVERY_SERVICE): no
// ClusterIP, no ports, selecting all of the App's runtime pods across
// revisions. Present for web, private and worker services, removed for every
// other type; a name another object owns is left alone, like the slug alias.
func (r *AppReconciler) reconcileDiscoveryService(ctx context.Context, app *appv1alpha1.App) error {
	name := renderenv.DiscoveryServiceName(app)
	existing := &corev1.Service{}
	err := r.Get(ctx, client.ObjectKey{Name: name, Namespace: app.Namespace}, existing)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	found := err == nil
	if found && !metav1.IsControlledBy(existing, app) {
		// Never adopt or rewrite a Service this App does not control.
		logf.FromContext(ctx).Info("discovery Service name is taken by another object; skipping it",
			"app", app.Name, "service", name)
		return nil
	}
	if !renderenv.HasDiscoveryService(app) {
		if !found {
			return nil
		}
		return client.IgnoreNotFound(r.Delete(ctx, existing))
	}
	pod := appPodLabels(app, false)
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace}}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec.ClusterIP = corev1.ClusterIPNone
		svc.Spec.Selector = map[string]string{labelApp: pod[labelApp], labelAppID: pod[labelAppID]}
		return controllerutil.SetControllerReference(app, svc, r.Scheme)
	})
	return err
}
