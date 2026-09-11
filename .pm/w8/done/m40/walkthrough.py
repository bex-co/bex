#!/usr/bin/env python3
"""w8/m40 two-Blueprint ownership walkthrough against local dev-8.

Uses kubectl exec for SQL (avoids host port-forward for DB reads) and retries
GraphQL on transient 'tenant onboarding unavailable' while the watchdog heals.
"""
from __future__ import annotations

import base64
import json
import subprocess
import sys
import tempfile
import time

KRATOS_ADM = "http://localhost:57080"
KRATOS_PUB = "http://localhost:51080"
BEX = "http://localhost:54080"
DASH = "http://localhost:50080"
EVIDENCE = ".pm/w8/m40/walkthrough-evidence.txt"
KUBECONFIG = "infra/local/bex.kubeconfig"


def log(msg: str) -> None:
    print(msg, flush=True)
    with open(EVIDENCE, "a") as f:
        f.write(msg + "\n")


def run(cmd, **kw):
    return subprocess.run(cmd, check=True, text=True, capture_output=True, **kw)


def curl_json(args: list[str]) -> dict:
    out = run(["curl", "-sS", "--max-time", "60", *args]).stdout
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
                # poke DB via API: owners list needs session later; check pg via exec
                run(
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
                        "select 1",
                    ]
                )
                return
        except subprocess.CalledProcessError:
            pass
        time.sleep(2)
    raise SystemExit("stack not healthy")


def psql(sql: str) -> str:
    """Run SQL inside the CNPG pod — no host port-forward."""
    # Prefer app user from secret
    user_b64 = run(
        [
            "kubectl",
            "--kubeconfig",
            KUBECONFIG,
            "-n",
            "dev-8-auth",
            "get",
            "secret",
            "bex-db-app",
            "-o",
            "jsonpath={.data.username}",
        ]
    ).stdout
    pw_b64 = run(
        [
            "kubectl",
            "--kubeconfig",
            KUBECONFIG,
            "-n",
            "dev-8-auth",
            "get",
            "secret",
            "bex-db-app",
            "-o",
            "jsonpath={.data.password}",
        ]
    ).stdout
    user = base64.b64decode(user_b64).decode()
    pw = base64.b64decode(pw_b64).decode()
    # CNPG app user connects via local socket as postgres superuser for admin ops
    # Use URI with password through localhost inside the pod.
    env_cmd = f"PGPASSWORD={pw} psql -h 127.0.0.1 -U {user} -d bex -Atc {json.dumps(sql)}"
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
            "bash",
            "-lc",
            env_cmd,
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
            "/tmp/m40-flow.json",
        ]
    )
    flow = json.load(open("/tmp/m40-flow.json"))
    action = flow["ui"]["action"]
    csrf = next(
        n["attributes"]["value"]
        for n in flow["ui"]["nodes"]
        if n["attributes"].get("name") == "csrf_token"
    )
    open("/tmp/m40-login.json", "w").write(
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
            "@/tmp/m40-login.json",
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


def main() -> int:
    open(EVIDENCE, "w").close()
    wait_healthy()
    log("stack healthy")

    email = f"m40-walk-{int(time.time())}@example.com"
    password = "Walkthrough-m40-pass!"
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

    jar_a, jar_b = login(email, password), login(email, password)
    owners = curl_json(["-b", jar_a, "-H", f"Origin: {DASH}", f"{BEX}/v1/owners"])
    owner = owners[0]["owner"]["id"]
    log(f"owner={owner} identity={iid}")

    ra = gql(
        jar_a,
        {
            "query": "mutation($repo:String!,$branch:String!,$path:String,$name:String,$ownerId:String){ createBlueprint(repo:$repo,branch:$branch,path:$path,name:$name,ownerId:$ownerId){ id status } }",
            "variables": {
                "repo": "https://github.com/bex-co/bex",
                "branch": "main",
                "path": "examples/hello-go/render.yaml",
                "name": f"m40-a-{int(time.time())}",
                "ownerId": owner,
            },
        },
    )
    log(f"createA: {json.dumps(ra)}")
    if not ra.get("data", {}).get("createBlueprint", {}).get("id"):
        log("FAIL: createA")
        return 1
    blp_a = ra["data"]["createBlueprint"]["id"]

    blp_b = f"blp-m40b{int(time.time())}"
    psql(
        f"INSERT INTO blueprints (id, tenant_id, name, repo, branch, path, manifest, status, auto_sync, execution_generation) "
        f"VALUES ('{blp_b}', '{owner}', 'm40-b', 'https://github.com/bex-co/bex-m40-other', 'main', 'render.yaml', '', 'active', false, 1)"
    )
    log(f"inserted B={blp_b}")

    manifest = """services:
  - type: web
    name: hello-go
    runtime: image
    image: {url: docker.io/traefik/whoami:v1.10.2}
    plan: starter
    numInstances: 1
"""
    conflict = gql(
        jar_b,
        {
            "query": "mutation($id:String!,$ownerId:String,$bexYaml:String){ syncBlueprint(id:$id, ownerId:$ownerId, bexYaml:$bexYaml){ blueprint { id status } } }",
            "variables": {"id": blp_b, "ownerId": owner, "bexYaml": manifest},
        },
    )
    log(f"conflictSync: {json.dumps(conflict)}")
    blob = json.dumps(conflict)
    conflict_ok = "BLUEPRINT_RESOURCE_CONFLICT" in blob
    log("CONFLICT_OK" if conflict_ok else "CONFLICT_MISS")

    takeover = gql(
        jar_b,
        {
            "query": "mutation($id:String!,$ownerId:String,$bexYaml:String,$confirm:String){ syncBlueprint(id:$id, ownerId:$ownerId, bexYaml:$bexYaml, confirm:$confirm){ blueprint { id status } } }",
            "variables": {
                "id": blp_b,
                "ownerId": owner,
                "bexYaml": manifest,
                "confirm": f"takeover blueprint {blp_a}",
            },
        },
    )
    log(f"takeover: {json.dumps(takeover)}")
    claim = psql(
        f"SELECT blueprint_id FROM blueprint_resource_claims WHERE tenant_id='{owner}' AND name='hello-go'"
    )
    log(f"claim_owner={claim} want={blp_b}")

    disc = gql(
        jar_b,
        {
            "query": "mutation($id:String!,$ownerId:String){ disconnectBlueprint(id:$id, ownerId:$ownerId) }",
            "variables": {"id": blp_b, "ownerId": owner},
        },
    )
    log(f"disconnectB: {json.dumps(disc)}")
    left = psql(
        f"SELECT count(*) FROM blueprint_resource_claims WHERE blueprint_id='{blp_b}'"
    )
    log(f"claims_for_B={left}")

    gql(
        jar_a,
        {
            "query": "mutation($id:String!,$ownerId:String){ disconnectBlueprint(id:$id, ownerId:$ownerId) }",
            "variables": {"id": blp_a, "ownerId": owner},
        },
    )
    ns = run(
        [
            "kubectl",
            "--kubeconfig",
            KUBECONFIG,
            "get",
            "ns",
            "-l",
            f"app.bex.co/workspace={owner}",
            "-o",
            "jsonpath={.items[0].metadata.name}",
        ]
    ).stdout.strip()
    if ns:
        subprocess.run(
            [
                "kubectl",
                "--kubeconfig",
                KUBECONFIG,
                "-n",
                ns,
                "delete",
                "apps.app.bex.co",
                "--all",
                "--wait=false",
            ],
            check=False,
            capture_output=True,
        )
        log(f"cleared apps in {ns}")
    code = run(
        [
            "curl",
            "-sS",
            "-o",
            "/dev/null",
            "-w",
            "%{http_code}",
            "-X",
            "DELETE",
            f"{KRATOS_ADM}/admin/identities/{iid}",
        ]
    ).stdout
    log(f"delete_identity:{code}")
    log("=== M40 WALKTHROUGH COMPLETE ===")

    ok = conflict_ok and claim == blp_b and left == "0"
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
