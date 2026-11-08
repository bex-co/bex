// Drive bex sandboxes with the unmodified Render TypeScript SDK (w32/m284).
// Prints `PASS <step>` / `FAIL <step>: <error>`; exits with the failure count.
// Reads RENDER_API_KEY, RENDER_WORKSPACE_ID and BEX_API_URL (bare origin).
import { Render } from "@renderinc/sdk";

const render = new Render({ baseUrl: process.env.BEX_API_URL });
const sandboxes = render.sandboxes;
let failures = 0;
const created = [];
const state = {};

async function step(name, fn) {
  try {
    const detail = await fn();
    console.log(`PASS ${name}${detail ? ` (${detail})` : ""}`);
  } catch (err) {
    failures++;
    console.log(`FAIL ${name}: ${err?.name ?? "Error"}: ${err?.message ?? err}`);
  }
}

async function run(command) {
  let out = "";
  let code;
  for await (const event of await sandboxes.exec(state.id, command)) {
    if (event.type === "output") out += event.data;
    if (event.type === "exit") code = event.exit_code;
  }
  return { out, code };
}

await step("create", async () => {
  const box = await sandboxes.create({ plan: "1c-2g", timeoutSeconds: 900, networkPolicy: { type: "deny-all" } });
  state.id = box.id;
  created.push(box.id);
  return box.id;
});
if (state.id) {
  await step("waitUntilRunning", async () => {
    const box = await sandboxes.waitUntilRunning(state.id);
    if (box.status !== "running") throw new Error(box.status);
  });
  await step("get", async () => `region=${(await sandboxes.get(state.id)).region}`);
  await step("list", async () => {
    const ids = (await sandboxes.list({ limit: 100 })).map((it) => it.sandbox.id);
    if (!ids.includes(state.id)) throw new Error(`not listed: ${ids}`);
    return `${ids.length} listed`;
  });
  await step("exec output and exit code", async () => {
    const { out, code } = await run("printf sdk-ok; exit 3");
    if (!out.includes("sdk-ok") || code !== 3) throw new Error(JSON.stringify({ out, code }));
  });
  await step("upload / download", async () => {
    await sandboxes.upload(state.id, "/tmp/sdk-compat-ts.txt", new TextEncoder().encode("payload-ts\n"));
    const got = await sandboxes.download(state.id, "/tmp/sdk-compat-ts.txt");
    const text = new TextDecoder().decode(got.data);
    if (text !== "payload-ts\n") throw new Error(JSON.stringify(text));
  });
  await step("terminate", async () => {
    await sandboxes.terminate(state.id);
    created.splice(created.indexOf(state.id), 1);
  });
}
for (const id of created) {
  await sandboxes.terminate(id).catch((err) => console.error(`cleanup: ${id}: ${err.message}`));
}
process.exit(failures);
