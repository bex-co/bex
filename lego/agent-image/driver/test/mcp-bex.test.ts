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
import { spawn } from "node:child_process";
import { mkdtemp, readFile, stat } from "node:fs/promises";
import { createServer, type IncomingMessage } from "node:http";
import type { AddressInfo } from "node:net";
import { tmpdir } from "node:os";
import path from "node:path";
import readline from "node:readline";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { maxBodyBytes, validateInput } from "../src/mcp-bex.js";

const here = path.dirname(fileURLToPath(import.meta.url));
const server = path.join(here, "..", "src", "mcp-bex.ts");

interface StubRequest {
  method?: string;
  url?: string;
  contentType?: string;
  body: unknown;
}

// stubGateway stands in for the gateway's pull-request verb.
async function stubGateway(
  reply: (body: Record<string, unknown>) => { status: number; body: unknown },
) {
  const requests: StubRequest[] = [];
  const http = createServer(async (request: IncomingMessage, response) => {
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(chunk as Buffer);
    const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
    requests.push({
      method: request.method,
      url: request.url,
      contentType: request.headers["content-type"],
      body,
    });
    const answer = reply(body);
    response.writeHead(answer.status, { "content-type": "application/json" });
    response.end(JSON.stringify(answer.body));
  });
  await new Promise<void>((resolve) => http.listen(0, "127.0.0.1", resolve));
  const port = (http.address() as AddressInfo).port;
  return {
    requests,
    endpoint: `http://127.0.0.1:${port}/git/bnM/c2lk/cmVwbw/YnJhbmNo/pull-request`,
    close: () => new Promise<void>((resolve) => http.close(() => resolve())),
  };
}

// mcpClient speaks newline-delimited JSON-RPC to the real server process.
function mcpClient(env: Record<string, string>) {
  const child = spawn(process.execPath, ["--import", "tsx", server], {
    cwd: path.join(here, ".."),
    env: { ...process.env, ...env },
    stdio: ["pipe", "pipe", "inherit"],
  });
  const pending = new Map<number, (message: any) => void>();
  readline.createInterface({ input: child.stdout }).on("line", (line) => {
    const message = JSON.parse(line);
    pending.get(message.id)?.(message);
    pending.delete(message.id);
  });
  let nextId = 1;
  return {
    request(method: string, params?: unknown): Promise<any> {
      const id = nextId++;
      child.stdin.write(
        `${JSON.stringify({ jsonrpc: "2.0", id, method, params })}\n`,
      );
      return new Promise((resolve) => pending.set(id, resolve));
    },
    notify(method: string) {
      child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", method })}\n`);
    },
    close() {
      child.stdin.end();
      return new Promise<void>((resolve) =>
        child.once("close", () => resolve()),
      );
    },
  };
}

test("bex MCP server lists one tool and opens the draft PR through the gateway", async (t) => {
  const gateway = await stubGateway(() => ({
    status: 200,
    body: {
      url: "https://github.com/acme/widgets/pull/12",
      number: 12,
      created: true,
      headSha: "a".repeat(40),
    },
  }));
  t.after(gateway.close);
  const root = await mkdtemp(path.join(tmpdir(), "bex-mcp-"));
  const record = path.join(root, "run", "pull-request.json");
  const client = mcpClient({
    BEX_PR_ENDPOINT: gateway.endpoint,
    BEX_PR_RECORD_PATH: record,
  });
  t.after(() => client.close());

  const init = await client.request("initialize", {
    protocolVersion: "2025-06-18",
    capabilities: {},
    clientInfo: { name: "test", version: "1" },
  });
  assert.equal(init.result.protocolVersion, "2025-06-18");
  assert.deepEqual(init.result.capabilities, { tools: { listChanged: false } });
  assert.equal(init.result.serverInfo.name, "bex");
  client.notify("notifications/initialized");
  assert.deepEqual((await client.request("ping")).result, {});

  const listed = await client.request("tools/list");
  assert.equal(listed.result.tools.length, 1);
  const [tool] = listed.result.tools;
  assert.equal(tool.name, "open_or_update_draft_pr");
  assert.deepEqual(tool.inputSchema.required, ["title", "body"]);
  assert.match(tool.description, /after you have committed and pushed/);
  assert.match(tool.description, /idempotent/);
  assert.match(tool.description, /footer is appended automatically/);
  assert.match(
    tool.description,
    /do not try other ways to create a pull request/,
  );

  const called = await client.request("tools/call", {
    name: "open_or_update_draft_pr",
    arguments: { title: "Fix the flaky test", body: "## Summary\nFixed." },
  });
  assert.equal(called.result.isError, false);
  assert.deepEqual(called.result.structuredContent, {
    url: "https://github.com/acme/widgets/pull/12",
    number: 12,
    created: true,
    headSha: "a".repeat(40),
  });
  assert.deepEqual(
    JSON.parse(called.result.content[0].text),
    called.result.structuredContent,
  );
  assert.deepEqual(gateway.requests, [
    {
      method: "POST",
      url: "/git/bnM/c2lk/cmVwbw/YnJhbmNo/pull-request",
      contentType: "application/json",
      body: { title: "Fix the flaky test", body: "## Summary\nFixed." },
    },
  ]);
  // The driver-owned record tells the post-turn delivery check a PR exists.
  const recorded = JSON.parse(await readFile(record, "utf8"));
  assert.equal(recorded.url, "https://github.com/acme/widgets/pull/12");
  assert.equal(recorded.number, 12);
  assert.equal((await stat(record)).mode & 0o777, 0o600);

  const unknown = await client.request("tools/call", { name: "gh_pr_create" });
  assert.equal(unknown.error.code, -32602);
  assert.equal((await client.request("resources/list")).error.code, -32601);
});

test("gateway errors pass through verbatim and record nothing", async (t) => {
  const error = {
    code: "branch_not_pushed",
    message: "bex-agent/ags-1 does not exist on the remote",
    hint: "Push the branch first: git push origin HEAD:bex-agent/ags-1",
  };
  const gateway = await stubGateway(() => ({ status: 409, body: { error } }));
  t.after(gateway.close);
  const root = await mkdtemp(path.join(tmpdir(), "bex-mcp-"));
  const record = path.join(root, "pull-request.json");
  const client = mcpClient({
    BEX_PR_ENDPOINT: gateway.endpoint,
    BEX_PR_RECORD_PATH: record,
  });
  t.after(() => client.close());
  const called = await client.request("tools/call", {
    name: "open_or_update_draft_pr",
    arguments: { title: "t", body: "b" },
  });
  assert.equal(called.result.isError, true);
  assert.deepEqual(called.result.structuredContent, { error });
  await assert.rejects(stat(record), { code: "ENOENT" });
});

test("oversize and malformed input is rejected before any request", async (t) => {
  const gateway = await stubGateway(() => ({ status: 500, body: {} }));
  t.after(gateway.close);
  const client = mcpClient({ BEX_PR_ENDPOINT: gateway.endpoint });
  t.after(() => client.close());
  const cases: [unknown, string][] = [
    [{ title: "x".repeat(257), body: "" }, "too_large"],
    [{ title: "ok", body: "é".repeat(maxBodyBytes / 2 + 1) }, "too_large"],
    [{ title: "two\nlines", body: "" }, "invalid_request"],
    [{ title: "  ", body: "" }, "invalid_request"],
    [{ title: "tab\there", body: "" }, "invalid_request"],
    [{ title: "bell\u0007", body: "" }, "invalid_request"],
    [{ title: "c1\u0085", body: "" }, "invalid_request"],
    [{ title: "ok", body: "nul\u0000byte" }, "invalid_request"],
    [{ title: "lone\ud800", body: "" }, "invalid_request"],
    [{ title: "ok", body: "lone\udc00" }, "invalid_request"],
    [{ title: "ok" }, "invalid_request"],
  ];
  for (const [args, code] of cases) {
    const called = await client.request("tools/call", {
      name: "open_or_update_draft_pr",
      arguments: args,
    });
    assert.equal(called.result.isError, true);
    assert.equal(called.result.structuredContent.error.code, code);
  }
  assert.equal(gateway.requests.length, 0);
  // 256 runes is the inclusive limit, counted by code point, not UTF-16 unit.
  assert.equal(validateInput({ title: "😀".repeat(256), body: "" }), undefined);
  // Like the gateway, the limit applies to the trimmed title.
  assert.equal(
    validateInput({ title: ` ${"x".repeat(256)} `, body: "" }),
    undefined,
  );
  assert.equal(
    validateInput({ title: "t", body: "x".repeat(maxBodyBytes) }),
    undefined,
  );
});

test("a non-JSON gateway failure becomes upstream_error", async (t) => {
  const gateway = await stubGateway(() => ({ status: 502, body: "bad" }));
  t.after(gateway.close);
  const client = mcpClient({ BEX_PR_ENDPOINT: gateway.endpoint });
  t.after(() => client.close());
  const called = await client.request("tools/call", {
    name: "open_or_update_draft_pr",
    arguments: { title: "t", body: "b" },
  });
  assert.equal(called.result.isError, true);
  assert.equal(called.result.structuredContent.error.code, "upstream_error");
});
