"""Offline original-predicate review only; never executes retained programs."""
from pathlib import Path
import base64
import hashlib
import json
import re
import struct

ROOT = Path(__file__).resolve().parent
ARTIFACT = ROOT / "intake-preparation/amd64/extracted/artifact"
SOURCE = "30b1f4dce5e25644c996a07fce84bb8338f1953f"
COMMON = "ghcr.io/bex-co/bex-cnb-run@sha256:0941c204f87643c2faad308e72acdf584976e67d8fb980a0646c5ab662dcc5d0"
PRODUCER = "1feba939b4697894e800ed0abb4f25a0bf415641"
CANDIDATE = "cc39f78a8b9f128babb362e0078007380a1b44b7799394d93b06e5fc52ab7036"
EXPECTED301 = Path("/private/tmp/bex-w8-d3-dc08-publication-plan-20261102-mn5rlwhv/CLEAN-GIT301-SOURCE-FILES.json")
LAUNCHES = ("npm", "yarn", "pnpm", "direct")


def need(ok, reason):
    if not ok:
        raise AssertionError(reason)


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def load(path):
    return json.loads(path.read_bytes())


def command(launch):
    tail = {"npm": "npm start", "yarn": "yarn start", "pnpm": "pnpm start", "direct": "node server.js"}[launch]
    return "node capture-env.js " + launch + " && exec " + tail


def mapped_pid1(expected, init):
    if init["PID1"] == "/native-init":
        return True
    return (init["PID1"].startswith("/") and expected["arch"] == "amd64"
            and init["Machine"] == 62 and init["PID1Machine"] == 183
            and re.fullmatch(r"(0|[1-9a-f][0-9a-f]*):(0|[1-9a-f][0-9a-f]*)", init["Device"]) is not None
            and re.fullmatch(r"[1-9][0-9]*", init["Inode"]) is not None
            and any(row["Path"] == "/native-init" and row["Permissions"] in ("r-xp", "r-xs")
                    and row["Device"] == init["Device"] and row["Inode"] == init["Inode"]
                    for row in init["Mappings"]))


def validate_proof(expected, proof, launch, drained):
    actual = proof["Identity"]
    init = actual["Init"]
    need(proof["Run"] == expected["run"] and actual["Node"] == expected["node"]
         and actual["Executable"] == "/usr/local/bin/node" and actual["BaseEnv"] == expected["env"]
         and actual["Launch"] == launch and init["SHA256"] == expected["init"]["sha256"]
         and init["Machine"] == expected["init"]["machine"] and init["Executable"] is True
         and mapped_pid1(expected, init)
         and init["Argv"] == ["/native-init", "/bin/bash", "-c", command(launch)], "original primary/PID1 identity")
    for key, value in expected["env"].items():
        if key != "PATH" or launch == "direct":
            need(actual["Env"][key] == value, "original child ENV:" + key)
    if drained:
        need(proof["Phase"] == "runtime" and proof["Pass"] is True
             and all(type(proof[key]) is int and proof[key] == 1 for key in ("Holds", "Completed", "Signals"))
             and proof["Events"] == ["listening", "hold_begin", "SIGTERM", "hold_end", "closed"], "original complete drain")


def go_proof(raw):
    """Exact JS JSON -> Go exported-field carrier written by the original test."""
    identity, init = raw["identity"], raw["identity"]["init"]
    return {"Phase": raw["phase"], "Run": raw["run"], "Events": raw["events"], "Holds": raw["holds"],
            "Completed": raw["completed"], "Signals": raw["signals"], "Pass": raw["pass"],
            "Identity": {"Node": identity["node"], "Executable": identity["executable"],
                         "Env": identity["env"], "BaseEnv": identity["baseEnv"], "Launch": identity["launch"],
                         "Init": {"SHA256": init["sha256"], "Machine": init["machine"], "Executable": init["executable"],
                                  "PID1": init["pid1"], "Argv": init["argv"], "PID1Machine": init["pid1Machine"],
                                  "Device": init["device"], "Inode": init["inode"],
                                  "Mappings": [{"Path": row["path"], "Permissions": row["permissions"],
                                                "Device": row["device"], "Inode": row["inode"]} for row in init["mappings"]]}}}


def accepted_exit(launch, status):
    return type(status) is int and (status == 0 or (launch != "direct" and status == 143) or (launch == "yarn" and status == 1))


def audit():
    run, jobs = load(ROOT / "run-final.json"), load(ROOT / "jobs-final.json")
    job = next(j for j in jobs["jobs"] if j["id"] == 125260213820)
    need(run["id"] == job["run_id"] == 41439172465 and run["head_sha"] == job["head_sha"] == SOURCE
         and run["run_attempt"] == job["run_attempt"] == 1 and run["head_branch"] == job["head_branch"] == "main"
         and run["event"] == "workflow_dispatch" and run["status"] == job["status"] == "completed"
         and run["conclusion"] == job["conclusion"] == "success", "actual completed CI identity")
    gates = {"measure the selected family case without claiming full-family acceptance", "record capacity observations before owned cleanup",
             "remove only this job's labeled resources, exact builder and credentials", "hash retained successes and failures"}
    steps = {row["name"]: row for row in job["steps"]}
    need(gates <= steps.keys() and all(steps[name]["status"] == "completed" and steps[name]["conclusion"] == "success" for name in gates), "actual job gates")
    manifest = load(ARTIFACT / "MANIFEST.json")
    entries = {row["path"]: row for row in manifest["files"]}
    actual = {str(p.relative_to(ARTIFACT)) for p in ARTIFACT.rglob("*") if p.is_file()}
    need(len(entries) == len(manifest["files"]) == 78 and actual == set(entries) | {"MANIFEST.json"}, "original full file bijection")
    for name, row in entries.items():
        path = ARTIFACT / name
        need(not path.is_symlink() and path.stat().st_size == row["bytes"] and sha(path) == row["sha256"], "original member bytes:" + name)
    need(sha(EXPECTED301) == "90b42e21a0e0b3358db2885d82d76f2ce1aef95edc207350680faac8c1afe13e", "Root-approved exact Git301 table")
    expected_files = {row["destination"]: {"sha256": row["sha256"], "bytes": row["bytes"], "mode": row["mode"],
                                          "type": "regular", "permissions": int(row["mode"][-3:], 8)} for row in load(EXPECTED301)["files"]}
    captured = load(ARTIFACT / "qualification-source.json")
    need(captured == {"source": SOURCE, "files": expected_files} and len(expected_files) == 301, "exact current captured source301")
    inputs = load(ARTIFACT / "inputs.json")
    need(inputs["common_image"] == COMMON and inputs["producer"] == inputs["verification"] == PRODUCER
         and inputs["generator_source"] == SOURCE and inputs["platform"] == "linux/amd64"
         and inputs["image_source"] == "reviewed-candidate" and inputs["candidate_receipt_sha256"] == CANDIDATE
         and inputs["baseline_status"] == "current-complete" and inputs["run_id"] == "41439172465"
         and inputs["run_attempt"] == "1", "actual reviewed image/job inputs")
    need(inputs["selection"]["profile"] == "native-init" and inputs["selection"]["case"] == "all-launches"
         and inputs["selection"]["selector"] == "^TestCommonNativeInitQualification$"
         and inputs["selection"]["flag"] == "BEX_NATIVE_TEST_INIT_PROFILE"
         and inputs["selection"]["go_timeout"] == "60m" and inputs["selection"]["job_minutes"] == "90", "unchanged original selection and bounds")
    directories = [p.parent for p in ARTIFACT.rglob("source-custody.json")]
    need(len(directories) == 1, "one native-init fixture")
    generated = directories[0]
    custody = load(generated / "source-custody.json")
    expected = custody["expected"]
    need(custody["common"] == COMMON and custody["platform"] == "linux/amd64" and custody["build_network"] == "none"
         and custody["choices"][0]["Label"] == "Node" and custody["choices"][0]["Line"] == "24"
         and custody["choices"][0]["Source"] == ".node-version" and expected["arch"] == "amd64"
         and re.fullmatch(r"24\.[0-9]+\.[0-9]+", expected["node"]) is not None
         and expected["commands"] == {name: command(name) for name in LAUNCHES}, "canonical selected Node24 fixture")
    binary = (generated / "init/native-init").read_bytes()
    need(binary[:6] == bytes([127, 69, 76, 70, 2, 1]) and struct.unpack_from("<HH", binary, 16) == (2, 62)
         and len(binary) == expected["init"]["bytes"] <= 16 << 20 and hashlib.sha256(binary).hexdigest() == expected["init"]["sha256"]
         and expected["init"]["machine"] == 62 and expected["init"]["executable"] is True, "actual staged guest ELF identity")
    image, platform, entrypoint = (generated / "image-identity.txt").read_text().splitlines()
    need(re.fullmatch(r"sha256:[a-f0-9]{64}", image) is not None and platform == "linux/amd64"
         and json.loads(entrypoint) == ["/native-init"], "actual generated image identity")
    recipe = (generated / "Dockerfile.canonical").read_text()
    need("\nFROM " + COMMON + "\n" in recipe and "ENTRYPOINT [\"/native-init\"]" in recipe
         and "COPY --link --from=bex-init /native-init /" in recipe, "canonical common/init recipe")
    summaries, ids = [], set()
    for launch in LAUNCHES:
        directory = generated / launch
        receipt = load(directory / "receipt.json")
        cid = (directory / "create.stdout").read_text().strip()
        need(cid not in ids, "four distinct acknowledged create IDs")
        ids.add(cid)
        intent = load(directory / "create-intent.json")
        need(intent["name"] == expected["run"] + "-" + launch and intent["image"] == image
             and intent["platform"] == "linux/amd64" and intent["owner"] == inputs["owner"]
             and intent["network"] == "bridge" and intent["publication"] == "127.0.0.1::10000", "original once-create intent")
        state, running = load(directory / "stopped.log"), load(directory / "running.log")
        need(re.fullmatch(r"[a-f0-9]{64}", cid) is not None and receipt["container"] == state
             and all(s["ID"] == cid and s["Image"] == image and s["Labels"]["co.bex.native-qualification"] == inputs["owner"]
                     and s["Network"] == "bridge" for s in (state, running))
             and running["Running"] is True and state["Running"] is False and accepted_exit(launch, state["ExitCode"]), "exact owned container/status:" + launch)
        ports = running["Ports"]
        need(set(ports) == {"10000/tcp"} and len(ports["10000/tcp"]) == 1
             and ports["10000/tcp"][0]["HostIP"] == "127.0.0.1"
             and 1 <= int(ports["10000/tcp"][0]["HostPort"]) <= 65535, "original localhost-only transport")
        need(receipt["common"] == COMMON and receipt["platform"] == "linux/amd64" and receipt["owner"] == inputs["owner"]
             and receipt["launch"] == launch and receipt["pass"] is True and receipt["response"] == "held 雪 λ🙂"
             and receipt["command_override"] == (None if launch == "npm" else ["/bin/bash", "-c", command(launch)]), "original launch/response:" + launch)
        validate_proof(expected, receipt["pre_stop"], launch, False)
        need(receipt["pre_stop"]["Events"] == ["listening", "hold_begin"], "original pre-stop hold")
        raw_runtime = load(directory / "qa-output/runtime.json")
        need(receipt["proof"] == go_proof(raw_runtime), "raw JS/Go proof carrier equality")
        validate_proof(expected, receipt["proof"], launch, True)
        launch_record = load(directory / "qa-output/launch.json")
        need(launch_record == {"run": expected["run"], "launch": launch,
                              "primary": {"node": expected["node"], "executable": "/usr/local/bin/node", "env": expected["env"]}}, "unforced base ENV capture")
        need(load(directory / "qa-output/build.json") == {"phase": "build", "pass": True,
             "primary": {"node": expected["node"], "executable": "/usr/local/bin/node", "env": expected["env"]}}, "original authored build identity")
        observation = load(directory / ("qa-output/identity-" + launch + ".json"))["nativeInitIdentityObservation"]
        need(observation["init"] == raw_runtime["identity"]["init"] and base64.b64decode(observation["pid1ArgvBase64"], validate=True)
             == b"\0".join(v.encode() for v in receipt["proof"]["Identity"]["Init"]["Argv"]) + b"\0", "retained original PID1 argv carrier")
        cleanup = load(directory / "cleanup.json")
        need(cleanup["container_id"] == cid and cleanup["owner"] == inputs["owner"] and cleanup["absence_proved"] is True
             and "no such object: " + cid in cleanup["inspection"].lower(), "exact original CID cleanup")
        need("--- PASS: TestCommonNativeInitQualification/" + launch in (ROOT / "job-125260213820.log").read_text(), "actual original Go subtest pass")
        summaries.append({"launch": launch, "node": expected["node"], "status": state["ExitCode"], "PID1": receipt["proof"]["Identity"]["Init"]["PID1"],
                          "guest_ELF_machine": 62, "drain_events": receipt["proof"]["Events"], "own_CID_absent": True})
    cleanup = load(ARTIFACT / "cleanup.json")
    need(cleanup["owner"] == inputs["owner"] and all(o.get("exit_code") == 0 for o in cleanup["operations"])
         and any(o["resource"] == "builder" and o["identity"] == inputs["owner"] and o["exit_code"] == 0 for o in cleanup["operations"])
         and any(o["resource"] == "docker-auth" and o["logout_exit_code"] == o["exit_code"] == 0 and o["removed"] is True for o in cleanup["operations"]), "declared owned builder/auth cleanup")
    return {"scope": "ACTUAL_ORIGINAL_NATIVE_INIT_AMD_TARGET_CONTENT_ONLY_NO_HOSTED_OR_ADOPTION_CLAIM", "pass": True,
            "source": SOURCE, "common": COMMON, "original_files": len(actual), "source_files": len(expected_files), "image_config": image,
            "staged_init_sha256": expected["init"]["sha256"], "launches": summaries,
            "limits": "AMD guest target executed on ARM CI runner with retained Rosetta mapping. HTTP200/4s/20s/2m predicates are source-bound original Go gates; separate raw HTTP headers/timer trace not retained. Original artifact transport does not establish local extracted executable mode."}


if __name__ == "__main__":
    print(json.dumps(audit(), indent=2, ensure_ascii=False))
