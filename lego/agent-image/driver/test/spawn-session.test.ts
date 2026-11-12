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
import { createServer, type IncomingMessage } from "node:http";
import type { AddressInfo } from "node:net";
import test from "node:test";
import { bexMcpServers } from "../src/acp.js";
import { loadConfig } from "../src/config.js";
import { handleMessage } from "../src/mcp-bex.js";
import { platformOverlay } from "../src/overlay.js";
import {
  cancelChildSession,
  cancelChildSessionToolDefinition,
  cancelChildSessionToolName,
  getChildSession,
  getChildSessionToolDefinition,
  getChildSessionToolName,
  messageChildSession,
  messageChildSessionToolDefinition,
  messageChildSessionToolName,
  spawnSession,
  spawnSessionToolDefinition,
  spawnSessionToolName,
  validateSpawnInput,
  waitChildSessions,
  waitChildSessionsToolDefinition,
  waitChildSessionsToolName,
} from "../src/spawn-session.js";

const childId = "ags-d1abcdefghijklmnopqr";
const otherChildId = "ags-d1abcdefghijklmnopqs";
const coordinatorToolNames = [
  waitChildSessionsToolName,
  messageChildSessionToolName,
  cancelChildSessionToolName,
];

// stubGateway stands in for the gateway's spawn verbs.
async function stubGateway(answer: { status: number; body: unknown }) {
  const requests: Array<{ method?: string; url?: string; body: any }> = [];
  const http = createServer(async (request: IncomingMessage, response) => {
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(chunk as Buffer);
    const text = Buffer.concat(chunks).toString("utf8");
    requests.push({
      method: request.method,
      url: request.url,
      body: text ? JSON.parse(text) : undefined,
    });
    response.writeHead(answer.status, { "content-type": "application/json" });
    response.end(JSON.stringify(answer.body));
  });
  await new Promise<void>((resolve) => http.listen(0, "127.0.0.1", resolve));
  const port = (http.address() as AddressInfo).port;
  return {
    requests,
    endpoint: `http://127.0.0.1:${port}/agent-children/bnM/c2lk`,
    close: () => new Promise<void>((resolve) => http.close(() => resolve())),
  };
}

test("the spawn_session schema is tight: task required, nothing else accepted", () => {
  const schema = spawnSessionToolDefinition.inputSchema;
  assert.deepEqual(schema.required, ["task"]);
  assert.equal(schema.additionalProperties, false);
  assert.deepEqual(Object.keys(schema.properties).sort(), [
    "branch",
    "idempotencyKey",
    "maxSpendUSD",
    "repo",
    "secretNames",
    "tags",
    "task",
    "title",
  ]);
  // The parent is the calling session: the agent has no way to name one.
  assert.ok(!("parentSessionId" in schema.properties));
  assert.deepEqual(getChildSessionToolDefinition.inputSchema.required, ["sessionId"]);
});

test("a spawn is validated before any request and the body carries only the child's fields", async (t) => {
  const gateway = await stubGateway({
    status: 200,
    body: { sessionId: childId, state: "working", created: true },
  });
  t.after(gateway.close);
  const env = { BEX_SPAWN_ENDPOINT: gateway.endpoint } as NodeJS.ProcessEnv;

  for (const args of [
    undefined,
    {},
    { task: "  " },
    { task: "t", parentSessionId: "ags-x" },
    { task: "t", maxSpendUSD: 0 },
    { task: "t", maxSpendUSD: "5" },
    { task: "t", tags: "a" },
    { task: "t", secretNames: [1] },
    { task: "t", idempotencyKey: "é" },
    { task: "t", idempotencyKey: "k".repeat(65) },
    { task: "t", title: 3 },
  ]) {
    const outcome = await spawnSession(args, env);
    assert.equal(outcome.ok, false, JSON.stringify(args));
    if (!outcome.ok) assert.equal(outcome.error.code, "invalid_request");
  }
  const huge = await spawnSession({ task: "x".repeat(100_001) }, env);
  assert.equal(huge.ok === false && huge.error.code, "too_large");
  assert.equal(gateway.requests.length, 0);

  const outcome = await spawnSession(
    { task: "fix the tests", tags: ["a"], secretNames: [], maxSpendUSD: 2.5, idempotencyKey: "k1" },
    env,
  );
  assert.deepEqual(outcome, {
    ok: true,
    result: { sessionId: childId, state: "working", created: true },
  });
  assert.deepEqual(gateway.requests, [
    {
      method: "POST",
      url: "/agent-children/bnM/c2lk",
      body: { task: "fix the tests", tags: ["a"], secretNames: [], maxSpendUSD: 2.5, idempotencyKey: "k1" },
    },
  ]);
  assert.equal(validateSpawnInput({ task: "t" }).ok, true);
});

test("a refusal from the create path reaches the agent verbatim", async (t) => {
  const refusal = {
    error: {
      code: "AGENT_SESSION_DEPTH_LIMIT",
      message: "agent sessions can be nested at most 3 levels deep",
      hint: "Do not retry.",
    },
  };
  const gateway = await stubGateway({ status: 409, body: refusal });
  t.after(gateway.close);
  const outcome = await spawnSession({ task: "t" }, { BEX_SPAWN_ENDPOINT: gateway.endpoint });
  assert.deepEqual(outcome, { ok: false, error: refusal.error });
  const reply = await handleMessage(
    { jsonrpc: "2.0", id: 1, method: "tools/call", params: { name: spawnSessionToolName, arguments: { task: "t" } } },
    { BEX_SPAWN_ENDPOINT: gateway.endpoint },
  );
  assert.equal((reply?.result as any).isError, true);
  assert.equal((reply?.result as any).structuredContent.error.code, "AGENT_SESSION_DEPTH_LIMIT");
});

test("get_child_session reads one id with GET and refuses anything that is not a session id", async (t) => {
  const view = { sessionId: childId, state: "succeeded", outcome: { state: "succeeded", result: { n: 1 } } };
  const gateway = await stubGateway({ status: 200, body: view });
  t.after(gateway.close);
  const env = { BEX_SPAWN_ENDPOINT: gateway.endpoint } as NodeJS.ProcessEnv;
  for (const args of [undefined, {}, { sessionId: "../x" }, { sessionId: "ags-short" }, { sessionId: 7 }]) {
    const outcome = await getChildSession(args, env);
    assert.equal(outcome.ok === false && outcome.error.code, "invalid_request");
  }
  assert.equal(gateway.requests.length, 0);
  assert.deepEqual(await getChildSession({ sessionId: childId }, env), { ok: true, result: view });
  assert.deepEqual(gateway.requests, [
    { method: "GET", url: `/agent-children/bnM/c2lk/${childId}`, body: undefined },
  ]);
});

test("without a spawn endpoint the tools are neither listed nor callable", async () => {
  const names = async (env: NodeJS.ProcessEnv) =>
    ((await handleMessage({ jsonrpc: "2.0", id: 1, method: "tools/list" }, env))?.result as any).tools.map(
      (tool: { name: string }) => tool.name,
    );
  assert.deepEqual(await names({}), []);
  assert.deepEqual(await names({ BEX_SPAWN_ENDPOINT: "http://gw/agent-children/a/b" }), [
    spawnSessionToolName,
    getChildSessionToolName,
    ...coordinatorToolNames,
  ]);
  const outcome = await spawnSession({ task: "t" }, {});
  assert.equal(outcome.ok === false && outcome.error.code, "unavailable");
  const reply = await handleMessage(
    { jsonrpc: "2.0", id: 1, method: "tools/call", params: { name: spawnSessionToolName, arguments: { task: "t" } } },
    {},
  );
  assert.equal((reply as any).error.code, -32602);
});

test("the MCP registration carries only the spawn URL, never a credential", () => {
  const base = { deliver: false, repoUrl: "", prRecordPath: "/x", shareUrl: "" };
  assert.deepEqual(bexMcpServers({ ...base, spawnUrl: "" }), []);
  const [server] = bexMcpServers({ ...base, spawnUrl: "http://gw/agent-children/a/b" });
  assert.deepEqual(server.env, [{ name: "BEX_SPAWN_ENDPOINT", value: "http://gw/agent-children/a/b" }]);

  const hosted = {
    BEX_AGENT_PROFILE: "claude",
    BEX_AGENT_MODEL_PROXY_URL: "http://gateway/model/ns/session",
    BEX_AGENT_SESSION_ID: "session",
    BEX_AGENT_MODEL_API_KEY: "bex-model-proxy-placeholder-session",
    BEX_AGENT_TURN_TIMEOUT_MS: "1000",
  } as NodeJS.ProcessEnv;
  assert.equal(
    loadConfig({ ...hosted, BEX_AGENT_SPAWN_URL: "http://gw/agent-children/a/b" }).spawnUrl,
    "http://gw/agent-children/a/b",
  );
  assert.equal(loadConfig(hosted).spawnUrl, "");
});

test("spawn requests carry no credential header and a replay reports created: false", async (t) => {
  let seen: Record<string, string | string[] | undefined> = {};
  const http = createServer((request, response) => {
    seen = request.headers;
    request.resume();
    response.writeHead(200, { "content-type": "application/json" });
    response.end(JSON.stringify({ sessionId: childId, state: "working", created: false }));
  });
  await new Promise<void>((resolve) => http.listen(0, "127.0.0.1", resolve));
  t.after(() => new Promise<void>((resolve) => http.close(() => resolve())));
  const endpoint = `http://127.0.0.1:${(http.address() as AddressInfo).port}/agent-children/a/b`;
  // A credential in the process environment must not leak into the request.
  const env = {
    BEX_SPAWN_ENDPOINT: endpoint,
    BEX_API_KEY: "secret-key",
    BEX_AGENT_MODEL_API_KEY: "secret-model",
  } as NodeJS.ProcessEnv;
  const outcome = await spawnSession({ task: "t", idempotencyKey: "k" }, env);
  assert.deepEqual(outcome, { ok: true, result: { sessionId: childId, state: "working", created: false } });
  for (const name of ["authorization", "x-api-key", "cookie"]) assert.equal(seen[name], undefined, name);
  await getChildSession({ sessionId: childId }, env);
  for (const name of ["authorization", "x-api-key", "cookie"]) assert.equal(seen[name], undefined, name);
  for (const call of [
    () => waitChildSessions({ childIds: [childId] }, env),
    () => messageChildSession({ childId, prompt: "p" }, env),
    () => cancelChildSession({ childId }, env),
  ]) {
    seen = {};
    await call();
    assert.ok(seen.host, "the request reached the stub");
    for (const name of ["authorization", "x-api-key", "cookie"]) assert.equal(seen[name], undefined, name);
  }
});

test("every limit and policy code passes through spawn_session unchanged", async (t) => {
  const codes = [
    "AGENT_SESSION_PARENT_NOT_FOUND",
    "AGENT_SESSION_PARENT_NOT_ACTIVE",
    "AGENT_SESSION_DEPTH_LIMIT",
    "AGENT_SESSION_CHILD_LIMIT",
    "AGENT_SESSION_CHILD_SPEND_EXCEEDS_PARENT",
    "AGENT_SESSION_CHILD_SPEND_EXHAUSTED",
    "AGENT_SESSION_CHILD_SECRET_NOT_INHERITED",
    "AGENT_SESSION_SPAWN_CREATOR_FORBIDDEN",
    "AGENT_SESSION_SPAWN_NO_CREATOR",
    "AGENT_SESSION_CHILD_REPO_MISMATCH",
  ];
  for (const code of codes) {
    const error = { code, message: `m ${code}` };
    const gateway = await stubGateway({ status: 400, body: { error } });
    const outcome = await spawnSession({ task: "t" }, { BEX_SPAWN_ENDPOINT: gateway.endpoint });
    await gateway.close();
    assert.deepEqual(outcome, { ok: false, error }, code);
  }
});

test("failure modes: not found on read, unparsable or malformed replies, unreachable gateway", async (t) => {
  const missing = await stubGateway({
    status: 404,
    body: { error: { code: "not_found", message: "no such session" } },
  });
  t.after(missing.close);
  const read = await getChildSession({ sessionId: childId }, { BEX_SPAWN_ENDPOINT: missing.endpoint });
  assert.deepEqual(read, { ok: false, error: { code: "not_found", message: "no such session" } });

  // 200 with a body that is not a child view is never passed to the agent.
  const odd = await stubGateway({ status: 200, body: { hello: "world" } });
  t.after(odd.close);
  const env = { BEX_SPAWN_ENDPOINT: odd.endpoint } as NodeJS.ProcessEnv;
  const spawned = await spawnSession({ task: "t" }, env);
  assert.equal(spawned.ok === false && spawned.error.code, "unavailable");
  const got = await getChildSession({ sessionId: childId }, env);
  assert.equal(got.ok === false && got.error.code, "unavailable");

  // A refusal-shaped 200 is not a child view either.
  const disguised = await stubGateway({
    status: 200,
    body: { sessionId: childId, error: { code: "x", message: "y" } },
  });
  t.after(disguised.close);
  const g2 = await getChildSession({ sessionId: childId }, { BEX_SPAWN_ENDPOINT: disguised.endpoint });
  assert.equal(g2.ok, false);

  // Nothing listening: a typed error, not a throw.
  const dead = await spawnSession({ task: "t" }, { BEX_SPAWN_ENDPOINT: "http://127.0.0.1:1/agent-children/a/b" });
  assert.equal(dead.ok === false && dead.error.code, "unavailable");
});

test("get_child_session is not callable from a session that cannot spawn", async () => {
  const outcome = await getChildSession({ sessionId: childId }, {});
  assert.equal(outcome.ok === false && outcome.error.code, "unavailable");
});

// The coordinator tools (w9/m346): wait on, message and cancel descendants
// through the same Pod-bound spawn URL.

const childView = {
  sessionId: childId,
  parentSessionId: "ags-d1abcdefghijklmnop00",
  state: "succeeded",
  depth: 1,
  outcome: { state: "succeeded", summary: "done" },
};
// A wait answers each child's id and state only.
const waitChild = { sessionId: childId, state: "succeeded" };

test("the coordinator schemas are tight: ids, timeout and prompt, nothing else", () => {
  const wait = waitChildSessionsToolDefinition.inputSchema;
  assert.deepEqual(wait.required, ["childIds"]);
  assert.equal(wait.additionalProperties, false);
  assert.deepEqual(Object.keys(wait.properties).sort(), ["childIds", "timeoutSeconds"]);
  assert.equal(wait.properties.childIds.minItems, 1);
  assert.equal(wait.properties.childIds.maxItems, 20);
  assert.deepEqual(
    [wait.properties.timeoutSeconds.type, wait.properties.timeoutSeconds.minimum, wait.properties.timeoutSeconds.maximum],
    ["integer", 1, 25],
  );
  assert.match(waitChildSessionsToolDefinition.description, /Use instead of polling get_child_session/);

  const message = messageChildSessionToolDefinition.inputSchema;
  assert.deepEqual(message.required, ["childId", "prompt"]);
  assert.equal(message.additionalProperties, false);
  assert.deepEqual(Object.keys(message.properties).sort(), ["childId", "prompt"]);
  assert.match(messageChildSessionToolDefinition.description, /AGENT_SESSION_TURN_IN_FLIGHT/);

  const cancel = cancelChildSessionToolDefinition.inputSchema;
  assert.deepEqual(cancel.required, ["childId"]);
  assert.equal(cancel.additionalProperties, false);
  assert.deepEqual(Object.keys(cancel.properties), ["childId"]);
  assert.equal(cancelChildSessionToolDefinition.annotations.destructiveHint, true);
});

test("wait_child_sessions is validated before any request and posts the ids and timeout to /wait", async (t) => {
  const answer = { children: [waitChild], settled: [childId], timedOut: false };
  const gateway = await stubGateway({ status: 200, body: answer });
  t.after(gateway.close);
  const env = { BEX_SPAWN_ENDPOINT: gateway.endpoint } as NodeJS.ProcessEnv;

  for (const args of [
    undefined,
    [],
    {},
    { childIds: [] },
    { childIds: childId },
    { childIds: ["ags-short"] },
    { childIds: ["../x"] },
    // Uppercase and w-z are outside the xid alphabet.
    { childIds: ["ags-d1abcdefghijklmnopqw"] },
    { childIds: ["ags-D1ABCDEFGHIJKLMNOPQR"] },
    { childIds: Array.from({ length: 21 }, () => childId) },
    { childIds: [childId], timeoutSeconds: 0 },
    { childIds: [childId], timeoutSeconds: 26 },
    { childIds: [childId], timeoutSeconds: 2.5 },
    { childIds: [childId], timeoutSeconds: "30" },
    { childIds: [childId], parentSessionId: childId },
  ]) {
    const outcome = await waitChildSessions(args, env);
    assert.equal(outcome.ok === false && outcome.error.code, "invalid_request", JSON.stringify(args));
  }
  assert.equal(gateway.requests.length, 0);

  assert.deepEqual(await waitChildSessions({ childIds: [childId, otherChildId], timeoutSeconds: 25 }, env), {
    ok: true,
    result: answer,
  });
  // An omitted timeout is left to the gateway's default.
  await waitChildSessions({ childIds: Array.from({ length: 20 }, () => childId) }, env);
  assert.deepEqual(gateway.requests, [
    {
      method: "POST",
      url: "/agent-children/bnM/c2lk/wait",
      body: { childIds: [childId, otherChildId], timeoutSeconds: 25 },
    },
    {
      method: "POST",
      url: "/agent-children/bnM/c2lk/wait",
      body: { childIds: Array.from({ length: 20 }, () => childId) },
    },
  ]);
});

test("a timed-out wait with nothing settled reports empty lists, and a malformed one is refused", async (t) => {
  // Go encodes a nil slice as null.
  const timedOut = await stubGateway({ status: 200, body: { children: [childView], settled: null, timedOut: true } });
  t.after(timedOut.close);
  assert.deepEqual(await waitChildSessions({ childIds: [childId] }, { BEX_SPAWN_ENDPOINT: timedOut.endpoint }), {
    ok: true,
    result: { children: [childView], settled: [], timedOut: true },
  });

  for (const body of [
    { hello: "world" },
    { children: [{ state: "working" }], settled: [], timedOut: false },
    { children: [childView], settled: [7], timedOut: false },
    { children: "x", settled: [], timedOut: false },
    { children: [], settled: [], timedOut: false, error: { code: "x", message: "y" } },
  ]) {
    const odd = await stubGateway({ status: 200, body });
    const outcome = await waitChildSessions({ childIds: [childId] }, { BEX_SPAWN_ENDPOINT: odd.endpoint });
    await odd.close();
    assert.equal(outcome.ok === false && outcome.error.code, "unavailable", JSON.stringify(body));
  }
});

test("message_child_session is validated before any request and posts the prompt to the child's route", async (t) => {
  const gateway = await stubGateway({ status: 200, body: { ...childView, state: "working" } });
  t.after(gateway.close);
  const env = { BEX_SPAWN_ENDPOINT: gateway.endpoint } as NodeJS.ProcessEnv;

  for (const args of [
    undefined,
    {},
    { prompt: "p" },
    { childId: "ags-short", prompt: "p" },
    { childId, prompt: "" },
    { childId, prompt: "   " },
    { childId, prompt: 7 },
    { childId, prompt: "p", task: "t" },
  ]) {
    const outcome = await messageChildSession(args, env);
    assert.equal(outcome.ok === false && outcome.error.code, "invalid_request", JSON.stringify(args));
  }
  // The bound is UTF-8 bytes: 33,334 three-byte characters are 100,002 bytes.
  for (const prompt of ["x".repeat(100_001), "€".repeat(33_334)]) {
    const huge = await messageChildSession({ childId, prompt }, env);
    assert.equal(huge.ok === false && huge.error.code, "too_large");
  }
  assert.equal(gateway.requests.length, 0);

  assert.deepEqual(await messageChildSession({ childId, prompt: "now add tests" }, env), {
    ok: true,
    result: { ...childView, state: "working" },
  });
  assert.equal((await messageChildSession({ childId, prompt: "x".repeat(100_000) }, env)).ok, true);
  assert.deepEqual(gateway.requests[0], {
    method: "POST",
    url: `/agent-children/bnM/c2lk/${childId}`,
    body: { prompt: "now add tests" },
  });
  assert.equal(gateway.requests.length, 2);
});

test("cancel_child_session sends DELETE to the child's route with no body", async (t) => {
  const gateway = await stubGateway({ status: 200, body: { ...childView, state: "canceled" } });
  t.after(gateway.close);
  const env = { BEX_SPAWN_ENDPOINT: gateway.endpoint } as NodeJS.ProcessEnv;
  for (const args of [undefined, {}, { childId: "ags-x" }, { childId, reason: "r" }]) {
    const outcome = await cancelChildSession(args, env);
    assert.equal(outcome.ok === false && outcome.error.code, "invalid_request", JSON.stringify(args));
  }
  assert.equal(gateway.requests.length, 0);
  assert.deepEqual(await cancelChildSession({ childId }, env), {
    ok: true,
    result: { ...childView, state: "canceled" },
  });
  assert.deepEqual(gateway.requests, [
    { method: "DELETE", url: `/agent-children/bnM/c2lk/${childId}`, body: undefined },
  ]);
});

test("coordinator refusals reach the agent verbatim through the MCP server", async (t) => {
  const cases: Array<{ name: string; args: Record<string, unknown>; status: number; error: Record<string, string> }> = [
    {
      name: messageChildSessionToolName,
      args: { childId, prompt: "p" },
      status: 409,
      error: { code: "AGENT_SESSION_TURN_IN_FLIGHT", message: "the session is working", hint: "Wait for it first." },
    },
    { name: waitChildSessionsToolName, args: { childIds: [childId] }, status: 404, error: { code: "not_found", message: "no such session" } },
    { name: cancelChildSessionToolName, args: { childId }, status: 403, error: { code: "forbidden", message: "not yours" } },
    { name: messageChildSessionToolName, args: { childId, prompt: "p" }, status: 429, error: { code: "rate_limited", message: "slow down" } },
    { name: waitChildSessionsToolName, args: { childIds: [childId] }, status: 503, error: { code: "unavailable", message: "later" } },
  ];
  for (const { name, args, status, error } of cases) {
    const gateway = await stubGateway({ status, body: { error } });
    t.after(gateway.close);
    const reply = await handleMessage(
      { jsonrpc: "2.0", id: 1, method: "tools/call", params: { name, arguments: args } },
      { BEX_SPAWN_ENDPOINT: gateway.endpoint },
    );
    const result = reply?.result as any;
    assert.equal(result.isError, true, name);
    assert.deepEqual(result.structuredContent, { error }, `${name} ${error.code}`);
    assert.deepEqual(JSON.parse(result.content[0].text), { error });
  }

  // A non-JSON failure becomes a typed error; nothing listening is not a throw.
  const broken = await stubGateway({ status: 502, body: "bad gateway" });
  t.after(broken.close);
  const failed = await cancelChildSession({ childId }, { BEX_SPAWN_ENDPOINT: broken.endpoint });
  assert.deepEqual(failed, { ok: false, error: { code: "unavailable", message: "spawn endpoint returned HTTP 502" } });
  const dead = { BEX_SPAWN_ENDPOINT: "http://127.0.0.1:1/agent-children/a/b" };
  for (const outcome of [
    await waitChildSessions({ childIds: [childId] }, dead),
    await messageChildSession({ childId, prompt: "p" }, dead),
    await cancelChildSession({ childId }, dead),
  ]) {
    assert.equal(outcome.ok === false && outcome.error.code, "unavailable");
  }
  // 200 with something that is not a child view.
  const odd = await stubGateway({ status: 200, body: { hello: "world" } });
  t.after(odd.close);
  for (const outcome of [
    await messageChildSession({ childId, prompt: "p" }, { BEX_SPAWN_ENDPOINT: odd.endpoint }),
    await cancelChildSession({ childId }, { BEX_SPAWN_ENDPOINT: odd.endpoint }),
  ]) {
    assert.equal(outcome.ok === false && outcome.error.code, "unavailable");
  }
});

test("a successful coordinator call returns the gateway's answer as structured content", async (t) => {
  const answer = { children: [waitChild], settled: [childId], timedOut: false };
  const gateway = await stubGateway({ status: 200, body: answer });
  t.after(gateway.close);
  const reply = await handleMessage(
    {
      jsonrpc: "2.0",
      id: 1,
      method: "tools/call",
      params: { name: waitChildSessionsToolName, arguments: { childIds: [childId], timeoutSeconds: 5 } },
    },
    { BEX_SPAWN_ENDPOINT: gateway.endpoint },
  );
  const result = reply?.result as any;
  assert.equal(result.isError, false);
  assert.deepEqual(result.structuredContent, answer);
});

test("a session that cannot spawn has no coordinator tools, and calling one is unknown", async () => {
  const listed = ((await handleMessage({ jsonrpc: "2.0", id: 1, method: "tools/list" }, {}))?.result as any).tools;
  assert.deepEqual(listed, []);
  for (const name of coordinatorToolNames) {
    const reply = await handleMessage(
      { jsonrpc: "2.0", id: 1, method: "tools/call", params: { name, arguments: { childId, childIds: [childId] } } },
      { BEX_PR_ENDPOINT: "http://gw/git/a/b/c/d/pull-request" },
    );
    assert.equal((reply as any).error.code, -32602, name);
  }
  for (const outcome of [
    await waitChildSessions({ childIds: [childId] }, {}),
    await messageChildSession({ childId, prompt: "p" }, {}),
    await cancelChildSession({ childId }, {}),
  ]) {
    assert.equal(outcome.ok === false && outcome.error.code, "unavailable");
  }
});

test("the server instructions and the overlay say to wait instead of polling, only when the session can spawn", async () => {
  const instructions = async (env: NodeJS.ProcessEnv) =>
    ((await handleMessage({ jsonrpc: "2.0", id: 1, method: "initialize", params: {} }, env))?.result as any)
      .instructions as string;
  const spawning = await instructions({ BEX_SPAWN_ENDPOINT: "http://gw/agent-children/a/b" });
  assert.match(spawning, /wait_child_sessions to wait until children finish instead of polling get_child_session/);
  assert.match(spawning, /message_child_session or cancel_child_session/);
  assert.doesNotMatch(await instructions({}), /child/);

  const base = { cwd: "/workspace", deliver: false, repoUrl: "", branch: "", baseBranch: "", repository: "" };
  assert.doesNotMatch(platformOverlay(base), /Child sessions|wait_child_sessions/);
  assert.doesNotMatch(platformOverlay({ ...base, spawnUrl: "" }), /Child sessions/);
  const overlay = platformOverlay({ ...base, spawnUrl: "http://gw/agent-children/a/b" });
  assert.match(
    overlay,
    /## Child sessions\n\nAfter you start child sessions with `spawn_session`, wait for them with `wait_child_sessions` instead of polling `get_child_session`, and message or cancel a child with `message_child_session` or `cancel_child_session`\./,
  );
  // The section adds one sentence and changes nothing else.
  assert.equal(
    overlay.replace(/\n\n## Child sessions\n\n[^\n]*/, ""),
    platformOverlay(base),
  );
});
