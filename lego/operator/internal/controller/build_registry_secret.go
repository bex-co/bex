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
	"encoding/json"
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
)

// prepareBuildRegistrySecret merges a Dockerfile build's explicit private-base
// credential with the platform push credential. The result is a deterministic
// Secret in the build namespace; BuildJob mounts only that Secret into the
// buildkitd/cosign container filesystem. No external binding returns the
// existing push Secret verbatim, preserving the historical Job byte-for-byte.
func (r *AppReconciler) prepareBuildRegistrySecret(ctx context.Context, app *appv1alpha1.App, buildNS, builder string) (string, error) {
	if !usesBuildRegistryConfig(app, builder) {
		return r.RegistryPushSecret, nil
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
	configs := [][]byte{externalConfig}
	if r.RegistryPushSecret != "" {
		var push corev1.Secret
		if err := cl.Get(ctx, client.ObjectKey{Namespace: buildNS, Name: r.RegistryPushSecret}, &push); err != nil {
			return "", fmt.Errorf("get registry push credential: %w", err)
		}
		pushConfig, err := dockerConfigData(&push)
		if err != nil {
			return "", err
		}
		configs = append(configs, pushConfig) // push auth wins a same-host collision
	}
	merged, err := mergeDockerConfigs(configs...)
	if err != nil {
		return "", err
	}
	name := build.JobName(app.Name, "registry-auth")
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: buildNS}}
	if _, err := controllerutil.CreateOrUpdate(ctx, cl, secret, func() error {
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = map[string][]byte{
			buildRegistryConfigKey:    merged,
			buildkitRegistryConfigKey: []byte(buildkitRegistryConfig(r.Registry)),
		}
		if secret.Labels == nil {
			secret.Labels = map[string]string{}
		}
		secret.Labels[labelApp] = app.Name
		secret.Labels["app.bex.co/component"] = "build-registry-secret"
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

func dockerConfigData(secret *corev1.Secret) ([]byte, error) {
	if data := secret.Data[buildRegistryConfigKey]; len(data) > 0 {
		return data, nil
	}
	if data := secret.Data[corev1.DockerConfigJsonKey]; len(data) > 0 {
		return data, nil
	}
	return nil, fmt.Errorf("registry credential Secret %s/%s has neither %s nor %s", secret.Namespace, secret.Name, buildRegistryConfigKey, corev1.DockerConfigJsonKey)
}

// mergeDockerConfigs unions Docker config auths in order. Later configs win a
// same-host collision, so the platform push credential cannot be shadowed by a
// tenant credential for the output registry. Non-auth top-level fields are
// preserved with the same last-writer rule.
func mergeDockerConfigs(configs ...[]byte) ([]byte, error) {
	doc := map[string]json.RawMessage{}
	auths := map[string]json.RawMessage{}
	for _, raw := range configs {
		var next map[string]json.RawMessage
		if err := json.Unmarshal(raw, &next); err != nil {
			return nil, fmt.Errorf("decode registry docker config: %w", err)
		}
		for key, value := range next {
			if key != "auths" {
				doc[key] = value
			}
		}
		if rawAuths := next["auths"]; len(rawAuths) > 0 {
			var nextAuths map[string]json.RawMessage
			if err := json.Unmarshal(rawAuths, &nextAuths); err != nil {
				return nil, fmt.Errorf("decode registry docker config auths: %w", err)
			}
			maps.Copy(auths, nextAuths)
		}
	}
	rawAuths, err := json.Marshal(auths)
	if err != nil {
		return nil, fmt.Errorf("encode registry docker config auths: %w", err)
	}
	doc["auths"] = rawAuths
	merged, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("encode registry docker config: %w", err)
	}
	return merged, nil
}
