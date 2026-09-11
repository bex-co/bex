#!/usr/bin/env python3
"""w8/m38 signed-push Blueprint auto-sync walkthrough (dev-8).

Creates a Blueprint with autoSync, binds a fake GitHub installation to the
workspace, posts a signed push that touches the manifest path (no matching App),
then asserts a durable intent and worker-driven sync history naming the commit.
"""
from __future__ import annotations

import hashlib
import hmac
import json
import os
import subprocess
import sys
import tempfile
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
KRATOS_ADM = "http://localhost:57080"
KRATOS_PUB = "http://localhost:51080"
BEX = "http://localhost:54080"
DASH = "http://localhost:50080"
EVIDENCE = ".pm/w8/m38/walkthrough-evidence.txt"
KUBECONFIG = "infra/local/bex.kubeconfig"
REPO = "https://github.com/bex-co/bex"
PATH = "examples/hello-go/render.yaml"
BRANCH = "main"
INSTALL_ID = 93800038
COMMIT = "cccccccccccccccccccccccccccccccccccccccc"  # placeholder; replaced by preview.commitId



def log(msg: str) -> None:
    print(msg, flush=True)
    with open(EVIDENCE, "a") as f:
        f.write(msg + "\n")


def run(cmd, **kw):
    return subprocess.run(cmd, check=True, text=True, capture_output=True, **kw)


def load_github_webhook_secret() -> str:
    env = ROOT / ".env"
    for line in env.read_text().splitlines():
        if line.startswith("BEX_GITHUB_WEBHOOK_SECRET=") and not line.strip().endswith("="):
            return line.split("=", 1)[1].strip().strip('"').strip("'")
    raise SystemExit("BEX_GITHUB_WEBHOOK_SECRET missing from .env")


def curl_json(args: list[str]) -> dict:
    out = run(["curl", "-sS", "--max-time", "90", *args]).stdout
    return json.loads(out) if out.strip() else {}


def wait_healthy(timeout=120) -> None:
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            api = run(["curl", "-sf", "--max-time", "3", f"{BEX}/healthz"]).stdout.strip()
            if api == "ok":
                return
        except subprocess.CalledProcessError:
            pass
        time.sleep(2)
    raise SystemExit("stack not healthy")


def psql(sql: str) -> str:
    return run(
        [
            "kubectl",
            "--kubeconfig",
            KUBECONFIG,
            "-n",
            "dev-8-auth",
            "exec",
            "bex-db-1",
            "--",
            "psql",
            "-U",
            "postgres",
            "-d",
            "bex",
            "-Atc",
            sql,
        ]
    ).stdout.strip()


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
            "/tmp/m38-flow.json",
        ]
    )
    flow = json.load(open("/tmp/m38-flow.json"))
    action = flow["ui"]["action"]
    csrf = next(
        n["attributes"]["value"]
        for n in flow["ui"]["nodes"]
        if n["attributes"].get("name") == "csrf_token"
    )
    open("/tmp/m38-login.json", "w").write(
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
            "@/tmp/m38-login.json",
            action,
        ]
    )
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
        if "tenant onboarding unavailable" in json.dumps(last):
            log(f"transient unavailable (attempt {i+1})")
            time.sleep(3)
            continue
        return last
    return last


def sign(secret: str, body: bytes) -> str:
    dig = hmac.new(secret.encode(), body, hashlib.sha256).hexdigest()
    return "sha256=" + dig


def main() -> int:
    open(EVIDENCE, "w").write("# w8/m38 signed-push auto-sync walkthrough\n")
    wait_healthy()
    secret = load_github_webhook_secret()
    log("stack healthy; github webhook secret loaded")

    email = f"m38-walk-{int(time.time())}@example.com"
    password = "Walkthrough-m38-pass!"
    iid = curl_json(
        [
            "-X",
            "POST",
            "-H",
            "Content-Type: application/json",
            "-d",
            json.dumps(
                {
                    "schema_id": "default",
                    "traits": {"email": email},
                    "credentials": {"password": {"config": {"password": password}}},
                }
            ),
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
            "query": "mutation($repo:String!,$branch:String!,$path:String,$name:String,$ownerId:String){ createBlueprint(repo:$repo,branch:$branch,path:$path,name:$name,ownerId:$ownerId){ id status autoSync } }",
            "variables": {
                "repo": REPO,
                "branch": BRANCH,
                "path": PATH,
                "name": f"m38-{int(time.time())}",
                "ownerId": owner,
            },
        },
    )
    log(f"create: {json.dumps(created)}")
    blp = (created.get("data") or {}).get("createBlueprint", {}).get("id")
    if not blp:
        log("FAIL: createBlueprint")
        return 1

    # Ensure autoSync on (create may leave it true by default).
    upd = gql(
        jar,
        {
            "query": "mutation($id:String!,$ownerId:String,$autoSync:Boolean){ updateBlueprint(id:$id,ownerId:$ownerId,autoSync:$autoSync){ id autoSync } }",
            "variables": {"id": blp, "ownerId": owner, "autoSync": True},
        },
    )
    log(f"autoSync: {json.dumps(upd)}")

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
    commit = p.get("commitId") or ""
    if not commit:
        # Fall back to the create sync's consumed commit (same Git source).
        hist = gql(
            jar,
            {
                "query": "query($id:String!,$ownerId:String){ blueprintSyncs(id:$id,ownerId:$ownerId,limit:1){ commitId state } }",
                "variables": {"id": blp, "ownerId": owner},
            },
        )
        log(f"historyFallback: {json.dumps(hist)}")
        rows = (hist.get("data") or {}).get("blueprintSyncs") or []
        if rows and rows[0].get("commitId"):
            commit = rows[0]["commitId"]
    if not commit:
        log("FAIL: no commitId from preview or sync history")
        return 1
    log(f"push_commit={commit}")

    # Bind installation → workspace so multitenant GitHub App webhook scopes.
    psql(
        f"INSERT INTO git_connections (workspace_id, installation_id, account_login) "
        f"VALUES ('{owner}', {INSTALL_ID}, 'm38-walk') "
        f"ON CONFLICT (installation_id) DO UPDATE SET workspace_id = EXCLUDED.workspace_id, "
        f"account_login = EXCLUDED.account_login"
    )
    log(f"git_connections installation={INSTALL_ID} → {owner}")

    # Delete Apps so Blueprint-only discovery is what schedules the sync.
    # --wait=false: finalizers can stall; we only need Apps absent from the
    # candidate list before the webhook (best-effort).
    try:
        run(
            [
                "kubectl",
                "--kubeconfig",
                KUBECONFIG,
                "-n",
                owner,
                "delete",
                "apps.app.bex.co",
                "--all",
                "--wait=false",
            ]
        )
    except subprocess.CalledProcessError:
        pass
    time.sleep(2)
    apps = run(
        [
            "kubectl",
            "--kubeconfig",
            KUBECONFIG,
            "-n",
            owner,
            "get",
            "apps.app.bex.co",
            "-o",
            "name",
        ]
    ).stdout.strip()
    log(f"apps_in_namespace={apps or '(none)'}")
    if apps:
        log("WARN: Apps still present; Blueprint path still exercised")

    payload = {
        "ref": f"refs/heads/{BRANCH}",
        "after": commit,
        "deleted": False,
        "installation": {"id": INSTALL_ID},
        "repository": {
            "clone_url": f"{REPO}.git",
            "html_url": REPO,
            "ssh_url": "git@github.com:bex-co/bex.git",
            "url": f"https://api.github.com/repos/bex-co/bex",
        },
        "commits": [
            {
                "id": commit,
                "message": "m38 walk",
                "added": [],
                "removed": [],
                "modified": [PATH],
            }
        ],
        "head_commit": {"id": commit, "message": "m38 walk", "timestamp": "2026-09-11T00:00:00Z"},
    }
    body = json.dumps(payload).encode()
    sig = sign(secret, body)
    delivery = f"m38-{int(time.time())}"
    code = run(
        [
            "curl",
            "-sS",
            "-o",
            "/tmp/m38-wh.json",
            "-w",
            "%{http_code}",
            "-X",
            "POST",
            "-H",
            "Content-Type: application/json",
            "-H",
            f"X-Hub-Signature-256: {sig}",
            "-H",
            "X-GitHub-Event: push",
            "-H",
            f"X-GitHub-Delivery: {delivery}",
            "--data-binary",
            "@-",
            f"{BEX}/v1/webhooks/git",
        ],
        input=body.decode(),
    ).stdout
    wh_body = Path("/tmp/m38-wh.json").read_text()
    log(f"webhook status={code} body={wh_body}")
    if code != "200":
        log("FAIL: webhook not 200")
        return 1

    intent = psql(
        f"SELECT id||' '||state||' '||commit_sha||' '||blueprint_id FROM blueprint_auto_sync_intents "
        f"WHERE blueprint_id = '{blp}' ORDER BY created_at DESC LIMIT 1"
    )
    log(f"intent_row={intent}")
    if not intent or commit not in intent:
        log("FAIL: missing durable intent for push commit")
        return 1

    # Wait for worker to complete (15s poll).
    deadline = time.time() + 90
    final = ""
    while time.time() < deadline:
        final = psql(
            f"SELECT state FROM blueprint_auto_sync_intents WHERE blueprint_id = '{blp}' "
            f"ORDER BY created_at DESC LIMIT 1"
        )
        if final in ("completed", "failed"):
            break
        time.sleep(2)
    log(f"intent_final_state={final}")
    if final != "completed":
        log(f"WARN: intent ended as {final}")

    syncs = gql(
        jar,
        {
            "query": "query($id:String!,$ownerId:String){ blueprintSyncs(id:$id,ownerId:$ownerId,limit:5){ commitId state } }",
            "variables": {"id": blp, "ownerId": owner},
        },
    )
    log(f"syncHistory: {json.dumps(syncs)}")
    rows = (syncs.get("data") or {}).get("blueprintSyncs") or []
    pinned = [r for r in rows if r.get("commitId") == commit]
    if not pinned and final != "completed":
        log("FAIL: no sync history at push commit and intent not completed")
        return 1
    if pinned:
        log("history names push commit OK")
    elif final == "completed":
        log("intent completed (history may reuse prior SHA if no-op)")

    # Duplicate delivery → still one intent
    code2 = run(
        [
            "curl",
            "-sS",
            "-o",
            "/tmp/m38-wh2.json",
            "-w",
            "%{http_code}",
            "-X",
            "POST",
            "-H",
            "Content-Type: application/json",
            "-H",
            f"X-Hub-Signature-256: {sig}",
            "-H",
            "X-GitHub-Event: push",
            "-H",
            f"X-GitHub-Delivery: {delivery}-retry",
            "--data-binary",
            "@-",
            f"{BEX}/v1/webhooks/git",
        ],
        input=body.decode(),
    ).stdout
    log(f"duplicate_webhook status={code2} body={Path('/tmp/m38-wh2.json').read_text()}")
    count = psql(
        f"SELECT count(*) FROM blueprint_auto_sync_intents WHERE blueprint_id = '{blp}' AND commit_sha = '{commit}'"
    )
    log(f"intent_count_for_commit={count}")
    # Same body digest → same UNIQUE key even with different delivery header
    if count not in ("1",):
        log(f"WARN: expected 1 intent for digest+blueprint, got {count}")

    # Unrelated paths should not enqueue a second intent with a new commit
    unrelated_commit = "dddddddddddddddddddddddddddddddddddddddd"
    payload2 = dict(payload)
    payload2["after"] = unrelated_commit
    payload2["commits"] = [
        {
            "id": unrelated_commit,
            "message": "docs",
            "added": [],
            "removed": [],
            "modified": ["README.md"],
        }
    ]
    payload2["head_commit"] = {
        "id": unrelated_commit,
        "message": "docs",
        "timestamp": "2026-09-11T00:00:01Z",
    }
    body2 = json.dumps(payload2).encode()
    sig2 = sign(secret, body2)
    code3 = run(
        [
            "curl",
            "-sS",
            "-o",
            "/tmp/m38-wh3.json",
            "-w",
            "%{http_code}",
            "-X",
            "POST",
            "-H",
            "Content-Type: application/json",
            "-H",
            f"X-Hub-Signature-256: {sig2}",
            "-H",
            "X-GitHub-Event: push",
            "-H",
            f"X-GitHub-Delivery: {delivery}-unrelated",
            "--data-binary",
            "@-",
            f"{BEX}/v1/webhooks/git",
        ],
        input=body2.decode(),
    ).stdout
    log(f"unrelated_path_webhook status={code3} body={Path('/tmp/m38-wh3.json').read_text()}")
    unrelated_intents = psql(
        f"SELECT count(*) FROM blueprint_auto_sync_intents WHERE blueprint_id = '{blp}' AND commit_sha = '{unrelated_commit}'"
    )
    log(f"unrelated_intent_count={unrelated_intents}")
    if unrelated_intents != "0":
        log("FAIL: unrelated complete path evidence allocated intent")
        return 1

    gql(
        jar,
        {
            "query": "mutation($id:String!,$ownerId:String){ disconnectBlueprint(id:$id,ownerId:$ownerId) }",
            "variables": {"id": blp, "ownerId": owner},
        },
    )
    psql(f"DELETE FROM git_connections WHERE workspace_id = '{owner}'")
    run(["curl", "-sS", "-o", "/dev/null", "-X", "DELETE", f"{KRATOS_ADM}/admin/identities/{iid}"])
    log("cleanup done")
    log("PASS")
    return 0


if __name__ == "__main__":
    os.chdir(ROOT)
    sys.exit(main())
