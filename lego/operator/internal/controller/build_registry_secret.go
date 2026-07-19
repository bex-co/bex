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
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = map[string][]byte{
			buildRegistryConfigKey:    externalConfig,
			buildkitRegistryConfigKey: []byte(buildkitRegistryConfig(r.Registry)),
		}
		if secret.Labels == nil {
			secret.Labels = map[string]string{}
		}
		secret.Labels[labelApp] = app.Name
		secret.Labels["app.bex.co/component"] = buildRegistryComponent
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

func (r *AppReconciler) deleteBuildRegistrySecret(ctx context.Context, appName, buildNS string) error {
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(appName, "registry-auth"), Namespace: buildNS}}
	return client.IgnoreNotFound(r.buildPlaneClient().Delete(ctx, secret))
}

// copyBuildRegistryCredential mirrors a per-App dockerconfigjson Secret into
// the execution namespace and adds the config.json filename consumed by
// skopeo/cosign. The original .dockerconfigjson key and Secret type remain
// intact for kubelet imagePullSecret use by publish/pre-deploy Jobs.
func (r *AppReconciler) copyBuildRegistryCredential(ctx context.Context, srcNS, dstNS, appName, name string) error {
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
		dst.Type = src.Type
		dst.Data = maps.Clone(src.Data)
		dst.Data[buildRegistryConfigKey] = config
		if dst.Labels == nil {
			dst.Labels = map[string]string{}
		}
		dst.Labels[labelApp] = appName
		dst.Labels["app.bex.co/component"] = buildRegistryComponent
		return nil
	})
	return err
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
