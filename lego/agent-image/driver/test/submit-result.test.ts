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
import { mkdtemp, readFile } from "node:fs/promises";
import { createServer, type IncomingMessage } from "node:http";
import type { AddressInfo } from "node:net";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { handleMessage } from "../src/mcp-bex.js";
import { platformOverlay } from "../src/overlay.js";
import { loadConfig } from "../src/config.js";
import { bexMcpServers } from "../src/acp.js";
import { runHeadlessTurn } from "../src/session.js";
import { UIMessageStreamHub } from "../src/stream-hub.js";
import {
  readResultRecord,
  submitResult,
  submitResultToolName,
} from "../src/submit-result.js";
import { childEnv, promptsSent, tempConfig } from "./helpers.js";

const schema = JSON.stringify({
  type: "object",
  properties: {
    tests_passed: { type: "boolean" },
    files_changed: { type: "integer", minimum: 0 },
  },
  required: ["tests_passed", "files_changed"],
  additionalProperties: false,
});

// stubGateway stands in for the gateway's result verb.
async function stubGateway(
  answer: { status: number; body: unknown } = {
    status: 200,
    body: { accepted: true, final: true },
  },
) {
  const requests: Array<{ contentType?: string; body: any }> = [];
  const http = createServer(async (request: IncomingMessage, response) => {
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(chunk as Buffer);
    requests.push({
      contentType: request.headers["content-type"],
      body: JSON.parse(Buffer.concat(chunks).toString("utf8")),
    });
    response.writeHead(answer.status, { "content-type": "application/json" });
    response.end(JSON.stringify(answer.body));
  });
  await new Promise<void>((resolve) => http.listen(0, "127.0.0.1", resolve));
  const port = (http.address() as AddressInfo).port;
  return {
    requests,
    endpoint: `http://127.0.0.1:${port}/agent-results/bnM/c2lk`,
    close: () => new Promise<void>((resolve) => http.close(() => resolve())),
  };
}

async function toolEnv(endpoint: string, extra: Record<string, string> = {}) {
  const root = await mkdtemp(path.join(tmpdir(), "bex-result-"));
  const record = path.join(root, "run", "result.json");
  return {
    record,
    env: {
      BEX_RESULT_ENDPOINT: endpoint,
      BEX_RESULT_SCHEMA: schema,
      BEX_RESULT_RECORD_PATH: record,
      ...extra,
    } as NodeJS.ProcessEnv,
  };
}

test("an invalid result goes back to the agent with the problems and is never posted", async (t) => {
  const gateway = await stubGateway();
  t.after(gateway.close);
  const { env, record } = await toolEnv(gateway.endpoint);
  const outcome = await submitResult(
    { result: { tests_passed: "yes", files_changed: -1 }, final: true },
    env,
  );
  assert.equal(outcome.ok, false);
  if (outcome.ok) return;
  assert.equal(outcome.error.code, "schema_mismatch");
  assert.match(outcome.error.message, /\/tests_passed: must be boolean/);
  assert.match(outcome.error.message, /\/files_changed: must be >= 0/);
  assert.match(outcome.error.hint ?? "", /call submit_result again/);
  assert.equal(gateway.requests.length, 0);
  assert.equal((await readResultRecord(record)).final, false);
});

test("a valid final result is posted once and recorded", async (t) => {
  const gateway = await stubGateway();
  t.after(gateway.close);
  const { env, record } = await toolEnv(gateway.endpoint);
  const outcome = await submitResult(
    { result: { tests_passed: true, files_changed: 3 }, final: true },
    env,
  );
  assert.deepEqual(outcome, { ok: true, result: { accepted: true, final: true } });
  assert.equal(gateway.requests.length, 1);
  assert.equal(gateway.requests[0].contentType, "application/json");
  assert.deepEqual(gateway.requests[0].body, {
    result: { tests_passed: true, files_changed: 3 },
    final: true,
  });
  assert.equal((await readResultRecord(record)).final, true);
});

test("a partial result is posted but a later partial never downgrades a final one", async (t) => {
  const gateway = await stubGateway({ status: 200, body: { accepted: true } });
  t.after(gateway.close);
  const { env, record } = await toolEnv(gateway.endpoint);
  const result = { tests_passed: false, files_changed: 0 };
  await submitResult({ result, final: false }, env);
  assert.equal((await readResultRecord(record)).final, false);
  await submitResult({ result, final: true }, env);
  assert.equal((await readResultRecord(record)).final, true);
  await submitResult({ result, final: false }, env);
  assert.equal((await readResultRecord(record)).final, true);
  assert.equal(gateway.requests.length, 3);
});

test("gateway refusals reach the agent verbatim and record nothing", async (t) => {
  const gateway = await stubGateway({
    status: 422,
    body: { error: { code: "schema_mismatch", message: "/x: bad", hint: "fix it" } },
  });
  t.after(gateway.close);
  const { env, record } = await toolEnv(gateway.endpoint);
  const outcome = await submitResult(
    { result: { tests_passed: true, files_changed: 1 }, final: true },
    env,
  );
  assert.deepEqual(outcome, {
    ok: false,
    error: { code: "schema_mismatch", message: "/x: bad", hint: "fix it" },
  });
  assert.equal((await readResultRecord(record)).final, false);
});

test("malformed arguments and oversize results are refused before any request", async (t) => {
  const gateway = await stubGateway();
  t.after(gateway.close);
  const { env } = await toolEnv(gateway.endpoint);
  for (const args of [null, [], {}, { result: {} }, { result: {}, final: "yes" }]) {
    const outcome = await submitResult(args, env);
    assert.equal(outcome.ok, false);
    if (!outcome.ok) assert.equal(outcome.error.code, "invalid_request");
  }
  const huge = await submitResult(
    { result: { blob: "x".repeat(300 * 1024) }, final: true },
    env,
  );
  assert.equal(huge.ok, false);
  if (!huge.ok) assert.equal(huge.error.code, "too_large");
  assert.equal(gateway.requests.length, 0);
});

test("a turn without a schema has no submit_result tool and refuses the call", async () => {
  const listed = await handleMessage({ jsonrpc: "2.0", id: 1, method: "tools/list" }, {});
  assert.deepEqual((listed?.result as any).tools, []);
  const outcome = await submitResult({ result: {}, final: true }, {});
  assert.equal(outcome.ok, false);
  if (!outcome.ok) assert.equal(outcome.error.code, "not_expected");
});

test("the bex MCP server lists submit_result beside the other tools only with a schema", async () => {
  const withSchema = await handleMessage(
    { jsonrpc: "2.0", id: 1, method: "tools/list" },
    { BEX_RESULT_ENDPOINT: "http://gw/agent-results/a/b", BEX_RESULT_SCHEMA: schema },
  );
  assert.deepEqual(
    (withSchema?.result as any).tools.map((tool: { name: string }) => tool.name),
    [submitResultToolName],
  );
  const endpointOnly = await handleMessage(
    { jsonrpc: "2.0", id: 1, method: "tools/list" },
    { BEX_RESULT_ENDPOINT: "http://gw/agent-results/a/b" },
  );
  assert.deepEqual((endpointOnly?.result as any).tools, []);
});

test("the MCP server registration carries the result endpoint, schema and record only for a schema turn", () => {
  const base = { deliver: false, repoUrl: "", prRecordPath: "/x", shareUrl: "" };
  assert.deepEqual(bexMcpServers(base), []);
  const [server] = bexMcpServers({
    ...base,
    resultUrl: "http://gw/agent-results/a/b",
    outputSchema: schema,
    resultRecordPath: "/run/result.json",
  });
  assert.deepEqual(
    server.env.map((entry) => entry.name),
    ["BEX_RESULT_ENDPOINT", "BEX_RESULT_SCHEMA", "BEX_RESULT_RECORD_PATH"],
  );
});

test("the overlay explains the structured result, with the schema, only for a schema turn", () => {
  const base = {
    cwd: "/workspace",
    deliver: false,
    repoUrl: "",
    branch: "",
    baseBranch: "",
    repository: "",
  };
  assert.doesNotMatch(platformOverlay(base), /submit_result|Structured result/);
  assert.doesNotMatch(
    platformOverlay({ ...base, outputSchema: schema }),
    /Structured result/,
    "a schema without a result URL offers no tool, so no paragraph",
  );
  const overlay = platformOverlay({
    ...base,
    outputSchema: schema,
    resultUrl: "http://gw/agent-results/a/b",
  });
  assert.match(overlay, /## Structured result[\s\S]*`submit_result`[\s\S]*`final: true`/);
  assert.match(overlay, /"tests_passed"/);
  assert.match(overlay, /The task is not complete until a final result is accepted/);
  assert.match(
    platformOverlay({ ...base, outputSchema: schema, resultUrl: "http://gw/x", outputRequired: false }),
    /Submitting it is optional/,
  );
});

const hosted = {
  BEX_AGENT_PROFILE: "claude",
  BEX_AGENT_MODEL_PROXY_URL: "http://gateway/model/ns/session",
  BEX_AGENT_SESSION_ID: "session",
  BEX_AGENT_MODEL_API_KEY: "bex-model-proxy-placeholder-session",
  BEX_AGENT_TURN_TIMEOUT_MS: "1000",
} as NodeJS.ProcessEnv;

test("config reads the turn's output contract and rejects a malformed schema", () => {
  const env = {
    ...hosted,
    BEX_AGENT_RESULT_URL: "http://gw/agent-results/a/b",
    BEX_AGENT_OUTPUT_SCHEMA: schema,
  };
  const config = loadConfig(env);
  assert.equal(config.outputSchema, schema);
  assert.equal(config.resultUrl, "http://gw/agent-results/a/b");
  assert.equal(config.outputRequired, true, "required unless the platform says otherwise");
  assert.equal(loadConfig({ ...env, BEX_AGENT_OUTPUT_REQUIRED: "0" }).outputRequired, false);
  assert.throws(() => loadConfig({ ...env, BEX_AGENT_OUTPUT_SCHEMA: "[1]" }));
  assert.throws(() => loadConfig({ ...env, BEX_AGENT_OUTPUT_SCHEMA: "not json" }));
  assert.throws(() => loadConfig({ ...env, BEX_AGENT_OUTPUT_SCHEMA: JSON.stringify({ d: "x".repeat(70_000) }) }));
  // No schema or no URL: the turn is free-form and the tool is not offered.
  const freeForm = loadConfig({ ...hosted, BEX_AGENT_OUTPUT_SCHEMA: schema });
  assert.equal(freeForm.outputSchema, "");
  assert.equal(freeForm.resultUrl, "");
});

// structuredTurn runs a turn whose fixture agent records a final result per mode.
async function structuredTurn(mode: string, outputRequired = true) {
  const config = await tempConfig({
    turnTimeoutMs: 20_000,
    outputSchema: schema,
    outputRequired,
    resultUrl: "http://gw/agent-results/a/b",
    resultRecordPath: "",
  });
  config.resultRecordPath = path.join(config.root, "result.json");
  const promptsLog = path.join(config.root, "prompts.log");
  config.agentEnv = { ACP_FIXTURE_RESULT: mode, ACP_FIXTURE_PROMPTS_LOG: promptsLog };
  const result = await runHeadlessTurn(config, childEnv(config), new UIMessageStreamHub());
  return { config, result, sent: await promptsSent(promptsLog) };
}

test("an agent that submits a final result gets no reminder", async () => {
  const { result, sent } = await structuredTurn("first");
  assert.equal(sent.length, 1);
  assert.deepEqual(result.output, { final: true, reminded: false });
});

test("a missing final result gets exactly one reminder inside the same turn", async () => {
  const { config, result, sent } = await structuredTurn("on-reminder");
  assert.equal(sent.length, 2);
  assert.match(sent[1], /^This task requires a structured result and you have not submitted a final one\. Call submit_result now with `final: true`/);
  assert.doesNotMatch(sent[1], /bex platform context/);
  assert.deepEqual(result.output, { final: true, reminded: true });
  const status = JSON.parse(await readFile(config.statusPath, "utf8"));
  assert.deepEqual(status.output, { final: true, reminded: true });
});

test("a second miss still ends the turn succeeded and reports no final result", async () => {
  const { result, sent } = await structuredTurn("never");
  assert.equal(sent.length, 2, "one reminder, never a second");
  assert.equal(result.state, "succeeded");
  assert.deepEqual(result.output, { final: false, reminded: true });
});

test("an optional result is never reminded about", async () => {
  const { result, sent } = await structuredTurn("never", false);
  assert.equal(sent.length, 1);
  assert.deepEqual(result.output, { final: false, reminded: false });
});

test("a free-form turn reports no output and sends no reminder", async () => {
  const config = await tempConfig({ turnTimeoutMs: 20_000 });
  const promptsLog = path.join(config.root, "prompts.log");
  config.agentEnv = { ACP_FIXTURE_PROMPTS_LOG: promptsLog };
  const result = await runHeadlessTurn(config, childEnv(config), new UIMessageStreamHub());
  assert.equal((await promptsSent(promptsLog)).length, 1);
  assert.equal(result.output, undefined);
});
