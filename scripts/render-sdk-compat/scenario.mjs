// Drive bex sandboxes with the unmodified Render TypeScript SDK (w32/m284).
// Walks every sandbox method of the client and its typed errors. Prints
// `PASS <step>` / `FAIL <step>: <error>`; exits with the failure count. Every
// sandbox and group it creates is removed at the end. Reads RENDER_API_KEY,
// RENDER_WORKSPACE_ID, BEX_API_URL (bare origin) and, optionally,
// SDK_COMPAT_IMAGE (a public linux/amd64 image to import).
import { randomBytes } from "node:crypto";
import {
  ClientError,
  Render,
  SandboxEndedError,
  SandboxGroupDefaultNotDeletableError,
  SandboxImageNotFoundError,
  SandboxSnapshotNotFoundError,
  SandboxSnapshotNotReadyError,
} from "@renderinc/sdk";

const render = new Render({ baseUrl: process.env.BEX_API_URL });
const sandboxes = render.sandboxes;
const run = randomBytes(3).toString("hex");
const imageSource = process.env.SDK_COMPAT_IMAGE || "docker.io/library/alpine:3.20";
const imageName = `sdk-compat-${run}`;
const denyAll = { type: "deny-all" };
let failures = 0;
const created = new Set();
const groups = new Set();
const state = { name: `sdk-compat-ts-${run}` };

async function step(name, fn) {
  try {
    const detail = await fn();
    console.log(`PASS ${name}${detail ? ` (${detail})` : ""}`);
  } catch (err) {
    failures++;
    console.log(`FAIL ${name}: ${err?.name ?? "Error"}: ${err?.message ?? err}`);
  }
}

async function raises(ErrorClass, fn) {
  try {
    await fn();
  } catch (err) {
    if (err instanceof ErrorClass) return `${err.constructor.name} ${err.statusCode ?? ""} ${err.code ?? ""}`.trim();
    throw err;
  }
  throw new Error(`expected ${ErrorClass.name}`);
}

// SDK v1.8.0 has no not-found class for a sandbox or a file: a plain 404.
async function notFound(fn) {
  const said = await raises(ClientError, fn);
  if (!said.includes(" 404")) throw new Error(said);
  return said;
}

async function runCommand(id, command) {
  let out = "";
  let code;
  for await (const event of await sandboxes.exec(id, command)) {
    if (event.type === "output") out += event.data;
    if (event.type === "exit") code = event.exit_code;
  }
  return { out, code };
}

const text = (download) => new TextDecoder().decode(download.data);
const assert = (ok, detail) => {
  if (!ok) throw new Error(JSON.stringify(detail));
};

await step("create", async () => {
  const box = await sandboxes.create({
    name: state.name,
    plan: "1c-2g",
    timeoutSeconds: 900,
    idleTimeoutSeconds: 300,
    networkPolicy: denyAll,
    env: { SDK_COMPAT_ENV: "env-ok" },
  });
  state.id = box.id;
  created.add(box.id);
  assert(box.name === state.name && box.timeoutSeconds === 900 && box.idleTimeoutSeconds === 300, box);
  assert(box.networkPolicy.type === "deny-all", box.networkPolicy);
  return box.id;
});
if (state.id) {
  await step("waitUntilRunning", async () => {
    const box = await sandboxes.waitUntilRunning(state.id);
    assert(box.status === "running", box.status);
  });
  await step("get", async () => {
    const box = await sandboxes.get(state.id);
    assert(box.name === state.name && box.createdAt && box.sandboxGroupId, box);
    state.defaultGroup = box.sandboxGroupId;
    return `region=${box.region}`;
  });
  await step("list", async () => {
    const ids = (await sandboxes.list({ limit: 100 })).map((it) => it.sandbox.id);
    assert(ids.includes(state.id), ids);
    return `${ids.length} listed`;
  });
  await step("list status filter", async () => {
    const running = (await sandboxes.list({ status: ["running"] })).map((it) => it.sandbox.id);
    const ended = (await sandboxes.list({ status: ["terminated"] })).map((it) => it.sandbox.id);
    assert(running.includes(state.id) && !ended.includes(state.id), { running, ended });
  });
  await step("list name filter", async () => {
    const ids = (await sandboxes.list({ name: state.name })).map((it) => it.sandbox.id);
    assert(ids.length === 1 && ids[0] === state.id, ids);
  });
  await step("exec output and exit code", async () => {
    const { out, code } = await runCommand(state.id, "printf sdk-ok; echo err >&2; exit 3");
    assert(out.includes("sdk-ok") && code === 3, { out, code });
  });
  await step("exec env, HOME and cwd", async () => {
    const { out, code } = await runCommand(state.id, 'printf %s "$SDK_COMPAT_ENV"; printf " %s" "$HOME"; printf " %s" "$PWD"');
    assert(code === 0 && out.startsWith("env-ok "), { out, code });
    return out;
  });
  await step("upload / download", async () => {
    await sandboxes.upload(state.id, "/tmp/sdk-compat-ts.txt", new TextEncoder().encode("payload-ts\n"));
    const got = text(await sandboxes.download(state.id, "/tmp/sdk-compat-ts.txt"));
    assert(got === "payload-ts\n", got);
  });
  await step("upload / download relative path", async () => {
    await sandboxes.upload(state.id, "sdk-compat-rel.txt", "relative\n");
    const { out } = await runCommand(state.id, 'cat "$HOME/sdk-compat-rel.txt"');
    assert(out === "relative\n", out);
    assert(text(await sandboxes.download(state.id, "sdk-compat-rel.txt")) === "relative\n", "download");
    return "under $HOME";
  });
  await step("download missing file", () => notFound(() => sandboxes.download(state.id, "/tmp/does-not-exist-sdk")));
  await step("get unknown sandbox", () => notFound(() => sandboxes.get("sbx-d0000000000000000000")));
  if (state.defaultGroup) {
    await step("listGroups", async () => {
      const list = await sandboxes.listGroups();
      const defaults = list.filter((it) => it.sandboxGroup.isDefault).map((it) => it.sandboxGroup.id);
      assert(defaults.length === 1 && defaults[0] === state.defaultGroup, defaults);
      return `${list.length} groups`;
    });
    await step("setDefaultGroup (current default)", async () => {
      const group = await sandboxes.setDefaultGroup(state.defaultGroup);
      assert(group.id === state.defaultGroup && group.isDefault, group);
    });
    await step("deleteGroup default refused", () =>
      raises(SandboxGroupDefaultNotDeletableError, () => sandboxes.deleteGroup(state.defaultGroup)),
    );
  }
  await step("createGroup", async () => {
    const group = await sandboxes.createGroup({ name: state.name });
    state.group = group.id;
    groups.add(group.id);
    assert(!group.isDefault, group);
    return group.id;
  });
  if (state.group) {
    await step("getGroup", async () => {
      const group = await sandboxes.getGroup(state.group);
      assert(group.id === state.group && group.name === state.name, group);
    });
    await step("updateGroup", async () => {
      const group = await sandboxes.updateGroup(state.group, 3);
      assert(group.concurrencyLimit === 3, group);
    });
    await step("create in group (wait) / list group filter", async () => {
      const box = await sandboxes.create({
        sandboxGroupId: state.group,
        plan: "1c-2g",
        timeoutSeconds: 600,
        networkPolicy: denyAll,
        wait: true,
      });
      created.add(box.id);
      state.grouped = box.id;
      assert(box.sandboxGroupId === state.group && box.status === "running", box);
      const ids = (await sandboxes.list({ sandboxGroupId: state.group })).map((it) => it.sandbox.id);
      assert(ids.length === 1 && ids[0] === box.id, ids);
    });
    await step("list limit and cursor", async () => {
      const first = await sandboxes.list({ limit: 1 });
      const second = await sandboxes.list({ limit: 1, cursor: first[0].cursor });
      assert(first.length === 1 && second.length === 1 && second[0].sandbox.id !== first[0].sandbox.id, { first, second });
    });
    await step("images.import / waitUntilReady / get", async () => {
      const image = await sandboxes.images.import({ sandboxGroupId: state.group, name: imageName, source: imageSource });
      state.image = image.id;
      const ready = await sandboxes.images.waitUntilReady({ sandboxGroupId: state.group, imageId: image.id }, { timeoutMs: 600_000 });
      assert(ready.status === "succeeded", ready);
      const again = await sandboxes.images.get({ sandboxGroupId: state.group, imageId: image.id });
      assert(again.id === image.id, again);
      return image.id;
    });
    if (state.image) {
      await step("create from image", async () => {
        const box = await sandboxes.create({
          sandboxGroupId: state.group,
          image: imageName,
          plan: "1c-2g",
          timeoutSeconds: 600,
          networkPolicy: denyAll,
          wait: true,
        });
        created.add(box.id);
        const { out, code } = await runCommand(box.id, "cat /etc/alpine-release");
        assert(code === 0 && out.trim(), { out, code });
        return `alpine ${out.trim()}`;
      });
      await step("builds.create / builds.get (deprecated)", async () => {
        // The same import again returns the image the group already has.
        const again = await sandboxes.builds.create({ sandboxGroupId: state.group, name: imageName, sourceImage: imageSource });
        assert(again.id === state.image, again);
        const build = await sandboxes.builds.get({ sandboxGroupId: state.group, buildId: state.image });
        assert(build.id === state.image, build);
      });
    }
    await step("create with unknown image", () =>
      raises(SandboxImageNotFoundError, () =>
        sandboxes.create({ sandboxGroupId: state.group, image: `sdk-compat-none-${run}`, networkPolicy: denyAll }),
      ),
    );
    if (state.grouped) {
      const group = state.group;
      const snapshotName = `sdk-compat-${run}`;
      // bex captures the filesystem only until w2/m322; Render's default kind
      // is runtime, so an omitted kind is refused by the same name (ADR018).
      await step("snapshots.create runtime kind refused", async () => {
        const said = [];
        for (const [label, input] of [
          ["default kind", { sandboxId: state.grouped }],
          ["kind runtime", { sandboxId: state.grouped, kind: "runtime" }],
        ]) {
          const got = await raises(ClientError, () => sandboxes.snapshots.create(input));
          assert(/ 400/.test(got), got);
          said.push(`${label}: ${got}`);
        }
        return said.join("; ");
      });
      await step("snapshots.create(filesystem, wait)", async () => {
        const { code } = await runCommand(state.grouped, `printf ${run} > $HOME/sdk-compat-marker.txt`);
        assert(code === 0, code);
        const snap = await sandboxes.snapshots.create({ sandboxId: state.grouped, kind: "filesystem", name: snapshotName, wait: true });
        state.snapshot = snap.id;
        assert(snap.status === "available" && snap.kind === "filesystem" && snap.sizeBytes && snap.capturedAt, snap);
        assert(snap.sandboxGroupId === group && snap.sourceSandboxId === state.grouped, snap);
        const box = await sandboxes.get(state.grouped);
        assert(box.status === "running", box.status);
        return `${snap.id} ${snap.sizeBytes} B, the sandbox kept running`;
      });
      if (state.snapshot) {
        await step("snapshots.get / list", async () => {
          const snap = await sandboxes.snapshots.get({ sandboxGroupId: group, snapshotId: state.snapshot });
          assert(snap.id === state.snapshot && snap.name === snapshotName, snap);
          const listed = (await sandboxes.snapshots.list({ sandboxGroupId: group })).map((it) => it.snapshot.id);
          const available = (await sandboxes.snapshots.list({ sandboxGroupId: group, status: ["available"] })).map((it) => it.snapshot.id);
          assert(listed.includes(state.snapshot) && available.includes(state.snapshot), { listed, available });
          return `${listed.length} listed`;
        });
        await step("create({snapshotId}) / create({snapshotName})", async () => {
          const said = [];
          for (const [label, source] of [
            ["snapshotId", { snapshotId: state.snapshot }],
            ["snapshotName", { snapshotName }],
          ]) {
            const box = await sandboxes.create({ sandboxGroupId: group, timeoutSeconds: 600, wait: true, ...source });
            created.add(box.id);
            const { out, code } = await runCommand(box.id, "cat $HOME/sdk-compat-marker.txt");
            assert(out === run && code === 0, { label, out, code });
            assert(box.networkPolicy.type === "deny-all", box.networkPolicy);
            // The group's concurrency limit is 3 (updateGroup): make room again.
            await sandboxes.terminate(box.id);
            created.delete(box.id);
            said.push(`${label}: marker back`);
          }
          await sandboxes.snapshots.get({ sandboxGroupId: group, snapshotId: state.snapshot });
          return `${said.join("; ")}; the snapshot remains`;
        });
        await step("snapshots.delete while creating", async () => {
          const snap = await sandboxes.snapshots.create({ sandboxId: state.grouped, kind: "filesystem" });
          const said = await raises(SandboxSnapshotNotReadyError, () =>
            sandboxes.snapshots.delete({ sandboxGroupId: group, snapshotId: snap.id }),
          );
          await sandboxes.snapshots.waitUntilAvailable({ sandboxGroupId: group, snapshotId: snap.id });
          await sandboxes.snapshots.delete({ sandboxGroupId: group, snapshotId: snap.id });
          return said;
        });
        await step("snapshots.delete / gone afterwards", async () => {
          const ref = { sandboxGroupId: group, snapshotId: state.snapshot };
          await sandboxes.snapshots.delete(ref);
          await raises(SandboxSnapshotNotFoundError, () => sandboxes.snapshots.get(ref));
          await sandboxes.snapshots.delete(ref); // 204 again
          await raises(SandboxSnapshotNotFoundError, () => sandboxes.create({ sandboxGroupId: group, snapshotId: state.snapshot }));
          const listed = (await sandboxes.snapshots.list({ sandboxGroupId: group })).map((it) => it.snapshot.id);
          assert(!listed.includes(state.snapshot), listed);
          return "reads, restores and lists as gone; a second delete succeeds";
        });
      }
      await step("snapshots.get unknown", () =>
        raises(SandboxSnapshotNotFoundError, () =>
          sandboxes.snapshots.get({ sandboxGroupId: group, snapshotId: "snp-d0000000000000000000" }),
        ),
      );
    }
    await step("deleteGroup", async () => {
      await sandboxes.deleteGroup(state.group);
      groups.delete(state.group);
      for (const id of [...created]) {
        if ((await sandboxes.get(id)).status === "terminated") created.delete(id);
      }
    });
  }
  await step("terminate", async () => {
    await sandboxes.terminate(state.id);
    created.delete(state.id);
    const box = await sandboxes.get(state.id);
    assert(box.status === "terminated" && box.terminatedAt, box);
  });
  // Render's pinned contract answers a mint for an ended sandbox with 409
  // sandbox_not_running; SDK v1.8.0 turns only a 404 into SandboxEndedError.
  const ended = async (fn) => {
    try {
      await fn();
    } catch (err) {
      const ok = err instanceof SandboxEndedError || (err instanceof ClientError && err.statusCode === 409 && /terminated at/.test(err.message));
      assert(ok, { name: err.name, status: err.statusCode, message: err.message });
      return `${err.constructor.name} ${err.statusCode}`;
    }
    throw new Error("an ended sandbox did not refuse");
  };
  await step("exec after terminate", () => ended(() => runCommand(state.id, "true")));
  await step("download after terminate", () => ended(() => sandboxes.download(state.id, "/tmp/sdk-compat-ts.txt")));
}
for (const id of created) {
  await sandboxes.terminate(id).catch((err) => console.error(`cleanup: ${id}: ${err.message}`));
}
for (const id of groups) {
  await sandboxes.deleteGroup(id).catch((err) => console.error(`cleanup: group ${id}: ${err.message}`));
}
process.exit(failures);
