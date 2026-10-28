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
import { execFileSync } from "node:child_process";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { loadConfig } from "../src/config.js";
import { agentEnvironment } from "../src/credentials.js";
import { runHeadlessTurn } from "../src/session.js";
import { createSessionProvider } from "../src/acp.js";
import { startDriverServer } from "../src/server.js";
import { UIMessageStreamHub } from "../src/stream-hub.js";
import { TurnLogSink } from "../src/turn-log.js";
import { isUtcTimestamp } from "../src/timestamp.js";
import { childEnv, tempConfig, turnGrant } from "./helpers.js";

const proxy =
  "http://bex-ssh-gateway.bex-system.svc.cluster.local:8084/model/ns/ags-1";

// hosted is the minimum environment bex-api provisions: the mandatory model
// proxy URL plus the session-bound placeholder credential.
function hosted(env: NodeJS.ProcessEnv = {}): NodeJS.ProcessEnv {
  return {
    BEX_AGENT_MODEL_PROXY_URL: proxy,
    BEX_AGENT_SESSION_ID: "ags-1",
    BEX_AGENT_MODEL_API_KEY: "bex-model-proxy-placeholder-ags-1",
    ...env,
  };
}

test("configuration accepts only operator-owned adapter paths", () => {
  const config = loadConfig(
    hosted({
      BEX_AGENT_COMMAND: "/usr/local/bin/gemini",
      BEX_AGENT_ARGS: '["--acp"]',
      BEX_AGENT_CWD: "/tmp/repo",
      BEX_AGENT_LISTEN_PORT: "9000",
      BEX_AGENT_ENV_JSON: "{}",
    }),
  );
  assert.equal(config.command, "/usr/local/bin/gemini");
  assert.equal(config.profile, "gemini");
  assert.deepEqual(config.args, ["--acp"]);
  assert.deepEqual(config.agentEnv, {});
  assert.equal(config.prRecordPath, "/var/run/bex-agent/pull-request.json");
  assert.equal(config.networkTier, "");
  assert.equal(
    loadConfig(hosted({ BEX_AGENT_NETWORK_TIER: "full" })).networkTier,
    "full",
  );
  // An unknown tier (a newer bex-api) only drops the overlay wording.
  assert.equal(
    loadConfig(hosted({ BEX_AGENT_NETWORK_TIER: "everything" })).networkTier,
    "",
  );
  assert.throws(
    () => loadConfig(hosted({ BEX_AGENT_ARGS: "--flag" })),
    /JSON array/,
  );
  assert.throws(
    () => loadConfig(hosted({ BEX_AGENT_COMMAND: "./tenant-adapter" })),
    /installed agent adapter path/,
  );
  assert.throws(
    () =>
      loadConfig(hosted({ BEX_AGENT_COMMAND: "/workspace/tenant-adapter" })),
    /installed agent adapter path/,
  );
  assert.throws(
    () =>
      loadConfig(
        hosted({ BEX_AGENT_ENV_JSON: '{"ANTHROPIC_API_KEY":"bypass"}' }),
      ),
    /model credentials must use BEX_AGENT_MODEL_API_KEY/,
  );
});

test("the driver refuses to start without the model proxy or with a real key", () => {
  // ADR062 / ADR090: there is no standalone direct-to-vendor mode.
  assert.throws(
    () => loadConfig(hosted({ BEX_AGENT_MODEL_PROXY_URL: "" })),
    /BEX_AGENT_MODEL_PROXY_URL is required/,
  );
  assert.throws(
    () => loadConfig(hosted({ BEX_AGENT_SESSION_ID: "" })),
    /BEX_AGENT_SESSION_ID is required/,
  );
  for (const credential of [
    "",
    "sk-ant-api03-real-looking-key",
    "sk-ant-oat01-oauth-token",
    "bex-model-proxy-placeholder-ags-other",
  ]) {
    assert.throws(
      () => loadConfig(hosted({ BEX_AGENT_MODEL_API_KEY: credential })),
      /model-proxy placeholder/,
    );
  }
});

test("model proxy routes each adapter's base URL and lands the placeholder in its native var", () => {
  // ADR062 D5: each adapter is pointed at the gateway proxy via its provider
  // base-URL env (with the provider's path suffix) and the placeholder lands
  // in the credential env that adapter reads.
  const claude = loadConfig(
    hosted({ BEX_AGENT_COMMAND: "/usr/local/bin/claude-agent-acp" }),
  );
  assert.equal(claude.modelBaseUrlEnvName, "ANTHROPIC_BASE_URL");
  assert.equal(claude.modelBaseUrl, proxy); // Anthropic base is the root: no suffix
  assert.equal(claude.credentialEnvName, "ANTHROPIC_API_KEY");

  const codex = loadConfig(
    hosted({ BEX_AGENT_COMMAND: "/usr/local/bin/codex-acp" }),
  );
  assert.equal(codex.modelBaseUrlEnvName, "OPENAI_BASE_URL");
  assert.equal(codex.modelBaseUrl, proxy + "/v1"); // OpenAI base includes /v1
  assert.equal(codex.credentialEnvName, "OPENAI_API_KEY");

  const gemini = loadConfig(
    hosted({ BEX_AGENT_COMMAND: "/usr/local/bin/gemini" }),
  );
  assert.equal(gemini.modelBaseUrlEnvName, "GOOGLE_GEMINI_BASE_URL");
  assert.equal(gemini.credentialEnvName, "GEMINI_API_KEY");

  // The base-URL env is surfaced to the agent child alongside the placeholder,
  // and both injection names leave the driver's own environment.
  const parent: NodeJS.ProcessEnv = {
    BEX_AGENT_MODEL_API_KEY: "bex-model-proxy-placeholder-ags-1",
    OPENAI_API_KEY: "bex-model-proxy-placeholder-ags-1",
  };
  const env = agentEnvironment(codex, parent);
  assert.equal(env.OPENAI_BASE_URL, proxy + "/v1");
  assert.equal(env.OPENAI_API_KEY, "bex-model-proxy-placeholder-ags-1");
  assert.equal(parent.BEX_AGENT_MODEL_API_KEY, undefined);
  assert.equal(parent.OPENAI_API_KEY, undefined);
});

test("one headless turn maps every fixture update and commits in the worktree", async () => {
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
  const result = await runHeadlessTurn(config, childEnv(config), hub);
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
  assert.ok(types.includes("text-delta"));
  assert.ok(types.includes("tool-output-available"));
  assert.deepEqual(
    hub.history.find((part) => part.type === "data-acp-available-commands")
      ?.data,
    {
      availableCommands: [
        { name: "review", description: "Review the changes" },
      ],
    },
  );
  assert.equal(
    hub.history.find((part) => part.type === "data-acp-current-mode-update")
      ?.transient,
    true,
  );
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
  await assert.rejects(
    runHeadlessTurn(config, childEnv(config), new UIMessageStreamHub()),
    /ACP session failed \(transport_lost\): model proxy transport failed/,
  );
  const calls = (await readFile(callLog, "utf8")).trim().split("\n");
  assert.ok(calls.indexOf("providers/set") > calls.indexOf("initialize"));
  assert.ok(calls.indexOf("providers/set") < calls.indexOf("session/new"));
  const status = JSON.parse(await readFile(config.statusPath, "utf8"));
  assert.equal(status.state, "failed");
  assert.match(status.error, /transport_lost/);
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
      childEnv(config),
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

// A driver restarted inside the same sandbox (rootfs intact) adopts the prior turn's persisted ACP session id and re-attaches via
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
      childEnv(config),
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
    childEnv(config),
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
    childEnv(config),
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
    childEnv(config),
    new UIMessageStreamHub(),
  );
  assert.equal(result.resume, "new");
  assert.equal(result.resumedFrom, undefined);
});

test("agent crash becomes a failed status instead of hanging", async () => {
  const config = await tempConfig({ agentEnv: { ACP_FIXTURE_CRASH: "1" } });
  let failure: unknown;
  try {
    await runHeadlessTurn(config, childEnv(config), new UIMessageStreamHub());
  } catch (error) {
    failure = error;
  }
  assert.ok(failure instanceof Error);
  assert.match(failure.message, /ACP agent exited \(code=23.*fixture crashed/);
  const status = JSON.parse(await readFile(config.statusPath, "utf8"));
  assert.equal(status.state, "failed");
});

test("agent input loss fails promptly instead of leaving a running turn", async () => {
  const config = await tempConfig({
    agentEnv: { ACP_FIXTURE_CLOSE_INPUT_AFTER_SESSION: "1" },
    turnTimeoutMs: 5_000,
  });
  await assert.rejects(
    runHeadlessTurn(config, childEnv(config), new UIMessageStreamHub()),
    /ACP agent stdin failed:/,
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
    runHeadlessTurn(config, childEnv(config), new UIMessageStreamHub()),
    /turn exceeded|connect timed out/,
  );
  const status = JSON.parse(await readFile(config.statusPath, "utf8"));
  assert.equal(status.state, "failed");
});

test("SSE replays the standard UI-message stream and headless needs no client", async () => {
  const config = await tempConfig();
  const hub = new UIMessageStreamHub();
  const listener = await startDriverServer(config, hub);
  try {
    await runHeadlessTurn(config, childEnv(config), hub);
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
  const hub = new UIMessageStreamHub();
  const runTurn = (
    prompt: string,
    onPart: (part: Record<string, unknown>) => void,
  ) =>
    runHeadlessTurn(config, childEnv(config), hub, {
      prompt,
      closeHub: false,
      onPart,
    });
  const listener = await startDriverServer(config, hub, { runTurn });
  try {
    const turnURL = `http://127.0.0.1:${(listener.address as { port: number }).port}/turn`;

    // Same-Pod code can reach localhost but cannot launch an agent turn
    // without the gateway's signed, action-bound grant.
    const unauthorized = await fetch(turnURL, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ prompt: "run without a grant" }),
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
  const listener = await startDriverServer(config, new UIMessageStreamHub());
  try {
    const port = (listener.address as { port: number }).port;
    const response = await fetch(`http://127.0.0.1:${port}/acp`, {
      method: "POST",
    });
    assert.equal(response.status, 404);
  } finally {
    await listener.close();
  }
});

test("turn log flush precedes the succeeded status", async () => {
  const config = await tempConfig();
  const frames: string[] = [];
  const hub = new UIMessageStreamHub();
  await runHeadlessTurn(config, childEnv(config), hub, {
    onPart: (part, frame) => {
      assert.equal(frame, `data: ${JSON.stringify(part)}\n\n`);
      frames.push(frame!);
    },
  });
  const records = (await readFile(config.sessionLogPath, "utf8"))
    .trim()
    .split("\n")
    .map((line) => JSON.parse(line));
  assert.equal(records.at(-1).part.type, "finish");
  assert.equal(frames.length, hub.history.length);
  const status = JSON.parse(await readFile(config.statusPath, "utf8"));
  assert.equal(status.state, "succeeded");
  // A repo-less turn carries no git facts and no driver-owned delivery fields
  // (ADR090).
  for (const removed of ["delivery", "evidence", "scrubbedFiles", "git"]) {
    assert.equal(status[removed], undefined, removed);
  }
});

test("log open and terminal close failures persist failed status", async (t) => {
  for (const operation of ["open", "close"] as const) {
    await t.test(operation, async (t) => {
      const config = await tempConfig();
      if (operation === "open") config.sessionLogPath = config.cwd;
      else {
        const close = TurnLogSink.prototype.close;
        const failure = new Error("injected terminal log failure");
        t.mock.method(
          TurnLogSink.prototype,
          "close",
          async function (this: TurnLogSink) {
            await close.call(this);
            throw failure;
          },
        );
      }
      await assert.rejects(
        runHeadlessTurn(config, childEnv(config), new UIMessageStreamHub()),
      );
      const status = JSON.parse(await readFile(config.statusPath, "utf8"));
      assert.equal(status.state, "failed");
      assert.match(
        status.error,
        operation === "open" ? /directory|EISDIR/ : /terminal log failure/,
      );
    });
  }
});

for (const mode of ["missing", "missingLocalized", "corrupt", "closed"]) {
  test(`unavailable native session (${mode}) restarts once and re-primes transcript`, async (t) => {
    const config = await tempConfig({
      existingSessionId: "prior-session",
      contextJson: JSON.stringify([
        {
          turn: 1,
          prompt: "Remember the codeword cedar",
          reply: "I will remember cedar",
        },
      ]),
    });
    const callsPath = path.join(config.root, "calls.log");
    const promptPath = path.join(config.root, "prompt.json");
    const pidPath = path.join(config.root, "pids.log");
    config.agentEnv = {
      ACP_FIXTURE_LOAD_SESSION: "1",
      ACP_FIXTURE_LOAD_ERROR: mode,
      ACP_FIXTURE_LOG: callsPath,
      ACP_FIXTURE_PROMPT_LOG: promptPath,
      ACP_FIXTURE_PID_LOG: pidPath,
    };
    const diagnostics: string[] = [];
    t.mock.method(console, "warn", (...args: unknown[]) => {
      diagnostics.push(args.join(" "));
    });
    const hub = new UIMessageStreamHub();
    const result = await runHeadlessTurn(config, childEnv(config), hub, {
      prompt: "What was the codeword?",
    });
    assert.equal(result.resume, "load-failed");
    assert.equal(result.continuity, "transcript-reprime");
    assert.equal(
      config.existingSessionId,
      "prior-session",
      "fallback must not mutate the session config",
    );
    const calls = (await readFile(callsPath, "utf8")).trim().split("\n");
    assert.deepEqual(
      calls.filter((method) =>
        [
          "initialize",
          "session/load",
          "session/new",
          "session/prompt",
        ].includes(method),
      ),
      [
        "initialize",
        "session/load",
        "initialize",
        "session/new",
        "session/prompt",
      ],
    );
    const prompt = JSON.parse(await readFile(promptPath, "utf8"))[0].text;
    assert.match(prompt, /Remember the codeword cedar/);
    assert.ok(prompt.endsWith("What was the codeword?"));
    assert.doesNotMatch(
      JSON.stringify(hub.history),
      /Remember the codeword cedar/,
    );
    assert.equal(diagnostics.length, 1);
    assert.match(diagnostics[0], /rebuilding continuity in a fresh process/);
    const pids = (await readFile(pidPath, "utf8"))
      .trim()
      .split("\n")
      .map(Number);
    assert.equal(new Set(pids).size, 2);
    for (const pid of pids)
      assert.throws(() => process.kill(pid, 0), { code: "ESRCH" });
  });
}

for (const mode of [
  "auth",
  "wrappedAuth",
  "routing",
  "unknown",
  "cancelled",
  "timeout",
]) {
  test(`session/load ${mode} remains fatal without a fresh-process retry`, async (t) => {
    const config = await tempConfig({
      existingSessionId: "prior-session",
      turnTimeoutMs: mode === "timeout" ? 150 : 1000,
    });
    const callsPath = path.join(config.root, "calls.log");
    config.agentEnv = {
      ACP_FIXTURE_LOAD_SESSION: "1",
      ACP_FIXTURE_LOAD_ERROR: mode,
      ACP_FIXTURE_LOG: callsPath,
    };
    const diagnostics: unknown[] = [];
    t.mock.method(console, "warn", (...args: unknown[]) => {
      diagnostics.push(args);
    });
    await assert.rejects(
      createSessionProvider(config, childEnv(config), {
        onUpdate: () => {},
      }).then((provider) => provider.cleanup()),
    );
    const calls = (await readFile(callsPath, "utf8")).trim().split("\n");
    assert.equal(calls.filter((method) => method === "initialize").length, 1);
    assert.equal(calls.includes("session/new"), false);
    assert.equal(diagnostics.length, 0);
  });
}

test("an abort between failed load and replacement spawn stays aborted", async (t) => {
  const config = await tempConfig({ existingSessionId: "prior-session" });
  const callsPath = path.join(config.root, "calls.log");
  config.agentEnv = {
    ACP_FIXTURE_LOAD_SESSION: "1",
    ACP_FIXTURE_LOAD_ERROR: "closed",
    ACP_FIXTURE_LOG: callsPath,
  };
  const controller = new AbortController();
  const reason = new Error("fixture cancellation");
  t.mock.method(console, "warn", () => controller.abort(reason));
  await assert.rejects(
    createSessionProvider(config, childEnv(config), {
      onUpdate: () => {},
      abortSignal: controller.signal,
    }).then((provider) => provider.cleanup()),
    reason,
  );
  const calls = (await readFile(callsPath, "utf8")).trim().split("\n");
  assert.equal(calls.filter((method) => method === "initialize").length, 1);
  assert.equal(calls.includes("session/new"), false);
});

test("replacement session/new failure is not retried recursively", async (t) => {
  const config = await tempConfig({ existingSessionId: "prior-session" });
  const callsPath = path.join(config.root, "calls.log");
  config.agentEnv = {
    ACP_FIXTURE_LOAD_SESSION: "1",
    ACP_FIXTURE_LOAD_ERROR: "closed",
    ACP_FIXTURE_FAIL_NEW: "1",
    ACP_FIXTURE_LOG: callsPath,
  };
  t.mock.method(console, "warn", () => {});
  await assert.rejects(
    runHeadlessTurn(config, childEnv(config), new UIMessageStreamHub()),
  );
  const calls = (await readFile(callsPath, "utf8")).trim().split("\n");
  assert.equal(calls.filter((method) => method === "initialize").length, 2);
  assert.equal(calls.filter((method) => method === "session/new").length, 1);
  assert.equal(
    JSON.parse(await readFile(config.statusPath, "utf8")).state,
    "failed",
  );
});

test(
  "cleanup kills inherited-stdio descendants after their ACP parent exits",
  { skip: process.platform === "win32", timeout: 3000 },
  async (t) => {
    const config = await tempConfig();
    const pidPath = path.join(config.root, "pids.log");
    config.agentEnv = {
      ACP_FIXTURE_ORPHAN_STDIO: "1",
      ACP_FIXTURE_PID_LOG: pidPath,
    };
    t.after(async () => {
      // Also keep a failing regression test from leaving an orphan behind.
      const pids = (await readFile(pidPath, "utf8").catch(() => ""))
        .trim()
        .split("\n")
        .map(Number);
      for (const pid of pids) {
        if (pid > 0)
          try {
            process.kill(pid, "SIGKILL");
          } catch {}
      }
    });
    await assert.rejects(
      createSessionProvider(config, childEnv(config), {
        onUpdate: () => {},
      }).then((provider) => provider.cleanup()),
      /ACP agent exited/,
    );
    // A resolved cleanup proves both stdio pipes closed, including the copies
    // held by the orphan. Killing only the exited parent never resolves this.
    const pids = (await readFile(pidPath, "utf8"))
      .trim()
      .split("\n")
      .map(Number);
    assert.equal(pids.length, 2);
    assert.throws(() => process.kill(pids[0], 0), { code: "ESRCH" });
  },
);

for (const failsLoad of [false, true]) {
  test(`native load history is excluded from the new turn (load fails=${failsLoad})`, async (t) => {
    const config = await tempConfig({
      existingSessionId: "prior-session",
      contextJson: JSON.stringify([
        { turn: 1, prompt: "Earlier question", reply: "Earlier answer" },
      ]),
    });
    config.agentEnv = {
      ACP_FIXTURE_LOAD_SESSION: "1",
      ACP_FIXTURE_REPLAY_LOAD: "1",
      ACP_FIXTURE_SETUP_METADATA: "1",
      ...(failsLoad ? { ACP_FIXTURE_LOAD_ERROR: "closed" } : {}),
    };
    t.mock.method(console, "warn", () => {});
    const hub = new UIMessageStreamHub();
    const mirror: Record<string, unknown>[] = [];
    const result = await runHeadlessTurn(config, childEnv(config), hub, {
      onPart: (part) => {
        mirror.push(part);
      },
    });
    assert.equal(result.resume, failsLoad ? "load-failed" : "loaded");
    assert.equal(
      result.continuity,
      failsLoad ? "transcript-reprime" : "session-load",
    );
    for (const body of [
      JSON.stringify(hub.history),
      JSON.stringify(mirror),
      await readFile(config.sessionLogPath, "utf8"),
    ]) {
      assert.doesNotMatch(
        body,
        /historical-answer-from-prior-turn|historical-tool/,
      );
      assert.match(body, /Task committed/);
      assert.match(body, /tool-1/);
    }
    if (failsLoad)
      assert.match(
        JSON.stringify(hub.history),
        /setup-command/,
        "fresh-session setup metadata remains available",
      );
  });
}
