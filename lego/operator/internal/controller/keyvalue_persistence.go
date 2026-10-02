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

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const keyValueContainerName = "valkey"
const keyValueSnapshotMode = "snapshot"
const keyValueOffMode = "off"

const keyValuePersistenceSourceAnnotation = "app.bex.co/persistence-source"
const keyValuePersistenceTokenAnnotation = "app.bex.co/persistence-handoff"

func keyValuePersistenceMode(mode string) string {
	if mode == "" {
		return "journal-snapshot"
	}
	return mode
}

func keyValuePersistenceSource(sts *appsv1.StatefulSet, desired string) string {
	if source := sts.Spec.Template.Annotations[keyValuePersistenceSourceAnnotation]; source != "" {
		return source
	}
	return keyValueTemplatePersistenceMode(sts, desired)
}

func keyValueTemplatePersistenceMode(sts *appsv1.StatefulSet, desired string) string {
	return keyValueContainersPersistenceMode(sts.Spec.Template.Spec.Containers, desired)
}

func keyValueContainersPersistenceMode(containers []corev1.Container, desired string) string {
	for _, c := range containers {
		if c.Name != keyValueContainerName {
			continue
		}
		mode := "journal-snapshot"
		for i := 0; i+1 < len(c.Args); i++ {
			if c.Args[i] == "--appendonly" && c.Args[i+1] == "no" {
				mode = keyValueSnapshotMode
			}
			if c.Args[i] == "--save" && c.Args[i+1] == "" {
				return keyValueOffMode
			}
		}
		return mode
	}
	return keyValuePersistenceMode(desired)
}

func keyValuePersistenceInit(kv *appv1alpha1.KeyValue, intent keyValueIntent, source string) corev1.Container {
	return corev1.Container{
		Name: "persistence-handoff", Image: valkeyImage(kv.Spec.Version),
		Command: []string{shellBinary, "-ceu", keyValuePersistenceScript},
		Env: []corev1.EnvVar{
			{Name: "PERSISTENCE_SOURCE", Value: source},
			{Name: "PERSISTENCE_TOKEN", Value: intent.persistenceToken},
			{Name: "PERSISTENCE_TARGET", Value: keyValuePersistenceMode(kv.Spec.PersistenceMode)},
			{Name: "VALKEY_PASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: intent.authSecretName}, Key: "password",
			}}},
		},
		Resources: kvResources(intent.plan), SecurityContext: valkeySecCtx(),
		VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: kvDataPath}},
	}
}

// The initializer has exclusive access to the PVC after the old server stopped.
// A PVC marker identifies the authoritative format, independent of generation,
// controller restarts and suspended template edits. Only commit the new format
// after Valkey successfully saved it and stopped. Retries use the old format.
// Off deliberately commits the discard intent BEFORE deleting files, so even
// an interrupted deletion followed by a mode reversal cannot resurrect data.
const keyValuePersistenceScript = `
set -eu
cd /data
marker=.bex-persistence-mode
source=$PERSISTENCE_SOURCE
target=$PERSISTENCE_TARGET
token=${PERSISTENCE_TOKEN:-}
consumed=
new=no
if [ -f "$marker" ]; then
 source=$(sed -n 1p "$marker")
 consumed=$(sed -n 2p "$marker")
else
 # Only the controller's absent-StatefulSet + absent-PVC path mints this.
 # A lost marker on a later populated volume must never authorize bootstrap.
 case "$source" in new:*)
  if [ "$target" != off ] && { [ -e dump.rdb ] || [ -e appendonly.aof ] || [ -e appendonlydir ]; }; then
   echo "Bootstrap marker missing on populated volume" >&2; exit 1
  fi
 ;; esac
fi
# A pending bootstrap marker proves no main process has served this volume.
case "$source" in new:*) new=yes; source=snapshot;; esac
require_journal() {
 if [ -f appendonlydir/appendonly.aof.manifest ]; then
  grep -Eq '^file[[:space:]].*[[:space:]]type[[:space:]]+[bi]([[:space:]]|$)' appendonlydir/appendonly.aof.manifest || {
   echo "Initialized journal manifest has no active files" >&2; exit 1
  }
 else
  [ -s appendonly.aof ] || { echo "Source journal missing; refusing empty conversion" >&2; exit 1; }
 fi
}
# Explicit Off is a destructive reset; it needs no guess of the old format.
if [ "$target" = off ]; then source=off; fi
if [ "$target" != off ] && [ -n "$token" ] && [ "$token" != "$consumed" ]; then
 source=journal-snapshot
 new=no
 require_journal
fi
case "$source:$target" in
  *[!a-z:-]*) echo "Invalid persistence marker" >&2; exit 1;;
esac
for mode in "$source" "$target"; do
 case "$mode" in journal-snapshot|snapshot|off) ;; *) echo "Unknown persistence mode" >&2; exit 1;; esac
done
commit_mode() {
 printf '%s\n%s\n' "$1" "$token" > "$marker.tmp"
 sync "$marker.tmp"
 mv "$marker.tmp" "$marker"
 sync .
}
if [ "$new" = yes ] && [ ! -f "$marker" ]; then commit_mode "new:$target"; fi
if [ "$target" = off ] || [ "$source" = off ]; then
 commit_mode off
 # These are the exact managed Valkey defaults, never glob the volume.
 rm -rf -- dump.rdb appendonly.aof appendonlydir
 sync .
 if [ "$target" = off ]; then exit 0; fi
 # Off's discard intent remains authoritative across interrupted bootstrap.
 source=snapshot
 new=yes
fi
if [ "$new" = no ]; then
 if [ "$source" = snapshot ]; then
  [ -s dump.rdb ] || { echo "Source snapshot missing; refusing empty conversion" >&2; exit 1; }
 else
  require_journal
 fi
 if [ "$source" = "$target" ]; then
  if [ -f "$marker" ] && [ "$token" = "$consumed" ]; then exit 0; fi
  commit_mode "$source"
  exit 0
 fi
 commit_mode "$source"
fi
export REDISCLI_AUTH="$VALKEY_PASSWORD"
socket=/tmp/bex-persistence.sock
appendonly=no
if [ "$source" = journal-snapshot ]; then appendonly=yes; fi
# Valkey quoted config escaping keeps the secret out of argv and logs.
umask 077
config=$(mktemp /tmp/bex-persistence.XXXXXX)
escaped=$(printf '%s' "$VALKEY_PASSWORD" | sed 's/\\/\\\\/g; s/"/\\"/g')
printf 'requirepass "%s"\n' "$escaped" > "$config"
valkey-server "$config" --dir /data --port 0 --unixsocket "$socket" --unixsocketperm 600 \
 --appendonly "$appendonly" --save "" &
pid=$!
# No client writes reach this process, so failure kills it without SAVE.
# SIGTERM can be refused while the first AOF rewrite is running.
trap 'kill -KILL "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; rm -f "$config" "$socket"' EXIT
trap 'exit 1' INT TERM
cli() { timeout 60 valkey-cli -s "$socket" --raw "$@"; }
ready=no
deadline=$(($(date +%s) + 120))
while [ "$(date +%s)" -lt "$deadline" ]; do
 kill -0 "$pid" 2>/dev/null || { echo "Persistence source failed to load" >&2; exit 1; }
 if [ "$(timeout 2 valkey-cli -s "$socket" --raw PING 2>/dev/null || true)" = PONG ]; then ready=yes; break; fi
 sleep 1
done
[ "$ready" = yes ] || { echo "Persistence source load timed out" >&2; exit 1; }
if [ "$target" = journal-snapshot ]; then
 [ "$(cli CONFIG SET appendonly yes)" = OK ] || { echo "Cannot enable journal" >&2; exit 1; }
 complete=no
 deadline=$(($(date +%s) + 300))
 while [ "$(date +%s)" -lt "$deadline" ]; do
  state=$(cli INFO persistence)
  state=$(printf '%s\n' "$state" | tr -d '\r')
  field() { printf '%s\n' "$state" | awk -F: -v key="$1" '$1 == key {print $2}'; }
  if [ "$(field aof_enabled)" = 1 ] && [ "$(field aof_rewrite_in_progress)" = 0 ] && \
     [ "$(field aof_rewrite_scheduled)" = 0 ] && [ "$(field aof_rewrites)" -ge 1 ]; then
   [ "$(field aof_last_bgrewrite_status)" = ok ] && [ "$(field aof_last_write_status)" = ok ] || \
    { echo "Journal rewrite failed; source snapshot retained" >&2; exit 1; }
   complete=yes
   break
  fi
  sleep 1
 done
 [ "$complete" = yes ] || { echo "Journal rewrite timed out; source snapshot retained" >&2; exit 1; }
fi
# SAVE errors (including full disk) must never advance the marker. The source
# AOF remains authoritative until a completed SAVE and clean process exit.
[ "$(cli SAVE)" = OK ] || { echo "Persistence snapshot failed" >&2; exit 1; }
cli SHUTDOWN NOSAVE
wait "$pid"
trap - EXIT INT TERM
rm -f "$config" "$socket"
commit_mode "$target"
`

// keyValuePersistenceProgress surfaces bounded, non-secret initializer state.
func (r *KeyValueReconciler) keyValuePersistenceProgress(ctx context.Context, kv *appv1alpha1.KeyValue, sts *appsv1.StatefulSet) (string, string) {
	pod := &corev1.Pod{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: kv.Namespace, Name: kv.Name + "-0"}, pod); err != nil || !metav1.IsControlledBy(pod, sts) {
		return "", ""
	}
	for _, status := range pod.Status.InitContainerStatuses {
		if status.Name != "persistence-handoff" {
			continue
		}
		if (status.State.Terminated != nil && status.State.Terminated.ExitCode != 0) ||
			(status.State.Waiting != nil && status.State.Waiting.Reason == "CrashLoopBackOff") {
			return "PersistenceTransitionFailed", "persistence conversion failed; the source format is retained and initialization will retry"
		}
		if status.State.Terminated == nil {
			return "PersistenceTransition", "converting durable data before starting Valkey"
		}
	}
	return "", ""
}
