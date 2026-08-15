/*
 * Copyright 2026.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import assert from "node:assert/strict";
import { generateKeyPairSync, randomUUID, sign } from "node:crypto";
import { execFileSync } from "node:child_process";
import {
  chmod,
  mkdtemp,
  mkdir,
  open,
  readFile,
  writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { loadConfig, type AgentDriverConfig } from "../src/config.js";
import { createCredentialManager, type CredentialManager } from "../src/credentials.js";
import { runHeadlessTurn } from "../src/session.js";
import { startDriverServer } from "../src/server.js";
import { UIMessageStreamHub } from "../src/stream-hub.js";

const here = path.dirname(fileURLToPath(import.meta.url));
const fixture = path.join(here, "..", "fixtures", "acp-agent.mjs");
const grantKeys = generateKeyPairSync("ed25519");
const grantPublicKey = (grantKeys.publicKey.export({ format: "jwk" }) as JsonWebKey).x!;

function turnGrant(sessionID = "ags-test"): string {
  const now = Math.floor(Date.now() / 1000);
  const body = Buffer.from(JSON.stringify({
    ses: sessionID, act: "turn", iat: now, exp: now + 30, jti: randomUUID(),
  })).toString("base64url");
  return `${body}.${sign(null, Buffer.from(body), grantKeys.privateKey).toString("base64url")}`;
}

type TestConfig = AgentDriverConfig & { root: string };

async function tempConfig(
  overrides: Partial<TestConfig> = {},
): Promise<TestConfig> {
  const root = await mkdtemp(path.join(tmpdir(), "bex-agent-driver-"));
  const workspace = path.join(root, "workspace");
  await mkdir(workspace);
  return {
    command: process.execPath,
    args: [fixture],
    cwd: workspace,
    prompt: "Make the requested change and commit it.",
    branch: "",
    repoUrl: "",
    baseBranch: "",
    deliver: false,
    gitName: "bex agent",
    gitEmail: "agent@bex.co",
    existingSessionId: "",
    persistSession: true,
    listenHost: "127.0.0.1",
    listenPort: 0,
    sessionLogPath: path.join(root, "session.jsonl"),
    statusPath: path.join(root, "status.json"),
    exitAfterTurn: false,
    turnTimeoutMs: 1_000,
    credentialEnvName: "ANTHROPIC_API_KEY",
    modelCredential: "test-model-key-never-log",
    sessionID: "ags-test",
    grantPublicKey,
    agentEnv: {},
    scrubRoots: [workspace],
    root,
    ...overrides,
  };
}

function manager(config: AgentDriverConfig): CredentialManager {
  return createCredentialManager(config, {});
}

test("configuration accepts only operator-owned adapter paths", () => {
  const config = loadConfig({
    BEX_AGENT_COMMAND: "/usr/local/bin/gemini",
    BEX_AGENT_ARGS: '["--experimental-acp"]',
    BEX_AGENT_CWD: "/tmp/repo",
    BEX_AGENT_LISTEN_PORT: "9000",
    BEX_AGENT_ENV_JSON: '{"MODE":"auto"}',
  });
  assert.equal(config.command, "/usr/local/bin/gemini");
  assert.deepEqual(config.args, ["--experimental-acp"]);
  assert.deepEqual(config.agentEnv, { MODE: "auto" });
  assert.deepEqual(config.scrubRoots, [
    "/tmp/repo",
    "/home/bex",
    "/tmp",
    "/var/tmp",
    "/var/log/bex-agent",
    "/var/run/bex-agent",
  ]);
  assert.throws(() => loadConfig({ BEX_AGENT_ARGS: "--flag" }), /JSON array/);
  assert.throws(
    () => loadConfig({ BEX_AGENT_COMMAND: "./tenant-adapter" }),
    /installed agent adapter path/,
  );
  assert.throws(
    () => loadConfig({ BEX_AGENT_COMMAND: "/workspace/tenant-adapter" }),
    /installed agent adapter path/,
  );
  assert.throws(
    () =>
      loadConfig({
        BEX_AGENT_ENV_JSON: '{"ANTHROPIC_API_KEY":"bypass"}',
      }),
    /model credentials must use BEX_AGENT_MODEL_API_KEY/,
  );
});

test("model credential routes to the agent-native env var by shape", () => {
  // An Anthropic OAuth token (`sk-ant-oat…`, from `claude setup-token`) must
  // reach claude-code as CLAUDE_CODE_OAUTH_TOKEN — delivering it as the x-api-key
  // ANTHROPIC_API_KEY is rejected upstream as "Invalid API key".
  assert.equal(
    loadConfig({ BEX_AGENT_MODEL_API_KEY: "sk-ant-oat01-abc" }).credentialEnvName,
    "CLAUDE_CODE_OAUTH_TOKEN",
  );
  // A conventional API key keeps the x-api-key default.
  assert.equal(
    loadConfig({ BEX_AGENT_MODEL_API_KEY: "sk-ant-api03-abc" }).credentialEnvName,
    "ANTHROPIC_API_KEY",
  );
  assert.equal(loadConfig({}).credentialEnvName, "ANTHROPIC_API_KEY");
  // An explicit override always wins over shape detection.
  assert.equal(
    loadConfig({
      BEX_AGENT_MODEL_API_KEY: "sk-ant-oat01-abc",
      BEX_AGENT_MODEL_API_KEY_ENV: "GEMINI_API_KEY",
    }).credentialEnvName,
    "GEMINI_API_KEY",
  );
});

test("one headless turn streams raw ACP data and commits in the worktree", async () => {
  const config = await tempConfig();
  execFileSync("git", ["init", "-q"], { cwd: config.cwd });
  execFileSync("git", ["config", "user.name", "bex test"], { cwd: config.cwd });
  execFileSync("git", ["config", "user.email", "bex-test@example.invalid"], {
    cwd: config.cwd,
  });
  await writeFile(path.join(config.cwd, "README.md"), "fixture\n");
  execFileSync("git", ["add", "README.md"], { cwd: config.cwd });
  execFileSync("git", ["commit", "-q", "-m", "initial"], { cwd: config.cwd });
  config.agentEnv = {
    ACP_FIXTURE_COMMIT_REPO: config.cwd,
    ACP_FIXTURE_REQUIRE_MODEL_KEY: "1",
  };

  const hub = new UIMessageStreamHub();
  const result = await runHeadlessTurn(config, manager(config), hub);
  assert.equal(result.state, "succeeded");
  assert.match(
    execFileSync("git", ["log", "-1", "--format=%s"], {
      cwd: config.cwd,
    }).toString(),
    /agent: complete task/,
  );
  // The ACP updates map to typed UI-message chunks — no generic `data-acp`
  // re-wrap, and no synthetic single-tool collapse.
  const types = hub.history.map((part) => part.type);
  assert.ok(types.includes("data-acp-plan"), "plan maps to a typed data part");
  assert.ok(types.includes("data-acp-diff"), "diff maps to a typed data part");
  assert.ok(types.includes("data-acp-terminal"), "terminal maps to a typed data part");
  assert.ok(!types.includes("data-acp"), "the generic data-acp re-wrap is gone");
  // The tool call rides a real dynamic tool part carrying its true title, not
  // the provider's old `acp_provider_agent_dynamic_tool` collapse.
  const toolStart = hub.history.find((part) => part.type === "tool-input-start");
  assert.equal(toolStart?.toolName, "Edit fixture");
  const serialized = JSON.stringify(hub.history);
  assert.doesNotMatch(serialized, /acp_provider_agent_dynamic_tool/);
  const log = await readFile(config.sessionLogPath, "utf8");
  assert.match(log, /ui-message/);
  assert.doesNotMatch(log, /test-model-key-never-log/);
});

// codex r7 #4 — the model credential must be redacted BEFORE the first
// fan-out: hub history (GET /stream attachers), the onPart mirror (the POST
// /turn response the gateway tees into the durable transcript), and the
// session log must all carry the sanitized representation.
test("credential emitted by the agent never reaches hub, mirror, or log", async () => {
  const config = await tempConfig();
  config.agentEnv = { ACP_FIXTURE_LEAK_CREDENTIAL: "1" };
  const hub = new UIMessageStreamHub();
  const mirrored: string[] = [];
  const result = await runHeadlessTurn(config, manager(config), hub, {
    onPart: (part) => mirrored.push(JSON.stringify(part)),
  });
  assert.equal(result.state, "succeeded");
  for (const [sink, text] of [
    ["hub history", JSON.stringify(hub.history)],
    ["turn mirror", mirrored.join("\n")],
    ["session log", await readFile(config.sessionLogPath, "utf8")],
  ]) {
    assert.doesNotMatch(text, /test-model-key-never-log/, `${sink} leaked the credential`);
    // The leak must have actually happened for the assertion above to mean
    // anything — the sanitized marker proves the fixture emitted the key.
    assert.match(text, /\[REDACTED\]/, `${sink} shows no redaction marker`);
  }
});

test("redactPart sanitizes nested structured parts and fails closed", async () => {
  const config = await tempConfig();
  const credentials = manager(config);
  const part = {
    type: "data-acp",
    data: { rawOutput: { stdout: `key=${config.modelCredential}` } },
  };
  const sanitized = credentials.redactPart(part);
  assert.doesNotMatch(JSON.stringify(sanitized), /test-model-key-never-log/);
  assert.match(JSON.stringify(sanitized), /\[REDACTED\]/);
  assert.doesNotMatch(JSON.stringify(part), /REDACTED/, "input part must not be mutated");
  // A clean part passes through untouched (same reference, no re-parse cost).
  const clean = { type: "text", text: "no secrets here" };
  assert.equal(credentials.redactPart(clean), clean);
  // A secret whose removal would corrupt the JSON document (here it overlaps
  // the structural `":"` between key and value) fails closed to a placeholder
  // part rather than ever returning the raw representation.
  const hostile = manager(await tempConfig({ modelCredential: '":"' }));
  assert.deepEqual(hostile.redactPart({ a: "x" }), { type: "data-redacted" });
});

for (const loadSession of [false, true]) {
  test(`existing session ${loadSession ? "uses" : "does not use"} session/load when advertised=${loadSession}`, async () => {
    const config = await tempConfig({ existingSessionId: "existing-session" });
    const callLog = path.join(config.root, "calls.log");
    config.agentEnv = {
      ACP_FIXTURE_LOAD_SESSION: loadSession ? "1" : "0",
      ACP_FIXTURE_LOG: callLog,
    };
    const result = await runHeadlessTurn(
      config,
      manager(config),
      new UIMessageStreamHub(),
    );
    const calls = (await readFile(callLog, "utf8")).trim().split("\n");
    assert.equal(calls.includes("session/load"), loadSession);
    assert.equal(calls.includes("session/new"), !loadSession);
    assert.equal(result.resume, loadSession ? "loaded" : "unsupported");
    assert.equal(
      JSON.stringify(result.usage).match(/\d+/g),
      null,
      "ACP usage stays empty",
    );
  });
}

// w3/m42 t003 — resume after hibernation: a restarted driver on a restored
// rootfs adopts the prior turn's persisted ACP session id and re-attaches via
// session/load only when the agent advertises loadSession.
for (const loadSession of [false, true]) {
  test(`rootfs-persisted session id is adopted after resume (loadSession=${loadSession})`, async () => {
    const config = await tempConfig();
    await writeFile(
      config.statusPath,
      `${JSON.stringify({ state: "succeeded", sessionId: "prior-session", resume: "new" })}\n`,
    );
    const callLog = path.join(config.root, "calls.log");
    config.agentEnv = {
      ACP_FIXTURE_LOAD_SESSION: loadSession ? "1" : "0",
      ACP_FIXTURE_LOG: callLog,
    };
    const result = await runHeadlessTurn(
      config,
      manager(config),
      new UIMessageStreamHub(),
    );
    const calls = (await readFile(callLog, "utf8")).trim().split("\n");
    assert.equal(calls.includes("session/load"), loadSession);
    assert.equal(result.resume, loadSession ? "loaded" : "unsupported");
    assert.equal(result.resumedFrom, "rootfs");
    const persisted = JSON.parse(await readFile(config.statusPath, "utf8"));
    assert.equal(persisted.resumedFrom, "rootfs");
  });
}

test("environment session id takes precedence over the rootfs status file", async () => {
  const config = await tempConfig({ existingSessionId: "env-session" });
  await writeFile(
    config.statusPath,
    `${JSON.stringify({ state: "succeeded", sessionId: "stale-rootfs-session" })}\n`,
  );
  config.agentEnv = { ACP_FIXTURE_LOAD_SESSION: "1" };
  const result = await runHeadlessTurn(
    config,
    manager(config),
    new UIMessageStreamHub(),
  );
  assert.equal(result.resume, "loaded");
  assert.equal(result.resumedFrom, "env");
  assert.equal(config.existingSessionId, "env-session");
});

test("a failed prior turn adopts nothing and starts a fresh session", async () => {
  const config = await tempConfig();
  await writeFile(
    config.statusPath,
    `${JSON.stringify({ state: "failed", error: "boom" })}\n`,
  );
  const callLog = path.join(config.root, "calls.log");
  config.agentEnv = {
    ACP_FIXTURE_LOAD_SESSION: "1",
    ACP_FIXTURE_LOG: callLog,
  };
  const result = await runHeadlessTurn(
    config,
    manager(config),
    new UIMessageStreamHub(),
  );
  const calls = (await readFile(callLog, "utf8")).trim().split("\n");
  assert.equal(calls.includes("session/new"), true);
  assert.equal(calls.includes("session/load"), false);
  assert.equal(result.resume, "new");
  assert.equal(result.resumedFrom, undefined);
});

test("a corrupt status file adopts nothing", async () => {
  const config = await tempConfig();
  await writeFile(config.statusPath, "not-json{");
  const result = await runHeadlessTurn(
    config,
    manager(config),
    new UIMessageStreamHub(),
  );
  assert.equal(result.resume, "new");
  assert.equal(result.resumedFrom, undefined);
});

test("agent crash becomes a failed status instead of hanging", async () => {
  const config = await tempConfig({ agentEnv: { ACP_FIXTURE_CRASH: "1" } });
  await assert.rejects(
    runHeadlessTurn(config, manager(config), new UIMessageStreamHub()),
  );
  const status = JSON.parse(await readFile(config.statusPath, "utf8"));
  assert.equal(status.state, "failed");
});

test("agent initialize stall is bounded by the turn timeout", async () => {
  const config = await tempConfig({
    agentEnv: { ACP_FIXTURE_HANG_INITIALIZE: "1" },
    turnTimeoutMs: 50,
  });
  await assert.rejects(
    runHeadlessTurn(config, manager(config), new UIMessageStreamHub()),
    /turn exceeded|connect timed out/,
  );
  const status = JSON.parse(await readFile(config.statusPath, "utf8"));
  assert.equal(status.state, "failed");
});

test("SSE replays the standard UI-message stream and headless needs no client", async () => {
  const config = await tempConfig();
  const credentials = manager(config);
  const hub = new UIMessageStreamHub();
  const listener = await startDriverServer(config, credentials, hub);
  try {
    await runHeadlessTurn(config, credentials, hub);
    const response = await fetch(
      `http://127.0.0.1:${(listener.address as { port: number }).port}/stream`,
    );
    assert.equal(response.headers.get("x-vercel-ai-ui-message-stream"), "v1");
    const body = await response.text();
    assert.match(body, /data: .*"type":"data-acp-plan"/);
    assert.match(body, /data: \[DONE\]/);
  } finally {
    await listener.close();
  }
});

test("POST /turn runs a live turn, streams its parts, and single-flights", async () => {
  const config = await tempConfig();
  const credentials = manager(config);
  const hub = new UIMessageStreamHub();
  const runTurn = (prompt: string, onPart: (part: Record<string, unknown>) => void) =>
    runHeadlessTurn(config, credentials, hub, { prompt, closeHub: false, onPart });
  const listener = await startDriverServer(config, credentials, hub, { runTurn });
  try {
    const turnURL = `http://127.0.0.1:${(listener.address as { port: number }).port}/turn`;

    // Same-Pod code can reach localhost but cannot launch a model-key-bearing
    // agent without the gateway's signed, action-bound grant.
    const unauthorized = await fetch(turnURL, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ prompt: "steal the model key" }),
    });
    assert.equal(unauthorized.status, 401);
    await unauthorized.text();

    // A missing prompt is a 400.
    const empty = await fetch(turnURL, {
      method: "POST",
      headers: { "content-type": "application/json", "x-bex-driver-grant": turnGrant() },
      body: "{}",
    });
    assert.equal(empty.status, 400);
    await empty.text();

    // A live turn streams UI-message parts (incl. the mapped data-acp part) and
    // terminates with [DONE]; the hub is NOT closed, so the session stays live.
    const response = await fetch(turnURL, {
      method: "POST",
      headers: { "content-type": "application/json", "x-bex-driver-grant": turnGrant() },
      body: JSON.stringify({ messages: [{ role: "user", parts: [{ type: "text", text: "make the change" }] }] }),
    });
    assert.equal(response.status, 200);
    assert.equal(response.headers.get("x-vercel-ai-ui-message-stream"), "v1");
    const body = await response.text();
    assert.match(body, /data: .*"type":"data-acp-plan"/);
    assert.match(body, /data: \[DONE\]/);

    // The same parts reached the hub's history (attached GET clients see them),
    // and the stream is still open for another turn.
    assert.ok(hub.history.some((part) => part.type === "data-acp-plan"));
    const second = await fetch(turnURL, {
      method: "POST",
      headers: { "content-type": "application/json", "x-bex-driver-grant": turnGrant() },
      body: JSON.stringify({ prompt: "another turn" }),
    });
    assert.equal(second.status, 200);
    await second.text();

    const replayGrant = turnGrant();
    const accepted = await fetch(turnURL, {
      method: "POST",
      headers: { "content-type": "application/json", "x-bex-driver-grant": replayGrant },
      body: JSON.stringify({ prompt: "one use" }),
    });
    assert.equal(accepted.status, 200);
    await accepted.text();
    const replayed = await fetch(turnURL, {
      method: "POST",
      headers: { "content-type": "application/json", "x-bex-driver-grant": replayGrant },
      body: JSON.stringify({ prompt: "replay" }),
    });
    assert.equal(replayed.status, 401);
    await replayed.text();
  } finally {
    await listener.close();
  }
});

test("raw ACP launch route is absent", async () => {
  const config = await tempConfig();
  const listener = await startDriverServer(
    config,
    manager(config),
    new UIMessageStreamHub(),
  );
  try {
    const response = await fetch(
      `http://127.0.0.1:${(listener.address as { port: number }).port}/acp`,
      { method: "POST" },
    );
    assert.equal(response.status, 404);
  } finally {
    await listener.close();
  }
});

test("snapshot scrub removes an injected key from persisted files and parent env", async () => {
  const config = await tempConfig();
  const leaked = path.join(config.cwd, "accidental-agent-cache");
  await writeFile(leaked, `prefix ${config.modelCredential} suffix`);
  const env: NodeJS.ProcessEnv = {
    BEX_AGENT_MODEL_API_KEY: config.modelCredential,
    ANTHROPIC_API_KEY: config.modelCredential,
  };
  const credentials = createCredentialManager(config, env);
  assert.equal(env.BEX_AGENT_MODEL_API_KEY, undefined);
  assert.equal(env.ANTHROPIC_API_KEY, undefined);
  assert.equal(
    credentials.redact(`error: ${config.modelCredential}`),
    "error: [REDACTED]",
  );
  assert.deepEqual(await credentials.scrubPersistedState(), [leaked]);
  assert.equal(
    (await readFile(leaked, "utf8")).includes(config.modelCredential),
    false,
  );
  assert.match(await readFile(leaked, "utf8"), /\[REDACTED\]/);
});

test("snapshot scrub preserves binary bytes and blocks an oversized credential leak", async () => {
  const config = await tempConfig();
  const binaryLeak = path.join(config.cwd, "binary-cache");
  await writeFile(
    binaryLeak,
    Buffer.concat([
      Buffer.from([0xff, 0x00]),
      Buffer.from(config.modelCredential),
      Buffer.from([0xfe, 0x01]),
    ]),
  );
  const credentials = manager(config);
  assert.deepEqual(await credentials.scrubPersistedState(), [binaryLeak]);
  const scrubbed = await readFile(binaryLeak);
  assert.deepEqual(scrubbed.subarray(0, 2), Buffer.from([0xff, 0x00]));
  assert.deepEqual(scrubbed.subarray(-2), Buffer.from([0xfe, 0x01]));

  const oversizedLeak = path.join(config.cwd, "oversized-cache");
  const file = await open(oversizedLeak, "w");
  try {
    const offset = 33 * 1024 * 1024;
    const needle = Buffer.from(config.modelCredential);
    await file.truncate(offset);
    await file.write(needle, 0, needle.length, offset);
  } finally {
    await file.close();
  }
  await assert.rejects(
    credentials.scrubPersistedState(),
    /credential found in oversized persisted file/,
  );
  await writeFile(oversizedLeak, "safe");

  const inaccessibleLeak = path.join(config.cwd, "inaccessible-cache");
  await writeFile(inaccessibleLeak, config.modelCredential, { mode: 0o600 });
  await chmod(inaccessibleLeak, 0o000);
  await assert.rejects(
    credentials.scrubPersistedState(),
    /cannot inspect agent-writable persisted path/,
  );
});

test("loopback snapshot hook scrubs persisted key material and forgets it", async () => {
  const config = await tempConfig();
  const leaked = path.join(config.cwd, "agent-cache");
  await writeFile(leaked, `cached=${config.modelCredential}\n`);
  const credentials = manager(config);
  const listener = await startDriverServer(
    config,
    credentials,
    new UIMessageStreamHub(),
  );
  try {
    const response = await fetch(
      `http://127.0.0.1:${(listener.address as { port: number }).port}/snapshot/scrub`,
      { method: "POST" },
    );
    assert.equal(response.status, 200);
    assert.deepEqual(await response.json(), { scrubbedFiles: 1 });
    assert.equal(credentials.configured(), false);
    assert.doesNotMatch(
      await readFile(leaked, "utf8"),
      /test-model-key-never-log/,
    );
  } finally {
    await listener.close();
  }
});
