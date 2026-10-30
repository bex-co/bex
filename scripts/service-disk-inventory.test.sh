#!/usr/bin/env bash
# Every kubectl command is a fake. Unexpected operations or full-object output
# are refused, and only App-owned volume metadata may be requested.
set -euo pipefail
repo_root=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
mkdir -p "$work/bin"
printf 'fake inventory kubeconfig\n' > "$work/test.kubeconfig"
export FIXTURE="$work" KUBECONFIG="$work/test.kubeconfig"
cat > "$work/bin/kubectl" <<'SH'
#!/usr/bin/env bash
set -eu
printf '%s\n' "$*" >> "$FIXTURE/calls"
case "$*" in
  *'spec.env'*|*'exec'*|*' -o json'|*' -o yaml'|'get secret '*|'get secrets '*|*' get secret '*|*' get secrets '*) echo 'forbidden inventory operation' >&2; exit 99 ;;
esac
case "$*" in
  'get apps.app.bex.co --all-namespaces -o jsonpath='*)
    printf 'tea-one\tapp\tapp-current\tsrv-owned\t1\t1\t1\t10\n' ;;
  'get persistentvolumeclaims --all-namespaces -o jsonpath='*)
    printf 'tea-one\tdisk-hash\tclaim-one\tApp/app/app-current\thcloud-volumes-luks\t1Gi\t10Gi\tBound\tpvc-owned\t\n'
    printf 'tea-one\tpg-data\tclaim-db\tStatefulSet/pg/db-id\thcloud-volumes\t10Gi\t10Gi\tBound\tpvc-database\t\n' ;;
  'get persistentvolume pvc-owned --ignore-not-found -o jsonpath='*)
    printf 'pvc-owned\thcloud-volumes-luks\t10Gi\tBound\tDelete\ttea-one\tdisk-hash\tclaim-one\tcsi.hetzner.cloud\t123456\n' ;;
  '-n bex-system get deployment bex-controller-manager bex-api -o jsonpath='*)
    printf 'bex-system\tbex-controller-manager\tmanager-id\t2\t2\t1\t1\t1\t1\tmanager=operator@sha256:one;\thcloud-volumes-luks-bounded\n'
    printf 'bex-system\tbex-api\tapi-id\t2\t2\t1\t1\t1\t1\tapi=operator@sha256:one;\t\n' ;;
  '-n bex-system get pods -l app.kubernetes.io/name in (control-plane,bex-api) -o jsonpath='*)
    printf 'bex-system\tmanager-one\tpod-id\tRunning\t\tmanager=operator@sha256:one;\tmanager=containerd://sha256:one;\tmanager=true;\n' ;;
  'get storageclass hcloud-volumes-luks hcloud-volumes-luks-bounded --ignore-not-found -o jsonpath='*)
    printf 'hcloud-volumes-luks\tlegacy-class-uid\t\tcsi.hetzner.cloud\ttrue\tWaitForFirstConsumer\tDelete\t\t\t${pvc.name}-luks\t${pvc.namespace}\t\t\n'
    if [ "${OMIT_BOUNDED_CLASS:-}" != 1 ]; then
      printf 'hcloud-volumes-luks-bounded\tbounded-class-uid\t\tcsi.hetzner.cloud\ttrue\tWaitForFirstConsumer\tDelete\text4\text4-v1\t${pvc.name}-luks\t${pvc.namespace}\t${pvc.name}-luks\t${pvc.namespace}\n'
    fi ;;
  *) echo 'unexpected inventory operation' >&2; exit 98 ;;
esac
SH
chmod +x "$work/bin/kubectl"
PATH="$work/bin:$PATH" bash "$repo_root/scripts/service-disk-inventory.sh" > "$work/inventory.json"
python3 - <<'PY'
import json,os
from pathlib import Path
report=json.loads((Path(os.environ['FIXTURE'])/'inventory.json').read_text())
app=report['appsWithDisks'][0]
assert app['disk']['requestedSizeGB']==1 and app['disk']['capacityGB']==10
assert app['claims'][0]['bindingMatches'] is True
assert app['claims'][0]['volume']['volumeHandle']=='123456'
assert len(report['platformImages']['deployments'])==2
assert report['platformImages']['deployments'][0]['diskStorageClass']=='hcloud-volumes-luks-bounded'
assert report['platformImages']['deployments'][1]['diskStorageClass']==''
assert report['platformImages']['pods'][0]['containerImageIds']['manager']=='containerd://sha256:one'
legacy,bounded=report['storageClasses']
assert legacy['name']=='hcloud-volumes-luks' and legacy['present'] is True
assert legacy['boundedFilesystem']=='' and legacy['expandSecretName']==''
assert bounded['name']=='hcloud-volumes-luks-bounded' and bounded['present'] is True
assert bounded['allowVolumeExpansion'] is True and bounded['boundedFilesystem']=='ext4-v1'
assert bounded['publishSecretNamespace']==bounded['expandSecretNamespace']=='${pvc.namespace}'
assert 'pvc-database' not in (Path(os.environ['FIXTURE'])/'calls').read_text()
PY
OMIT_BOUNDED_CLASS=1 PATH="$work/bin:$PATH" bash "$repo_root/scripts/service-disk-inventory.sh" > "$work/missing-class.json"
python3 - <<'PY'
import json,os
from pathlib import Path
report=json.loads((Path(os.environ['FIXTURE'])/'missing-class.json').read_text())
assert report['storageClasses'][0]['present'] is True
assert report['storageClasses'][1]=={'name':'hcloud-volumes-luks-bounded','present':False}
PY
if KUBECONFIG= PATH="$work/bin:$PATH" bash "$repo_root/scripts/service-disk-inventory.sh" > /dev/null 2>&1; then
  echo 'FAIL: accepted an implicit kubeconfig' >&2; exit 1
fi
printf 'PASS: disk/platform inventory and two fixed StorageClasses use only selected read-only projections; missing class is explicit\n'
