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

// The `bex` stdio MCP server (ADR090, ADR095 D6). The driver registers it on
// session/new and session/load; the agent's adapter launches it. It lists a
// tool only when the session has its endpoint:
//   - open_or_update_draft_pr (repo sessions) forwards to the gateway's
//     pull-request verb on the session's Pod-bound Git proxy path. Owner,
//     repository, head and base are fixed server-side by that path, so the
//     agent supplies only a title and a body.
//   - reply_on_pull_request (repo sessions, ADR047 D11) forwards to the
//     gateway's pull-request/comment verb on the same path: one comment on the
//     session's own pull request, whose number is fixed server-side.
//   - share_file (when the deployment stores attachments) posts a file to the
//     session's Pod-bound share URL.
//   - submit_result (when the turn carries an outputSchema, ADR094 D5)
//     validates a structured result against that schema and posts it to the
//     session's Pod-bound result URL.
//   - spawn_session and get_child_session (when the session can still start
//     children, ADR102 D9) start a child session and read the sessions this
//     one started, through the session's Pod-bound spawn URL.
//   - wait_child_sessions, message_child_session and cancel_child_session
//     (same condition and URL, w9/m346) block until named descendants finish,
//     start a follow-up turn on one, or cancel one.
// No secret is involved: the gateway authenticates the source Pod.
//
// The transport is newline-delimited JSON-RPC 2.0 over stdio, hand-rolled to
// keep the image free of another runtime dependency.

import path from "node:path";
import readline from "node:readline";
import { fileURLToPath } from "node:url";
import { describeError } from "./errors.js";
import { postToGateway, type ToolError } from "./gateway.js";
import { writeJsonAtomic } from "./fs-util.js";
import {
  submitResult,
  submitResultToolDefinition,
  submitResultToolName,
} from "./submit-result.js";
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
  waitChildSessions,
  waitChildSessionsToolDefinition,
  waitChildSessionsToolName,
} from "./spawn-session.js";
import {
  shareFile,
  shareFileToolDefinition,
  shareFileToolName,
} from "./share-file.js";
import {
  replyOnPullRequest,
  replyOnPullRequestToolDefinition,
  replyOnPullRequestToolName,
} from "./reply-on-pr.js";

export const toolName = "open_or_update_draft_pr";
export const maxTitleRunes = 256;
export const maxBodyBytes = 60_000;
const requestTimeoutMs = 60_000;
const supportedProtocolVersions = ["2025-06-18", "2025-03-26", "2024-11-05"];

const toolDescription =
  "Open the draft pull request for this session's branch, or update it if it already exists. " +
  "Use it after you have committed and pushed your work to the session branch. " +
  "It is idempotent: calling it again updates the same pull request's title and body. " +
  "The title and body you send replace the existing ones, so they must describe every commit on the branch since the base " +
  "(run `git log <base>..HEAD`), including work from earlier turns, not only this turn's change. " +
  "The head and base branches are fixed by the platform, and the bex footer is appended automatically. " +
  "If it returns an error, report that error to the user; do not try other ways to create a pull request.";

const toolDefinition = {
  name: toolName,
  title: "Open or update the draft pull request",
  description: toolDescription,
  inputSchema: {
    type: "object",
    properties: {
      title: {
        type: "string",
        description: `Pull request title: one line, at most ${maxTitleRunes} characters.`,
      },
      body: {
        type: "string",
        description:
          "Pull request description in Markdown for the whole branch, filled in from the repository's pull request template when it has one.",
      },
    },
    required: ["title", "body"],
    additionalProperties: false,
  },
  annotations: { idempotentHint: true, openWorldHint: true },
};


interface PullRequestResult {
  url: string;
  number: number;
  created: boolean;
  headSha: string;
}

type ToolOutcome =
  { ok: true; result: PullRequestResult } | { ok: false; error: ToolError };

function invalid(message: string, hint?: string): ToolOutcome {
  return {
    ok: false,
    error: { code: "invalid_request", message, ...(hint ? { hint } : {}) },
  };
}

// validateInput mirrors the gateway's PullRequestContent.Validate (bex-api
// internal/agentsession) before any network call, so a malformed or oversized
// request fails fast with the same error vocabulary.
export function validateInput(args: unknown): ToolOutcome | undefined {
  if (!args || typeof args !== "object" || Array.isArray(args)) {
    return invalid("arguments must be an object with title and body");
  }
  const { title: rawTitle, body } = args as Record<string, unknown>;
  if (typeof rawTitle !== "string" || typeof body !== "string") {
    return invalid("title and body must be strings");
  }
  // A lone surrogate cannot be encoded as UTF-8; NUL is rejected by GitHub.
  if (/\p{Cs}/u.test(rawTitle) || /[\p{Cs}\0]/u.test(body)) {
    return invalid("title and body must be valid UTF-8 text");
  }
  const title = rawTitle.trim();
  if (!title) return invalid("title is required");
  if (/\p{Cc}/u.test(title)) return invalid("title must be a single line");
  if ([...title].length > maxTitleRunes) {
    return {
      ok: false,
      error: {
        code: "too_large",
        message: `title exceeds ${maxTitleRunes} characters`,
        hint: "Shorten the title.",
      },
    };
  }
  if (Buffer.byteLength(body, "utf8") > maxBodyBytes) {
    return {
      ok: false,
      error: {
        code: "too_large",
        message: `body exceeds ${maxBodyBytes} bytes`,
        hint: "Summarize the description; link to details instead of pasting them.",
      },
    };
  }
  return undefined;
}

function upstreamError(message: string): ToolOutcome {
  return { ok: false, error: { code: "upstream_error", message } };
}


function asResult(value: unknown): PullRequestResult | undefined {
  if (!value || typeof value !== "object") return undefined;
  const { url, number, created, headSha } = value as Record<string, unknown>;
  if (typeof url !== "string" || !url || typeof number !== "number") {
    return undefined;
  }
  return {
    url,
    number,
    created: created === true,
    headSha: typeof headSha === "string" ? headSha : "",
  };
}

async function recordResult(
  recordPath: string,
  result: PullRequestResult,
): Promise<void> {
  if (!recordPath) return;
  await writeJsonAtomic(recordPath, {
    ...result,
    recordedAt: new Date().toISOString(),
  });
}

export async function openOrUpdateDraftPr(
  args: unknown,
  env: NodeJS.ProcessEnv = process.env,
): Promise<ToolOutcome> {
  const rejected = validateInput(args);
  if (rejected) return rejected;
  const endpoint = env.BEX_PR_ENDPOINT || "";
  if (!endpoint) {
    return upstreamError(
      "this session has no pull-request endpoint configured",
    );
  }
  const { title, body } = args as { title: string; body: string };
  const reply = await postToGateway(
    endpoint,
    {
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ title: title.trim(), body }),
    },
    {
      label: "pull-request endpoint",
      fallbackCode: "upstream_error",
      timeoutMs: requestTimeoutMs,
    },
  );
  if (!reply.ok) return reply;
  const result = asResult(reply.payload);
  if (!result) {
    return upstreamError(
      "pull-request endpoint returned an unexpected response",
    );
  }
  try {
    await recordResult(env.BEX_PR_RECORD_PATH || "", result);
  } catch (error) {
    // The PR exists either way; the record only informs the driver's
    // post-turn delivery check, so a write failure must not hide the URL.
    process.stderr.write(
      `bex mcp: could not record pull request: ${describeError(error)}\n`,
    );
  }
  return { ok: true, result };
}

// availableTools lists a tool only when this session has its endpoint, so the
// agent is never offered a tool that can only fail.
export function availableTools(
  env: NodeJS.ProcessEnv = process.env,
): Array<{ name: string }> {
  const tools: Array<{ name: string }> = [];
  if (env.BEX_PR_ENDPOINT) tools.push(toolDefinition);
  if (env.BEX_PR_COMMENT_ENDPOINT) {
    tools.push(replyOnPullRequestToolDefinition);
  }
  if (env.BEX_SHARE_ENDPOINT) tools.push(shareFileToolDefinition);
  if (env.BEX_RESULT_ENDPOINT && env.BEX_RESULT_SCHEMA) {
    tools.push(submitResultToolDefinition);
  }
  if (env.BEX_SPAWN_ENDPOINT) {
    tools.push(
      spawnSessionToolDefinition,
      getChildSessionToolDefinition,
      waitChildSessionsToolDefinition,
      messageChildSessionToolDefinition,
      cancelChildSessionToolDefinition,
    );
  }
  return tools;
}

function serverInstructions(env: NodeJS.ProcessEnv): string {
  const lines: string[] = [];
  if (env.BEX_PR_ENDPOINT) {
    lines.push(`Use ${toolName} after pushing the session branch to open or update its draft pull request.`);
  }
  if (env.BEX_PR_COMMENT_ENDPOINT) {
    lines.push(`Use ${replyOnPullRequestToolName} to answer pull request feedback on the session's pull request.`);
  }
  if (env.BEX_SHARE_ENDPOINT) {
    lines.push(`Use ${shareFileToolName} to hand the user a file that should not be committed.`);
  }
  if (env.BEX_RESULT_ENDPOINT && env.BEX_RESULT_SCHEMA) {
    lines.push(`Use ${submitResultToolName} to hand back this task's structured result.`);
  }
  if (env.BEX_SPAWN_ENDPOINT) {
    lines.push(`Use ${spawnSessionToolName} to hand a separate task to a child session, ${waitChildSessionsToolName} to wait until children finish instead of polling ${getChildSessionToolName}, and ${messageChildSessionToolName} or ${cancelChildSessionToolName} to follow up on or stop one.`);
  }
  return lines.join(" ");
}

type ToolHandler = (
  args: unknown,
  env: NodeJS.ProcessEnv,
) => Promise<{ ok: true; result: unknown } | { ok: false; error: ToolError }>;

// toolHandlers maps every tool availableTools can list to its implementation.
const toolHandlers: Record<string, ToolHandler> = {
  [toolName]: openOrUpdateDraftPr,
  [shareFileToolName]: shareFile,
  [submitResultToolName]: submitResult,
  [spawnSessionToolName]: spawnSession,
  [getChildSessionToolName]: getChildSession,
  [waitChildSessionsToolName]: waitChildSessions,
  [messageChildSessionToolName]: messageChildSession,
  [cancelChildSessionToolName]: cancelChildSession,
  [replyOnPullRequestToolName]: replyOnPullRequest,
};

interface JsonRpcMessage {
  jsonrpc?: string;
  id?: string | number | null;
  method?: string;
  params?: Record<string, unknown>;
}

type Reply = Record<string, unknown> | undefined;

function rpcResult(id: JsonRpcMessage["id"], result: unknown): Reply {
  return { jsonrpc: "2.0", id, result };
}

function rpcError(
  id: JsonRpcMessage["id"],
  code: number,
  message: string,
): Reply {
  return { jsonrpc: "2.0", id: id ?? null, error: { code, message } };
}

export async function handleMessage(
  message: JsonRpcMessage,
  env: NodeJS.ProcessEnv = process.env,
): Promise<Reply> {
  const isRequest = message.id !== undefined && message.id !== null;
  switch (message.method) {
    case "initialize": {
      const requested = message.params?.protocolVersion;
      const protocolVersion =
        typeof requested === "string" &&
        supportedProtocolVersions.includes(requested)
          ? requested
          : supportedProtocolVersions[0];
      return rpcResult(message.id, {
        protocolVersion,
        capabilities: { tools: { listChanged: false } },
        serverInfo: { name: "bex", version: "1.0.0" },
        instructions: serverInstructions(env),
      });
    }
    case "ping":
      return rpcResult(message.id, {});
    case "tools/list":
      return rpcResult(message.id, { tools: availableTools(env) });
    case "tools/call": {
      const name = message.params?.name;
      if (!availableTools(env).some((tool) => tool.name === name)) {
        return rpcError(message.id, -32602, `unknown tool: ${String(name)}`);
      }
      const outcome = await toolHandlers[name as string](message.params?.arguments, env);
      const structured = outcome.ok ? outcome.result : { error: outcome.error };
      return rpcResult(message.id, {
        content: [{ type: "text", text: JSON.stringify(structured) }],
        structuredContent: structured,
        isError: !outcome.ok,
      });
    }
    default:
      // Notifications (notifications/initialized, cancellations) need no reply.
      if (!isRequest) return undefined;
      return rpcError(
        message.id,
        -32601,
        `method not found: ${message.method}`,
      );
  }
}

export function serve(
  input: NodeJS.ReadableStream = process.stdin,
  output: NodeJS.WritableStream = process.stdout,
  env: NodeJS.ProcessEnv = process.env,
): void {
  const lines = readline.createInterface({ input });
  lines.on("line", (line) => {
    if (!line.trim()) return;
    let message: JsonRpcMessage;
    try {
      message = JSON.parse(line) as JsonRpcMessage;
    } catch {
      output.write(
        `${JSON.stringify(rpcError(null, -32700, "parse error"))}\n`,
      );
      return;
    }
    void handleMessage(message, env).then(
      (reply) => {
        if (reply) output.write(`${JSON.stringify(reply)}\n`);
      },
      (error) => {
        output.write(
          `${JSON.stringify(rpcError(message.id, -32603, describeError(error)))}\n`,
        );
      },
    );
  });
}

// Serve only when executed as the MCP entrypoint, so tests can import the
// validation and handler functions without consuming stdin.
if (
  process.argv[1] &&
  path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  serve();
}
