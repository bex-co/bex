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
import { availableTools, handleMessage } from "../src/mcp-bex.js";
import {
  maxCommentBytes,
  replyOnPullRequest,
  validateReplyInput,
} from "../src/reply-on-pr.js";

const commentPath = "/git/bnM/c2lk/cmVwbw/YnJhbmNo/pull-request/comment";

// stubGateway stands in for the gateway's pull-request/comment verb.
async function stubGateway(answer: { status: number; body: unknown }) {
  const requests: {
    method?: string;
    url?: string;
    contentType?: string;
    body: string;
  }[] = [];
  const http = createServer(async (request: IncomingMessage, response) => {
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(chunk as Buffer);
    requests.push({
      method: request.method,
      url: request.url,
      contentType: request.headers["content-type"],
      body: Buffer.concat(chunks).toString("utf8"),
    });
    response.writeHead(answer.status, { "content-type": "application/json" });
    response.end(
      typeof answer.body === "string" ? answer.body : JSON.stringify(answer.body),
    );
  });
  await new Promise<void>((resolve) => http.listen(0, "127.0.0.1", resolve));
  const port = (http.address() as AddressInfo).port;
  return {
    requests,
    env: { BEX_PR_COMMENT_ENDPOINT: `http://127.0.0.1:${port}${commentPath}` },
    close: () => new Promise<void>((resolve) => http.close(() => resolve())),
  };
}

test("reply_on_pull_request posts only the body to the gateway's comment verb", async (t) => {
  const gateway = await stubGateway({
    status: 200,
    body: { url: "https://github.com/acme/widgets/pull/12#issuecomment-1", number: 12 },
  });
  t.after(gateway.close);
  const reply = await handleMessage(
    {
      jsonrpc: "2.0",
      id: 1,
      method: "tools/call",
      params: {
        name: "reply_on_pull_request",
        arguments: { body: "Fixed the nil check in `retry.go`." },
      },
    },
    gateway.env,
  );
  const result = (reply as { result: any }).result;
  assert.equal(result.isError, false);
  assert.deepEqual(result.structuredContent, {
    url: "https://github.com/acme/widgets/pull/12#issuecomment-1",
    number: 12,
  });
  assert.deepEqual(gateway.requests, [
    {
      method: "POST",
      url: commentPath,
      contentType: "application/json",
      body: JSON.stringify({ body: "Fixed the nil check in `retry.go`." }),
    },
  ]);
});

test("gateway refusals pass through verbatim; other failures become upstream_error", async (t) => {
  const error = {
    code: "no_pull_request",
    message: "this session has not opened its pull request yet",
    hint: "Push your work and open the draft pull request first, then call this tool again.",
  };
  const refusing = await stubGateway({ status: 409, body: { error } });
  t.after(refusing.close);
  assert.deepEqual(await replyOnPullRequest({ body: "b" }, refusing.env), {
    ok: false,
    error,
  });

  const broken = await stubGateway({ status: 502, body: "bad gateway" });
  t.after(broken.close);
  const failed = await replyOnPullRequest({ body: "b" }, broken.env);
  assert.equal(failed.ok, false);
  assert.equal(!failed.ok && failed.error.code, "upstream_error");

  const odd = await stubGateway({ status: 200, body: { posted: true } });
  t.after(odd.close);
  const unexpected = await replyOnPullRequest({ body: "b" }, odd.env);
  assert.equal(!unexpected.ok && unexpected.error.code, "upstream_error");

  const unconfigured = await replyOnPullRequest({ body: "b" }, {});
  assert.equal(!unconfigured.ok && unconfigured.error.code, "upstream_error");
});

test("malformed and oversize comments are rejected before any request", async (t) => {
  const gateway = await stubGateway({ status: 500, body: {} });
  t.after(gateway.close);
  const cases: [unknown, string][] = [
    [{ body: "" }, "invalid_request"],
    [{ body: " \n\t" }, "invalid_request"],
    [{}, "invalid_request"],
    [{ body: 7 }, "invalid_request"],
    [[], "invalid_request"],
    [{ body: "nul\u0000byte" }, "invalid_request"],
    [{ body: "lone\ud800" }, "invalid_request"],
    [{ body: "é".repeat(maxCommentBytes / 2 + 1) }, "too_large"],
  ];
  for (const [args, code] of cases) {
    const outcome = await replyOnPullRequest(args, gateway.env);
    assert.equal(outcome.ok, false, JSON.stringify(args));
    assert.equal(!outcome.ok && outcome.error.code, code);
  }
  assert.equal(gateway.requests.length, 0);
  assert.equal(validateReplyInput({ body: "x".repeat(maxCommentBytes) }), undefined);
});

test("only repo sessions get reply_on_pull_request, beside the PR tool", async () => {
  const names = (env: NodeJS.ProcessEnv) =>
    availableTools(env).map((tool) => tool.name);
  const [repo] = bexMcpServers({
    deliver: true,
    repoUrl: "http://gw:8082/git/bnM/c2lk/cmVwbw/YnJhbmNo/",
    prRecordPath: "/run/pull-request.json",
    shareUrl: "",
  });
  const env = Object.fromEntries(
    repo.env.map(({ name, value }) => [name, value]),
  );
  assert.equal(
    env.BEX_PR_COMMENT_ENDPOINT,
    `http://gw:8082${commentPath}`,
  );
  assert.deepEqual(names(env), [
    "open_or_update_draft_pr",
    "reply_on_pull_request",
  ]);
  // A repo-less session has no pull request to reply on.
  const [scratch] = bexMcpServers({
    deliver: false,
    repoUrl: "",
    prRecordPath: "/x",
    shareUrl: "http://gw:8082/agent-attachments/bnM/c2lk",
  });
  assert.ok(!scratch.env.some(({ name }) => name === "BEX_PR_COMMENT_ENDPOINT"));
  assert.ok(!names({ BEX_SHARE_ENDPOINT: "http://gw/share" }).includes("reply_on_pull_request"));
  // Without its endpoint the tool is unknown, not callable.
  const reply = await handleMessage(
    {
      jsonrpc: "2.0",
      id: 1,
      method: "tools/call",
      params: { name: "reply_on_pull_request", arguments: { body: "b" } },
    },
    { BEX_PR_ENDPOINT: "http://gw/pr" },
  );
  assert.equal((reply as { error?: { code: number } }).error?.code, -32602);
});
