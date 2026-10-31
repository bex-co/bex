"""Execute real publisher shell with controlled tools; no network/crypto/Docker proof."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/native-base-image.yml"
WORKFLOW_TEXT = WORKFLOW.read_text()
REPOSITORY = "ghcr.io/bex-co/bex-cnb-run"
DIGEST = "sha256:" + "a" * 64
REF = REPOSITORY + "@" + DIGEST
MATRIX_NAME = "node-18-bookworm"
BASE = "node:18-bookworm@sha256:" + "d" * 64
TRIVY_TYPE = "https://aquasecurity.github.io/trivy/attestation/vuln/v1"


def run_block(marker):
    anchor = "      - " + marker + "\n"
    if WORKFLOW_TEXT.count(anchor) != 1:
        raise AssertionError("publisher step missing or ambiguous")
    tail = WORKFLOW_TEXT.split(anchor, 1)[1].split("        run: |\n", 1)[1]
    lines = []
    for line in tail.splitlines():
        if line and not line.startswith("          "):
            break
        lines.append(line[10:])
    return "\n".join(lines) + "\n"


def signing_shell():
    shell = run_block("name: sign and attest the immutable candidate")
    # GitHub evaluates these context literals before invoking the same shell.
    for expression, value in {"${{ env.NATIVE_IMAGE }}": REPOSITORY, "${{ matrix.name }}": MATRIX_NAME, "${{ matrix.base }}": BASE}.items():
        shell = shell.replace(expression, value)
    if "${{" in shell:
        raise AssertionError("unrecognized unrendered GitHub expression")
    return shell


STUB = r'''
import hashlib,json,os,pathlib,subprocess,sys
root=pathlib.Path(os.environ['CONTROL_ROOT']);mode=os.environ['CONTROL_MODE'];tool=pathlib.Path(__file__).name;args=sys.argv[1:]
events=root/'events.jsonl';past=[json.loads(x) for x in events.read_text().splitlines()] if events.exists() else []
def event(**fields):
 with events.open('a') as f:f.write(json.dumps(dict(tool=tool,argv=args,**fields))+'\n')
def option(name):return args[args.index(name)+1]
if tool=='df':
 assert args[0]=='-Pk';event(work_dir=args[1]);available=1 if mode=='low_disk' else 40*1024*1024
 print('Filesystem 1024-blocks Used Available Capacity Mounted on');print('controlled 999999999 1',available,'1%',args[1]);sys.exit(0)
if tool=='sleep':
 assert args in (['5'],['10']);event(reports_present={a:(root/('native-'+a+'.spdx.json')).exists() for a in ('amd64','arm64')});sys.exit(0)
if tool=='timeout':
 assert args[0]=='--kill-after=10s';assert args[1]==('5m' if args[2]=='cosign' else '30m');assert args[2] in ('python3','syft','cosign');event();sys.exit(subprocess.call(args[2:]))
if tool=='native-oci-stage.py':
 arch=option('--platform').split('/')[1];layout=pathlib.Path(option('--output'));proof=pathlib.Path(option('--proof'))
 assert args==['--ref',os.environ['REF'],'--platform','linux/'+arch,'--output',str(layout),'--proof',str(root/('native-'+arch+'-oci-stage.json'))]
 assert layout.name=='layout' and layout.parent.name==arch and layout.parent.parent.parent==root/'runner'
 assert layout.parent.parent.stat().st_mode&0o077==0
 report=root/('native-'+arch+'.spdx.json');assert not report.exists() and not report.is_symlink()
 layout.mkdir(mode=0o700);(layout/'owned-layer').write_text('synthetic opaque layer');(layout.parent/'cache'/'owned-cache').write_text('controlled cache')
 fail=(mode=='stage_amd_124' and arch=='amd64') or (mode=='stage_arm_fail' and arch=='arm64')
 proof.write_text(json.dumps({'CONTROL_ONLY':True,'complete':not fail,'ref':os.environ['REF'],'platform':'linux/'+arch}))
 event(arch=arch,layout=str(layout),proof=str(proof));sys.exit(124 if mode=='stage_amd_124' and fail else 23 if fail else 0)
if tool=='syft':
 arch=option('--platform').split('/')[1];layout=pathlib.Path(args[1].removeprefix('oci-dir:'));cache=pathlib.Path(os.environ['TMPDIR'])
 assert args==['scan','oci-dir:'+str(layout),'--source-name',os.environ['NATIVE_IMAGE'],'--source-version',os.environ['REF'].split('@')[1],'--platform','linux/'+arch,'-o','spdx-json']
 assert cache==layout.parent/'cache' and cache.is_dir() and (layout/'owned-layer').is_file()
 assert layout.parent.parent.stat().st_mode&0o077==0
 cache_clean=not any(cache.iterdir());assert cache_clean
 (cache/'dirty-extraction-from-this-attempt').write_text('must be removed before retry')
 assert os.environ['GODEBUG']=='http2client=0' and os.environ['SYFT_CHECK_FOR_APP_UPDATE']=='false'
 report=root/('native-'+arch+'.spdx.json');assert report.stat().st_size==0
 calls=sum(x['tool']=='syft' and x.get('arch')==arch for x in past)
 event(arch=arch,TMPDIR=str(cache),GODEBUG=os.environ['GODEBUG'],check_updates=os.environ['SYFT_CHECK_FOR_APP_UPDATE'],cache_clean=cache_clean,layout_sha256=hashlib.sha256((layout/'owned-layer').read_bytes()).hexdigest())
 if (mode=='syft_amd_transient' and arch=='amd64' and calls<2) or (mode=='syft_arm_fail' and arch=='arm64'):
  print('{"partial":');sys.exit(17)
 print(json.dumps({'CONTROL_ONLY':True,'spdxVersion':'SPDX-2.3','root':os.environ['REF'],'platform':'linux/'+arch,'unfilteredExtraField':{'unfixed':True,'allFieldsRetained':[1,2,3]}}));sys.exit(0)
if tool=='docker':
 assert args[:4]==['buildx','imagetools','create','--tag'] and args[-1]==os.environ['REF'];event();sys.exit(0)
assert tool=='cosign' and os.environ['GODEBUG']=='gctrace=0'
event(GODEBUG=os.environ['GODEBUG'])
if args[0]=='signing-config':
 assert args==['signing-config','create','--with-default-rekor-v2','--out',str(root/'runner/native-signing-config.json')]
 if mode=='config_fail':sys.exit(18)
 pathlib.Path(option('--out')).write_text('{"controlledTUF":"default-Rekor-v2"}');sys.exit(0)
assert args[0] in ('sign','attest') and '--yes' in args and '--oidc-provider=github-actions' in args and args[-1]==os.environ['REF']
assert pathlib.Path(option('--signing-config')).is_file()
if args[0]=='sign':sys.exit(19 if mode=='sign_fail' else 0)
predicate=option('--predicate');obj=json.loads(pathlib.Path(predicate).read_text())
if predicate.endswith('.spdx.json'):
 assert option('--type')=='spdxjson' and obj['unfilteredExtraField']=={'unfixed':True,'allFieldsRetained':[1,2,3]}
else:
 assert option('--type')=='https://aquasecurity.github.io/trivy/attestation/vuln/v1'
 assert obj=={'allFindings':['unfixed-critical-preserved','medium-preserved'],'unfilteredExtra':{'kept':True}}
sys.exit(21 if mode=='last_attest_fail' and predicate=='native-arm64.trivy.json' else 0)
'''


class PublisherShellControls(unittest.TestCase):
    def run_case(self, mode):
        temporary = tempfile.TemporaryDirectory(prefix="native-oci-workflow-")
        self.addCleanup(temporary.cleanup)
        root = Path(temporary.name).resolve()
        binary, runner = root / "bin", root / "runner"
        binary.mkdir()
        runner.mkdir()
        sentinel = runner / "foreign-sentinel"
        sentinel.write_text("untouched")
        fake_stage = root / ".github/scripts/native-oci-stage.py"
        fake_stage.parent.mkdir(parents=True)
        fake_stage.write_text(STUB)
        for name in ("timeout", "df", "sleep", "syft", "cosign", "docker"):
            path = binary / name
            path.write_text("#!" + sys.executable + "\n" + STUB)
            path.chmod(0o755)
        for arch in ("amd64", "arm64"):
            stale = root / ("sentinel-" + arch)
            stale.write_text("foreign stale target retained")
            (root / ("native-" + arch + ".spdx.json")).symlink_to(stale)
            (root / ("native-" + arch + ".trivy.json")).write_text(json.dumps({"allFindings": ["unfixed-critical-preserved", "medium-preserved"], "unfilteredExtra": {"kept": True}}))
        shell = root / "actual-signing.sh"
        shell.write_text(signing_shell())
        env = {"PATH": str(binary) + os.pathsep + str(Path(sys.executable).parent) + os.pathsep + os.defpath, "CONTROL_ROOT": str(root), "CONTROL_MODE": mode, "RUNNER_TEMP": str(runner), "REF": REF, "NATIVE_IMAGE": REPOSITORY, "PRODUCER": "b" * 40, "GITHUB_SHA": "c" * 40, "GITHUB_STEP_SUMMARY": str(root / "summary"), "GODEBUG": "gctrace=0", "PYTHONDONTWRITEBYTECODE": "1"}
        process = subprocess.run(["/bin/bash", "-euo", "pipefail", str(shell)], cwd=root, env=env, capture_output=True, text=True, timeout=20)
        events = [json.loads(line) for line in (root / "events.jsonl").read_text().splitlines()]
        self.assertEqual(sentinel.read_text(), "untouched")
        for arch in ("amd64", "arm64"):
            self.assertEqual((root / ("sentinel-" + arch)).read_text(), "foreign stale target retained")
        work = [Path(event["work_dir"]) for event in events if event["tool"] == "df"]
        self.assertEqual(len(work), 1)
        self.assertFalse(work[0].exists(), "EXIT cleanup must remove owned staging/cache on every outcome")
        self.assertEqual(list(runner.glob("native-syft.*")), [])
        return process, root, events

    @staticmethod
    def tools(events, name):
        return [event for event in events if event["tool"] == name]

    def attests(self, events):
        return [event for event in self.tools(events, "cosign") if event["argv"][0] == "attest"]

    def assert_counts(self, events, stage, syft, attest, tags):
        for tool, expected in (("native-oci-stage.py", stage), ("syft", syft)):
            self.assertEqual(tuple(sum(event.get("arch") == arch for event in self.tools(events, tool)) for arch in ("amd64", "arm64")), expected)
        self.assertEqual(len(self.attests(events)), attest)
        self.assertEqual(len(self.tools(events, "docker")), tags)

    def test_happy_both_architectures_five_proofs_and_two_tags(self):
        process, root, events = self.run_case("happy")
        self.assertEqual(process.returncode, 0, process.stderr)
        self.assert_counts(events, (1, 1), (1, 1), 4, 2)
        calls = self.tools(events, "cosign")
        self.assertEqual(sum(x["argv"][0] in ("sign", "attest") for x in calls), 5)
        self.assertEqual([x["argv"][x["argv"].index("--type") + 1] for x in self.attests(events)], ["spdxjson", TRIVY_TYPE] * 2)
        self.assertEqual([x["argv"][4] for x in self.tools(events, "docker")], [REPOSITORY + ":native-" + MATRIX_NAME + "-" + "c" * 40, REPOSITORY + ":native-" + MATRIX_NAME])
        for arch in ("amd64", "arm64"):
            proof = (root / ("native-" + arch + "-oci-stage.json")).read_text()
            self.assertTrue(json.loads(proof)["complete"])
            self.assertIn(proof, process.stdout)
            self.assertTrue(json.loads((root / ("native-" + arch + ".spdx.json")).read_text())["unfilteredExtraField"]["unfixed"])
        self.assertIn(BASE + " -> " + REF, (root / "summary").read_text())

    def test_stage_amd_deadline_status_blocks_syft_attest_and_discovery(self):
        process, root, events = self.run_case("stage_amd_124")
        self.assertEqual(process.returncode, 124)
        self.assert_counts(events, (1, 0), (0, 0), 0, 0)
        self.assertFalse((root / "native-amd64.spdx.json").exists())
        self.assertIn("failed with exit 124", process.stderr)
        self.assertIn((root / "native-amd64-oci-stage.json").read_text(), process.stdout)

    def test_stage_arm_failure_preserves_amd_proofs_but_never_publishes(self):
        process, root, events = self.run_case("stage_arm_fail")
        self.assertEqual(process.returncode, 23)
        self.assert_counts(events, (1, 1), (1, 0), 2, 0)
        self.assertFalse((root / "native-arm64.spdx.json").exists())
        self.assertTrue(all("native-arm64" not in " ".join(x["argv"]) for x in self.attests(events)))

    def test_syft_transient_failure_retries_scan_not_staging(self):
        process, _, events = self.run_case("syft_amd_transient")
        self.assertEqual(process.returncode, 0, process.stderr)
        self.assert_counts(events, (1, 1), (3, 1), 4, 2)
        amd_scans = [event for event in self.tools(events, "syft") if event["arch"] == "amd64"]
        self.assertTrue(all(event["cache_clean"] for event in amd_scans))
        self.assertEqual(len({event["layout_sha256"] for event in amd_scans}), 1)
        sleeps = self.tools(events, "sleep")
        self.assertEqual([x["argv"] for x in sleeps], [["5"], ["10"]])
        self.assertTrue(all(not x["reports_present"]["amd64"] for x in sleeps))

    def test_syft_arm_exhaustion_removes_partial_and_blocks_arm_proofs(self):
        process, root, events = self.run_case("syft_arm_fail")
        self.assertNotEqual(process.returncode, 0)
        self.assert_counts(events, (1, 1), (1, 3), 2, 0)
        self.assertFalse((root / "native-arm64.spdx.json").exists())
        sleeps = self.tools(events, "sleep")
        self.assertEqual([x["argv"] for x in sleeps], [["5"], ["10"]])
        self.assertTrue(all(not x["reports_present"]["arm64"] for x in sleeps))

    def test_cosign_exhaustion_blocks_downstream_and_preserves_trust_flags(self):
        for mode, sign_calls in (("config_fail", 0), ("sign_fail", 3)):
            with self.subTest(mode=mode):
                process, _, events = self.run_case(mode)
                self.assertNotEqual(process.returncode, 0)
                self.assert_counts(events, (0, 0), (0, 0), 0, 0)
                calls = self.tools(events, "cosign")
                self.assertEqual(sum(x["argv"][0] == "sign" for x in calls), sign_calls)
                self.assertEqual([x["argv"] for x in self.tools(events, "sleep")], [["5"], ["10"]])

    def test_last_attestation_failure_never_moves_discovery(self):
        process, _, events = self.run_case("last_attest_fail")
        self.assertNotEqual(process.returncode, 0)
        self.assert_counts(events, (1, 1), (1, 1), 6, 0)
        self.assertEqual(sum(x["argv"][-2:] == ["native-arm64.trivy.json", REF] for x in self.attests(events)), 3)

    def test_low_disk_exits_before_any_stage_scan_or_sign(self):
        process, _, events = self.run_case("low_disk")
        self.assertNotEqual(process.returncode, 0)
        self.assertEqual([event["tool"] for event in events], ["df"])
        self.assertIn("28 GiB", process.stderr)


class PublisherInputControls(unittest.TestCase):
    def test_only_common_foundation_is_automatically_published(self):
        triggers = "\n" + WORKFLOW_TEXT.split("\non:\n", 1)[1].split("\npermissions:", 1)[0]
        self.assertNotIn("\n  push:", triggers)
        self.assertNotIn("\n  schedule:", triggers)
        self.assertIn("\n  workflow_dispatch:", triggers)
        self.assertIn("\n  workflow_call:", triggers)
        common = (ROOT / ".github/workflows/native-common-image.yml").read_text()
        push = common.split("  push:\n", 1)[1].split("  workflow_dispatch:", 1)[0]
        self.assertRegex(push, r'(?m)^      - "deploy/native-base/\*\*"$')
        self.assertRegex(push, r'(?m)^      - "deploy/native-common/\*\*"$')
        self.assertRegex(push, r'(?m)^      - "\.github/scripts/native-oci-stage\.py"$')
        self.assertNotIn("lego/operator/", push)
        self.assertRegex(common, r"(?m)^    uses: \./\.github/workflows/native-base-image\.yml$")
        self.assertRegex(common, r"(?m)^      runtime: common-bookworm$")
        self.assertRegex(WORKFLOW_TEXT, r"(?m)^          syft-version: v1\.55\.0$")

    def select(self, runtime):
        with tempfile.TemporaryDirectory(prefix="native-oci-publish-input-") as temporary:
            root = Path(temporary)
            output = root / "github-output"
            env = dict(os.environ, TARGET_RUNTIME=runtime, GITHUB_OUTPUT=str(output), PYTHONDONTWRITEBYTECODE="1")
            process = subprocess.run(["/bin/bash", "-euo", "pipefail", "-c", run_block("id: matrix")], cwd=ROOT, env=env, capture_output=True, text=True, timeout=20)
            records = dict(line.split("=", 1) for line in output.read_text().splitlines()) if output.exists() else {}
            return process, records

    def test_actual_publisher_selects_only_common_or_one_explicit_legacy_runtime(self):
        for runtime in ("common-bookworm", "node-24-bookworm"):
            with self.subTest(runtime=runtime):
                process, records = self.select(runtime)
                self.assertEqual(process.returncode, 0, process.stderr)
                self.assertEqual([row["name"] for row in json.loads(records["images"])["include"]], [runtime])
                self.assertEqual(records["publish"], "true")

    def test_empty_or_unknown_manual_request_cannot_launch_a_full_c_matrix(self):
        for runtime in ("", "unknown-line"):
            with self.subTest(runtime=runtime):
                process, records = self.select(runtime)
                self.assertNotEqual(process.returncode, 0)
                self.assertNotIn("images", records)


if __name__ == "__main__":
    unittest.main(verbosity=2)
