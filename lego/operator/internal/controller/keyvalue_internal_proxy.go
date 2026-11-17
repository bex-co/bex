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
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/types/tiers"
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
		before := dep.Spec.Template.DeepCopy()
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
		dep.Spec.Template.Annotations = map[string]string{credentialRevisionAnnotation: intent.credentialRevision}
		env := []corev1.EnvVar{
			{Name: "BEX_KV_UPSTREAM_ADDR", Value: fmt.Sprintf("%s.%s.svc:%d", backend.Name, kv.Namespace, kvPort)},
			kvSecretEnv(kvPasswordEnv, intent.authSecretName),
			// The plan's limit, one budget across both listeners (w6/m456).
			{Name: "BEX_KV_MAX_CONNECTIONS", Value: strconv.Itoa(int(intent.plan.ConnectionLimit))},
		}
		ports := []corev1.ContainerPort{{Name: keyValueContainerName, ContainerPort: kvPort}}
		var mounts []corev1.VolumeMount
		var volumes []corev1.Volume
		if intent.tlsForwarded() {
			env = append(env,
				corev1.EnvVar{Name: "BEX_KV_TLS_ADDR", Value: ":6380"},
				corev1.EnvVar{Name: "BEX_KV_TLS_UPSTREAM_ADDR", Value: fmt.Sprintf("%s.%s.svc:%d", backend.Name, kv.Namespace, kvTLSPort)},
				// The store's own certificate answers a TLS client past the
				// budget with Valkey's refusal. Optional, as on the server: the
				// Secret exists only while the store is (or was) public.
				corev1.EnvVar{Name: "BEX_KV_TLS_CERT_FILE", Value: kvTLSCertFile},
				corev1.EnvVar{Name: "BEX_KV_TLS_KEY_FILE", Value: kvTLSKeyFile},
			)
			ports = append(ports, corev1.ContainerPort{Name: "valkey-tls", ContainerPort: kvTLSPort})
			mounts = []corev1.VolumeMount{{Name: kvTLSVolume, MountPath: kvTLSMountPath, ReadOnly: true}}
			volumes = []corev1.Volume{{Name: kvTLSVolume, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
				SecretName: intent.tlsSecretName, Optional: new(true),
			}}}}
		}
		dep.Spec.Template.Spec.Volumes = volumes
		dep.Spec.Template.Spec.Containers = []corev1.Container{{
			Name: "internal-proxy", Image: r.InternalProxyImage,
			Command: []string{"/kv-internal-proxy"}, Env: env, Ports: ports, VolumeMounts: mounts,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("25m"), corev1.ResourceMemory: resource.MustParse("16Mi")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("250m"), corev1.ResourceMemory: kvInternalProxyMemoryLimit(intent.plan)},
			}, SecurityContext: execution.RestrictedContainerSecurityContext(),
			ReadinessProbe: &corev1.Probe{
				ProbeHandler:   corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{"/kv-internal-proxy", "--check"}}},
				TimeoutSeconds: 6, PeriodSeconds: 5,
			},
		}}
		dep.Spec.Template.Spec.AutomountServiceAccountToken = new(false)
		applyPodSpecServerDefaults(&dep.Spec.Template.Spec)
		holdDatastorePriority(&dep.Spec.Template, !dep.CreationTimestamp.IsZero(), before)
		// An isolation toggle alone must not roll the one proxy replica: its pods
		// are relabelled in place (syncKeyValuePodIsolation, w8/220).
		holdIsolationLabel(&dep.Spec.Template, !dep.CreationTimestamp.IsZero(), before, kv.Labels)
		return controllerutil.SetControllerReference(kv, dep, r.Scheme)
	}); err != nil {
		return false, err
	}
	return keyValueInternalDeploymentReady(dep), nil
}

// kvInternalProxyMemoryLimit lets the proxy hold its plan's connections: 32Mi
// plus 48KiB per connection, never below the 128Mi every proxy had. Measured
// 2026-11-17 (w6/m456) with 5,000 idle clients through the proxy to Valkey 8
// on linux/arm64: the container's cgroup held 195.7 MB (104 MB anonymous, 85 MB
// kernel, the sockets and splice pipes), about 39 KB per connection.
func kvInternalProxyMemoryLimit(plan tiers.ValkeyTier) resource.Quantity {
	limit := resource.MustParse("128Mi")
	scaled := resource.NewQuantity((32<<20)+int64(plan.ConnectionLimit)*(48<<10), resource.BinarySI)
	if scaled.Cmp(limit) > 0 {
		return *scaled
	}
	return limit
}

func keyValueInternalDeploymentReady(dep *appsv1.Deployment) bool {
	return dep.Spec.Replicas != nil && *dep.Spec.Replicas > 0 &&
		dep.Status.ReadyReplicas >= *dep.Spec.Replicas && deploymentRolloutReady(dep, *dep.Spec.Replicas)
}

func (r *KeyValueReconciler) keyValueInternalProxyReady(ctx context.Context, kv *appv1alpha1.KeyValue) bool {
	if r.InternalProxyImage == "" || kv.Spec.InternalAuth {
		return true
	}
	dep := &appsv1.Deployment{}
	reader := r.APIReader
	if reader == nil {
		reader = r.Client
	}
	return reader.Get(ctx, client.ObjectKey{Name: keyValueInternalProxyName(kv), Namespace: kv.Namespace}, dep) == nil && keyValueInternalDeploymentReady(dep)
}

// reconcileKeyValueInternalRoute points the stable Service at the
// password-less proxy once it is ready (w8/204) or, with internal auth enabled
// (w5/m268), straight at Valkey — then retires the proxy, dropping the
// connections it carried unauthenticated, as Render's restart does.
func (r *KeyValueReconciler) reconcileKeyValueInternalRoute(ctx context.Context, kv *appv1alpha1.KeyValue, intent *keyValueIntent) error {
	if kv.Spec.InternalAuth {
		kv.Status.InternalAuthOptional = false
		intent.internalProxy = false
		if err := r.reconcileKeyValueService(ctx, kv, *intent); err != nil {
			return err
		}
		return r.retireKeyValueInternalProxy(ctx, kv)
	}
	proxyReady, err := r.reconcileKeyValueInternalProxy(ctx, kv, *intent)
	if err != nil {
		return err
	}
	intent.internalProxy = proxyReady || kv.Status.InternalAuthOptional
	if err := r.reconcileKeyValueService(ctx, kv, *intent); err != nil {
		return err
	}
	if proxyReady {
		kv.Status.InternalAuthOptional = true
	}
	return nil
}

// retireKeyValueInternalProxy removes the password-less front of a store whose
// internal auth was enabled: its Deployment, backend Service and the two
// NetworkPolicies that existed only for it.
func (r *KeyValueReconciler) retireKeyValueInternalProxy(ctx context.Context, kv *appv1alpha1.KeyValue) error {
	for _, obj := range []client.Object{
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: keyValueInternalProxyName(kv), Namespace: kv.Namespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: keyValueInternalBackendName(kv), Namespace: kv.Namespace}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: kv.Name + "-internal-proxy-environment-ingress", Namespace: kv.Namespace}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: kv.Name + "-internal-proxy-backend-ingress", Namespace: kv.Namespace}},
	} {
		if _, err := deleteAndWait(ctx, r.Client, obj); err != nil {
			return err
		}
	}
	return nil
}
