"""Helpers for scripts/terraform-provider-verify.sh (w9/m474).

Four subcommands, all stdlib-only and run with `python3 -I`:

  snapshot OUT          list every resource the provider's tests can create in
                        RENDER_OWNER_ID and write their ids to OUT (JSON)
  sweep BEFORE          delete every resource listed now that BEFORE did not
                        hold, then confirm none of them is still listed
  tests SRC PKG         print `<dir> TAB <test> TAB <bex|offline>` for every
                        top-level test of provider package PKG, whose sources
                        live under SRC/internal/provider/PKG
  summarize SRC PKG LOG print one table row per top-level test in a
                        `go test -json` LOG of package PKG

snapshot and sweep read BEX_API_URL (the bex-api origin, with or without /v1)
and RENDER_API_KEY / RENDER_OWNER_ID from the environment. The key is sent as a
bearer header and never printed.
"""

import json
import os
import re
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

# Every owner-scoped collection a provider test can leave a resource in, in
# deletion order: services before the datastores and env groups they may link,
# projects last because their environments hold the rest. Dedicated IPs, log
# streams and metrics streams are recorded omissions on bex (ADR018), so there
# is nothing of theirs to sweep; notification settings are per-owner settings,
# not created resources.
COLLECTIONS = [
    "services",
    "postgres",
    "key-value",
    "redis",
    "env-groups",
    "webhooks",
    "registrycredentials",
    "projects",
]
PAGE = 100


def api_base():
    url = os.environ["BEX_API_URL"].rstrip("/")
    if url.endswith("/v1"):
        url = url[: -len("/v1")]
    return url + "/v1"


def request(method, path, query=None):
    url = api_base() + path
    if query:
        url += "?" + urllib.parse.urlencode(query)
    req = urllib.request.Request(url, method=method)
    req.add_header("Authorization", "Bearer " + os.environ["RENDER_API_KEY"])
    req.add_header("Accept", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            body = resp.read()
            return resp.status, json.loads(body) if body else None
    except urllib.error.HTTPError as err:
        body = err.read()
        try:
            return err.code, json.loads(body) if body else None
        except ValueError:
            return err.code, None


def unwrap(item):
    """A Render list item is {cursor, <kind>: {...}}; a few lists are flat."""
    if isinstance(item, dict) and "id" in item:
        return item
    if isinstance(item, dict):
        for key, value in item.items():
            if key != "cursor" and isinstance(value, dict) and "id" in value:
                return value
    return None


def list_collection(collection):
    """Every {id, name} in the collection, following Render's cursor paging.

    A collection the target does not serve (404) or cannot serve right now
    (an unconfigured secret store answers 503 for env groups on a bare dev
    stack) reads as empty, with a note on stderr: nothing can have been
    created there either.
    """
    owner = os.environ["RENDER_OWNER_ID"]
    found, cursor = {}, None
    while True:
        query = {"ownerId": owner, "limit": PAGE}
        if cursor:
            query["cursor"] = cursor
        status, body = request("GET", "/" + collection, query)
        if status != 200 or not isinstance(body, list):
            print(f"  note: GET /{collection} answered {status}; treated as empty", file=sys.stderr)
            return found
        for item in body:
            resource = unwrap(item)
            if resource:
                found[resource["id"]] = resource.get("name", "")
        if len(body) < PAGE:
            return found
        cursor = body[-1].get("cursor") if isinstance(body[-1], dict) else None
        if not cursor:
            return found


def cmd_snapshot(out):
    with open(out, "w") as fh:
        json.dump({c: list_collection(c) for c in COLLECTIONS}, fh)
    return 0


def cmd_sweep(before_path, wait_seconds=120):
    with open(before_path) as fh:
        before = json.load(fh)
    created = {}
    for collection in COLLECTIONS:
        new = {
            rid: name
            for rid, name in list_collection(collection).items()
            if rid not in before.get(collection, {})
        }
        for rid, name in new.items():
            status, _ = request("DELETE", f"/{collection}/{urllib.parse.quote(rid)}")
            print(f"  swept {collection}/{rid} ({name}): DELETE {status}")
        created[collection] = new
    deadline = time.time() + wait_seconds
    while True:
        left = [
            f"{c}/{rid}"
            for c, ids in created.items()
            if ids
            for rid in set(ids) & set(list_collection(c))
        ]
        if not left:
            return 0
        if time.time() > deadline:
            print("  error: still listed after the sweep: " + ", ".join(sorted(left)), file=sys.stderr)
            return 1
        time.sleep(5)


TEST_FUNC = re.compile(r"^func (Test\w+)\(t \*testing\.T\)", re.M)


def test_targets(pkg_dir):
    """Map each test function to `bex` or `offline`.

    A test whose body sets up the recording provider talks to RENDER_HOST
    when UPDATE_RECORDINGS=true; every other test drives an httptest fake
    or no server at all, so its result says nothing about bex.
    """
    targets = {}
    for root, _, files in os.walk(pkg_dir):
        for name in files:
            if not name.endswith("_test.go"):
                continue
            with open(os.path.join(root, name)) as fh:
                src = fh.read()
            rel = os.path.relpath(root, pkg_dir)
            matches = list(TEST_FUNC.finditer(src))
            for i, match in enumerate(matches):
                end = matches[i + 1].start() if i + 1 < len(matches) else len(src)
                body = src[match.start() : end]
                targets[(rel, match.group(1))] = "bex" if "SetupRecordingProvider" in body else "offline"
    return targets


STEP_ERROR = re.compile(r"Step \d+/\d+ error: .*")
DETAIL_SKIP = re.compile(r"^(with |on |\d+: |Error: |│|\s*$)")


def first_error(lines):
    """One line naming why a test failed: the step and the first detail line."""
    text = [line.strip() for line in "".join(lines).splitlines()]
    step, detail = None, []
    for i, line in enumerate(text):
        if step is None:
            match = STEP_ERROR.search(line)
            if match:
                step = match.group(0)
            continue
        if line.startswith("Error: "):
            detail.append(line[len("Error: ") :])
            for follow in text[i + 1 :]:
                if follow.startswith("--- FAIL") or follow.startswith("Error: "):
                    break
                if not DETAIL_SKIP.match(follow):
                    detail.append(follow)
                if len(detail) >= 3:
                    break
            break
        # A non-empty plan after apply: name the attributes Terraform would
        # change (its diff lines carry "->" or "forces replacement").
        if " -> " in line or "forces replacement" in line:
            detail.append(line)
            if len(detail) >= 2:
                break
    if step is None:
        for line in text:
            if line and not line.startswith(("=== ", "--- ", "provider.go:")):
                step = line
                break
    out = " — ".join(x for x in [step, " ".join(detail)] if x)
    out = out.replace("|", "\\|")
    return out[:300] if out else "-"


def cmd_tests(src, pkg):
    for (rel, test), target in sorted(test_targets(os.path.join(src, "internal", "provider", pkg)).items()):
        print(f"{rel}\t{test}\t{target}")
    return 0


def cmd_summarize(src, pkg, log_path):
    pkg_dir = os.path.join(src, "internal", "provider", pkg)
    targets = test_targets(pkg_dir)
    prefix = "terraform-provider-render/internal/provider/" + pkg
    results, outputs, build_failures = {}, {}, []
    with open(log_path) as fh:
        for raw in fh:
            try:
                event = json.loads(raw)
            except ValueError:
                continue
            package = event.get("Package", "")
            rel = (package[len(prefix) :].lstrip("/") or ".") if package.startswith(prefix) else package
            test = event.get("Test")
            action = event.get("Action")
            if not test:
                if event.get("FailedBuild") or "[build failed]" in event.get("Output", ""):
                    build_failures.append(rel)
                continue
            if "/" in test:
                continue
            key = (rel, test)
            if action == "output":
                outputs.setdefault(key, []).append(event.get("Output", ""))
            elif action in ("pass", "fail", "skip"):
                results[key] = (action, event.get("Elapsed", 0))
    failed = 0
    for key in sorted(results):
        rel, test = key
        action, elapsed = results[key]
        target = targets.get(key, "bex")
        reason = first_error(outputs.get(key, [])) if action == "fail" else "-"
        if action == "fail" and target == "bex":
            failed += 1
        where = pkg if rel == "." else f"{pkg}/{rel}"
        print(f"| {where} | {test} | {target} | {action.upper()} | {elapsed:.0f} | {reason} |")
    for rel in sorted(set(build_failures)):
        failed += 1
        print(f"| {pkg}/{rel} | (build) | - | FAIL | 0 | the package did not compile |")
    if not results and not build_failures:
        print(f"| {pkg} | (none) | - | - | 0 | no test ran: none at the pinned commit, or none matched TF_VERIFY_RUN |")
    return 1 if failed else 0


def main(argv):
    if len(argv) >= 2 and argv[0] == "snapshot":
        return cmd_snapshot(argv[1])
    if len(argv) >= 2 and argv[0] == "sweep":
        return cmd_sweep(argv[1], int(os.environ.get("TF_VERIFY_SWEEP_WAIT", "120")))
    if len(argv) >= 3 and argv[0] == "tests":
        return cmd_tests(argv[1], argv[2])
    if len(argv) >= 4 and argv[0] == "summarize":
        return cmd_summarize(argv[1], argv[2], argv[3])
    print(__doc__, file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
