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

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const keyValuePersistenceSourceAnnotation = "app.bex.co/persistence-source"
const kvServerContainerName = "valkey"

// The on-volume marker, not the desired template or generation, identifies the
// authoritative disk format. Conversion runs before any network listener can
// accept writes. A retry or superseding generation therefore resumes from the
// last committed format, including when an earlier init container failed.
//
// journal -> snapshot: load the journal, SAVE, shut down, commit snapshot.
// snapshot -> journal: load RDB with AOF disabled, enable AOF, await a successful
// rewrite, shut down (flush), commit journal. Old AOF is never loaded here.
// durable -> off and off -> durable: commit off, discard engine files, commit target.
// same durable mode: retain files; empty legacy mode means journal-snapshot.
const keyValuePersistenceHandoffScript = `set -eu
cd "${DATA_DIR:-/data}"
marker=.bex-persistence-mode
source_mode="$SOURCE_MODE"
if [ -f "$marker" ]; then source_mode="$(cat "$marker")"; fi
case "$TARGET_MODE" in journal-snapshot|snapshot|off) ;; *) exit 1 ;; esac
commit_mode() {
  # Engine files must reach disk before a format marker can become authoritative.
  sync
  printf '%s\n' "$1" > "$marker.tmp"
  mv "$marker.tmp" "$marker"
  sync
}
if [ "$source_mode" = off ] || [ "$TARGET_MODE" = off ]; then
  # Commit the destructive boundary first. If interrupted while removing files,
  # a desired-mode reversal must finish discarding, never load a partial dataset.
  commit_mode off
  rm -rf dump.rdb appendonly.aof appendonlydir
  if [ "$TARGET_MODE" = off ]; then exit 0; fi
  source_mode=new:off
fi
if [ "$source_mode" = unknown ]; then
  echo "persistence source unavailable: retained storage needs its committed mode marker or operator recovery" >&2
  exit 1
fi
case "$source_mode:$TARGET_MODE" in
  new:journal-snapshot:*|new:snapshot:*|new:off:*|journal-snapshot:*|snapshot:*|off:*) ;;
  *) echo "invalid persistence source" >&2; exit 1 ;;
esac
initializing=no
case "$source_mode" in
  new:*)
    # Before the first committed marker no client has served this storage.
    # Retrying a failed initialization may safely replace its incomplete files.
    rm -rf dump.rdb appendonly.aof appendonlydir
    initializing=yes
    source_mode=snapshot
    ;;
  journal-snapshot)
    if [ -e appendonlydir/appendonly.aof.manifest ]; then
      # Valkey accepts a comment-only manifest as an empty dataset. An
      # initialized journal must name at least one active base/increment file.
      grep -Eq '^file[[:space:]].*[[:space:]]type[[:space:]]+[bi]([[:space:]]|$)' appendonlydir/appendonly.aof.manifest || {
        echo "initialized journal manifest has no active files" >&2; exit 1;
      }
    else
      [ -s appendonly.aof ] || {
        echo "initialized journal is missing; refusing to start an empty dataset" >&2; exit 1;
      }
    fi
    ;;
  snapshot)
    [ -s dump.rdb ] || { echo "initialized snapshot is missing" >&2; exit 1; }
    ;;
esac
if [ "$initializing" = no ] && [ "$source_mode" = "$TARGET_MODE" ]; then
  if [ ! -f "$marker" ]; then commit_mode "$TARGET_MODE"; fi
  exit 0
fi
socket=/tmp/bex-persistence.sock
cli() { timeout 5 valkey-cli -e --raw -s "$socket" "$@"; }
cleanup() { cli SHUTDOWN NOSAVE >/dev/null 2>&1 || true; }
trap cleanup EXIT
trap 'exit 1' INT TERM
appendonly=no
if [ "$source_mode" = journal-snapshot ]; then appendonly=yes; fi
# No TCP or TLS listener exists during conversion. No client can acknowledge
# writes until the authoritative marker has been committed and init exits.
valkey-server --dir "$PWD" --port 0 --unixsocket "$socket" --unixsocketperm 600 \
  --appendonly "$appendonly" --save "" \
  --daemonize yes --pidfile /tmp/bex-persistence.pid --logfile /tmp/bex-persistence.log
ready=no
for attempt in $(seq 1 120); do
  if [ "$(cli PING 2>/dev/null || true)" = PONG ]; then ready=yes; break; fi
  sleep 1
done
[ "$ready" = yes ] || { echo "persistence conversion startup timed out" >&2; exit 1; }
if [ "$TARGET_MODE" = journal-snapshot ]; then
  [ "$(cli CONFIG SET appendonly yes)" = OK ] || exit 1
  complete=no
  for attempt in $(seq 1 120); do
    info="$(cli INFO persistence | tr -d '\r')"
    field() { printf '%s\n' "$info" | sed -n "s/^$1://p"; }
    if [ "$(field aof_enabled)" = 1 ] &&
       [ "$(field aof_rewrite_in_progress)" = 0 ] &&
       [ "$(field aof_rewrite_scheduled)" = 0 ] &&
       [ "$(field aof_rewrites)" -ge 1 ] &&
       [ "$(field aof_last_bgrewrite_status)" = ok ] &&
       [ "$(field aof_last_write_status)" = ok ]; then
      complete=yes; break
    fi
    sleep 1
  done
  [ "$complete" = yes ] || { echo "persistence journal rewrite did not complete successfully" >&2; exit 1; }
fi
[ "$(cli SAVE)" = OK ] || { echo "persistence snapshot failed" >&2; exit 1; }
cli SHUTDOWN NOSAVE
trap - EXIT INT TERM
commit_mode "$TARGET_MODE"
`

// Bootstrap from the exact owned running pod, never desired StatefulSet flags:
// a pending legacy rollout can contain journaling flags while the old serving
// pod is still snapshot-only. The PVC marker supersedes this frozen seed.
func (r *KeyValueReconciler) seedKeyValuePersistence(ctx context.Context, kv *appv1alpha1.KeyValue, sts *appsv1.StatefulSet) (string, error) {
	if source := sts.Spec.Template.Annotations[keyValuePersistenceSourceAnnotation]; source != "" {
		return source, nil
	}
	reader := r.APIReader
	if reader == nil {
		reader = r.Client
	}
	pod := &corev1.Pod{}
	// Bootstrap evidence must come from an uncached reader in production.
	err := reader.Get(ctx, client.ObjectKey{Namespace: kv.Namespace, Name: kv.Name + "-0"}, pod)
	if err != nil && !apierrors.IsNotFound(err) {
		return "", err
	}
	if err == nil && sts.UID != "" && metav1.IsControlledBy(pod, sts) && pod.DeletionTimestamp == nil {
		if sts.Status.UpdateRevision != "" && (sts.Status.UpdateRevision != sts.Status.CurrentRevision || pod.Labels[appsv1.ControllerRevisionHashLabelKey] != sts.Status.UpdateRevision) {
			return "", fmt.Errorf("waiting for legacy Valkey rollout to settle before establishing persistence source")
		}
		running := false
		for _, status := range pod.Status.ContainerStatuses {
			if status.Name == kvServerContainerName && status.State.Running != nil {
				running = true
			}
		}
		if running {
			for _, container := range pod.Spec.Containers {
				if container.Name == kvServerContainerName {
					return keyValueContainerPersistence(container), nil
				}
			}
		}
	}
	pvc := &corev1.PersistentVolumeClaim{}
	err = reader.Get(ctx, client.ObjectKey{Namespace: kv.Namespace, Name: keyValuePVCName(kv.Name)}, pvc)
	if err != nil && !apierrors.IsNotFound(err) {
		return "", err
	}
	if apierrors.IsNotFound(err) && sts.ResourceVersion == "" {
		return "new:" + normalizedKeyValuePersistence(kv.Spec.PersistenceMode), nil
	}
	// A retained claim or missing serving pod has no trustworthy legacy seed.
	// Startup may recover from its existing marker, otherwise it fails closed.
	return "unknown", nil
}

func keyValueContainerPersistence(container corev1.Container) string {
	mode := "journal-snapshot"
	for i := 0; i+1 < len(container.Args); i++ {
		if container.Args[i] == "--appendonly" && container.Args[i+1] == "no" {
			mode = "snapshot"
		}
	}
	for i := 0; i+1 < len(container.Args); i++ {
		if container.Args[i] == "--save" && container.Args[i+1] == "" {
			return "off"
		}
	}
	return mode
}

func normalizedKeyValuePersistence(mode string) string {
	if mode == "" {
		return "journal-snapshot"
	}
	return mode
}

func keyValuePersistenceInit(kv *appv1alpha1.KeyValue, intent keyValueIntent, source string) corev1.Container {
	return corev1.Container{
		Name: "persistence-handoff", Image: valkeyImage(kv.Spec.Version),
		Command: []string{shellBinary, "-c", keyValuePersistenceHandoffScript},
		Env: []corev1.EnvVar{
			{Name: "SOURCE_MODE", Value: source},
			{Name: "TARGET_MODE", Value: normalizedKeyValuePersistence(kv.Spec.PersistenceMode)},
		},
		Resources: kvResources(intent.plan), SecurityContext: valkeySecCtx(),
		VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: kvDataPath}},
	}
}
