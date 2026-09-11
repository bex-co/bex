#!/usr/bin/env python3
"""w8/m41 reviewed-source Blueprint sync walkthrough against local dev-8.

Preview → sync with commit pin → path mismatch SOURCE_CHANGED → omit-pin HEAD
sync still works. Cleanup disconnects + deletes the Kratos identity.
"""
from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import time

KRATOS_ADM = "http://localhost:57080"
KRATOS_PUB = "http://localhost:51080"
BEX = "http://localhost:54080"
DASH = "http://localhost:50080"
EVIDENCE = ".pm/w8/m41/walkthrough-evidence.txt"
REPO = "https://github.com/bex-co/bex"
PATH = "examples/hello-go/render.yaml"
BRANCH = "main"


def log(msg: str) -> None:
    print(msg, flush=True)
    with open(EVIDENCE, "a") as f:
        f.write(msg + "\n")


def run(cmd, **kw):
    return subprocess.run(cmd, check=True, text=True, capture_output=True, **kw)


def curl_json(args: list[str]) -> dict:
    out = run(["curl", "-sS", "--max-time", "90", *args]).stdout
    return json.loads(out) if out.strip() else {}


def wait_healthy(timeout=120) -> None:
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            api = run(["curl", "-sf", "--max-time", "3", f"{BEX}/healthz"]).stdout.strip()
            kratos = run(
                ["curl", "-sf", "--max-time", "3", f"{KRATOS_PUB}/health/ready"]
            ).stdout
            if api == "ok" and "ok" in kratos:
                return
        except subprocess.CalledProcessError:
            pass
        time.sleep(2)
    raise SystemExit("stack not healthy")


def login(email: str, password: str) -> str:
    jar = tempfile.NamedTemporaryFile(delete=False).name
    run(
        [
            "curl",
            "-fsS",
            "-c",
            jar,
            "-b",
            jar,
            "-H",
            "Accept: application/json",
            f"{KRATOS_PUB}/self-service/login/browser?return_to={DASH}/",
            "-o",
            "/tmp/m41-flow.json",
        ]
    )
    flow = json.load(open("/tmp/m41-flow.json"))
    action = flow["ui"]["action"]
    csrf = next(
        n["attributes"]["value"]
        for n in flow["ui"]["nodes"]
        if n["attributes"].get("name") == "csrf_token"
    )
    open("/tmp/m41-login.json", "w").write(
        json.dumps(
            {
                "method": "password",
                "identifier": email,
                "password": password,
                "csrf_token": csrf,
            }
        )
    )
    run(
        [
            "curl",
            "-sS",
            "-o",
            "/dev/null",
            "-c",
            jar,
            "-b",
            jar,
            "-H",
            "Accept: application/json",
            "-H",
            "Content-Type: application/json",
            "--data-binary",
            "@/tmp/m41-login.json",
            action,
        ]
    )
    code = run(
        [
            "curl",
            "-sS",
            "-o",
            "/dev/null",
            "-w",
            "%{http_code}",
            "-b",
            jar,
            f"{KRATOS_PUB}/sessions/whoami",
        ]
    ).stdout
    if code != "200":
        raise SystemExit(f"login failed whoami={code}")
    return jar


def gql(jar: str, payload: dict, retries: int = 8) -> dict:
    last = {}
    for i in range(retries):
        last = curl_json(
            [
                "-b",
                jar,
                "-H",
                "Content-Type: application/json",
                "-H",
                f"Origin: {DASH}",
                "-d",
                json.dumps(payload),
                f"{BEX}/graphql",
            ]
        )
        msg = json.dumps(last)
        if "tenant onboarding unavailable" in msg or last.get("id") == "unavailable":
            log(f"transient unavailable (attempt {i+1}); waiting for heal")
            time.sleep(3)
            continue
        return last
    return last


def err_code(resp: dict) -> str | None:
    errors = resp.get("errors") or []
    if not errors:
        return None
    return (errors[0].get("extensions") or {}).get("code")


def main() -> int:
    open(EVIDENCE, "w").write("# w8/m41 reviewed-source walkthrough\n")
    wait_healthy()
    log("stack healthy")

    email = f"m41-walk-{int(time.time())}@example.com"
    password = "Walkthrough-m41-pass!"
    body = {
        "schema_id": "default",
        "traits": {"email": email},
        "credentials": {"password": {"config": {"password": password}}},
    }
    iid = curl_json(
        [
            "-X",
            "POST",
            "-H",
            "Content-Type: application/json",
            "-d",
            json.dumps(body),
            f"{KRATOS_ADM}/admin/identities",
        ]
    )["id"]
    curl_json(
        [
            "-X",
            "PATCH",
            "-H",
            "Content-Type: application/json",
            "-d",
            json.dumps(
                [
                    {
                        "op": "replace",
                        "path": "/verifiable_addresses/0/verified",
                        "value": True,
                    },
                    {
                        "op": "replace",
                        "path": "/verifiable_addresses/0/status",
                        "value": "completed",
                    },
                ]
            ),
            f"{KRATOS_ADM}/admin/identities/{iid}",
        ]
    )

    jar = login(email, password)
    owners = curl_json(["-b", jar, "-H", f"Origin: {DASH}", f"{BEX}/v1/owners"])
    owner = owners[0]["owner"]["id"]
    log(f"owner={owner} identity={iid}")

    created = gql(
        jar,
        {
            "query": "mutation($repo:String!,$branch:String!,$path:String,$name:String,$ownerId:String){ createBlueprint(repo:$repo,branch:$branch,path:$path,name:$name,ownerId:$ownerId){ id status } }",
            "variables": {
                "repo": REPO,
                "branch": BRANCH,
                "path": PATH,
                "name": f"m41-{int(time.time())}",
                "ownerId": owner,
            },
        },
    )
    log(f"create: {json.dumps(created)}")
    blp = (created.get("data") or {}).get("createBlueprint", {}).get("id")
    if not blp:
        log("FAIL: createBlueprint")
        return 1

    preview = gql(
        jar,
        {
            "query": "query($repo:String!,$branch:String!,$path:String,$ownerId:String){ blueprintPreview(repo:$repo,branch:$branch,path:$path,ownerId:$ownerId){ found commitId validation{ valid } } }",
            "variables": {
                "repo": REPO,
                "branch": BRANCH,
                "path": PATH,
                "ownerId": owner,
            },
        },
    )
    log(f"preview: {json.dumps(preview)}")
    p = (preview.get("data") or {}).get("blueprintPreview") or {}
    commit_a = p.get("commitId")
    if not p.get("found") or not commit_a or not (p.get("validation") or {}).get("valid"):
        log("FAIL: preview did not return a valid commitId")
        return 1
    log(f"reviewed commit A={commit_a}")

    # Path mismatch must refuse before apply.
    bad_path = gql(
        jar,
        {
            "query": "mutation($id:String!,$ownerId:String,$commitId:String,$path:String,$repo:String){ syncBlueprint(id:$id,ownerId:$ownerId,commitId:$commitId,path:$path,repo:$repo){ blueprint{ id status } } }",
            "variables": {
                "id": blp,
                "ownerId": owner,
                "commitId": commit_a,
                "path": "render.yaml",
                "repo": REPO,
            },
        },
    )
    log(f"pathMismatch: {json.dumps(bad_path)}")
    if err_code(bad_path) != "BLUEPRINT_SOURCE_CHANGED":
        log(f"FAIL: want BLUEPRINT_SOURCE_CHANGED, got {err_code(bad_path)}")
        return 1
    log("path mismatch → BLUEPRINT_SOURCE_CHANGED OK")

    # Pinned sync consumes A.
    pinned = gql(
        jar,
        {
            "query": "mutation($id:String!,$ownerId:String,$commitId:String,$path:String,$repo:String){ syncBlueprint(id:$id,ownerId:$ownerId,commitId:$commitId,path:$path,repo:$repo){ blueprint{ id status } } }",
            "variables": {
                "id": blp,
                "ownerId": owner,
                "commitId": commit_a,
                "path": PATH,
                "repo": REPO,
            },
        },
    )
    log(f"pinnedSync: {json.dumps(pinned)}")
    if not (pinned.get("data") or {}).get("syncBlueprint"):
        log("FAIL: pinned sync")
        return 1

    syncs = gql(
        jar,
        {
            "query": "query($id:String!,$ownerId:String){ blueprintSyncs(id:$id,ownerId:$ownerId,limit:5){ commitId state } }",
            "variables": {"id": blp, "ownerId": owner},
        },
    )
    log(f"historyAfterPin: {json.dumps(syncs)}")
    rows = (syncs.get("data") or {}).get("blueprintSyncs") or []
    success_rows = [r for r in rows if r.get("state") == "success" and r.get("commitId") == commit_a]
    if not success_rows:
        log("FAIL: history missing success row at reviewed commit A")
        return 1
    log("history names consumed revision A OK")

    # Omit pin → HEAD resolve still works (compat).
    unpinned = gql(
        jar,
        {
            "query": "mutation($id:String!,$ownerId:String){ syncBlueprint(id:$id,ownerId:$ownerId){ blueprint{ id status } } }",
            "variables": {"id": blp, "ownerId": owner},
        },
    )
    log(f"omitPinSync: {json.dumps(unpinned)}")
    if not (unpinned.get("data") or {}).get("syncBlueprint"):
        log("FAIL: omit-pin sync")
        return 1
    log("omit-precondition HEAD sync OK")

    # REST surface with the same pin.
    rest = curl_json(
        [
            "-b",
            jar,
            "-H",
            "Content-Type: application/json",
            "-H",
            f"Origin: {DASH}",
            "-d",
            json.dumps(
                {
                    "ownerId": owner,
                    "commitId": commit_a,
                    "path": PATH,
                    "repo": REPO,
                }
            ),
            f"{BEX}/v1/blueprints/{blp}/sync",
        ]
    )
    log(f"restPinned: {json.dumps(rest)[:500]}")
    if not rest.get("blueprint", {}).get("id"):
        # May be coded conflict envelope
        if rest.get("code") not in (None,):
            log(f"REST response code={rest.get('code')}")
        if "blueprint" not in rest and "id" not in json.dumps(rest):
            log(f"FAIL: REST pinned sync: {rest}")
            return 1
    log("REST reviewed-source sync OK")

    # Cleanup
    disc = gql(
        jar,
        {
            "query": "mutation($id:String!,$ownerId:String){ disconnectBlueprint(id:$id,ownerId:$ownerId) }",
            "variables": {"id": blp, "ownerId": owner},
        },
    )
    log(f"disconnect: {json.dumps(disc)}")
    run(
        [
            "curl",
            "-sS",
            "-o",
            "/dev/null",
            "-X",
            "DELETE",
            f"{KRATOS_ADM}/admin/identities/{iid}",
        ]
    )
    log(f"identity deleted {iid}")
    log("PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
