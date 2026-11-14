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

// reply_on_pull_request (ADR047 D11): the agent answers the pull request
// feedback that steered it with one comment on the session's own pull request.
// It posts {body} to the gateway's pull-request/comment verb on the session's
// Pod-bound Git proxy path; the repository and PR number are fixed server-side
// from the session record, and bex appends a footer naming the agent. No
// secret is involved.

import { postToGateway, type ToolError } from "./gateway.js";

export const replyOnPullRequestToolName = "reply_on_pull_request";
// The gateway's MaxPullRequestCommentBytes: one value on both sides of the hop.
export const maxCommentBytes = 60_000;
const requestTimeoutMs = 60_000;

export const replyOnPullRequestToolDefinition = {
  name: replyOnPullRequestToolName,
  title: "Reply on the pull request",
  description:
    "Post a comment on this session's own pull request, for example to answer the review comments or the failing checks " +
    "that started this turn by saying what you changed. " +
    "Push your commits and update the pull request first, so the reply describes work that is on the branch. " +
    "The repository and pull request are fixed by the platform, and the bex footer is appended automatically. " +
    "If it returns an error, report that error to the user; do not try other ways to comment on the pull request.",
  inputSchema: {
    type: "object",
    properties: {
      body: {
        type: "string",
        description: `Comment text in Markdown, at most ${maxCommentBytes} bytes.`,
      },
    },
    required: ["body"],
    additionalProperties: false,
  },
  annotations: { openWorldHint: true },
};

export interface PostedComment {
  url: string;
  number: number;
}

export type ReplyOutcome =
  | { ok: true; result: PostedComment }
  | { ok: false; error: ToolError };

function invalid(message: string): ReplyOutcome {
  return { ok: false, error: { code: "invalid_request", message } };
}

// validateReplyInput mirrors the gateway's PullRequestCommentContent.Validate
// (bex-api internal/agentsession) before any network call.
export function validateReplyInput(args: unknown): ReplyOutcome | undefined {
  if (!args || typeof args !== "object" || Array.isArray(args)) {
    return invalid("arguments must be an object with body");
  }
  const { body } = args as Record<string, unknown>;
  if (typeof body !== "string") return invalid("body must be a string");
  // A lone surrogate cannot be encoded as UTF-8; NUL is rejected by GitHub.
  if (/[\p{Cs}\0]/u.test(body)) {
    return invalid("body must be valid UTF-8 text");
  }
  if (!body.trim()) return invalid("body is required");
  if (Buffer.byteLength(body, "utf8") > maxCommentBytes) {
    return {
      ok: false,
      error: {
        code: "too_large",
        message: `body exceeds ${maxCommentBytes} bytes`,
        hint: "Shorten the comment; link to details instead of pasting them.",
      },
    };
  }
  return undefined;
}

export async function replyOnPullRequest(
  args: unknown,
  env: NodeJS.ProcessEnv = process.env,
): Promise<ReplyOutcome> {
  const rejected = validateReplyInput(args);
  if (rejected) return rejected;
  const endpoint = env.BEX_PR_COMMENT_ENDPOINT || "";
  if (!endpoint) {
    return {
      ok: false,
      error: {
        code: "upstream_error",
        message: "this session has no pull-request comment endpoint configured",
      },
    };
  }
  const { body } = args as { body: string };
  const reply = await postToGateway(
    endpoint,
    {
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ body }),
    },
    {
      label: "pull-request comment endpoint",
      fallbackCode: "upstream_error",
      timeoutMs: requestTimeoutMs,
    },
  );
  if (!reply.ok) return reply;
  const { url, number } = (reply.payload ?? {}) as Record<string, unknown>;
  if (typeof url !== "string" || !url || typeof number !== "number") {
    return {
      ok: false,
      error: {
        code: "upstream_error",
        message:
          "pull-request comment endpoint returned an unexpected response",
      },
    };
  }
  return { ok: true, result: { url, number } };
}
