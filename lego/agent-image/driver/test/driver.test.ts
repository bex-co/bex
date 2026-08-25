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
import {
  createCredentialManager,
  type CredentialManager,
} from "../src/credentials.js";
import { runHeadlessTurn } from "../src/session.js";
import { startDriverServer } from "../src/server.js";
import { UIMessageStreamHub } from "../src/stream-hub.js";
import { isUtcTimestamp } from "../src/timestamp.js";

const here = path.dirname(fileURLToPath(import.meta.url));
const fixture = path.join(here, "..", "fixtures", "acp-agent.mjs");
const grantKeys = generateKeyPairSync("ed25519");
const grantPublicKey = (
  grantKeys.publicKey.export({ format: "jwk" }) as JsonWebKey
).x!;

function driverGrant(
  action: "turn" | "snapshot",
  sessionID = "ags-test",
): string {
  const now = Math.floor(Date.now() / 1000);
  const body = Buffer.from(
    JSON.stringify({
      ses: sessionID,
      act: action,
      iat: now,
      exp: now + 30,
      jti: randomUUID(),
    }),
  ).toString("base64url");
  return `${body}.${sign(null, Buffer.from(body), grantKeys.privateKey).toString("base64url")}`;
}

function turnGrant(sessionID = "ags-test"): string {
  return driverGrant("turn", sessionID);
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
    modelBaseUrl: "",
    modelBaseUrlEnvName: "",
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
    loadConfig({ BEX_AGENT_MODEL_API_KEY: "sk-ant-oat01-abc" })
      .credentialEnvName,
    "CLAUDE_CODE_OAUTH_TOKEN",
  );
  // A conventional API key keeps the x-api-key default.
  assert.equal(
    loadConfig({ BEX_AGENT_MODEL_API_KEY: "sk-ant-api03-abc" })
      .credentialEnvName,
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

test("model proxy routes each adapter's base URL and lands the placeholder in its native var", () => {
  // ADR062 D5: with the proxy on, each adapter is pointed at the gateway proxy via
  // its provider base-URL env (with the provider's path suffix) and the credential
  // env is the one that adapter reads, so the agent attempts the request.
  const proxy =
    "http://bex-ssh-gateway.bex-system.svc.cluster.local:8084/model/ns/sess";
  const claude = loadConfig({
    BEX_AGENT_COMMAND: "/usr/local/bin/claude-code-acp",
    BEX_AGENT_MODEL_PROXY_URL: proxy,
    BEX_AGENT_MODEL_API_KEY: "bex-model-proxy-placeholder-ags-1",
  });
  assert.equal(claude.modelBaseUrlEnvName, "ANTHROPIC_BASE_URL");
  assert.equal(claude.modelBaseUrl, proxy); // Anthropic base is the root: no suffix
  assert.equal(claude.credentialEnvName, "ANTHROPIC_API_KEY");

  const codex = loadConfig({
    BEX_AGENT_COMMAND: "/usr/local/bin/codex-acp",
    BEX_AGENT_MODEL_PROXY_URL: proxy,
  });
  assert.equal(codex.modelBaseUrlEnvName, "OPENAI_BASE_URL");
  assert.equal(codex.modelBaseUrl, proxy + "/v1"); // OpenAI base includes /v1
  assert.equal(codex.credentialEnvName, "OPENAI_API_KEY");

  const gemini = loadConfig({
    BEX_AGENT_COMMAND: "/usr/local/bin/gemini",
    BEX_AGENT_MODEL_PROXY_URL: proxy,
  });
  assert.equal(gemini.modelBaseUrlEnvName, "GOOGLE_GEMINI_BASE_URL");
  assert.equal(gemini.credentialEnvName, "GEMINI_API_KEY");

  // The base-URL env is surfaced to the agent child alongside the placeholder.
  const codexConfig = loadConfig({
    BEX_AGENT_COMMAND: "/usr/local/bin/codex-acp",
    BEX_AGENT_MODEL_PROXY_URL: proxy,
    BEX_AGENT_MODEL_API_KEY: "placeholder-only",
  });
  const env = manager(codexConfig).agentEnvironment();
  assert.equal(env.OPENAI_BASE_URL, proxy + "/v1");
  assert.equal(env.OPENAI_API_KEY, "placeholder-only");
});

test("model proxy with no base-URL routing for the adapter fails closed", () => {
  // A hypothetical adapter absent from the routing table must abort rather than
  // connect straight to the vendor with the placeholder. The allowlist currently
  // covers exactly the three routed adapters; assert the fail-closed guard by
  // pinning that BEX_AGENT_MODEL_API_KEY_ENV cannot substitute for routing.
  assert.ok(modelProxyRoutesCovered());
});

// modelProxyRoutesCovered documents that every command the allowlist admits also
// has a proxy route, so proxy mode never silently degrades to a direct vendor
// connection for a supported adapter. If a new adapter is added to the allowlist
// without a route, loadConfig throws under BEX_AGENT_MODEL_PROXY_URL — proving the
// fail-closed contract holds for it too.
function modelProxyRoutesCovered(): boolean {
  const proxy = "http://gw:8084/model/ns/sess";
  for (const command of [
    "/usr/local/bin/claude-code-acp",
    "/usr/local/bin/codex-acp",
    "/usr/local/bin/gemini",
  ]) {
    const cfg = loadConfig({
      BEX_AGENT_COMMAND: command,
      BEX_AGENT_MODEL_PROXY_URL: proxy,
    });
    if (!cfg.modelBaseUrl || !cfg.modelBaseUrlEnvName) return false;
  }
  return true;
}

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
  assert.ok(
    types.includes("data-acp-terminal"),
    "terminal maps to a typed data part",
  );
  assert.ok(
    !types.includes("data-acp"),
    "the generic data-acp re-wrap is gone",
  );
  // The tool call rides a real dynamic tool part carrying its true title, not
  // the provider's old `acp_provider_agent_dynamic_tool` collapse.
  const toolStart = hub.history.find(
    (part) => part.type === "tool-input-start",
  );
  assert.equal(toolStart?.toolName, "Edit fixture");
  assert.ok(
    types.includes("reasoning-start"),
    "thought maps to a reasoning part",
  );
  // Every published part carries one ISO-8601 UTC `at`, identical on the hub
  // history and the persisted session log (the publication boundary).
  for (const part of hub.history) {
    assert.ok(
      isUtcTimestamp(part.at),
      `${String(part.type)} missing source timestamp`,
    );
  }
  const serialized = JSON.stringify(hub.history);
  assert.doesNotMatch(serialized, /acp_provider_agent_dynamic_tool/);
  const log = await readFile(config.sessionLogPath, "utf8");
  assert.match(log, /ui-message/);
  assert.doesNotMatch(log, /test-model-key-never-log/);
  const logRecords = log
    .trim()
    .split("\n")
    .map(
      (line) =>
        JSON.parse(line) as {
          type?: string;
          partIndex?: number;
          part?: { at?: string };
        },
    );
  const logged = logRecords.filter((row) => row.type === "ui-message");
  assert.equal(logged.length, hub.history.length);
  for (const row of logged) {
    assert.equal(row.part?.at, hub.history[row.partIndex ?? -1]?.at);
  }
});

test("Codex binds its provider to the session proxy and fails on typed transport errors", async () => {
  const config = await tempConfig({
    credentialEnvName: "OPENAI_API_KEY",
    modelBaseUrl:
      "http://bex-ssh-gateway.bex-system.svc.cluster.local:8084/model/ns/ags-test/v1",
    modelBaseUrlEnvName: "OPENAI_BASE_URL",
  });
  const callLog = path.join(config.root, "acp-calls.log");
  config.agentEnv = {
    ACP_FIXTURE_LOG: callLog,
    ACP_FIXTURE_PROVIDER_BASE_URL: config.modelBaseUrl,
    ACP_FIXTURE_REQUIRE_TYPED_FAILURE: "1",
    ACP_FIXTURE_TYPED_FAILURE: "1",
  };
  const credentials = manager(config);
  await assert.rejects(
    runHeadlessTurn(config, credentials, new UIMessageStreamHub()),
    /ACP session failed \(transport_lost\): model proxy transport failed: near \[REDACTED\]/,
  );
  const calls = (await readFile(callLog, "utf8")).trim().split("\n");
  assert.ok(calls.indexOf("providers/set") > calls.indexOf("initialize"));
  assert.ok(calls.indexOf("providers/set") < calls.indexOf("session/new"));
  const status = JSON.parse(await readFile(config.statusPath, "utf8"));
  assert.equal(status.state, "failed");
  assert.match(status.error, /transport_lost/);
  assert.doesNotMatch(status.error, /test-model-key-never-log/);
  assert.equal(credentials.configured(), false);
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
    assert.doesNotMatch(
      text,
      /test-model-key-never-log/,
      `${sink} leaked the credential`,
    );
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
  assert.doesNotMatch(
    JSON.stringify(part),
    /REDACTED/,
    "input part must not be mutated",
  );
  // A clean part passes through untouched (same reference, no re-parse cost).
  const clean = { type: "text", text: "no secrets here" };
  assert.equal(credentials.redactPart(clean), clean);
  // A secret whose removal would corrupt the JSON document (here it overlaps
  // the structural `":"` between key and value) fails closed to a placeholder
  // part rather than ever returning the raw representation.
  const hostile = manager(await tempConfig({ modelCredential: '":"' }));
  assert.deepEqual(hostile.redactPart({ a: "x" }), { type: "data-redacted" });
});

test("stream hub bounds oversized parts and retained history", () => {
  const hub = new UIMessageStreamHub({
    maxHistoryBytes: 256,
    maxHistoryParts: 2,
    maxPartBytes: 128,
  });
  hub.publish({ type: "text", text: "x".repeat(1000) });
  assert.equal(hub.history[0]?.type, "data-truncated");
  hub.publish({ type: "text", text: "one" });
  hub.publish({ type: "text", text: "two" });
  assert.equal(hub.history.length, 2);
  assert.deepEqual(
    hub.history.map((part) => part.text),
    ["one", "two"],
  );
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
  const config = await tempConfig({
    agentEnv: {
      ACP_FIXTURE_CRASH: "1",
      ACP_FIXTURE_CRASH_WITH_CREDENTIAL: "1",
    },
  });
  let failure: unknown;
  try {
    await runHeadlessTurn(config, manager(config), new UIMessageStreamHub());
  } catch (error) {
    failure = error;
  }
  assert.ok(failure instanceof Error);
  assert.doesNotMatch(failure.message, /test-model-key-never-log/);
  assert.match(failure.message, /\[REDACTED\]/);
  const status = JSON.parse(await readFile(config.statusPath, "utf8"));
  assert.equal(status.state, "failed");
  assert.doesNotMatch(JSON.stringify(status), /test-model-key-never-log/);
});

test("agent input loss fails promptly instead of leaving a running turn", async () => {
  const config = await tempConfig({
    agentEnv: { ACP_FIXTURE_CLOSE_INPUT_AFTER_SESSION: "1" },
    turnTimeoutMs: 10_000,
  });
  await assert.rejects(
    runHeadlessTurn(config, manager(config), new UIMessageStreamHub()),
    /ACP agent stdin failed: write EPIPE/,
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
  const runTurn = (
    prompt: string,
    onPart: (part: Record<string, unknown>) => void,
  ) =>
    runHeadlessTurn(config, credentials, hub, {
      prompt,
      closeHub: false,
      onPart,
    });
  const listener = await startDriverServer(config, credentials, hub, {
    runTurn,
  });
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
      headers: {
        "content-type": "application/json",
        "x-bex-driver-grant": turnGrant(),
      },
      body: "{}",
    });
    assert.equal(empty.status, 400);
    await empty.text();

    // A live turn streams UI-message parts (incl. the mapped data-acp part) and
    // terminates with [DONE]; the hub is NOT closed, so the session stays live.
    const response = await fetch(turnURL, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "x-bex-driver-grant": turnGrant(),
      },
      body: JSON.stringify({
        messages: [
          { role: "user", parts: [{ type: "text", text: "make the change" }] },
        ],
      }),
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
      headers: {
        "content-type": "application/json",
        "x-bex-driver-grant": turnGrant(),
      },
      body: JSON.stringify({ prompt: "another turn" }),
    });
    assert.equal(second.status, 200);
    await second.text();

    const replayGrant = turnGrant();
    const accepted = await fetch(turnURL, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "x-bex-driver-grant": replayGrant,
      },
      body: JSON.stringify({ prompt: "one use" }),
    });
    assert.equal(accepted.status, 200);
    await accepted.text();
    const replayed = await fetch(turnURL, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "x-bex-driver-grant": replayGrant,
      },
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

test("persisted credential scan fails closed on aggregate, depth, and cancellation bounds", async () => {
  const config = await tempConfig();
  const credentials = manager(config);
  await writeFile(path.join(config.cwd, "one"), "1");
  await writeFile(path.join(config.cwd, "two"), "2");
  await assert.rejects(
    credentials.scrubPersistedState(undefined, { maxFiles: 1 }),
    /file limit/,
  );
  await assert.rejects(
    credentials.scrubPersistedState(undefined, { maxEntries: 1 }),
    /entry limit/,
  );
  await assert.rejects(
    credentials.scrubPersistedState(undefined, { maxBytes: 1 }),
    /byte limit/,
  );
  const nested = path.join(config.cwd, "a", "b", "c");
  await mkdir(nested, { recursive: true });
  await assert.rejects(
    credentials.scrubPersistedState(undefined, { maxDepth: 1 }),
    /depth limit/,
  );
  const controller = new AbortController();
  controller.abort();
  await assert.rejects(
    credentials.scrubPersistedState(undefined, { signal: controller.signal }),
    /aborted/,
  );
});

test("persisted credential scan accepts a safe sparse repository larger than 1 GiB", async () => {
  const config = await tempConfig();
  const large = path.join(config.cwd, "large-safe-fixture");
  const file = await open(large, "w");
  try {
    await file.truncate((1 << 30) + 1);
  } finally {
    await file.close();
  }
  await assert.doesNotReject(
    manager(config).scrubPersistedState(undefined, { timeoutMs: 120_000 }),
  );
});

for (const reachable of [true, false]) {
  test(`persisted credential scan fails closed on a ${reachable ? "reachable" : "unreachable"} Git object`, async () => {
    const config = await tempConfig();
    execFileSync("git", ["init", "-q"], { cwd: config.cwd });
    execFileSync("git", ["config", "user.name", "bex test"], {
      cwd: config.cwd,
    });
    execFileSync("git", ["config", "user.email", "bex-test@example.invalid"], {
      cwd: config.cwd,
    });
    const leaked = path.join(config.cwd, "credential-object");
    await writeFile(leaked, `key=${config.modelCredential}\n`);
    if (reachable) {
      execFileSync("git", ["add", "credential-object"], { cwd: config.cwd });
      execFileSync("git", ["commit", "-q", "-m", "credential fixture"], {
        cwd: config.cwd,
      });
      await writeFile(leaked, "safe working tree\n");
    } else {
      execFileSync("git", ["hash-object", "-w", "credential-object"], {
        cwd: config.cwd,
      });
      await writeFile(leaked, "safe working tree\n");
    }
    await assert.rejects(
      manager(config).scrubPersistedState(),
      /credential found in persisted Git object database/,
    );
  });
}

test("Git metadata paths are scrubbed after the decompressed object proof", async () => {
  const config = await tempConfig();
  execFileSync("git", ["init", "-q"], { cwd: config.cwd });
  const targets = [
    path.join(config.cwd, ".git", "config"),
    path.join(config.cwd, ".git", "logs", "credential-fixture"),
    path.join(config.cwd, ".git", "refs", "credential-fixture"),
    path.join(config.cwd, ".git", "index"),
  ];
  await mkdir(path.dirname(targets[1]), { recursive: true });
  await mkdir(path.dirname(targets[2]), { recursive: true });
  for (const target of targets) {
    await writeFile(target, `metadata=${config.modelCredential}\n`);
  }
  const scrubbed = await manager(config).scrubPersistedState();
  for (const target of targets) {
    assert.ok(scrubbed.includes(target));
    assert.doesNotMatch(await readFile(target, "utf8"), /never-log/);
  }
});

test("a scrub failure records one safe failed verdict and forgets credentials", async () => {
  const config = await tempConfig();
  const credentials = manager(config);
  let scrubs = 0;
  const failing: CredentialManager = {
    ...credentials,
    async scrubPersistedState() {
      scrubs += 1;
      throw new Error(`scan failed near ${config.modelCredential}`);
    },
  };
  await assert.rejects(
    runHeadlessTurn(config, failing, new UIMessageStreamHub()),
    /persisted credential cleanup failed/,
  );
  assert.equal(
    scrubs,
    1,
    "the failed scrub must not be retried in a catch path",
  );
  assert.equal(credentials.configured(), false);
  const status = JSON.parse(await readFile(config.statusPath, "utf8"));
  assert.equal(status.state, "failed");
  assert.match(status.error, /persisted credential cleanup failed/);
  assert.doesNotMatch(status.error, /test-model-key-never-log/);
});

test("snapshot hook requires an action-bound grant, terminalizes, scrubs, and forgets", async () => {
  const config = await tempConfig();
  const leaked = path.join(config.cwd, "agent-cache");
  await writeFile(leaked, `cached=${config.modelCredential}\n`);
  const credentials = manager(config);
  let terminalized = false;
  const listener = await startDriverServer(
    config,
    credentials,
    new UIMessageStreamHub(),
    {
      terminalize: async () => {
        terminalized = true;
      },
    },
  );
  try {
    const url = `http://127.0.0.1:${(listener.address as { port: number }).port}/snapshot/scrub`;
    const unauthorized = await fetch(url, { method: "POST" });
    assert.equal(unauthorized.status, 401);
    await unauthorized.text();
    const wrongAction = await fetch(url, {
      method: "POST",
      headers: { "x-bex-driver-grant": turnGrant() },
    });
    assert.equal(wrongAction.status, 401);
    await wrongAction.text();
    const response = await fetch(url, {
      method: "POST",
      headers: { "x-bex-driver-grant": driverGrant("snapshot") },
    });
    assert.equal(response.status, 200);
    assert.deepEqual(await response.json(), { scrubbedFiles: 1 });
    assert.equal(terminalized, true);
    assert.equal(credentials.configured(), false);
    assert.doesNotMatch(
      await readFile(leaked, "utf8"),
      /test-model-key-never-log/,
    );
  } finally {
    await listener.close();
  }
});

test("snapshot scrub forgets credentials even when persisted cleanup fails", async () => {
  const config = await tempConfig();
  const credentials = manager(config);
  const failing: CredentialManager = {
    ...credentials,
    async scrubPersistedState() {
      throw new Error(`snapshot scrub failed for ${config.modelCredential}`);
    },
  };
  const listener = await startDriverServer(
    config,
    failing,
    new UIMessageStreamHub(),
    { terminalize: async () => undefined },
  );
  try {
    const url = `http://127.0.0.1:${(listener.address as { port: number }).port}/snapshot/scrub`;
    const response = await fetch(url, {
      method: "POST",
      headers: { "x-bex-driver-grant": driverGrant("snapshot") },
    });
    assert.equal(response.status, 500);
    const body = await response.text();
    assert.match(body, /snapshot scrub failed/);
    assert.doesNotMatch(body, /test-model-key-never-log/);
    assert.equal(credentials.configured(), false);
  } finally {
    await listener.close();
  }
});
