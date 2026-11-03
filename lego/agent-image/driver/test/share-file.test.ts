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
import { createHash } from "node:crypto";
import { mkdir, mkdtemp, symlink, truncate, writeFile } from "node:fs/promises";
import { createServer, type IncomingHttpHeaders } from "node:http";
import type { AddressInfo } from "node:net";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { availableTools, handleMessage } from "../src/mcp-bex.js";
import {
  maxShareFileBytes,
  maxShareFileNameBytes,
  shareFile,
} from "../src/share-file.js";

interface Received {
  headers: IncomingHttpHeaders;
  body: Buffer;
}

// stubShare stands in for the gateway's share route.
async function stubShare(answer: { status: number; body: unknown }) {
  const received: Received[] = [];
  const http = createServer(async (request, response) => {
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(chunk as Buffer);
    received.push({ headers: request.headers, body: Buffer.concat(chunks) });
    response.writeHead(answer.status, { "content-type": "application/json" });
    response.end(JSON.stringify(answer.body));
  });
  await new Promise<void>((resolve) => http.listen(0, "127.0.0.1", resolve));
  const port = (http.address() as AddressInfo).port;
  return {
    endpoint: `http://127.0.0.1:${port}/agent-attachments/bnM/c2lk`,
    received,
    close: () => new Promise<void>((resolve) => http.close(() => resolve())),
  };
}

// layout builds a workspace and an outside directory under a fresh temp root;
// the workspace is the only allowed root in these tests.
async function layout() {
  const root = await mkdtemp(path.join(tmpdir(), "bex-share-"));
  const workspace = path.join(root, "workspace");
  const outside = path.join(root, "outside");
  await mkdir(workspace);
  await mkdir(outside);
  return { root, workspace, outside };
}

test("share_file posts the bytes with name and digest", async (t) => {
  const gateway = await stubShare({
    status: 200,
    body: { attachmentId: "att-1", name: "report é.csv", mediaType: "text/csv", size: 6 },
  });
  t.after(gateway.close);
  const { workspace } = await layout();
  const file = path.join(workspace, "out.csv");
  await writeFile(file, "a,b\n1\n");

  const outcome = await shareFile(
    { path: file, name: "report é.csv" },
    { BEX_SHARE_ENDPOINT: gateway.endpoint },
    [workspace],
  );
  assert.deepEqual(outcome, {
    ok: true,
    result: { attachmentId: "att-1", name: "report é.csv", mediaType: "text/csv", size: 6 },
  });
  const [sent] = gateway.received;
  assert.equal(sent.body.toString(), "a,b\n1\n");
  assert.equal(decodeURIComponent(String(sent.headers["x-bex-share-name"])), "report é.csv");
  assert.equal(sent.headers["x-bex-share-description"], undefined);
  assert.equal(
    sent.headers["x-bex-share-sha256"],
    createHash("sha256").update("a,b\n1\n").digest("hex"),
  );
  assert.equal(sent.headers["content-type"], "application/octet-stream");
});

test("share_file refuses paths outside the roots, through symlinks too", async (t) => {
  const gateway = await stubShare({ status: 200, body: {} });
  t.after(gateway.close);
  const { workspace, outside } = await layout();
  const secret = path.join(outside, "secret.txt");
  await writeFile(secret, "nope");
  const link = path.join(workspace, "link.txt");
  await symlink(secret, link);
  const env = { BEX_SHARE_ENDPOINT: gateway.endpoint };

  for (const target of [secret, link, path.join(workspace, "..", "outside", "secret.txt")]) {
    const outcome = await shareFile({ path: target }, env, [workspace]);
    assert.equal(outcome.ok, false, target);
    assert.equal(!outcome.ok && outcome.error.code, "invalid_request");
  }
  assert.equal(gateway.received.length, 0, "nothing may leave the sandbox");
});

test("share_file refuses directories, empty files, missing files and oversize files", async (t) => {
  const gateway = await stubShare({ status: 200, body: {} });
  t.after(gateway.close);
  const { workspace } = await layout();
  const env = { BEX_SHARE_ENDPOINT: gateway.endpoint };
  const empty = path.join(workspace, "empty");
  await writeFile(empty, "");
  const huge = path.join(workspace, "huge.bin");
  await writeFile(huge, "");
  await truncate(huge, maxShareFileBytes + 1);

  const codes = [];
  for (const target of [workspace, empty, path.join(workspace, "missing"), huge]) {
    const outcome = await shareFile({ path: target }, env, [workspace]);
    codes.push(!outcome.ok && outcome.error.code);
  }
  assert.deepEqual(codes, ["invalid_request", "invalid_request", "invalid_request", "too_large"]);
  assert.equal(gateway.received.length, 0);
});

test("gateway refusals reach the agent verbatim", async (t) => {
  const gateway = await stubShare({
    status: 409,
    body: { error: { code: "quota_exceeded", message: "a session holds at most 50 files", hint: "tell the user" } },
  });
  t.after(gateway.close);
  const { workspace } = await layout();
  const file = path.join(workspace, "a.txt");
  await writeFile(file, "x");
  const outcome = await shareFile({ path: file }, { BEX_SHARE_ENDPOINT: gateway.endpoint }, [workspace]);
  assert.deepEqual(outcome, {
    ok: false,
    error: { code: "quota_exceeded", message: "a session holds at most 50 files", hint: "tell the user" },
  });
});

test("the bex MCP server lists each tool only with its endpoint", async () => {
  const names = (env: NodeJS.ProcessEnv) => availableTools(env).map((tool) => tool.name);
  assert.deepEqual(names({}), []);
  assert.deepEqual(names({ BEX_PR_ENDPOINT: "http://gw/pr" }), ["open_or_update_draft_pr"]);
  assert.deepEqual(names({ BEX_SHARE_ENDPOINT: "http://gw/share" }), ["share_file"]);
  assert.deepEqual(names({ BEX_PR_ENDPOINT: "http://gw/pr", BEX_SHARE_ENDPOINT: "http://gw/share" }), [
    "open_or_update_draft_pr",
    "share_file",
  ]);
  // A tool the session lacks is unknown, not callable.
  const reply = await handleMessage(
    { jsonrpc: "2.0", id: 1, method: "tools/call", params: { name: "share_file", arguments: {} } },
    { BEX_PR_ENDPOINT: "http://gw/pr" },
  );
  assert.equal((reply as { error?: { code: number } }).error?.code, -32602);
});

test("share_file refuses a name past the gateway's cap before posting", async (t) => {
  const gateway = await stubShare({ status: 200, body: {} });
  t.after(gateway.close);
  const { workspace } = await layout();
  const file = path.join(workspace, "a.txt");
  await writeFile(file, "x");
  const outcome = await shareFile(
    { path: file, name: "é".repeat(maxShareFileNameBytes) },
    { BEX_SHARE_ENDPOINT: gateway.endpoint },
    [workspace],
  );
  assert.equal(!outcome.ok && outcome.error.code, "invalid_request");
  assert.equal(gateway.received.length, 0);
});
