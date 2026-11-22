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
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

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
// Every store has one: it holds the plan's connection limit (w6/m456), and it
// authenticates internal clients unless the store requires them to
// authenticate themselves (spec.internalAuth, w6/m463). Its pod template moves
// only when the store's own change rolls it (keyValueInternalProxyHolds,
// w6/m517); replicas, strategy and selector are rendered on every pass.
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
	inputs := keyValueInternalProxyInputs(kv, intent)
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: keyValueInternalProxyName(kv), Namespace: kv.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		before := dep.Spec.Template.DeepCopy()
		ran := dep.Spec.Replicas == nil || *dep.Spec.Replicas > 0
		replicas := int32(1)
		if kv.Spec.Suspended {
			replicas = 0
		}
		held := keyValueInternalProxyHolds(dep, ran && replicas > 0, inputs)
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
				// The external clients the store's allow list refuses read
				// Render's refusal inside TLS with the same certificate, from a
				// listener that dials nothing (w8/m562).
				corev1.EnvVar{Name: "BEX_KV_ALLOWLIST_REFUSAL_ADDR", Value: fmt.Sprintf(":%d", KeyValueAllowListRefusalPort)},
			)
			ports = append(ports,
				corev1.ContainerPort{Name: "valkey-tls", ContainerPort: kvTLSPort},
				corev1.ContainerPort{Name: kvAllowListRefusalPortName, ContainerPort: KeyValueAllowListRefusalPort})
			mounts = []corev1.VolumeMount{{Name: kvTLSVolume, MountPath: kvTLSMountPath, ReadOnly: true}}
			volumes = []corev1.Volume{{Name: kvTLSVolume, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
				SecretName: intent.tlsSecretName, Optional: new(true),
			}}}}
		}
		dep.Spec.Template.Spec.Volumes = volumes
		dep.Spec.Template.Spec.Containers = []corev1.Container{{
			Name: kvInternalProxyContainer, Image: r.InternalProxyImage,
			Command: []string{"/kv-internal-proxy"}, Env: env, Ports: ports, VolumeMounts: mounts,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("25m"), corev1.ResourceMemory: resource.MustParse("16Mi")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("250m"), corev1.ResourceMemory: kvInternalProxyMemoryLimit(intent.plan)},
			}, SecurityContext: execution.RestrictedContainerSecurityContext(),
			ReadinessProbe: &corev1.Probe{
				ProbeHandler:   corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{"/kv-internal-proxy", "--check"}}},
				TimeoutSeconds: 6, PeriodSeconds: 5,
			},
			Lifecycle: &corev1.Lifecycle{PreStop: &corev1.LifecycleHandler{
				Sleep: &corev1.SleepAction{Seconds: kvInternalProxyPauseSeconds},
			}},
		}}
		dep.Spec.Template.Spec.TerminationGracePeriodSeconds = new(int64(kvInternalProxyGraceSeconds))
		dep.Spec.Template.Spec.AutomountServiceAccountToken = new(false)
		applyPodSpecServerDefaults(&dep.Spec.Template.Spec)
		holdDatastorePriority(&dep.Spec.Template, !dep.CreationTimestamp.IsZero(), before)
		// An isolation toggle alone must not roll the one proxy replica: its pods
		// are relabelled in place (syncKeyValuePodIsolation, w8/220).
		holdIsolationLabel(&dep.Spec.Template, !dep.CreationTimestamp.IsZero(), before, kv.Labels)
		if held {
			holdKeyValueInternalProxyTemplate(ctx, kv, dep, before)
		}
		recordKeyValueInternalProxyInputs(dep, inputs)
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
	// kvInternalProxyPauseSeconds is how long a deleted proxy pod keeps
	// accepting before its SIGTERM starts the drain (the preStop sleep,
	// w9/135). Its endpoint takes a moment to leave every node's service
	// rules, and a client routed to it meanwhile is served instead of
	// refused. The SNI proxies' pause is the same 5 s (w6/m515).
	kvInternalProxyPauseSeconds = 5
	// kvInternalProxyGraceSeconds covers the pause and the drain, and lets the
	// process close what the drain leaves itself, before the kubelet's SIGKILL.
	kvInternalProxyGraceSeconds = kvInternalProxyPauseSeconds + kvInternalProxyDrainSeconds + 5
	// kvInternalProxyInputsAnnotation records, on a proxy Deployment's own
	// metadata, the digest of the store's roll inputs its pod template was
	// written for (keyValueInternalProxyInputs, w6/m517). Never on the pod
	// template: adding it there would roll every proxy once.
	kvInternalProxyInputsAnnotation = "app.bex.co/kv-internal-proxy-inputs"
	// kvInternalProxyRollRevisionAnnotation records the
	// kvInternalProxyRollRevision its pod template was written under.
	kvInternalProxyRollRevisionAnnotation = "app.bex.co/kv-internal-proxy-roll-revision"
	// kvInternalProxyRollRevision is the override (w6/m517). A proxy whose
	// Deployment records a lower revision takes the current rendering on the
	// next pass, so bumping it rolls every store's proxy once, closing its
	// held clients after the drain. Bump it in a reviewed commit only for a
	// security fix in /kv-internal-proxy or its template, or when a held
	// template can no longer work: one that relies on something the operator
	// renamed or removed (the -backend Service or its ports, the auth or TLS
	// Secret, the pod labels the NetworkPolicy and the internal-auth cut
	// select, the --check command). Never lower it.
	// TestKeyValueInternalProxyRenderingIsPinned makes every rendering change
	// decide.
	kvInternalProxyRollRevision = 1
)

// keyValueInternalProxyHolds reports whether a proxy's pod template stays as
// it runs on this pass (w6/m517). Replacing the proxy's pod closes every
// client it relays, after the w6/m512 drain. Render breaks a store's
// connections when the customer changes the store, not when Render ships its
// own platform (ADR021 § 10). So the template stays exactly as it runs,
// whatever the manager's image, binary or rendering, until one of these moves
// it onto the manager's image and the full current rendering, in one write:
//
//   - the Deployment is new;
//   - it runs no pod before or after this pass (a suspension or a resume);
//   - it records no roll inputs (written before w6/m517): a template already
//     equal to the rendering stays as it is, and any other rolls once;
//   - the store's roll inputs (keyValueInternalProxyInputs) changed since the
//     template was written;
//   - its recorded override revision is below kvInternalProxyRollRevision,
//     or unreadable.
//
// running is whether the proxy runs a pod both before and after this pass.
func keyValueInternalProxyHolds(dep *appsv1.Deployment, running bool, inputs string) bool {
	revision, err := strconv.Atoi(dep.Annotations[kvInternalProxyRollRevisionAnnotation])
	return dep.ResourceVersion != "" && running &&
		dep.Annotations[kvInternalProxyInputsAnnotation] == inputs &&
		err == nil && revision >= kvInternalProxyRollRevision
}

// keyValueInternalProxyInputs digests what a store's own changes write into
// its proxy's pod template (w6/m517):
//
//   - the plan and its connection limit (BEX_KV_MAX_CONNECTIONS, the memory
//     limit). The limit is the store's, which Valkey's maxclients follows at
//     runtime, so the proxy's budget must follow it too;
//   - the internal-auth mode (BEX_KV_INTERNAL_AUTH);
//   - the credential: its revision (the pod annotation) and its Secret;
//   - whether the TLS port is forwarded, and from which Secret: making an
//     older store with a static listener public (a runtime-switch store
//     forwards it always, ADR021 § 3).
//
// A suspension needs no entry: a proxy that runs no pod takes the rendering
// anyway. Each input is one "name=value" line, so the digest never depends on
// how Go formats a struct. Changing the lines changes every store's digest,
// which rolls every proxy as an override bump does.
func keyValueInternalProxyInputs(kv *appv1alpha1.KeyValue, intent keyValueIntent) string {
	lines := []string{
		"plan=" + intent.plan.ID,
		"connectionLimit=" + strconv.Itoa(int(intent.plan.ConnectionLimit)),
		"internalAuth=" + strconv.FormatBool(kv.Spec.InternalAuth),
		"credentialRevision=" + intent.credentialRevision,
		"authSecret=" + intent.authSecretName,
		"tlsForwarded=" + strconv.FormatBool(intent.tlsForwarded()),
	}
	if intent.tlsForwarded() {
		lines = append(lines, "tlsSecret="+intent.tlsSecretName)
	}
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

// recordKeyValueInternalProxyInputs records the inputs and the override
// revision the Deployment's pod template now stands for. A higher revision,
// left by a newer operator before a rollback, stays, so moving forward again
// rolls nothing.
func recordKeyValueInternalProxyInputs(dep *appsv1.Deployment, inputs string) {
	metav1.SetMetaDataAnnotation(&dep.ObjectMeta, kvInternalProxyInputsAnnotation, inputs)
	if recorded, err := strconv.Atoi(dep.Annotations[kvInternalProxyRollRevisionAnnotation]); err != nil || recorded < kvInternalProxyRollRevision {
		metav1.SetMetaDataAnnotation(&dep.ObjectMeta, kvInternalProxyRollRevisionAnnotation, strconv.Itoa(kvInternalProxyRollRevision))
	}
}

// kvProxyHeldLogged holds, for this manager process, each store whose proxy
// was seen keeping a template other than the current rendering.
var kvProxyHeldLogged sync.Map

// holdKeyValueInternalProxyTemplate puts back the template the proxy runs
// (before) in place of the current rendering. When the two differ it says so
// once per store and manager process; the store's next own change, or an
// override bump, brings the rendering.
func holdKeyValueInternalProxyTemplate(ctx context.Context, kv *appv1alpha1.KeyValue, dep *appsv1.Deployment, before *corev1.PodTemplateSpec) {
	if !equality.Semantic.DeepEqual(&dep.Spec.Template, before) {
		if _, logged := kvProxyHeldLogged.LoadOrStore(kv.Namespace+"/"+kv.Name, true); !logged {
			logf.FromContext(ctx).Info("the Key Value proxy keeps the template it runs until the store's own change rolls it",
				"namespace", kv.Namespace, "name", kv.Name)
		}
	}
	dep.Spec.Template = *before
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

// routeKeyValueFromBirth decides, before the credential Secrets are written,
// whether this pass creates the store's stable Service. A store whose Service
// does not exist yet has no client connected straight to Valkey, so its
// Service selects the internal proxy from birth and its internal URL is
// credential-free from the first read, as Render's is (w9/m521). Until the
// proxy is Ready the Service has no endpoints, as while any store is still
// creating, and readiness waits for the proxy. A store whose Service already
// exists keeps the w8/204 cutover, which waits for the Ready proxy before it
// moves the Service. A missing Service is confirmed uncached: a lagging cache
// must never take a running store's direct Service for a missing one.
func (r *KeyValueReconciler) routeKeyValueFromBirth(ctx context.Context, kv *appv1alpha1.KeyValue, intent *keyValueIntent) error {
	if r.InternalProxyImage == "" {
		return nil
	}
	key := client.ObjectKeyFromObject(kv)
	if err := r.Get(ctx, key, &corev1.Service{}); !apierrors.IsNotFound(err) {
		return err
	}
	if err := cmp.Or(r.APIReader, client.Reader(r.Client)).Get(ctx, key, &corev1.Service{}); !apierrors.IsNotFound(err) {
		return err
	}
	intent.internalProxy = true
	if !kv.Spec.InternalAuth {
		kv.Status.InternalAuthOptional = true
	}
	return nil
}

// reconcileKeyValueInternalRoute points the stable Service at the store's
// internal proxy once it is ready (w8/204), or from birth for a new store
// (routeKeyValueFromBirth), and keeps it there, so every tenant connection
// meets the plan's limit (w6/m456). With internal auth enabled (w5/m268) the
// proxy forwards each client's own AUTH (w6/m463): enabling it rolls the
// proxy, whose old pod drops the connections it authenticated, as Render's
// restart does, and the internal URL carries the password again.
func (r *KeyValueReconciler) reconcileKeyValueInternalRoute(ctx context.Context, kv *appv1alpha1.KeyValue, intent *keyValueIntent) error {
	proxyReady, err := r.reconcileKeyValueInternalProxy(ctx, kv, *intent)
	if err != nil {
		return err
	}
	intent.internalProxy = intent.internalProxy || proxyReady || kv.Status.InternalAuthOptional
	if err := r.reconcileKeyValueService(ctx, kv, *intent); err != nil {
		return err
	}
	kv.Status.InternalAuthOptional = intent.internalProxy && !kv.Spec.InternalAuth
	return nil
}
