#!/usr/bin/env python3
"""m163 live acceptance against disposable real PostgreSQL/Ory/OpenFGA.

Run from the repository root after starting the acceptance API and dependencies:
  python3 .pm/w2/done/m163/evidence/live_acceptance.py --env-file /private/runner.env

The API runner uses the production HTTP composition/authentication boundary.
Its Kubernetes client is in-memory; this credential-only walk creates no apps.
Secrets stay in memory/private configuration and are never included in evidence.
"""

import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import secrets
import shlex
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request


parser = argparse.ArgumentParser()
parser.add_argument("--env-file", required=True)
parser.add_argument("--output", default=str(Path(__file__).with_name("live-results.jsonl")))
args = parser.parse_args()
config = dict(line.split("=", 1) for line in Path(args.env_file).read_text().splitlines() if line and not line.startswith("#"))
config = {key: shlex.split(value)[0] if value else "" for key, value in config.items()}
api = "http://" + config.get("BEX_API_ADDR", "127.0.0.1:54020")
hydra = config["BEX_HYDRA_ADMIN_URL"]
issuer = config["BEX_OAUTH_ISSUER"].rstrip("/")
kratos = config["BEX_KRATOS_URL"]
kratos_admin = config["BEX_KRATOS_ADMIN_URL"]
fga = config["BEX_OPENFGA_URL"]
out = open(args.output, "w", buffering=1)
sequence = 0


def record(event, **values):
    global sequence
    sequence += 1
    row = {"sequence": sequence, "at": datetime.datetime.now(datetime.timezone.utc).isoformat(), "event": event, **values}
    out.write(json.dumps(row, sort_keys=True) + "\n")
    print(json.dumps(row, sort_keys=True), flush=True)


def call(base, method, route, payload=None, session=None, bearer=None, form=False, expected=None):
    headers = {"Accept": "application/json"}
    if base == fga and config.get("BEX_OPENFGA_TOKEN"):
        headers["Authorization"] = "Bearer " + config["BEX_OPENFGA_TOKEN"]
    if payload is not None:
        headers["Content-Type"] = "application/x-www-form-urlencoded" if form else "application/json"
        payload = (urllib.parse.urlencode(payload) if form else json.dumps(payload)).encode()
    if session:
        headers["X-Session-Token"] = session
    if bearer:
        headers["Authorization"] = "Bearer " + bearer
    request = urllib.request.Request(base + route, data=payload, headers=headers, method=method)
    start = time.monotonic()
    try:
        response = urllib.request.urlopen(request, timeout=30)
    except urllib.error.HTTPError as exc:
        response = exc
    status = response.status
    raw = response.read()
    body = json.loads(raw) if raw else None
    record("http", origin=base, method=method, path=route.split("?")[0], status=status, duration_ms=round((time.monotonic() - start) * 1000))
    if expected is not None and status != expected:
        private_error = Path(args.env_file).with_name("last-error.json")
        private_error.write_text(json.dumps({"status": status, "body": body}))
        private_error.chmod(0o600)
        raise AssertionError(f"{method} {base}{route.split('?')[0]}: expected {expected}, got {status}; details saved privately")
    return status, body


def sql(query):
    uri = urllib.parse.urlparse(config["BEX_CP_DB_URI"])
    env = dict(os.environ, PGHOST=uri.hostname, PGPORT=str(uri.port), PGUSER=uri.username,
               PGPASSWORD=uri.password or "", PGDATABASE=uri.path.lstrip("/"), PGCONNECT_TIMEOUT="10")
    result = subprocess.run(["psql", "-X", "-tA", "-c", query], env=env, text=True, capture_output=True, check=True, timeout=30)
    return result.stdout.strip()


def literal(value):
    return "'" + value.replace("'", "''") + "'"


def gql(query, session):
    _, body = call(api, "POST", "/graphql", {"query": query}, session=session, expected=200)
    if body.get("errors"):
        raise AssertionError("GraphQL operation failed: " + json.dumps(body["errors"]))
    return body["data"]


def identity(label, stamp):
    email = f"m163-{stamp}-{label}@example.test"
    password = secrets.token_urlsafe(30)
    _, created = call(kratos_admin, "POST", "/admin/identities", {
        "schema_id": "default", "traits": {"email": email},
        "credentials": {"password": {"config": {"password": password}}},
        "verifiable_addresses": [{"value": email, "verified": True, "via": "email", "status": "completed"}],
    }, expected=201)
    _, flow = call(kratos, "GET", "/self-service/login/api", expected=200)
    _, login = call(kratos, "POST", "/self-service/login?flow=" + flow["id"], {
        "method": "password", "identifier": email, "password": password,
    }, expected=200)
    assert login["session"]["identity"]["id"] == created["id"]
    assert any(address["verified"] for address in login["session"]["identity"]["verifiable_addresses"])
    record("human_session_created", label=label, subject=created["id"], verified_email=True)
    return {"subject": created["id"], "email": email, "session": login["session_token"]}


def owners(person):
    _, body = call(api, "GET", "/v1/owners", session=person["session"], expected=200)
    return [entry["owner"] for entry in body]


def key(person, workspace, name):
    _, created = call(api, "POST", "/v1/api-keys", {"ownerId": workspace, "name": name}, session=person["session"], expected=201)
    assert created["createdBy"] == person["subject"]
    _, token = call(issuer, "POST", "/oauth2/token", {
        "grant_type": "client_credentials", "client_id": created["id"], "client_secret": created["secret"],
    }, form=True, expected=200)
    _, introspection = call(hydra, "POST", "/admin/oauth2/introspect", {"token": token["access_token"]}, form=True, expected=200)
    record("token_introspection", key_id=created["id"], active=introspection.get("active"),
           subject=introspection.get("sub"), issuer=introspection.get("iss"), audience=introspection.get("aud"), scope=introspection.get("scope"))
    assert introspection.get("active") is True
    record("key_created", label=name, key_id=created["id"], creator=person["subject"], workspace=workspace)
    return {"id": created["id"], "workspace": workspace, "token": token["access_token"], "label": name}


def key_request(machine, expected=200):
    status, _ = call(api, "GET", "/v1/viewer/capabilities?ownerId=" + machine["workspace"], bearer=machine["token"], expected=expected)
    return status


def tuple_count(subject, workspace):
    _, body = call(fga, "POST", f"/stores/{store_id}/read", {
        "tuple_key": {"user": "user:" + subject, "object": "workspace:" + workspace},
    }, expected=200)
    return len(body.get("tuples", []))


def binding_count(subject, workspace):
    return int(sql("SELECT count(*) FROM tenant_members WHERE subject=" + literal(subject) + " AND tenant_id=" + literal(workspace)))


def check_machine(machine, survives):
    api_status = key_request(machine, expected=200 if survives else 401)
    status, _ = call(hydra, "GET", "/admin/clients/" + machine["id"], expected=200 if survives else 404)
    rows = binding_count(machine["id"], machine["workspace"])
    tuples = tuple_count(machine["id"], machine["workspace"])
    assert rows == int(survives), (machine["label"], "binding", rows)
    assert tuples == int(survives), (machine["label"], "tuple", tuples)
    record("machine_assertion", label=machine["label"], key_id=machine["id"], workspace=machine["workspace"],
           survives=survives, api_status=api_status, hydra_status=status, membership_rows=rows, fga_tuples=tuples)


source_diff = subprocess.check_output(["git", "diff", "--", "lego/backend"])
record("run_started", source_sha=subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip(),
       backend_working_diff_sha256=hashlib.sha256(source_diff).hexdigest(),
       backend_modified_files=subprocess.check_output(["git", "diff", "--name-only", "--", "lego/backend"], text=True).splitlines(),
       kubernetes="in-memory client; no app/resource operations", postgres=sql("SHOW server_version"),
       auth="real Kratos session + real Hydra client_credentials token + production API gate + enforced OpenFGA")
_, stores = call(fga, "GET", "/stores?page_size=100", expected=200)
existing = [entry for entry in stores["stores"] if entry["name"] == "bex"]
if existing:
    store_id = existing[0]["id"]
else:
    _, created_store = call(fga, "POST", "/stores", {"name": "bex"}, expected=201)
    store_id = created_store["id"]
model = json.loads(Path("deploy/gitops/authz/model.json").read_text())
_, installed = call(fga, "POST", f"/stores/{store_id}/authorization-models", model, expected=201)
record("fga_model_installed", store_id=store_id, model_id=installed["authorization_model_id"])
call(api, "GET", "/v1/owners", expected=401)
stamp = datetime.datetime.now(datetime.timezone.utc).strftime("%H%M%S")
admin = identity("remaining-admin", stamp)
owners(admin)

for scenario in ("admin-remove", "self-leave", "account-delete"):
    # Separate workspaces keep every fixture within the ordinary issuance
    # burst limit; the production key quota/rate controls remain enabled.
    name = "m163-" + stamp + "-" + scenario
    workspace = gql('mutation { createWorkspace(name: ' + json.dumps(name) + ', plan: "pro") { id } }', admin["session"])["createWorkspace"]["id"]
    remaining = key(admin, workspace, scenario + "-remaining-member")
    if scenario == "admin-remove":
        # A real authorization negative control: the SQL binding remains
        # present, but removing its FGA tuple denies a fresh HTTP check.
        key_request(remaining)
        tuple_key = {"user": "user:" + remaining["id"], "relation": "developer", "object": "workspace:" + workspace}
        call(fga, "POST", f"/stores/{store_id}/write", {"deletes": {"tuple_keys": [tuple_key]}}, expected=200)
        call(api, "GET", "/v1/viewer/capabilities?ownerId=" + workspace + "&fresh=true", bearer=remaining["token"], expected=403)
        assert binding_count(remaining["id"], workspace) == 1
        call(fga, "POST", f"/stores/{store_id}/write", {"writes": {"tuple_keys": [tuple_key]}}, expected=200)
        call(api, "GET", "/v1/viewer/capabilities?ownerId=" + workspace + "&fresh=true", bearer=remaining["token"], expected=200)
        record("fga_enforcement_proven", bound_key_without_tuple=403, restored_tuple=200)
    departing = identity(scenario, stamp)
    # Invite before the first bex request so the production login onboarding
    # path redeems the verified email and writes the real membership/FGA tuple.
    call(api, "POST", f"/v1/workspaces/{workspace}/members", {"email": departing["email"], "role": "developer"}, session=admin["session"], expected=201)
    joined = owners(departing)
    assert workspace in [entry["id"] for entry in joined]
    other_workspace = next(entry["id"] for entry in joined if entry["id"] != workspace)
    assert binding_count(departing["subject"], workspace) == 1
    assert tuple_count(departing["subject"], workspace) == 1
    departed_keys = [key(departing, workspace, scenario + "-target-" + str(index)) for index in (1, 2)]
    elsewhere = key(departing, other_workspace, scenario + "-other-workspace")
    for machine in departed_keys + [elsewhere, remaining]:
        check_machine(machine, True)
    # Warm all auth, workspace-membership and OpenFGA caches immediately before
    # exit; the after checks use exactly those already-issued bearer tokens.
    for machine in departed_keys + [elsewhere, remaining]:
        key_request(machine)
    warm_done = time.monotonic()
    if scenario == "admin-remove":
        call(api, "DELETE", f"/v1/workspaces/{workspace}/members/{departing['subject']}", session=admin["session"], expected=204)
    elif scenario == "self-leave":
        call(api, "DELETE", f"/v1/workspaces/{workspace}/members/me", session=departing["session"], expected=204)
    else:
        _, preview = call(api, "GET", "/v1/users/deletion-preview", session=departing["session"], expected=200)
        assert not preview["blocked"]
        assert workspace in [entry["id"] for entry in preview["leave"]]
        call(api, "DELETE", "/v1/users", {"confirmation": "delete my account"}, session=departing["session"], expected=202)
        deadline = time.monotonic() + 60
        while time.monotonic() < deadline:
            state = sql("SELECT state FROM account_deletions WHERE subject=" + literal(departing["subject"]))
            if state == "done":
                break
            time.sleep(0.2)
        assert state == "done", "account-deletion worker did not complete within 60 seconds"
        record("account_worker_done", subject=departing["subject"])
    statuses = [key_request(machine, 401) for machine in departed_keys]
    record("warm_token_revocation", scenario=scenario, target_keys=len(departed_keys), statuses=statuses,
           elapsed_since_warm_ms=round((time.monotonic() - warm_done) * 1000))
    for machine in departed_keys:
        check_machine(machine, False)
    check_machine(elsewhere, scenario != "account-delete")
    check_machine(remaining, True)
    rows = binding_count(departing["subject"], workspace)
    tuples = tuple_count(departing["subject"], workspace)
    assert rows == 0 and tuples == 0
    _, listed = call(api, "GET", "/v1/api-keys?ownerId=" + workspace, session=admin["session"], expected=200)
    assert remaining["id"] in [entry["id"] for entry in listed]
    assert not set(machine["id"] for machine in departed_keys).intersection(entry["id"] for entry in listed)
    if scenario != "account-delete":
        call(api, "GET", "/v1/viewer/capabilities?ownerId=" + workspace + "&fresh=true", session=departing["session"], expected=403)
    else:
        call(kratos_admin, "GET", "/admin/identities/" + departing["subject"], expected=404)
    record("scenario_passed", scenario=scenario, workspace=workspace, departing_subject=departing["subject"],
           human_membership_rows=rows, human_fga_tuples=tuples, target_keys_revoked=2,
           other_workspace_key_survives=scenario != "account-delete", remaining_member_key_survives=True)

record("run_passed", scenarios=["admin-remove", "self-leave", "account-delete"],
       cleanup="Stop only this run's API/dependencies and remove its disposable databases; recorded separately in the closeout.")
