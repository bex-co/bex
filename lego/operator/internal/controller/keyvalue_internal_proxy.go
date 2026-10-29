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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// tlsForwarded reports whether the auth proxy and backend Service carry the
// TLS port: always for a runtime-switch store, so opening its listener rolls
// nothing, and while an older store's static listener runs.
func (i keyValueIntent) tlsForwarded() bool { return i.tlsRuntime || i.tlsListener }

func keyValueInternalProxyName(kv *appv1alpha1.KeyValue) string   { return kv.Name + "-internal" }
func keyValueInternalBackendName(kv *appv1alpha1.KeyValue) string { return kv.Name + "-backend" }

// reconcileKeyValueInternalProxy leaves the Valkey pod template and data alone.
// The stable Service switches only after an authenticated readiness check; the
// backend Service always selects the original server, never the proxy itself.
func (r *KeyValueReconciler) reconcileKeyValueInternalProxy(ctx context.Context, kv *appv1alpha1.KeyValue, intent keyValueIntent) (bool, error) {
	if r.InternalProxyImage == "" {
		return false, nil
	} // legacy embedding without a helper image
	if err := r.reconcileKeyValueInternalProxyPolicy(ctx, kv); err != nil {
		return false, err
	}
	backend := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: keyValueInternalBackendName(kv), Namespace: kv.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, backend, func() error {
		backend.Spec.Type = corev1.ServiceTypeClusterIP
		backend.Spec.Selector = map[string]string{labelKeyValue: kv.Name}
		backend.Spec.Ports = []corev1.ServicePort{{Name: keyValueContainerName, Port: kvPort, TargetPort: intstr.FromInt(kvPort)}}
		if intent.tlsForwarded() {
			backend.Spec.Ports = append(backend.Spec.Ports, corev1.ServicePort{Name: "valkey-tls", Port: kvTLSPort, TargetPort: intstr.FromInt(kvTLSPort)})
		}
		applyServicePortServerDefaults(backend.Spec.Ports)
		return controllerutil.SetControllerReference(kv, backend, r.Scheme)
	}); err != nil {
		return false, err
	}
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: keyValueInternalProxyName(kv), Namespace: kv.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		replicas := int32(1)
		if kv.Spec.Suspended {
			replicas = 0
		}
		dep.Spec.Replicas = &replicas
		dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyValueInternalProxy: kv.Name}}
		dep.Spec.Strategy = appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
			RollingUpdate: &appsv1.RollingUpdateDeployment{
				MaxUnavailable: new(intstr.FromInt(0)), MaxSurge: new(intstr.FromInt(1)),
			},
		}
		dep.Spec.Template.Labels = keyValueInternalProxyPodLabels(kv)
		dep.Spec.Template.Annotations = map[string]string{"app.bex.co/credential-revision": intent.credentialRevision}
		env := []corev1.EnvVar{
			{Name: "BEX_KV_UPSTREAM_ADDR", Value: fmt.Sprintf("%s.%s.svc:%d", backend.Name, kv.Namespace, kvPort)},
			kvSecretEnv(kvPasswordEnv, intent.authSecretName),
		}
		ports := []corev1.ContainerPort{{Name: keyValueContainerName, ContainerPort: kvPort}}
		if intent.tlsForwarded() {
			env = append(env,
				corev1.EnvVar{Name: "BEX_KV_TLS_ADDR", Value: ":6380"},
				corev1.EnvVar{Name: "BEX_KV_TLS_UPSTREAM_ADDR", Value: fmt.Sprintf("%s.%s.svc:%d", backend.Name, kv.Namespace, kvTLSPort)},
			)
			ports = append(ports, corev1.ContainerPort{Name: "valkey-tls", ContainerPort: kvTLSPort})
		}
		dep.Spec.Template.Spec.Containers = []corev1.Container{{
			Name: "internal-proxy", Image: r.InternalProxyImage,
			Command: []string{"/kv-internal-proxy"}, Env: env, Ports: ports,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("25m"), corev1.ResourceMemory: resource.MustParse("16Mi")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("250m"), corev1.ResourceMemory: resource.MustParse("128Mi")},
			}, SecurityContext: tenantSecCtx(),
			ReadinessProbe: &corev1.Probe{
				ProbeHandler:   corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{"/kv-internal-proxy", "--check"}}},
				TimeoutSeconds: 6, PeriodSeconds: 5,
			},
		}}
		dep.Spec.Template.Spec.AutomountServiceAccountToken = new(false)
		applyPodSpecServerDefaults(&dep.Spec.Template.Spec)
		return controllerutil.SetControllerReference(kv, dep, r.Scheme)
	}); err != nil {
		return false, err
	}
	return keyValueInternalDeploymentReady(dep), nil
}

func keyValueInternalDeploymentReady(dep *appsv1.Deployment) bool {
	return dep.Spec.Replicas != nil && *dep.Spec.Replicas > 0 &&
		dep.Status.ReadyReplicas >= *dep.Spec.Replicas && deploymentRolloutReady(dep, *dep.Spec.Replicas)
}

func (r *KeyValueReconciler) keyValueInternalProxyReady(ctx context.Context, kv *appv1alpha1.KeyValue) bool {
	if r.InternalProxyImage == "" {
		return true
	}
	dep := &appsv1.Deployment{}
	reader := r.APIReader
	if reader == nil {
		reader = r.Client
	}
	return reader.Get(ctx, client.ObjectKey{Name: keyValueInternalProxyName(kv), Namespace: kv.Namespace}, dep) == nil && keyValueInternalDeploymentReady(dep)
}
