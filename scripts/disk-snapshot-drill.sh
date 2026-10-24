#!/usr/bin/env bash
# Diagnose, or take, a managed disk's snapshot on the cluster in $KUBECONFIG
# (w6/089). Driven by .github/workflows/disk-snapshot-drill.yml; runnable by
# hand with the same env.
#
#   NS=tea-... SERVICE_ID=srv-... bash scripts/disk-snapshot-drill.sh diagnose
#   NS=tea-... SERVICE_ID=srv-... CONFIRM=srv-... bash scripts/disk-snapshot-drill.sh snapshot
#
# diagnose only reads. snapshot creates one Job from the App's own dskbak-
# CronJob (the operator-authored, admission-validated template) and waits for
# it, printing where its pod ran beside the service's pod — an RWO volume can
# only attach on the node already holding it.
set -euo pipefail

mode="${1:?usage: disk-snapshot-drill.sh diagnose|snapshot}"
: "${NS:?NS (tenant namespace) is required}"
: "${SERVICE_ID:?SERVICE_ID (srv-...) is required}"
[[ "$NS" =~ ^tea-[a-z0-9]+$ ]] || { echo "NS must be a tenant namespace (tea-...)" >&2; exit 1; }
[[ "$SERVICE_ID" =~ ^srv-[a-z0-9]+$ ]] || { echo "SERVICE_ID must be a service id (srv-...)" >&2; exit 1; }

app="$(kubectl -n "$NS" get apps.app.bex.co -l "bex.co/app-id=$SERVICE_ID" -o jsonpath='{.items[0].metadata.name}')"
[ -n "$app" ] || { echo "no App labelled bex.co/app-id=$SERVICE_ID in $NS" >&2; exit 1; }
backup_selector="app.bex.co/app=$app,app.kubernetes.io/name=disk-backup"
cronjob="$(kubectl -n "$NS" get cronjobs -l "$backup_selector" -o jsonpath='{.items[0].metadata.name}')"

section() { printf '\n=== %s\n' "$*"; }

where_pods() {
  kubectl -n "$NS" get pods -l "$1" \
    -o custom-columns='POD:.metadata.name,PHASE:.status.phase,NODE:.spec.nodeName,HOSTUSERS:.spec.hostUsers,STARTED:.status.startTime' 2>/dev/null || true
}

diagnose() {
  section "App $app"
  kubectl -n "$NS" get apps.app.bex.co "$app" \
    -o jsonpath='{range .status.conditions[*]}{.type}={.status} {.reason}: {.message}{"\n"}{end}'
  section "service pods (the volume is attached where these run)"
  where_pods "app.bex.co/app=$app,app.kubernetes.io/name!=disk-backup"
  section "PVC and attachment"
  kubectl -n "$NS" get pvc -l "app.bex.co/app=$app" -o wide 2>/dev/null || true
  for pv in $(kubectl -n "$NS" get pvc -l "app.bex.co/app=$app" -o jsonpath='{.items[*].spec.volumeName}'); do
    kubectl get volumeattachments -o custom-columns='PV:.spec.source.persistentVolumeName,NODE:.spec.nodeName,ATTACHED:.status.attached' \
      | awk -v pv="$pv" 'NR==1 || $1==pv'
  done
  section "backup CronJob"
  if [ -z "$cronjob" ]; then
    echo "no disk-backup CronJob for $app (the operator did not schedule one)"
  else
    kubectl -n "$NS" get cronjob "$cronjob" \
      -o custom-columns='NAME:.metadata.name,SCHEDULE:.spec.schedule,TZ:.spec.timeZone,SUSPEND:.spec.suspend,LAST:.status.lastScheduleTime,LAST_OK:.status.lastSuccessfulTime'
  fi
  section "backup Jobs"
  kubectl -n "$NS" get jobs -l "$backup_selector" \
    -o custom-columns='JOB:.metadata.name,START:.status.startTime,DONE:.status.completionTime,OK:.status.succeeded,FAILED:.status.failed,REASON:.status.conditions[-1].reason' 2>/dev/null || true
  section "backup pods"
  where_pods "$backup_selector"
  section "recent events for the backup and the volume"
  kubectl -n "$NS" get events --sort-by=.lastTimestamp \
    -o custom-columns='TIME:.lastTimestamp,KIND:.involvedObject.kind,OBJECT:.involvedObject.name,REASON:.reason,MESSAGE:.message' \
    | grep -E "TIME|dskbak|disk-|$app" | tail -40 || true
  section "latest backup pod log (tail)"
  latest="$(kubectl -n "$NS" get pods -l "$backup_selector" --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1:].metadata.name}' 2>/dev/null || true)"
  if [ -n "$latest" ]; then
    kubectl -n "$NS" logs "$latest" --tail=40 2>&1 || true
  else
    echo "(no backup pod exists — Jobs are kept only briefly, so a missed night may leave none)"
  fi
}

snapshot() {
  [ "${CONFIRM:-}" = "$SERVICE_ID" ] || { echo "CONFIRM must exactly match SERVICE_ID to run a snapshot" >&2; exit 1; }
  [ -n "$cronjob" ] || { echo "no disk-backup CronJob to run" >&2; exit 1; }
  job="${cronjob:0:40}-drill-$(date -u +%m%d%H%M)"
  section "creating $job from cronjob/$cronjob"
  kubectl -n "$NS" create job "$job" --from="cronjob/$cronjob"
  # Poll rather than wait on one condition: a failing Job retries quickly and
  # its pods do not outlive it, so each failed pod's exit and log are printed
  # the moment it fails. 90m covers a stuck attach, short of the 6h deadline.
  local seen="" deadline=$((SECONDS + 5400)) state=""
  while [ "$SECONDS" -lt "$deadline" ]; do
    state="$(kubectl -n "$NS" get "job/$job" -o jsonpath='{range .status.conditions[?(@.status=="True")]}{.type} {end}')"
    for pod in $(kubectl -n "$NS" get pods -l "job-name=$job" -o jsonpath='{range .items[?(@.status.phase=="Failed")]}{.metadata.name} {end}'); do
      case " $seen " in *" $pod "*) continue ;; esac
      seen="$seen $pod"
      section "pod $pod failed"
      kubectl -n "$NS" get pod "$pod" -o jsonpath='node={.spec.nodeName}{"\n"}{range .status.containerStatuses[*]}{.name}: exit={.state.terminated.exitCode} reason={.state.terminated.reason}{"\n"}message={.state.terminated.message}{"\n"}{end}'
      kubectl -n "$NS" get events --field-selector "involvedObject.name=$pod" \
        -o custom-columns='TIME:.lastTimestamp,REASON:.reason,MESSAGE:.message' 2>/dev/null | tail -15 || true
      kubectl -n "$NS" logs "$pod" --tail=60 2>&1 || true
    done
    case "$state" in
      *Complete*) break ;;
      *Failed*)
        section "job/$job failed"
        kubectl -n "$NS" describe "job/$job" | tail -15
        exit 1 ;;
    esac
    sleep 5
  done
  case "$state" in *Complete*) ;; *)
    section "job/$job still running after 90m"
    where_pods "job-name=$job"
    kubectl -n "$NS" get events --sort-by=.lastTimestamp | grep "$job" | tail -20 || true
    exit 1 ;;
  esac
  section "job/$job completed"
  where_pods "job-name=$job"
}

case "$mode" in
  diagnose) diagnose ;;
  snapshot) snapshot ;;
  *) echo "unknown mode: $mode" >&2; exit 2 ;;
esac
