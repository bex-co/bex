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
	"errors"
	"strings"

	"github.com/redis/go-redis/v9"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Every Render credential string names the store's id as its user
// (rediss://red-…:<password>@…, --user red-…). A bex store's tenant ACL has a
// second user named after its id, with the default user's permissions and
// password (w9/m521, ADR021 § Internal authentication):
//
//   - a new template starts the server with it (valkeyArgs);
//   - a template started before it keeps running without it, so the operator
//     upgrade restarts no store, and the operator adds the user to that
//     server with ACL SETUSER as kvPlatformUser, again after every restart
//     (annotKVUsername);
//   - status.username names it once every server the store runs knows it,
//     and only then do the connection strings move from default to the id.
//
// The default user keeps working: the proxy's injected AUTH, the readiness
// probe and any client that copied an earlier string use it.

// annotKVUsername records on the StatefulSet the credential revision and the
// server process (persistenceProcess) ACL SETUSER gave the store's id user to,
// so a settled store is not dialed again until its server restarts. Not on the
// pod: the operator may change no pod annotation (annotKVMaxclients).
const annotKVUsername = "app.bex.co/valkey-username"

// reconcileKeyValueUsername brings the store's id user to the server it runs
// and records status.username once every server knows it. It reports whether
// a runtime add is still pending. It never fails the store: until the user
// holds, the strings keep naming default, which every server knows.
func (r *KeyValueReconciler) reconcileKeyValueUsername(ctx context.Context, kv *appv1alpha1.KeyValue, sts *appsv1.StatefulSet, platform *corev1.Secret, intent keyValueIntent) (pending bool) {
	templated := valkeyHasUser(sts.Spec.Template.Spec.Containers, kv.Name)
	pod := &corev1.Pod{}
	err := r.Get(ctx, client.ObjectKey{Namespace: kv.Namespace, Name: kv.Name + "-0"}, pod)
	running := err == nil && metav1.IsControlledBy(pod, sts) && pod.DeletionTimestamp.IsZero() && persistenceProcess(pod) != ""
	switch {
	case !running:
		// No server runs now. One that starts from a template carrying the
		// user knows it; a held template's next server is added to when it
		// runs (the pod watch brings that pass).
		if templated {
			kv.Status.Username = kv.Name
		}
		return false
	case valkeyHasUser(pod.Spec.Containers, kv.Name):
		kv.Status.Username = kv.Name
		return false
	case !valkeyHasPlatformUser(pod.Spec.Containers) || pod.Status.PodIP == "":
		// A server started before the platform user cannot be changed at
		// runtime; its strings keep naming default until it restarts.
		return false
	}
	applied := intent.credentialRevision + "/" + persistenceProcess(pod)
	if sts.Annotations[annotKVUsername] == applied {
		kv.Status.Username = kv.Name
		return false
	}
	setUser := r.setValkeyUser
	if setUser == nil {
		setUser = setValkeyUserAtPod
	}
	if err := setUser(ctx, pod, platform, kv.Name, intent.credentialRevision); err != nil {
		// The command carries only the password's hash, and the error can
		// name the pod address; it goes to the log, not status.
		logf.FromContext(ctx).Info("Valkey user add did not complete; retrying", "name", kv.Name, "error", err.Error())
		return true
	}
	patch := client.MergeFrom(sts.DeepCopy())
	if sts.Annotations == nil {
		sts.Annotations = map[string]string{}
	}
	sts.Annotations[annotKVUsername] = applied
	if err := r.Patch(ctx, sts, patch); err != nil {
		// The server knows the user; without the record the next pass adds
		// it again, which changes nothing.
		logf.FromContext(ctx).Info("Valkey user added but not recorded", "name", kv.Name, "error", err.Error())
	}
	kv.Status.Username = kv.Name
	return false
}

// setValkeyUserAtPod runs setValkeyUser as kvPlatformUser: ACL SETUSER is an
// @admin verb no tenant user may run.
func setValkeyUserAtPod(ctx context.Context, pod *corev1.Pod, platform *corev1.Secret, user, credentialRevision string) error {
	_, err := withPlatformValkey(ctx, pod, platform, func(ctx context.Context, c *redis.Client) (struct{}, error) {
		return struct{}{}, setValkeyUser(ctx, c, user, credentialRevision)
	})
	return err
}

// setValkeyUser gives the running server the tenant ACL user with exactly the
// default user's permissions (valkeyTenantUserRule). The password goes as its
// SHA-256 (credentialRevision is "sha256:<hex>", Valkey's #<hex> form), so
// the plaintext never leaves its Secret. reset first, so a user that exists
// with other rules ends with exactly these.
func setValkeyUser(ctx context.Context, c *redis.Client, user, credentialRevision string) error {
	hash, ok := strings.CutPrefix(credentialRevision, "sha256:")
	if !ok || len(hash) != 64 {
		return errUnhashedCredential
	}
	rule := valkeyTenantUserRule(user, "#"+hash)
	args := make([]any, 0, 3+len(rule))
	args = append(args, "ACL", "SETUSER", user, "reset")
	for _, token := range rule[1:] {
		args = append(args, token)
	}
	return c.Do(ctx, args...).Err()
}

var errUnhashedCredential = errors.New("the credential revision is not a SHA-256 hash")
