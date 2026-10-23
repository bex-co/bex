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

// The `bex` stdio MCP server (ADR090). The driver registers it on
// session/new and session/load of repo sessions; the agent's adapter launches
// it. It exposes exactly one tool, open_or_update_draft_pr, which forwards to
// the gateway's pull-request verb on the session's Pod-bound Git proxy path.
// Owner, repository, head and base are fixed server-side by that path, so the
// agent supplies only a title and a body. No secret is involved: the gateway
// authenticates the source Pod.
//
// The transport is newline-delimited JSON-RPC 2.0 over stdio, hand-rolled to
// keep the image free of another runtime dependency.

import path from "node:path";
import readline from "node:readline";
import { fileURLToPath } from "node:url";
import { describeError } from "./errors.js";
import { writeJsonAtomic } from "./fs-util.js";

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

interface ToolError {
  code: string;
  message: string;
  hint?: string;
}

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

function asToolError(value: unknown): ToolError | undefined {
  if (!value || typeof value !== "object") return undefined;
  const error = (value as Record<string, unknown>).error;
  if (!error || typeof error !== "object") return undefined;
  const { code, message, hint } = error as Record<string, unknown>;
  if (typeof code !== "string" || typeof message !== "string") return undefined;
  return { code, message, ...(typeof hint === "string" ? { hint } : {}) };
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
  let response: Response;
  try {
    response = await fetch(endpoint, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ title: title.trim(), body }),
      signal: AbortSignal.timeout(requestTimeoutMs),
    });
  } catch (error) {
    return upstreamError(
      `pull-request endpoint unreachable: ${describeError(error)}`,
    );
  }
  const text = await response.text();
  let payload: unknown;
  try {
    payload = text ? JSON.parse(text) : undefined;
  } catch {
    payload = undefined;
  }
  if (!response.ok) {
    return {
      ok: false,
      error: asToolError(payload) ?? {
        code: "upstream_error",
        message: `pull-request endpoint returned HTTP ${response.status}`,
      },
    };
  }
  const result = asResult(payload);
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
        instructions: `Use ${toolName} after pushing the session branch to open or update its draft pull request.`,
      });
    }
    case "ping":
      return rpcResult(message.id, {});
    case "tools/list":
      return rpcResult(message.id, { tools: [toolDefinition] });
    case "tools/call": {
      const name = message.params?.name;
      if (name !== toolName) {
        return rpcError(message.id, -32602, `unknown tool: ${String(name)}`);
      }
      const outcome = await openOrUpdateDraftPr(message.params?.arguments, env);
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
