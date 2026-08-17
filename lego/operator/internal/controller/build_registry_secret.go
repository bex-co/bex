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
	"maps"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	buildRegistryConfigKey    = "config.json"
	buildkitRegistryConfigKey = "buildkitd.toml"
	buildRegistryComponent    = "build-registry-secret"
)

// prepareBuildRegistrySecret copies only a Dockerfile build's explicit private-
// base pull credential into a deterministic Secret in the execution namespace.
// Platform push auth deliberately stays in RegistryPushSecret and is mounted
// only by the post-build push/sign phases.
func (r *AppReconciler) prepareBuildRegistrySecret(ctx context.Context, app *appv1alpha1.App, buildNS, builder string) (string, error) {
	if !usesBuildRegistryConfig(app, builder) {
		return "", nil
	}
	cl := r.buildPlaneClient()
	var external corev1.Secret
	if err := cl.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Spec.ExternalRegistryPullSecret}, &external); err != nil {
		return "", fmt.Errorf("get Docker-build registry credential: %w", err)
	}
	externalConfig, err := dockerConfigData(&external)
	if err != nil {
		return "", err
	}
	name := build.JobName(app.Name, "registry-auth")
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: buildNS}}
	if _, err := controllerutil.CreateOrUpdate(ctx, cl, secret, func() error {
		if err := checkOwnedArtifact(secret, app); err != nil {
			return err
		}
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = map[string][]byte{
			buildRegistryConfigKey:    externalConfig,
			buildkitRegistryConfigKey: []byte(buildkitRegistryConfig(r.Registry)),
		}
		secret.Labels = artifactLabels(app, buildRegistryComponent)
		return nil
	}); err != nil {
		return "", fmt.Errorf("write merged build registry credential: %w", err)
	}
	return name, nil
}

func usesBuildRegistryConfig(app *appv1alpha1.App, builder string) bool {
	return builder == build.BuilderDockerfile && app.Spec.ExternalRegistryPullSecret != ""
}

// buildkitRegistryConfig makes the already-insecure platform output registry
// available as a Dockerfile base-image source too. BuildKit's output flag only
// controls pushes; its source resolver otherwise assumes HTTPS for the same
// cluster-local Zot endpoint.
func buildkitRegistryConfig(registry string) string {
	registry = strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(registry)
	return fmt.Sprintf("[registry.%q]\n  http = true\n", registry)
}

// copyBuildRegistryCredential mirrors a per-App dockerconfigjson Secret into
// the execution namespace and adds the config.json filename consumed by
// skopeo/cosign. The original .dockerconfigjson key and Secret type remain
// intact for kubelet imagePullSecret use by publish/pre-deploy Jobs.
func (r *AppReconciler) copyBuildRegistryCredential(ctx context.Context, app *appv1alpha1.App, srcNS, dstNS, name string) error {
	cl := r.buildPlaneClient()
	var src corev1.Secret
	if err := cl.Get(ctx, client.ObjectKey{Namespace: srcNS, Name: name}, &src); err != nil {
		return err
	}
	config, err := dockerConfigData(&src)
	if err != nil {
		return err
	}
	dst := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: dstNS}}
	_, err = controllerutil.CreateOrUpdate(ctx, cl, dst, func() error {
		if err := checkOwnedArtifact(dst, app); err != nil {
			return err
		}
		dst.Type = src.Type
		dst.Data = maps.Clone(src.Data)
		dst.Data[buildRegistryConfigKey] = config
		dst.Labels = artifactLabels(app, buildRegistryComponent)
		return nil
	})
	return err
}

// checkOwnedArtifact is the shared CreateOrUpdate mutate-guard for objects the
// reconciler writes into the SHARED build namespace under a name derived from
// the workspace-local App name (codex-security round 12, finding 1). App names
// are unique only per workspace, so a same-named App in another workspace
// resolves to the same deterministic object name; without this gate its
// reconcile would silently overwrite this App's credential bytes (and relabel
// ownership), swapping one tenant's external-registry credential into the
// other's BuildKit pod. Fail closed exactly like the build/pre-deploy Jobs
// (build.EnsureBuild's CheckOwner): a foreign-owned object is an error, never
// an adoption. A not-yet-existing object (empty ResourceVersion) is a plain
// create and always passes.
func checkOwnedArtifact(obj metav1.Object, app *appv1alpha1.App) error {
	if obj.GetResourceVersion() == "" {
		return nil // fresh create — nothing to own yet
	}
	identity := execution.ArtifactIdentity{Name: app.Name, UID: string(app.UID),
		Workspace: app.Labels[labelWorkspace], Namespace: app.Namespace}
	if err := identity.CheckOwner(obj); err != nil {
		return fmt.Errorf("refusing to overwrite %s/%s owned by a different App lifetime: %w",
			obj.GetNamespace(), obj.GetName(), err)
	}
	return nil
}

func artifactLabels(app *appv1alpha1.App, component string) map[string]string {
	return execution.PodLabels(app.Name, string(app.UID), component,
		app.Labels[labelWorkspace], app.Namespace, false)
}

func dockerConfigData(secret *corev1.Secret) ([]byte, error) {
	if data := secret.Data[buildRegistryConfigKey]; len(data) > 0 {
		return data, nil
	}
	if data := secret.Data[corev1.DockerConfigJsonKey]; len(data) > 0 {
		return data, nil
	}
	return nil, fmt.Errorf("registry credential Secret %s/%s has neither %s nor %s", secret.Namespace, secret.Name, buildRegistryConfigKey, corev1.DockerConfigJsonKey)
}
