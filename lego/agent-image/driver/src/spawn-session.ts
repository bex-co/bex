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


// spawn_session and get_child_session (ADR102 D9): a running agent starts a
// child session and reads its own descendants. Both call the session's
// Pod-bound spawn URL on the gateway, which authenticates the source Pod: no
// bex credential, user token or API key is involved, and the parent is always
// this session. The child inherits this session's repository, agent, model,
// network tier and (optionally narrowed) secrets; its spend counts against
// this session's ceiling.

import { asToolError, postToGateway, type ToolError } from "./gateway.js";

export const spawnSessionToolName = "spawn_session";
export const getChildSessionToolName = "get_child_session";
// The gateway's MaxSpawnTaskBytes and MaxSpawnKeyBytes: one value on both
// sides of the hop.
export const maxTaskBytes = 100_000;
export const maxIdempotencyKeyBytes = 64;
export const maxTags = 20;
const requestTimeoutMs = 60_000;
// The ids bex mints for agent sessions: "ags-" and an xid.
const sessionIdPattern = /^ags-[0-9a-z]{20}$/;

export const spawnSessionToolDefinition = {
  name: spawnSessionToolName,
  title: "Start a child session",
  description:
    "Start one child agent session to work on a separate task in parallel, then keep working on your own. " +
    "The child works in this session's repository (on its own new branch unless you name one), with this session's agent and model, " +
    "and can use your secrets or a subset you name in `secretNames` (never more). " +
    "Its model spend counts against your own spend limit, and `maxSpendUSD` cannot exceed what you have left. " +
    "Sessions nest at most 3 levels deep and a session can have at most 5 children running at once; " +
    "when a limit is hit the error says which, and retrying the same call fails the same way. " +
    "Pass the same `idempotencyKey` when you retry a call so it never starts two children. " +
    "It returns the child's `sessionId` and `state`; read its outcome and result later with get_child_session. " +
    "Write a self-contained `task`: the child sees only what you put in it.",
  inputSchema: {
    type: "object",
    properties: {
      task: {
        type: "string",
        minLength: 1,
        description: "The child's complete task, written so it needs no other context.",
      },
      repo: {
        type: "string",
        description:
          "Optional. Must be this session's own repository (owner/name); omit it to use that repository.",
      },
      branch: {
        type: "string",
        description:
          "Optional branch for the child, starting with bex-agent/. Omit it for a new bex-agent/<id> branch.",
      },
      title: {
        type: "string",
        description: "Optional display title (default: the task's first line).",
      },
      tags: {
        type: "array",
        items: { type: "string" },
        maxItems: maxTags,
        description:
          "Optional labels for filtering and cost attribution, each [a-z0-9:_-]{1,64}.",
      },
      maxSpendUSD: {
        type: "number",
        exclusiveMinimum: 0,
        description:
          "Optional ceiling on the child's model spend in USD. It cannot exceed what is left of your own ceiling.",
      },
      secretNames: {
        type: "array",
        items: { type: "string" },
        description:
          "Optional subset of your secrets (by name) the child gets. Omit it to give the child all of them; [] gives it none.",
      },
      idempotencyKey: {
        type: "string",
        maxLength: maxIdempotencyKeyBytes,
        description:
          "Optional key (printable ASCII, at most 64 characters) that makes a retried call return the first child instead of starting another.",
      },
    },
    required: ["task"],
    additionalProperties: false,
  },
  annotations: { idempotentHint: false, openWorldHint: true },
};

export const getChildSessionToolDefinition = {
  name: getChildSessionToolName,
  title: "Read a child session",
  description:
    "Read the state of a session you started, directly or through one of your children: " +
    "`state` (working, succeeded, failed or canceled) and, once it has run, its `outcome` with the summary, " +
    "the structured `result` if its task asked for one, any pull request and its cost. " +
    "You can read only sessions you started; any other id is reported as not found. Poll sparingly: a child usually takes minutes.",
  inputSchema: {
    type: "object",
    properties: {
      sessionId: {
        type: "string",
        description: "The id spawn_session returned.",
      },
    },
    required: ["sessionId"],
    additionalProperties: false,
  },
  annotations: { readOnlyHint: true, idempotentHint: true, openWorldHint: true },
};

type Refusal = { ok: false; error: ToolError };

export type SpawnOutcome =
  | { ok: true; result: Record<string, unknown> }
  | Refusal;

function refuse(code: string, message: string, hint?: string): Refusal {
  return { ok: false, error: { code, message, ...(hint ? { hint } : {}) } };
}

const spawnFields = [
  "task",
  "repo",
  "branch",
  "title",
  "tags",
  "maxSpendUSD",
  "secretNames",
  "idempotencyKey",
];

// validateSpawnInput mirrors the gateway's SpawnContent.Validate and its
// strict body decoding before any network call. It returns the body to send.
export function validateSpawnInput(
  args: unknown,
): { ok: true; body: Record<string, unknown> } | Refusal {
  const bad = (message: string) => refuse("invalid_request", message);
  if (!args || typeof args !== "object" || Array.isArray(args)) {
    return bad("arguments must be an object with a task");
  }
  const input = args as Record<string, unknown>;
  const unknown = Object.keys(input).find((key) => !spawnFields.includes(key));
  if (unknown) return bad(`unknown argument: ${unknown}`);
  const { task, tags, maxSpendUSD, secretNames, idempotencyKey } = input;
  if (typeof task !== "string" || !task.trim()) return bad("task is required");
  if (Buffer.byteLength(task, "utf8") > maxTaskBytes) {
    return refuse(
      "too_large",
      `the task is larger than ${maxTaskBytes} bytes`,
      "Write a shorter task; put detail in the repository instead.",
    );
  }
  for (const field of ["repo", "branch", "title"]) {
    if (input[field] !== undefined && typeof input[field] !== "string") {
      return bad(`${field} must be a string`);
    }
  }
  const stringList = (value: unknown) =>
    Array.isArray(value) && value.every((item) => typeof item === "string");
  if (tags !== undefined && (!stringList(tags) || (tags as string[]).length > maxTags)) {
    return bad(`tags must be a list of at most ${maxTags} strings`);
  }
  if (secretNames !== undefined && !stringList(secretNames)) {
    return bad("secretNames must be a list of strings");
  }
  if (
    maxSpendUSD !== undefined &&
    (typeof maxSpendUSD !== "number" || !Number.isFinite(maxSpendUSD) || maxSpendUSD <= 0)
  ) {
    return bad("maxSpendUSD must be a number greater than 0");
  }
  if (idempotencyKey !== undefined) {
    if (
      typeof idempotencyKey !== "string" ||
      Buffer.byteLength(idempotencyKey, "utf8") > maxIdempotencyKeyBytes ||
      !/^[\x20-\x7e]*$/.test(idempotencyKey)
    ) {
      return bad(`idempotencyKey must be at most ${maxIdempotencyKeyBytes} printable ASCII characters`);
    }
  }
  const body: Record<string, unknown> = {};
  for (const field of spawnFields) {
    if (input[field] !== undefined) body[field] = input[field];
  }
  return { ok: true, body };
}

export async function spawnSession(
  args: unknown,
  env: NodeJS.ProcessEnv = process.env,
): Promise<SpawnOutcome> {
  const endpoint = env.BEX_SPAWN_ENDPOINT || "";
  if (!endpoint) {
    return refuse(
      "unavailable",
      "this session cannot start child sessions",
      "Do the work yourself.",
    );
  }
  const checked = validateSpawnInput(args);
  if (!checked.ok) return checked;
  const reply = await postToGateway(
    endpoint,
    {
      headers: { "content-type": "application/json" },
      body: JSON.stringify(checked.body),
    },
    { label: "spawn endpoint", fallbackCode: "unavailable", timeoutMs: requestTimeoutMs },
  );
  if (!reply.ok) return reply;
  const payload = reply.payload as Record<string, unknown> | undefined;
  if (!payload || typeof payload.sessionId !== "string" || typeof payload.state !== "string") {
    return refuse("unavailable", "spawn endpoint returned an unexpected response");
  }
  return {
    ok: true,
    result: {
      sessionId: payload.sessionId,
      state: payload.state,
      created: payload.created !== false,
    },
  };
}

export async function getChildSession(
  args: unknown,
  env: NodeJS.ProcessEnv = process.env,
): Promise<SpawnOutcome> {
  const endpoint = env.BEX_SPAWN_ENDPOINT || "";
  if (!endpoint) {
    return refuse("unavailable", "this session has no child sessions to read");
  }
  const sessionId = (args as Record<string, unknown> | null | undefined)?.sessionId;
  if (typeof sessionId !== "string" || !sessionIdPattern.test(sessionId)) {
    return refuse("invalid_request", "sessionId must be an id returned by spawn_session");
  }
  const reply = await postToGateway(
    `${endpoint.replace(/\/+$/, "")}/${sessionId}`,
    { headers: { accept: "application/json" } },
    {
      label: "spawn endpoint",
      fallbackCode: "unavailable",
      timeoutMs: requestTimeoutMs,
      method: "GET",
    },
  );
  if (!reply.ok) return reply;
  const payload = reply.payload;
  if (
    !payload ||
    typeof payload !== "object" ||
    asToolError(payload) ||
    typeof (payload as Record<string, unknown>).sessionId !== "string"
  ) {
    return refuse("unavailable", "spawn endpoint returned an unexpected response");
  }
  return { ok: true, result: payload as Record<string, unknown> };
}
