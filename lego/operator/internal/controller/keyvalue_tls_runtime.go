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
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A store with the runtime TLS switch (w8/m134) turns its TLS listener on and
// off without a restart, so no external-access change ever restarts it:
//
//   - Its pod mounts the certificate Secret optional: true from birth, so the
//     pod starts whether or not a Certificate was ever requested.
//   - Its server starts through kvTLSWrapperScript, which adds the listener
//     flags only when the certificate files are present. A pod restarted while
//     the store is public comes back listening from the mounted Secret.
//   - On an enable, once the Secret exists, the operator sets the certificate
//     files and then tls-port with CONFIG SET as the platform user; on a
//     disable it sets tls-port 0. Neither touches the pod template or data.
//
// Older stores keep the static listener in their template (w4/m233) until the
// first enable that would add it, which is the one restart they still take; it
// adopts the runtime switch in the same roll.
const (
	kvTLSMountPath = "/tls"
	kvTLSCertFile  = kvTLSMountPath + "/" + corev1.TLSCertKey
	kvTLSKeyFile   = kvTLSMountPath + "/" + corev1.TLSPrivateKeyKey
	// kvTLSPendingRequeue paces the wait for the Certificate's Secret and for
	// the kubelet to project it into the running pod.
	kvTLSPendingRequeue = 5 * time.Second
)

// valkeyTLSConfig is the listener's configuration, in the order a runtime
// enable applies it: the files first, then the port that opens the listener.
// It is both the startup flags and the CONFIG SET pairs, so the two cannot
// drift.
func valkeyTLSConfig() [][2]string {
	return [][2]string{
		{"tls-cert-file", kvTLSCertFile},
		{"tls-key-file", kvTLSKeyFile},
		{"tls-ca-cert-file", kvTLSCertFile},
		{"tls-auth-clients", "no"},
		{"tls-port", strconv.Itoa(kvTLSPort)},
	}
}

// valkeyTLSArgs renders valkeyTLSConfig as valkey-server flags.
func valkeyTLSArgs() []string {
	config := valkeyTLSConfig()
	args := make([]string, 0, 2*len(config))
	for _, setting := range config {
		args = append(args, "--"+setting[0], setting[1])
	}
	return args
}

// kvTLSWrapperScript starts valkey-server with the TLS listener only when the
// optionally mounted certificate is present. It runs as `sh -c <script>
// valkey-server <server args...>` through the image's own entrypoint (which
// sets the umask), so the server args, ACL users included, stay verbatim in
// the container's args. It holds no $(...) so Kubernetes' $(VAR) expansion
// leaves it alone.
var kvTLSWrapperScript = `if [ -s ` + kvTLSCertFile + ` ] && [ -s ` + kvTLSKeyFile + ` ]; then set -- "$@" ` +
	strings.Join(valkeyTLSArgs(), " ") + `; fi
exec valkey-server "$@"`

// keyValueTLSVolume returns the certificate volume among volumes, if any.
func keyValueTLSVolume(volumes []corev1.Volume) *corev1.Volume {
	for i := range volumes {
		if volumes[i].Name == kvTLSVolume {
			return &volumes[i]
		}
	}
	return nil
}

// keyValueVolumesSwitchTLS reports whether volumes mount the certificate
// optional, which only the runtime-switch layout does.
func keyValueVolumesSwitchTLS(volumes []corev1.Volume) bool {
	v := keyValueTLSVolume(volumes)
	return v != nil && v.Secret != nil && v.Secret.Optional != nil && *v.Secret.Optional
}

// keyValueTLSRuntime decides the layout: a new StatefulSet, one already
// switching at runtime, and an older store's first enable (which would add the
// static listener and restart anyway) all use the runtime switch. Any other
// older store keeps its template, so the operator upgrade restarts nothing.
func keyValueTLSRuntime(kv *appv1alpha1.KeyValue, sts *appsv1.StatefulSet, kvDomain string, listening bool) bool {
	return sts.ResourceVersion == "" || keyValueVolumesSwitchTLS(sts.Spec.Template.Spec.Volumes) ||
		kv.Status.TLSRuntimeSwitch || (kv.Spec.Public && kvDomain != "" && !listening)
}

// reconcileKeyValueTLSListener brings the running process's TLS listener to
// the store's external access and reports whether it listens, plus whether a
// switch is still pending. It never fails the store: a store that cannot open
// its listener yet still serves internally, and the next pass retries.
func (r *KeyValueReconciler) reconcileKeyValueTLSListener(ctx context.Context, kv *appv1alpha1.KeyValue, sts *appsv1.StatefulSet, platform *corev1.Secret, intent keyValueIntent) (listening, pending bool) {
	if !intent.tlsRuntime {
		return intent.tlsListener, false
	}
	if intent.replicas == 0 {
		return false, false
	}
	reader := cmp.Or(r.APIReader, client.Reader(r.Client))
	pod := &corev1.Pod{}
	if err := reader.Get(ctx, client.ObjectKey{Namespace: kv.Namespace, Name: kv.Name + "-0"}, pod); err != nil ||
		!metav1.IsControlledBy(pod, sts) || !pod.DeletionTimestamp.IsZero() || pod.Status.PodIP == "" || !podReady(pod) ||
		!keyValueVolumesSwitchTLS(pod.Spec.Volumes) {
		// No serving process with the switch yet (still starting, or an older
		// pod mid-roll); the pod watch brings the next pass.
		return false, intent.public
	}
	if intent.public {
		// The listener can open only once cert-manager has written the Secret
		// (and the kubelet has projected it, which CONFIG SET tls-port checks).
		secret := &corev1.Secret{}
		if err := r.secretClient().Get(ctx, client.ObjectKey{Namespace: kv.Namespace, Name: intent.tlsSecretName}, secret); err != nil ||
			len(secret.Data[corev1.TLSCertKey]) == 0 || len(secret.Data[corev1.TLSPrivateKeyKey]) == 0 {
			return false, true
		}
	}
	switchTLS := r.switchTLS
	if switchTLS == nil {
		switchTLS = switchValkeyTLSAtPod
	}
	listening, err := switchTLS(ctx, pod, platform, intent.public)
	if err != nil {
		// The error can name the pod address; it goes to the log, not status.
		logf.FromContext(ctx).Info("Valkey TLS listener switch did not complete; retrying", "name", kv.Name, "on", intent.public, "error", err.Error())
		return kv.Status.TLSListener, true
	}
	return listening, listening != intent.public
}

// switchValkeyTLSAtPod runs switchValkeyTLS as kvPlatformUser: CONFIG SET is
// an @admin verb the tenant's default user cannot run.
func switchValkeyTLSAtPod(ctx context.Context, pod *corev1.Pod, platform *corev1.Secret, on bool) (bool, error) {
	timeout, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	c := platformValkeyClient(net.JoinHostPort(pod.Status.PodIP, strconv.Itoa(kvPort)), platform)
	defer func() { _ = c.Close() }()
	return switchValkeyTLS(timeout, c, on)
}

// switchValkeyTLS turns the TLS listener on or off in the running process and
// reports whether it listens afterwards. Enabling sets the certificate files
// first, then tls-port: Valkey loads the certificate when the port opens, and
// refuses (keeping tls-port 0) while the files are missing. tls-port 0 closes
// the listener. Neither changes the keyspace.
func switchValkeyTLS(ctx context.Context, c *redis.Client, on bool) (bool, error) {
	current, err := c.ConfigGet(ctx, "tls-port").Result()
	if err != nil {
		return false, err
	}
	listening := current["tls-port"] != "" && current["tls-port"] != "0"
	if listening == on {
		return listening, nil
	}
	if !on {
		if err := c.ConfigSet(ctx, "tls-port", "0").Err(); err != nil {
			return true, err
		}
		return false, nil
	}
	config := valkeyTLSConfig()
	files := []any{"CONFIG", "SET"}
	for _, setting := range config[:len(config)-1] {
		files = append(files, setting[0], setting[1])
	}
	if err := c.Do(ctx, files...).Err(); err != nil {
		return false, err
	}
	port := config[len(config)-1]
	if err := c.ConfigSet(ctx, port[0], port[1]).Err(); err != nil {
		return false, err
	}
	return true, nil
}
