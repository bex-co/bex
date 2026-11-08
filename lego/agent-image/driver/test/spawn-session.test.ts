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
import {
  getChildSession,
  getChildSessionToolDefinition,
  getChildSessionToolName,
  spawnSession,
  spawnSessionToolDefinition,
  spawnSessionToolName,
  validateSpawnInput,
} from "../src/spawn-session.js";

const childId = "ags-d1abcdefghijklmnopqr";

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
