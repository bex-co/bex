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
// child session and reads its own descendants. wait_child_sessions,
// message_child_session and cancel_child_session (w9/m346) let it coordinate
// them: block until they finish, start a follow-up turn on one, or stop one.
// All call the session's Pod-bound spawn URL on the gateway, which
// authenticates the source Pod: no bex credential, user token or API key is
// involved, the parent is always this session, and only its own descendants
// can be named. The child inherits this session's repository, agent, model,
// network tier and (optionally narrowed) secrets; its spend counts against
// this session's ceiling.

import {
  asToolError,
  postToGateway,
  type GatewayReply,
  type ToolError,
} from "./gateway.js";

export const spawnSessionToolName = "spawn_session";
export const getChildSessionToolName = "get_child_session";
export const waitChildSessionsToolName = "wait_child_sessions";
export const messageChildSessionToolName = "message_child_session";
export const cancelChildSessionToolName = "cancel_child_session";
// The gateway's MaxSpawnTaskBytes and MaxSpawnKeyBytes: one value on both
// sides of the hop. A follow-up prompt has the task's bound.
export const maxTaskBytes = 100_000;
export const maxIdempotencyKeyBytes = 64;
export const maxTags = 20;
// The gateway's MaxSpawnWaitChildren and MaxSpawnWaitSeconds; a wait without
// a timeout lasts the longest, 25 seconds. That stays under bex-api's 30-second
// write timeout, so the agent calls again to keep waiting.
export const maxWaitChildren = 20;
export const maxWaitSeconds = 25;
const requestTimeoutMs = 60_000;
// The ids bex mints for agent sessions: "ags-" and an xid (base32hex).
const sessionIdPattern = /^ags-[0-9a-v]{20}$/;

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
        description:
          "The child's complete task, written so it needs no other context.",
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
  annotations: {
    readOnlyHint: true,
    idempotentHint: true,
    openWorldHint: true,
  },
};

const childIdProperty = {
  type: "string",
  description: "The child's id, as spawn_session returned it.",
};

export const waitChildSessionsToolDefinition = {
  name: waitChildSessionsToolName,
  title: "Wait for child sessions",
  description:
    "Block until the named child sessions finish or the timeout passes; call again to keep waiting. " +
    "Use instead of polling get_child_session. " +
    "It returns as soon as every named child has stopped working (succeeded, failed or canceled), " +
    `or after \`timeoutSeconds\` (at most and by default ${maxWaitSeconds}) with \`timedOut: true\`. ` +
    "`children` holds each child's current `state` and `settled` lists the ids that are no longer working; " +
    "read a settled child's outcome and result with get_child_session. " +
    "You can wait only on sessions you started, directly or through one of your children; any other id is reported as not found.",
  inputSchema: {
    type: "object",
    properties: {
      childIds: {
        type: "array",
        items: { type: "string" },
        minItems: 1,
        maxItems: maxWaitChildren,
        description: `The ids spawn_session returned, 1 to ${maxWaitChildren} of them.`,
      },
      timeoutSeconds: {
        type: "integer",
        minimum: 1,
        maximum: maxWaitSeconds,
        description: `Optional. How long to wait at most, in seconds, 1 to ${maxWaitSeconds} (default ${maxWaitSeconds}).`,
      },
    },
    required: ["childIds"],
    additionalProperties: false,
  },
  annotations: {
    readOnlyHint: true,
    idempotentHint: true,
    openWorldHint: true,
  },
};

export const messageChildSessionToolDefinition = {
  name: messageChildSessionToolName,
  title: "Message a child session",
  description:
    "Send a follow-up message to a session you started: it starts a new turn on that child with your text as the prompt, " +
    "and the child continues where it left off, on its own branch. " +
    "The child must not be working: wait for it with wait_child_sessions first " +
    "(a working child refuses with AGENT_SESSION_TURN_IN_FLIGHT). " +
    "It returns the child's `sessionId` and new `state`; wait for it with wait_child_sessions, then read the turn's outcome with get_child_session.",
  inputSchema: {
    type: "object",
    properties: {
      childId: childIdProperty,
      prompt: {
        type: "string",
        minLength: 1,
        description: `The follow-up instructions, at most ${maxTaskBytes} bytes.`,
      },
    },
    required: ["childId", "prompt"],
    additionalProperties: false,
  },
  annotations: { idempotentHint: false, openWorldHint: true },
};

export const cancelChildSessionToolDefinition = {
  name: cancelChildSessionToolName,
  title: "Cancel a child session",
  description:
    "Cancel a session you started when its work is no longer needed or it has gone off track: " +
    "it stops working, and any sessions it started are canceled too. " +
    "Work it already pushed stays on its branch. It returns the child's `sessionId` and `state`.",
  inputSchema: {
    type: "object",
    properties: { childId: childIdProperty },
    required: ["childId"],
    additionalProperties: false,
  },
  annotations: {
    destructiveHint: true,
    idempotentHint: true,
    openWorldHint: true,
  },
};

type Refusal = { ok: false; error: ToolError };

export type SpawnOutcome =
  | { ok: true; result: Record<string, unknown> }
  | Refusal;

function refuse(code: string, message: string, hint?: string): Refusal {
  return { ok: false, error: { code, message, ...(hint ? { hint } : {}) } };
}

const bad = (message: string) => refuse("invalid_request", message);

// toolArgs mirrors the gateway's strict body decoding: an object carrying only
// the tool's own fields.
function toolArgs(
  args: unknown,
  definition: { inputSchema: { properties: object } },
  shape: string,
): { ok: true; input: Record<string, unknown> } | Refusal {
  if (!args || typeof args !== "object" || Array.isArray(args)) {
    return bad(`arguments must be an object with ${shape}`);
  }
  const input = args as Record<string, unknown>;
  const fields = Object.keys(definition.inputSchema.properties);
  const unknown = Object.keys(input).find((key) => !fields.includes(key));
  if (unknown) return bad(`unknown argument: ${unknown}`);
  return { ok: true, input };
}

const isSessionId = (value: unknown): value is string =>
  typeof value === "string" && sessionIdPattern.test(value);

const spawnFields = Object.keys(
  spawnSessionToolDefinition.inputSchema.properties,
);

// validateSpawnInput mirrors the gateway's SpawnContent.Validate and its
// strict body decoding before any network call. It returns the body to send.
export function validateSpawnInput(
  args: unknown,
): { ok: true; body: Record<string, unknown> } | Refusal {
  const checked = toolArgs(args, spawnSessionToolDefinition, "a task");
  if (!checked.ok) return checked;
  const input = checked.input;
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
  if (
    tags !== undefined &&
    (!stringList(tags) || (tags as string[]).length > maxTags)
  ) {
    return bad(`tags must be a list of at most ${maxTags} strings`);
  }
  if (secretNames !== undefined && !stringList(secretNames)) {
    return bad("secretNames must be a list of strings");
  }
  if (
    maxSpendUSD !== undefined &&
    (typeof maxSpendUSD !== "number" ||
      !Number.isFinite(maxSpendUSD) ||
      maxSpendUSD <= 0)
  ) {
    return bad("maxSpendUSD must be a number greater than 0");
  }
  if (idempotencyKey !== undefined) {
    if (
      typeof idempotencyKey !== "string" ||
      Buffer.byteLength(idempotencyKey, "utf8") > maxIdempotencyKeyBytes ||
      !/^[\x20-\x7e]*$/.test(idempotencyKey)
    ) {
      return bad(
        `idempotencyKey must be at most ${maxIdempotencyKeyBytes} printable ASCII characters`,
      );
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
    {
      label: "spawn endpoint",
      fallbackCode: "unavailable",
      timeoutMs: requestTimeoutMs,
    },
  );
  if (!reply.ok) return reply;
  const payload = reply.payload as Record<string, unknown> | undefined;
  if (
    !payload ||
    typeof payload.sessionId !== "string" ||
    typeof payload.state !== "string"
  ) {
    return refuse(
      "unavailable",
      "spawn endpoint returned an unexpected response",
    );
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

// callSpawnRoute sends one request to the spawn URL or one of its child
// routes (segment: a child id or "wait").
function callSpawnRoute(
  endpoint: string,
  segment: string,
  method: "GET" | "POST" | "DELETE",
  body?: Record<string, unknown>,
): Promise<GatewayReply> {
  return postToGateway(
    `${endpoint.replace(/\/+$/, "")}/${segment}`,
    body === undefined
      ? { headers: { accept: "application/json" } }
      : {
          headers: { "content-type": "application/json" },
          body: JSON.stringify(body),
        },
    {
      label: "spawn endpoint",
      fallbackCode: "unavailable",
      timeoutMs: requestTimeoutMs,
      method,
    },
  );
}

// asChildView accepts a gateway ChildView or wait child and nothing else: a refusal
// dressed as a 200 is never passed to the agent as a child.
function asChildView(payload: unknown): Record<string, unknown> | undefined {
  if (
    !payload ||
    typeof payload !== "object" ||
    Array.isArray(payload) ||
    asToolError(payload) ||
    typeof (payload as Record<string, unknown>).sessionId !== "string"
  ) {
    return undefined;
  }
  return payload as Record<string, unknown>;
}

function childOutcome(reply: GatewayReply): SpawnOutcome {
  if (!reply.ok) return reply;
  const view = asChildView(reply.payload);
  if (!view)
    return refuse(
      "unavailable",
      "spawn endpoint returned an unexpected response",
    );
  return { ok: true, result: view };
}

const noChildren = () =>
  refuse("unavailable", "this session has no child sessions");
const badChildId = () => bad("childId must be an id returned by spawn_session");

export async function getChildSession(
  args: unknown,
  env: NodeJS.ProcessEnv = process.env,
): Promise<SpawnOutcome> {
  const endpoint = env.BEX_SPAWN_ENDPOINT || "";
  if (!endpoint) {
    return refuse("unavailable", "this session has no child sessions to read");
  }
  const sessionId = (args as Record<string, unknown> | null | undefined)
    ?.sessionId;
  if (!isSessionId(sessionId)) {
    return refuse(
      "invalid_request",
      "sessionId must be an id returned by spawn_session",
    );
  }
  return childOutcome(await callSpawnRoute(endpoint, sessionId, "GET"));
}

// validateWaitInput mirrors the gateway's SpawnWait.Validate. It returns the
// body to send; an omitted timeout is left to the gateway's default.
function validateWaitInput(
  args: unknown,
):
  | { ok: true; body: { childIds: string[]; timeoutSeconds?: number } }
  | Refusal {
  const checked = toolArgs(args, waitChildSessionsToolDefinition, "childIds");
  if (!checked.ok) return checked;
  const { childIds, timeoutSeconds } = checked.input;
  if (
    !Array.isArray(childIds) ||
    childIds.length < 1 ||
    childIds.length > maxWaitChildren ||
    !childIds.every(isSessionId)
  ) {
    return bad(
      `childIds must list 1 to ${maxWaitChildren} ids returned by spawn_session`,
    );
  }
  if (timeoutSeconds === undefined) return { ok: true, body: { childIds } };
  if (
    typeof timeoutSeconds !== "number" ||
    !Number.isInteger(timeoutSeconds) ||
    timeoutSeconds < 1 ||
    timeoutSeconds > maxWaitSeconds
  ) {
    return bad(
      `timeoutSeconds must be a whole number from 1 to ${maxWaitSeconds}; call again to keep waiting`,
    );
  }
  return { ok: true, body: { childIds, timeoutSeconds } };
}

export async function waitChildSessions(
  args: unknown,
  env: NodeJS.ProcessEnv = process.env,
): Promise<SpawnOutcome> {
  const endpoint = env.BEX_SPAWN_ENDPOINT || "";
  if (!endpoint) return noChildren();
  const checked = validateWaitInput(args);
  if (!checked.ok) return checked;
  // The longest wait (maxWaitSeconds) ends before the hop's request timeout,
  // so the gateway's own deadline always answers first.
  const reply = await callSpawnRoute(endpoint, "wait", "POST", checked.body);
  if (!reply.ok) return reply;
  const payload = (reply.payload ?? {}) as Record<string, unknown>;
  // Go encodes an empty list as null.
  const children = payload.children ?? [];
  const settled = payload.settled ?? [];
  if (
    typeof payload !== "object" ||
    asToolError(payload) ||
    typeof payload.timedOut !== "boolean" ||
    !Array.isArray(children) ||
    !children.every((child) => asChildView(child)) ||
    !Array.isArray(settled) ||
    !settled.every((item) => typeof item === "string")
  ) {
    return refuse(
      "unavailable",
      "spawn endpoint returned an unexpected response",
    );
  }
  return {
    ok: true,
    result: { children, settled, timedOut: payload.timedOut },
  };
}

// validateMessageInput mirrors the gateway's SpawnMessage.Validate.
function validateMessageInput(
  args: unknown,
): { ok: true; childId: string; prompt: string } | Refusal {
  const checked = toolArgs(
    args,
    messageChildSessionToolDefinition,
    "childId and prompt",
  );
  if (!checked.ok) return checked;
  const { childId, prompt } = checked.input;
  if (!isSessionId(childId)) return badChildId();
  if (typeof prompt !== "string" || !prompt.trim())
    return bad("prompt is required");
  if (Buffer.byteLength(prompt, "utf8") > maxTaskBytes) {
    return refuse(
      "too_large",
      `the prompt is larger than ${maxTaskBytes} bytes`,
      "Send a shorter message; put detail in the repository instead.",
    );
  }
  return { ok: true, childId, prompt };
}

export async function messageChildSession(
  args: unknown,
  env: NodeJS.ProcessEnv = process.env,
): Promise<SpawnOutcome> {
  const endpoint = env.BEX_SPAWN_ENDPOINT || "";
  if (!endpoint) return noChildren();
  const checked = validateMessageInput(args);
  if (!checked.ok) return checked;
  return childOutcome(
    await callSpawnRoute(endpoint, checked.childId, "POST", {
      prompt: checked.prompt,
    }),
  );
}

export async function cancelChildSession(
  args: unknown,
  env: NodeJS.ProcessEnv = process.env,
): Promise<SpawnOutcome> {
  const endpoint = env.BEX_SPAWN_ENDPOINT || "";
  if (!endpoint) return noChildren();
  const checked = toolArgs(args, cancelChildSessionToolDefinition, "a childId");
  if (!checked.ok) return checked;
  const { childId } = checked.input;
  if (!isSessionId(childId)) return badChildId();
  return childOutcome(await callSpawnRoute(endpoint, childId, "DELETE"));
}
