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
	"runtime"
	"strconv"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/registry"
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
// Every store has one: it holds the plan's connection limit (w6/m456), and it
// authenticates internal clients unless the store requires them to
// authenticate themselves (spec.internalAuth, w6/m463).
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
			// A pod that stops lets its open commands finish (w6/m512).
			{Name: "BEX_KV_DRAIN_SECONDS", Value: strconv.Itoa(kvInternalProxyDrainSeconds)},
		}
		if kv.Spec.InternalAuth {
			// The plain listener forwards the client's own AUTH (w6/m463).
			// Enabling internal auth rolls only this proxy, closing its old
			// pod's unauthenticated connections; set only then, so the
			// operator upgrade rolls no other store's proxy.
			env = append(env, corev1.EnvVar{Name: "BEX_KV_INTERNAL_AUTH", Value: "client"})
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
			Name: kvInternalProxyContainer, Image: r.keyValueInternalProxyImage(ctx, dep, before),
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
		dep.Spec.Template.Spec.TerminationGracePeriodSeconds = new(int64(kvInternalProxyGraceSeconds))
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

const (
	kvInternalProxyContainer = "internal-proxy"
	// kvInternalProxyDrainSeconds is how long a proxy pod that gets SIGTERM
	// keeps relaying the streams it holds (BEX_KV_DRAIN_SECONDS, w6/m512):
	// Render's default shutdown delay for its own instances (render.com/docs/deploys,
	// read 2026-11-19). A command in flight finishes in milliseconds, and the
	// drain closes as soon as the last stream ends, so the bound only decides
	// when a held client (a pool, pub/sub, a long blocking command) is closed.
	kvInternalProxyDrainSeconds = 30
	// kvInternalProxyGraceSeconds lets the process close what the drain leaves
	// itself, before the kubelet's SIGKILL.
	kvInternalProxyGraceSeconds = kvInternalProxyDrainSeconds + 5
	// kvInternalProxyBinaryAnnotation records, on a proxy Deployment's own
	// metadata, the sha256 of the /kv-internal-proxy its pod template's image
	// carries (w6/m469). Never on the pod template: adding it there would roll
	// every proxy once.
	kvInternalProxyBinaryAnnotation = "app.bex.co/kv-internal-proxy-sha256"
	// kvInternalProxyStep is the image config history entry lego/Dockerfile's
	// COPY of the proxy binary leaves.
	kvInternalProxyStep = "COPY /workspace/kv-internal-proxy /kv-internal-proxy # buildkit"
)

// InternalProxyBinaryFromRegistry is the manager's InternalProxyBinaryOf: it
// reads /kv-internal-proxy out of a digest-pinned image's registry, once per
// digest, since every proxy written before w6/m469 runs the same image.
func InternalProxyBinaryFromRegistry() func(ctx context.Context, image string) (string, error) {
	var mu sync.Mutex
	known := map[string]string{}
	return func(ctx context.Context, image string) (string, error) {
		host, repo, _, digest, ok := parseExternalImageReference(image)
		if !ok || !registry.IsDigest(digest) {
			return "", fmt.Errorf("image %q is not pinned by digest", image)
		}
		mu.Lock()
		sum, hit := known[digest]
		mu.Unlock()
		if hit {
			return sum, nil
		}
		// The manager compares the image with its own binary, built for its
		// own architecture.
		platform := registry.Platform{OS: "linux", Architecture: runtime.GOARCH}
		sum, err := registry.ImageFileSHA256(ctx, nil, host, repo, digest, platform, kvInternalProxyStep, "kv-internal-proxy")
		if err != nil {
			return "", err
		}
		mu.Lock()
		known[digest] = sum
		mu.Unlock()
		return sum, nil
	}
}

// keyValueInternalProxyImage picks a proxy's image (w6/m469): the one it runs
// while that image carries the manager's own /kv-internal-proxy, so an
// operator deploy that leaves the proxy's code alone changes nothing in its
// pod template and closes none of its connections. Otherwise the proxy rolls
// onto the manager's image, as before. Every other template field is rendered
// as always, so an env, annotation or resource change still rolls the proxy,
// on the held image. before is the Deployment's template as read.
func (r *KeyValueReconciler) keyValueInternalProxyImage(ctx context.Context, dep *appsv1.Deployment, before *corev1.PodTemplateSpec) string {
	if r.InternalProxyBinary == "" {
		delete(dep.Annotations, kvInternalProxyBinaryAnnotation)
		return r.InternalProxyImage
	}
	running := ""
	for _, c := range before.Spec.Containers {
		if c.Name == kvInternalProxyContainer {
			running = c.Image
		}
	}
	if running != "" && running != r.InternalProxyImage && r.carriesInternalProxyBinary(ctx, dep, running) {
		return running
	}
	metav1.SetMetaDataAnnotation(&dep.ObjectMeta, kvInternalProxyBinaryAnnotation, r.InternalProxyBinary)
	return r.InternalProxyImage
}

// carriesInternalProxyBinary reports whether a proxy's running image carries
// the manager's /kv-internal-proxy: by the hash its Deployment records, or,
// for a Deployment that records none (one the operator wrote before w6/m469),
// by the binary read out of the image's registry, which it then records. A
// failed lookup answers false, and the proxy rolls as it always did.
func (r *KeyValueReconciler) carriesInternalProxyBinary(ctx context.Context, dep *appsv1.Deployment, image string) bool {
	if recorded := dep.Annotations[kvInternalProxyBinaryAnnotation]; recorded != "" || r.InternalProxyBinaryOf == nil {
		return recorded == r.InternalProxyBinary
	}
	sum, err := r.InternalProxyBinaryOf(ctx, image)
	if err != nil {
		logf.FromContext(ctx).Info("rolling the Key Value proxy onto the operator's image: its own image's binary is unknown",
			"deployment", dep.Name, "image", image, "reason", err.Error())
		return false
	}
	if sum != r.InternalProxyBinary {
		return false
	}
	metav1.SetMetaDataAnnotation(&dep.ObjectMeta, kvInternalProxyBinaryAnnotation, sum)
	return true
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

// reconcileKeyValueInternalRoute points the stable Service at the store's
// internal proxy once it is ready (w8/204) and keeps it there, so every tenant
// connection meets the plan's limit (w6/m456). With internal auth enabled
// (w5/m268) the proxy forwards each client's own AUTH (w6/m463): enabling it
// rolls the proxy, whose old pod drops the connections it authenticated, as
// Render's restart does, and the internal URL carries the password again.
func (r *KeyValueReconciler) reconcileKeyValueInternalRoute(ctx context.Context, kv *appv1alpha1.KeyValue, intent *keyValueIntent) error {
	proxyReady, err := r.reconcileKeyValueInternalProxy(ctx, kv, *intent)
	if err != nil {
		return err
	}
	intent.internalProxy = proxyReady || kv.Status.InternalAuthOptional
	if err := r.reconcileKeyValueService(ctx, kv, *intent); err != nil {
		return err
	}
	kv.Status.InternalAuthOptional = intent.internalProxy && !kv.Spec.InternalAuth
	return nil
}
