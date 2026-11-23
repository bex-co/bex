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
import { mkdtemp, readFile, writeFile } from "node:fs/promises";
import { createServer, type IncomingMessage } from "node:http";
import type { AddressInfo } from "node:net";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { bexMcpServers } from "../src/acp.js";
import {
  markPullRequestReady,
  maxCommitMessageBytes,
  maxCommitTitleRunes,
  mergePullRequest,
  validateMergeInput,
} from "../src/merge-pr.js";
import { availableTools, handleMessage } from "../src/mcp-bex.js";

const base = "/git/bnM/c2lk/cmVwbw/YnJhbmNo/pull-request";

// stubGateway stands in for the gateway's pull-request/ready and
// pull-request/merge verbs, answering each path with its own answer.
async function stubGateway(
  answers: Record<string, { status: number; body: unknown }>,
) {
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
    const answer = answers[request.url ?? ""] ?? { status: 404, body: {} };
    response.writeHead(answer.status, { "content-type": "application/json" });
    response.end(
      typeof answer.body === "string"
        ? answer.body
        : JSON.stringify(answer.body),
    );
  });
  await new Promise<void>((resolve) => http.listen(0, "127.0.0.1", resolve));
  const origin = `http://127.0.0.1:${(http.address() as AddressInfo).port}`;
  return {
    requests,
    env: {
      BEX_PR_READY_ENDPOINT: `${origin}${base}/ready`,
      BEX_PR_MERGE_ENDPOINT: `${origin}${base}/merge`,
    } as NodeJS.ProcessEnv,
    close: () => new Promise<void>((resolve) => http.close(() => resolve())),
  };
}

const merged = {
  url: "https://github.com/acme/widgets/pull/12",
  number: 12,
  merged: true,
  sha: "fedcba9876543210fedcba9876543210fedcba98",
  method: "squash",
  headSha: "0123456789abcdef0123456789abcdef01234567",
};

test("mark_pull_request_ready posts an empty object to the gateway's ready verb", async (t) => {
  const gateway = await stubGateway({
    [`${base}/ready`]: { status: 200, body: { url: merged.url, number: 12 } },
  });
  t.after(gateway.close);
  const reply = await handleMessage(
    {
      jsonrpc: "2.0",
      id: 1,
      method: "tools/call",
      params: { name: "mark_pull_request_ready", arguments: {} },
    },
    gateway.env,
  );
  const result = (reply as { result: any }).result;
  assert.equal(result.isError, false);
  assert.deepEqual(result.structuredContent, { url: merged.url, number: 12 });
  assert.deepEqual(gateway.requests, [
    {
      method: "POST",
      url: `${base}/ready`,
      contentType: "application/json",
      body: "{}",
    },
  ]);
  // Arguments name nothing: a pull request number is refused before any request.
  const named = await markPullRequestReady({ number: 13 }, gateway.env);
  assert.equal(!named.ok && named.error.code, "invalid_request");
  assert.equal(gateway.requests.length, 1);
});

test("merge_pull_request sends only the method and commit text, and records the merged head", async (t) => {
  const gateway = await stubGateway({
    [`${base}/merge`]: { status: 200, body: merged },
  });
  t.after(gateway.close);
  const dir = await mkdtemp(path.join(tmpdir(), "bex-merge-"));
  const record = path.join(dir, "pull-request.json");
  await writeFile(
    record,
    JSON.stringify({
      url: merged.url,
      number: 12,
      created: true,
      headSha: merged.headSha,
    }),
  );
  const env = { ...gateway.env, BEX_PR_RECORD_PATH: record };

  const outcome = await mergePullRequest(
    {
      method: "squash",
      commitTitle: "  Add retries (#12) ",
      commitMessage: "",
    },
    env,
  );
  assert.deepEqual(outcome, { ok: true, result: merged });
  assert.deepEqual(gateway.requests, [
    {
      method: "POST",
      url: `${base}/merge`,
      contentType: "application/json",
      body: JSON.stringify({
        method: "squash",
        commitTitle: "Add retries (#12)",
      }),
    },
  ]);
  const written = JSON.parse(await readFile(record, "utf8"));
  assert.equal(written.merged, true);
  assert.equal(written.mergeSha, merged.sha);
  assert.equal(written.headSha, merged.headSha);
  assert.equal(written.created, true);

  // No arguments merge with the repository's default.
  const bare = await mergePullRequest(undefined, gateway.env);
  assert.equal(bare.ok, true);
  assert.equal(gateway.requests[1].body, "{}");
});

test("merge_pull_request with auto arms auto-merge, returns the armed answer and records nothing merged", async (t) => {
  const armed = {
    url: merged.url,
    number: 12,
    merged: false,
    method: "squash",
    headSha: merged.headSha,
    autoMerge: true,
  };
  const gateway = await stubGateway({
    [`${base}/merge`]: { status: 200, body: armed },
  });
  t.after(gateway.close);
  const dir = await mkdtemp(path.join(tmpdir(), "bex-merge-"));
  const record = path.join(dir, "pull-request.json");
  const opened = JSON.stringify({
    url: merged.url,
    number: 12,
    created: true,
    headSha: merged.headSha,
  });
  await writeFile(record, opened);
  const env = { ...gateway.env, BEX_PR_RECORD_PATH: record };

  const reply = await handleMessage(
    {
      jsonrpc: "2.0",
      id: 3,
      method: "tools/call",
      params: {
        name: "merge_pull_request",
        arguments: { auto: true, method: "squash" },
      },
    },
    env,
  );
  const result = (reply as { result: any }).result;
  assert.equal(result.isError, false);
  assert.deepEqual(result.structuredContent, armed);
  assert.equal(
    gateway.requests[0].body,
    JSON.stringify({ method: "squash", auto: true }),
  );
  // The delivery check reads no merged head for an armed pull request.
  assert.equal(await readFile(record, "utf8"), opened);

  // A queued answer and an undo come back as they are, and record nothing.
  for (const answer of [
    {
      url: merged.url,
      number: 12,
      merged: false,
      headSha: merged.headSha,
      mergeQueue: true,
    },
    {
      url: merged.url,
      number: 12,
      merged: false,
      headSha: merged.headSha,
      autoMergeDisabled: true,
    },
  ]) {
    const later = await stubGateway({
      [`${base}/merge`]: { status: 200, body: answer },
    });
    t.after(later.close);
    const outcome = await mergePullRequest(
      answer.autoMergeDisabled ? { disableAuto: true } : {},
      { ...later.env, BEX_PR_RECORD_PATH: record },
    );
    assert.deepEqual(outcome, { ok: true, result: answer });
    assert.equal(
      later.requests[0].body,
      answer.autoMergeDisabled ? JSON.stringify({ disableAuto: true }) : "{}",
    );
    assert.equal(await readFile(record, "utf8"), opened);
  }
});

test("an answer with no outcome, two outcomes or a merge without its commit is refused", async (t) => {
  for (const body of [
    { url: merged.url, number: 12, merged: false, headSha: merged.headSha },
    {
      url: merged.url,
      number: 12,
      merged: false,
      autoMerge: true,
      mergeQueue: true,
    },
    { ...merged, autoMerge: true },
    { ...merged, sha: "" },
    { url: merged.url, number: 12, autoMerge: true },
    { url: "", number: 12, merged: false, autoMerge: true },
    { url: merged.url, number: 12, merged: false, autoMerge: "yes" },
  ]) {
    const gateway = await stubGateway({
      [`${base}/merge`]: { status: 200, body },
    });
    t.after(gateway.close);
    const outcome = await mergePullRequest({ auto: true }, gateway.env);
    assert.equal(
      !outcome.ok && outcome.error.code,
      "upstream_error",
      JSON.stringify(body),
    );
  }
});

test("GitHub's refusals pass through verbatim; other failures become upstream_error", async (t) => {
  const error = {
    code: "merge_blocked",
    message: 'GitHub: Required status check "test" is expected.',
    hint: "Required checks pending: wait until they finish (`gh pr checks --watch`), then call merge_pull_request once more. Do not call merge_pull_request in a loop.",
  };
  const refusing = await stubGateway({
    [`${base}/merge`]: { status: 409, body: { error } },
  });
  t.after(refusing.close);
  const reply = await handleMessage(
    {
      jsonrpc: "2.0",
      id: 2,
      method: "tools/call",
      params: { name: "merge_pull_request", arguments: {} },
    },
    refusing.env,
  );
  const result = (reply as { result: any }).result;
  assert.equal(result.isError, true);
  assert.deepEqual(result.structuredContent, { error });

  const odd = await stubGateway({
    [`${base}/merge`]: { status: 200, body: { ...merged, merged: false } },
    [`${base}/ready`]: { status: 200, body: { ready: true } },
  });
  t.after(odd.close);
  const notMerged = await mergePullRequest({}, odd.env);
  assert.equal(!notMerged.ok && notMerged.error.code, "upstream_error");
  const notReady = await markPullRequestReady({}, odd.env);
  assert.equal(!notReady.ok && notReady.error.code, "upstream_error");

  const unconfigured = await mergePullRequest({}, {});
  assert.equal(!unconfigured.ok && unconfigured.error.code, "upstream_error");
});

test("malformed merges are rejected before any request", async (t) => {
  const gateway = await stubGateway({});
  t.after(gateway.close);
  const cases: [unknown, string][] = [
    [[], "invalid_request"],
    ["squash", "invalid_request"],
    [{ number: 12 }, "invalid_request"],
    [{ sha: "abc" }, "invalid_request"],
    [{ merge_method: "squash" }, "invalid_request"],
    [{ method: "octopus" }, "invalid_request"],
    [{ method: 1 }, "invalid_request"],
    [{ commitTitle: "two\nlines" }, "invalid_request"],
    [{ commitTitle: "lone\ud800" }, "invalid_request"],
    [{ commitMessage: "nul\u0000byte" }, "invalid_request"],
    [{ commitTitle: "é".repeat(maxCommitTitleRunes + 1) }, "too_large"],
    [{ commitMessage: "é".repeat(maxCommitMessageBytes / 2 + 1) }, "too_large"],
    [{ auto: "true" }, "invalid_request"],
    [{ auto: 1 }, "invalid_request"],
    [{ disableAuto: "yes" }, "invalid_request"],
    [{ disableAuto: true, auto: true }, "invalid_request"],
    [{ disableAuto: true, method: "squash" }, "invalid_request"],
    [{ disableAuto: true, commitTitle: "x" }, "invalid_request"],
    [{ auto: true, expectedHeadOid: "abc" }, "invalid_request"],
  ];
  for (const [args, code] of cases) {
    const outcome = await mergePullRequest(args, gateway.env);
    assert.equal(outcome.ok, false, JSON.stringify(args));
    assert.equal(!outcome.ok && outcome.error.code, code, JSON.stringify(args));
  }
  assert.equal(gateway.requests.length, 0);
  assert.deepEqual(
    validateMergeInput({ commitTitle: "é".repeat(maxCommitTitleRunes) }),
    {
      ok: true,
      input: { commitTitle: "é".repeat(maxCommitTitleRunes) },
    },
  );
  // false is the default and is not sent; an empty commit text beside
  // disableAuto is none, as the gateway reads it.
  assert.deepEqual(validateMergeInput({ auto: false, disableAuto: false }), {
    ok: true,
    input: {},
  });
  assert.deepEqual(
    validateMergeInput({ disableAuto: true, auto: false, commitTitle: " " }),
    { ok: true, input: { disableAuto: true } },
  );
});

test("only repo sessions list the ready and merge tools, saying to use them only when asked", async () => {
  const [repo] = bexMcpServers({
    deliver: true,
    repoUrl: "http://gw:8082/git/bnM/c2lk/cmVwbw/YnJhbmNo/",
    prRecordPath: "/run/pull-request.json",
    shareUrl: "",
  });
  const env = Object.fromEntries(
    repo.env.map(({ name, value }) => [name, value]),
  );
  assert.equal(env.BEX_PR_READY_ENDPOINT, `http://gw:8082${base}/ready`);
  assert.equal(env.BEX_PR_MERGE_ENDPOINT, `http://gw:8082${base}/merge`);
  const tools = availableTools(env) as Array<{
    name: string;
    description: string;
    inputSchema: any;
  }>;
  for (const name of ["mark_pull_request_ready", "merge_pull_request"]) {
    const tool = tools.find((candidate) => candidate.name === name);
    assert.ok(tool, name);
    assert.match(
      tool.description,
      /only when the task or a person's message asks for it/,
    );
    assert.equal(tool.inputSchema.additionalProperties, false);
    assert.ok(
      !("number" in tool.inputSchema.properties),
      "no tool takes a pull request number",
    );
  }
  // w10/m577: the merge tool offers auto and its undo, and says to arm it
  // and end the turn rather than wait for the checks.
  const merge = tools.find((tool) => tool.name === "merge_pull_request")!;
  assert.equal(merge.inputSchema.properties.auto.type, "boolean");
  assert.equal(merge.inputSchema.properties.disableAuto.type, "boolean");
  assert.match(
    merge.description,
    /When asked to merge once the checks pass, call it with auto: true and end your turn/,
  );
  assert.match(merge.description, /merge queue/);
  assert.match(merge.description, /only disableAuto: true/);
  const init = await handleMessage(
    { jsonrpc: "2.0", id: 1, method: "initialize", params: {} },
    env,
  );
  assert.match(
    (init as { result: { instructions: string } }).result.instructions,
    /mark_pull_request_ready and merge_pull_request to mark the session's pull request ready and merge it, only when the task or a person's message asks/,
  );

  // A repo-less session has no pull request to merge.
  const [scratch] = bexMcpServers({
    deliver: false,
    repoUrl: "",
    prRecordPath: "/x",
    shareUrl: "http://gw:8082/agent-attachments/bnM/c2lk",
  });
  const scratchEnv = Object.fromEntries(
    scratch.env.map(({ name, value }) => [name, value]),
  );
  const scratchNames = availableTools(scratchEnv).map((tool) => tool.name);
  assert.ok(!scratchNames.includes("mark_pull_request_ready"));
  assert.ok(!scratchNames.includes("merge_pull_request"));
  const reply = await handleMessage(
    {
      jsonrpc: "2.0",
      id: 1,
      method: "tools/call",
      params: { name: "merge_pull_request", arguments: {} },
    },
    scratchEnv,
  );
  assert.equal((reply as { error?: { code: number } }).error?.code, -32602);
});
