"""Drive bex sandboxes with the unmodified Render Python SDK (w32/m284).

Walks every sandbox method of the sync client, the async client's core path,
the generated execution and image-list calls, and the typed errors. Each step
prints `PASS <step>` or `FAIL <step>: <error>`; the exit status is the number
of failed steps. Every sandbox and group it creates is removed at the end.
Reads RENDER_API_KEY (a bex bearer token), RENDER_WORKSPACE_ID, BEX_API_URL
(the bare origin; the SDK appends /v1 itself) and, optionally,
SDK_COMPAT_IMAGE (a public linux/amd64 image to import) and SDK_COMPAT_SUSPEND
(set to also wait out an idle suspension, about six minutes, and resume).
"""

import asyncio
import os
import secrets
import sys
import tempfile
import time
import traceback
import warnings

from render import Render, RenderAsync
from render.client.errors import ClientError
from render.experimental.sandbox import (
    ImageNotFoundError,
    SandboxFileNotFoundError,
    SandboxGroupDefaultNotDeletableError,
    SandboxNotFoundError,
    SnapshotNotFoundError,
    SnapshotNotReadyError,
)
from render.experimental.sandbox.types import SandboxExecExit, SandboxExecOutput
from render.public_api.api.sandboxes import (
    list_sandbox_builds,
    list_sandbox_executions,
    list_sandbox_group_credentials,
    retrieve_sandbox_execution,
)

failures = 0
created = []
groups = []
run = secrets.token_hex(3)
image_source = os.environ.get("SDK_COMPAT_IMAGE", "docker.io/library/alpine:3.20")
image_name = f"sdk-compat-{run}"
missing_snapshot = "snp-d0000000000000000000"


def step(name, fn):
    global failures
    try:
        detail = fn()
        print(f"PASS {name}" + (f" ({detail})" if detail else ""), flush=True)
    except Exception as exc:  # noqa: BLE001 - every failure is a grade
        failures += 1
        print(f"FAIL {name}: {type(exc).__name__}: {exc}", flush=True)
        if os.environ.get("SDK_COMPAT_TRACE"):
            traceback.print_exc()


def raises(error_class, fn):
    """The call must raise error_class; returns what it said."""
    try:
        fn()
    except error_class as exc:
        return f"{type(exc).__name__}: {exc}"
    raise AssertionError(f"expected {error_class.__name__}")


render = Render(base_url=os.environ["BEX_API_URL"])
sandboxes = render.sandboxes
owner = os.environ["RENDER_WORKSPACE_ID"]
state = {"name": f"sdk-compat-py-{run}"}


def write(path, text):
    with open(path, "w") as f:
        f.write(text)


def read(path):
    with open(path) as f:
        return f.read()


def round_trip(sandbox_id, remote_path, text):
    """copy_to then copy_from one file; the bytes must come back unchanged."""
    with tempfile.TemporaryDirectory() as tmp:
        write(os.path.join(tmp, "in"), text)
        sandboxes.copy_to(sandbox_id, os.path.join(tmp, "in"), remote_path)
        sandboxes.copy_from(sandbox_id, remote_path, os.path.join(tmp, "out"))
        got = read(os.path.join(tmp, "out"))
    assert got == text, got


def run_command(sandbox_id, command):
    out, code = [], None
    for event in sandboxes.exec(sandbox_id, command):
        if isinstance(event, SandboxExecOutput):
            out.append(event.data)
        elif isinstance(event, SandboxExecExit):
            code = event.exit_code
    return "".join(out), code


# --- one sandbox in the default group -------------------------------------


def create():
    box = sandboxes.create(
        name=state["name"],
        plan="1c-2g",
        timeout_seconds=900,
        idle_timeout_seconds=300,
        network_policy="deny-all",
        env={"SDK_COMPAT_ENV": "env-ok"},
    )
    created.append(box.id)
    state["id"] = box.id
    assert box.name == state["name"], box.name
    assert (box.plan, box.timeout_seconds, box.idle_timeout_seconds) == ("1c-2g", 900, 300), box
    assert box.network_policy.type == "deny-all", box.network_policy
    return box.id


def wait():
    box = sandboxes.wait_until_running(state["id"])
    assert box.status == "running", box.status
    return box.status


def get():
    box = sandboxes.from_id(state["id"])
    assert box.id == state["id"] and box.name == state["name"], box
    assert box.created_at and box.sandbox_group_id, box
    state["default_group"] = box.sandbox_group_id
    return f"region={box.region}"


def listed():
    page = sandboxes.list(limit=100)
    ids = [item.id for item in page.sandboxes]
    assert state["id"] in ids, ids
    return f"{len(ids)} listed"


def list_status():
    running = [b.id for b in sandboxes.list(status="running").sandboxes]
    ended = [b.id for b in sandboxes.list(status=["terminated"]).sandboxes]
    assert state["id"] in running and state["id"] not in ended, (running, ended)
    return "running includes it, terminated does not"


def list_name():
    ids = [b.id for b in sandboxes.list(name=state["name"]).sandboxes]
    assert ids == [state["id"]], ids
    return "exact match"


def exec_output():
    out, code = run_command(state["id"], "printf sdk-ok; echo err >&2; exit 3")
    assert "sdk-ok" in out and code == 3, (out, code)
    return "exit 3"


def exec_env():
    out, code = run_command(state["id"], 'printf %s "$SDK_COMPAT_ENV"; printf " %s" "$HOME"; printf " %s" "$PWD"')
    assert code == 0 and out.startswith("env-ok "), (out, code)
    return out


def exec_quiet():
    # Longer than the SDK's 45 s read timeout: it lives only on keep-alives.
    started = time.monotonic()
    out, code = run_command(state["id"], "sleep 50; printf done")
    assert out == "done" and code == 0, (out, code)
    return f"{time.monotonic() - started:.0f}s"


def files():
    round_trip(state["id"], "/tmp/sdk-compat.txt", "payload-1\n")
    return "round trip"


def files_relative():
    round_trip(state["id"], "sdk-compat-rel.txt", "relative\n")
    out, code = run_command(state["id"], 'cat "$HOME/sdk-compat-rel.txt"')
    assert (out, code) == ("relative\n", 0), (out, code)
    return "under $HOME"


def files_directory():
    with tempfile.TemporaryDirectory() as tmp:
        tree = os.path.join(tmp, "tree")
        os.makedirs(os.path.join(tree, "sub"))
        write(os.path.join(tree, "sub", "a.txt"), "a\n")
        os.symlink("sub/a.txt", os.path.join(tree, "link"))
        sandboxes.copy_to(state["id"], tree, "/tmp/sdk-compat-tree")
        back = os.path.join(tmp, "back")
        sandboxes.copy_from(state["id"], "/tmp/sdk-compat-tree", back)
        root = back if os.path.exists(os.path.join(back, "sub")) else os.path.join(back, "sdk-compat-tree")
        assert read(os.path.join(root, "sub", "a.txt")) == "a\n"
        assert os.readlink(os.path.join(root, "link")) == "sub/a.txt"
    return "file and symlink"


def missing_file():
    with tempfile.TemporaryDirectory() as tmp:
        return raises(
            SandboxFileNotFoundError,
            lambda: sandboxes.copy_from(state["id"], "/tmp/does-not-exist-sdk", os.path.join(tmp, "x")),
        )


def executions():
    resp = list_sandbox_executions.sync_detailed(state["id"], client=sandboxes.client, owner_id=owner, limit=5)
    assert resp.status_code == 200, (resp.status_code, resp.content[:200])
    items = resp.parsed
    assert isinstance(items, list) and items, items
    newest = items[0].execution
    one = retrieve_sandbox_execution.sync_detailed(state["id"], newest.id, client=sandboxes.client, owner_id=owner)
    assert one.status_code == 200 and one.parsed.id == newest.id, (one.status_code, one.content[:200])
    return f"{len(items)} listed, newest {newest.type_.value}/{newest.operation.value}"


def unknown_sandbox():
    return raises(SandboxNotFoundError, lambda: sandboxes.from_id("sbx-d0000000000000000000"))


def terminate():
    sandboxes.terminate(state["id"])
    created.remove(state["id"])
    box = sandboxes.from_id(state["id"])
    assert box.status == "terminated" and box.terminated_at, box
    return "reads terminated"


def ended(fn):
    # Render's pinned contract: a connect-token mint for a terminated or errored
    # sandbox answers 409 sandbox_not_running (SDK v1.8.0 has no class for it).
    try:
        fn()
    except ClientError as exc:
        # The SDK's sandbox helpers name the status in the message only.
        said = str(exc)
        assert "status 409" in said and "sandbox_not_running" in said and "terminated at" in said, said
        return f"{type(exc).__name__} 409 sandbox_not_running"
    raise AssertionError("an ended sandbox did not refuse")


def exec_after_terminate():
    return ended(lambda: run_command(state["id"], "true"))


def copy_after_terminate():
    with tempfile.TemporaryDirectory() as tmp:
        return ended(lambda: sandboxes.copy_from(state["id"], "/tmp/sdk-compat.txt", os.path.join(tmp, "x")))


def suspend_resume():
    # Opt-in (SDK_COMPAT_SUSPEND=1): idle suspension takes 300 s at least.
    box = sandboxes.create(plan="1c-2g", timeout_seconds=1800, idle_timeout_seconds=300, network_policy="deny-all", wait=True)
    created.append(box.id)
    run_command(box.id, "printf kept > $HOME/kept.txt")
    deadline = time.monotonic() + 900
    while sandboxes.from_id(box.id).status != "suspended":
        assert time.monotonic() < deadline, "not suspended after 15 minutes"
        time.sleep(15)
    started = time.monotonic()
    with tempfile.TemporaryDirectory() as tmp:
        sandboxes.copy_from(box.id, "kept.txt", os.path.join(tmp, "kept.txt"))
        assert read(os.path.join(tmp, "kept.txt")) == "kept"
    download = time.monotonic() - started
    out, code = run_command(box.id, "printf resumed")
    assert (out, code) == ("resumed", 0), (out, code)
    sandboxes.terminate(box.id)
    created.remove(box.id)
    return f"download from suspended took {download:.0f}s"


# --- groups and images ----------------------------------------------------


def list_groups():
    page = sandboxes.list_groups()
    defaults = [g.id for g in page.groups if g.is_default]
    assert defaults == [state["default_group"]], (defaults, state.get("default_group"))
    return f"{len(page.groups)} groups, default {defaults[0]}"


def create_group():
    group = sandboxes.create_group(state["name"])
    groups.append(group.id)
    state["group"] = group.id
    assert not group.is_default and group.owner_id == owner, group
    return group.id


def get_group():
    group = sandboxes.get_group(state["group"])
    assert group.id == state["group"] and group.name == state["name"], group
    return f"concurrencyLimit={group.concurrency_limit}"


def update_group():
    group = sandboxes.update_group(state["group"], concurrency_limit=3)
    assert group.concurrency_limit == 3, group
    return "concurrencyLimit=3"


def set_default_unchanged():
    # Setting the current default returns it unchanged: no workspace change.
    group = sandboxes.set_default_group(state["default_group"])
    assert group.id == state["default_group"] and group.is_default, group
    return "current default kept"


def delete_default_refused():
    return raises(SandboxGroupDefaultNotDeletableError, lambda: sandboxes.delete_group(state["default_group"]))


def create_in_group():
    box = sandboxes.create(sandbox_group_id=state["group"], plan="1c-2g", timeout_seconds=600, network_policy="deny-all", wait=True)
    created.append(box.id)
    state["grouped"] = box.id
    assert box.sandbox_group_id == state["group"] and box.status == "running", box
    ids = [b.id for b in sandboxes.list(sandbox_group_id=state["group"]).sandboxes]
    assert ids == [box.id], ids
    return f"{box.id}, list filter exact"


def list_paging():
    first = sandboxes.list(limit=1)
    assert len(first.sandboxes) == 1 and first.next_cursor, first
    second = sandboxes.list(limit=1, cursor=first.next_cursor)
    assert second.sandboxes and second.sandboxes[0].id != first.sandboxes[0].id, (first, second)
    return "limit=1 then cursor"


def image_import():
    image = sandboxes.images.import_(sandbox_group_id=state["group"], name=image_name, source=image_source)
    state["image"] = image.id
    image = sandboxes.images.wait_until_ready(sandbox_group_id=state["group"], image_id=image.id, timeout_seconds=600)
    assert image.status == "succeeded", image
    again = sandboxes.images.from_id(sandbox_group_id=state["group"], image_id=image.id)
    assert again.id == image.id and again.source_digest, again
    return f"{image.id} {image.source.repository}/{image.source.image}:{image.source.reference}"


def image_list():
    resp = list_sandbox_builds.sync_detailed(state["group"], client=sandboxes.client)
    assert resp.status_code == 200, (resp.status_code, resp.content[:200])
    ids = [item.build.id for item in resp.parsed]
    assert state["image"] in ids, ids
    return f"{len(ids)} listed"


def create_from_image():
    box = sandboxes.create(sandbox_group_id=state["group"], image=image_name, plan="1c-2g", timeout_seconds=600, network_policy="deny-all", wait=True)
    created.append(box.id)
    out, code = run_command(box.id, "cat /etc/alpine-release")
    assert code == 0 and out.strip(), (out, code)
    return f"alpine {out.strip()}"


def builds_deprecated():
    # The same import again returns the image the group already has.
    with warnings.catch_warnings():
        warnings.simplefilter("ignore", DeprecationWarning)
        again = sandboxes.builds.create(sandbox_group_id=state["group"], name=image_name, source_image=image_source)
        build = sandboxes.builds.from_id(sandbox_group_id=state["group"], build_id=state["image"])
    assert again.id == state["image"], again
    assert build.id == state["image"], build
    return "create returns the existing import; from_id"


def unknown_image():
    return raises(
        ImageNotFoundError,
        lambda: sandboxes.create(sandbox_group_id=state["group"], image=f"sdk-compat-none-{run}", network_policy="deny-all"),
    )


def runtime_refused():
    # bex captures the filesystem only until w2/m322; Render's default kind is
    # runtime, so an omitted kind is refused by the same name (ADR018).
    said = []
    for label, kind in [("default kind", None), ("kind=runtime", "runtime")]:
        try:
            sandboxes.snapshots.create(state["grouped"], kind=kind)
        except ClientError as exc:
            assert "SANDBOX_SNAPSHOT_RUNTIME_UNSUPPORTED" in f"{getattr(exc, 'code', '')} {exc}", exc
            said.append(f"{label}: 400 SANDBOX_SNAPSHOT_RUNTIME_UNSUPPORTED")
            continue
        raise AssertionError(f"a {label} snapshot was not refused")
    return "; ".join(said)


def snapshot_create():
    out, code = run_command(state["grouped"], f"printf {run} > $HOME/sdk-compat-marker.txt")
    assert code == 0, (out, code)
    snap = sandboxes.snapshots.create(state["grouped"], kind="filesystem", name=f"sdk-compat-{run}", wait=True)
    state["snapshot"] = snap.id
    assert snap.status == "available" and snap.kind == "filesystem" and snap.size_bytes and snap.captured_at, snap
    assert snap.sandbox_group_id == state["group"] and snap.source_sandbox_id == state["grouped"], snap
    box = sandboxes.from_id(state["grouped"])
    assert box.status == "running", box.status
    return f"{snap.id} {snap.size_bytes} B, the sandbox kept running"


def snapshot_read():
    snap = sandboxes.snapshots.from_id(sandbox_group_id=state["group"], snapshot_id=state["snapshot"])
    assert snap.id == state["snapshot"] and snap.name == f"sdk-compat-{run}", snap
    listed = [s.id for s in sandboxes.snapshots.list(sandbox_group_id=state["group"]).snapshots]
    available = [s.id for s in sandboxes.snapshots.list(sandbox_group_id=state["group"], status="available").snapshots]
    assert state["snapshot"] in listed and state["snapshot"] in available, (listed, available)
    return f"{len(listed)} listed"


def snapshot_restore():
    said = []
    for label, source in [("snapshot_id", {"snapshot_id": state["snapshot"]}), ("snapshot_name", {"snapshot_name": f"sdk-compat-{run}"})]:
        box = sandboxes.create(sandbox_group_id=state["group"], timeout_seconds=600, wait=True, **source)
        created.append(box.id)
        out, code = run_command(box.id, "cat $HOME/sdk-compat-marker.txt")
        assert (out, code) == (run, 0), (label, out, code)
        assert box.network_policy.type == "deny-all", box.network_policy
        # The group's concurrency limit is 3 (update_group): make room again.
        sandboxes.terminate(box.id)
        created.remove(box.id)
        said.append(f"{label}: marker back")
    sandboxes.snapshots.from_id(sandbox_group_id=state["group"], snapshot_id=state["snapshot"])
    return "; ".join(said) + "; the snapshot remains"


def snapshot_delete_creating():
    snap = sandboxes.snapshots.create(state["grouped"], kind="filesystem")
    said = raises(
        SnapshotNotReadyError,
        lambda: sandboxes.snapshots.delete(sandbox_group_id=state["group"], snapshot_id=snap.id),
    )
    sandboxes.snapshots.wait_until_available(sandbox_group_id=state["group"], snapshot_id=snap.id)
    sandboxes.snapshots.delete(sandbox_group_id=state["group"], snapshot_id=snap.id)
    return said


def snapshot_delete():
    group, snap = state["group"], state["snapshot"]
    sandboxes.snapshots.delete(sandbox_group_id=group, snapshot_id=snap)
    raises(SnapshotNotFoundError, lambda: sandboxes.snapshots.from_id(sandbox_group_id=group, snapshot_id=snap))
    sandboxes.snapshots.delete(sandbox_group_id=group, snapshot_id=snap)  # 204 again
    raises(SnapshotNotFoundError, lambda: sandboxes.create(sandbox_group_id=group, snapshot_id=snap))
    listed = [s.id for s in sandboxes.snapshots.list(sandbox_group_id=group).snapshots]
    assert snap not in listed, listed
    return "reads, restores and lists as gone; a second delete succeeds"


def snapshot_unknown():
    return raises(SnapshotNotFoundError, lambda: sandboxes.snapshots.from_id(sandbox_group_id=state["group"], snapshot_id=missing_snapshot))


def credentials_refused():
    resp = list_sandbox_group_credentials.sync_detailed(state["group"], client=sandboxes.client)
    assert resp.status_code == 404 and b"SANDBOX_CREDENTIALS_UNSUPPORTED" in resp.content, (resp.status_code, resp.content[:200])
    return "404 SANDBOX_CREDENTIALS_UNSUPPORTED"


def delete_group():
    sandboxes.delete_group(state["group"])
    groups.remove(state["group"])
    gone = [i for i in created if sandboxes.from_id(i).status == "terminated"]
    for i in gone:
        created.remove(i)
    return f"{len(gone)} group sandboxes terminated"


# --- the async client -----------------------------------------------------


async def async_core():
    client = RenderAsync(base_url=os.environ["BEX_API_URL"]).sandboxes
    box = await client.create(plan="1c-2g", timeout_seconds=600, network_policy="deny-all", wait=True)
    created.append(box.id)
    out, code = [], None
    async for event in client.exec(box.id, "printf async-ok; exit 4"):
        if isinstance(event, SandboxExecOutput):
            out.append(event.data)
        elif isinstance(event, SandboxExecExit):
            code = event.exit_code
    assert ("".join(out), code) == ("async-ok", 4), (out, code)
    with tempfile.TemporaryDirectory() as tmp:
        write(os.path.join(tmp, "a.txt"), "async\n")
        await client.copy_to(box.id, os.path.join(tmp, "a.txt"), "/tmp/a.txt")
        await client.copy_from(box.id, "/tmp/a.txt", os.path.join(tmp, "b.txt"))
        assert read(os.path.join(tmp, "b.txt")) == "async\n"
    page = await client.list(limit=100)
    assert box.id in [b.id for b in page.sandboxes]
    await client.terminate(box.id)
    created.remove(box.id)
    return "create(wait) / exec / copy_to / copy_from / list / terminate"


step("create", create)
if "id" in state:
    step("wait_until_running", wait)
    step("from_id", get)
    step("list", listed)
    step("list status filter", list_status)
    step("list name filter", list_name)
    step("exec output and exit code", exec_output)
    step("exec env, HOME and cwd", exec_env)
    step("exec quiet for 50s", exec_quiet)
    step("copy_to / copy_from", files)
    step("copy_to / copy_from relative path", files_relative)
    step("copy_to / copy_from directory", files_directory)
    step("copy_from missing file", missing_file)
    step("generated list/retrieve executions", executions)
    step("from_id unknown sandbox", unknown_sandbox)
    if "default_group" in state:
        step("list_groups", list_groups)
        step("set_default_group (current default)", set_default_unchanged)
        step("delete_group default refused", delete_default_refused)
    step("create_group", create_group)
    if "group" in state:
        step("get_group", get_group)
        step("update_group", update_group)
        step("create in group / list group filter", create_in_group)
        step("list limit and cursor", list_paging)
        step("images.import_ / wait_until_ready / from_id", image_import)
        if "image" in state:
            step("generated list_sandbox_builds", image_list)
            step("create from image", create_from_image)
            step("builds.create / builds.from_id (deprecated)", builds_deprecated)
        step("create with unknown image", unknown_image)
        if "grouped" in state:
            step("snapshots.create runtime kind refused", runtime_refused)
            step("snapshots.create(filesystem, wait=True)", snapshot_create)
            if "snapshot" in state:
                step("snapshots.from_id / list", snapshot_read)
                step("create(snapshot_id) / create(snapshot_name)", snapshot_restore)
                step("snapshots.delete while creating", snapshot_delete_creating)
                step("snapshots.delete / gone afterwards", snapshot_delete)
            step("snapshots.from_id unknown", snapshot_unknown)
        step("group credentials refused by name", credentials_refused)
        step("delete_group", delete_group)
    step("terminate", terminate)
    step("exec after terminate", exec_after_terminate)
    step("copy_from after terminate", copy_after_terminate)
step("async client core", lambda: asyncio.run(async_core()))
if os.environ.get("SDK_COMPAT_SUSPEND"):
    step("copy_from and exec resume a suspended sandbox", suspend_resume)
for leftover in created:
    try:
        sandboxes.terminate(leftover)
    except Exception as exc:  # noqa: BLE001
        print(f"cleanup: could not terminate {leftover}: {exc}", file=sys.stderr)
for leftover in groups:
    try:
        sandboxes.delete_group(leftover)
    except Exception as exc:  # noqa: BLE001
        print(f"cleanup: could not delete group {leftover}: {exc}", file=sys.stderr)
sys.exit(failures)
