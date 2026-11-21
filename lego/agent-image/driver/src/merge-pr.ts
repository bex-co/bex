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

// mark_pull_request_ready and merge_pull_request (ADR090 D3 amendment
// w10/m538): when the task or a person's message asks, the agent undrafts its
// own pull request and merges it, as Devin does. Each posts to a gateway verb
// on the session's Pod-bound Git proxy path (pull-request/ready and
// pull-request/merge). The pull request, its repository and the head the merge
// is pinned to are fixed server-side from the session record; the merge says
// at most which method and what commit text. Branch protection decides, and
// GitHub's refusal comes back with its reason. No secret is involved.

import { readFile } from "node:fs/promises";
import { describeError } from "./errors.js";
import { writeJsonAtomic } from "./fs-util.js";
import { postToGateway, type ToolError } from "./gateway.js";

export const markPullRequestReadyToolName = "mark_pull_request_ready";
export const mergePullRequestToolName = "merge_pull_request";
// The gateway's MaxMergeCommitTitleRunes and MaxMergeCommitMessageBytes.
export const maxCommitTitleRunes = 256;
export const maxCommitMessageBytes = 60_000;
export const mergeMethods = ["merge", "squash", "rebase"] as const;
const requestTimeoutMs = 60_000;

const onlyWhenAsked =
  "Use it only when the task or a person's message asks for it; otherwise leave the pull request as a draft for the requester to review.";

export const markPullRequestReadyToolDefinition = {
  name: markPullRequestReadyToolName,
  title: "Mark the pull request ready for review",
  description:
    "Mark this session's own pull request ready for review (undraft it). " +
    onlyWhenAsked +
    " The pull request is fixed by the platform. If it returns an error, report it to the user; do not use `gh pr ready` or another route.",
  inputSchema: {
    type: "object",
    properties: {},
    additionalProperties: false,
  },
  annotations: { idempotentHint: true, openWorldHint: true },
};

export const mergePullRequestToolDefinition = {
  name: mergePullRequestToolName,
  title: "Merge the pull request",
  description:
    "Merge this session's own pull request into its base branch. " +
    onlyWhenAsked +
    " Mark it ready first, and merge once its required checks pass (`gh pr checks --watch`). " +
    "The merge is pinned to the head bex recorded when you last called open_or_update_draft_pr, so push and call that tool first. " +
    "The repository's branch protection decides: if GitHub refuses (checks pending or failing, a required review, a conflict), the error gives GitHub's reason and what to do; " +
    "report it and do not call this tool in a loop, and never use `gh pr merge`.",
  inputSchema: {
    type: "object",
    properties: {
      method: {
        type: "string",
        enum: [...mergeMethods],
        description:
          "Merge method. Omit it to use the first method the repository allows (merge, then squash, then rebase).",
      },
      commitTitle: {
        type: "string",
        description: `Optional title of the merge or squash commit: one line, at most ${maxCommitTitleRunes} characters. GitHub's default is used when omitted.`,
      },
      commitMessage: {
        type: "string",
        description: `Optional message of the merge or squash commit, at most ${maxCommitMessageBytes} bytes.`,
      },
    },
    additionalProperties: false,
  },
  annotations: { openWorldHint: true },
};

export interface ReadyPullRequest {
  url: string;
  number: number;
  alreadyReady?: true;
}

export interface MergedPullRequest {
  url: string;
  number: number;
  merged: true;
  sha: string;
  method: string;
  // The head bex merged: the one open_or_update_draft_pr last recorded.
  headSha: string;
}

type Outcome<T> = { ok: true; result: T } | { ok: false; error: ToolError };

function invalid(message: string): { ok: false; error: ToolError } {
  return { ok: false, error: { code: "invalid_request", message } };
}

function upstreamError(message: string): { ok: false; error: ToolError } {
  return { ok: false, error: { code: "upstream_error", message } };
}

// validateReadyInput accepts no arguments, or an empty object.
export function validateReadyInput(
  args: unknown,
): { ok: false; error: ToolError } | undefined {
  if (args === undefined || args === null) return undefined;
  if (typeof args !== "object" || Array.isArray(args)) {
    return invalid("mark_pull_request_ready takes no arguments");
  }
  if (Object.keys(args).length > 0) {
    return invalid(
      "mark_pull_request_ready takes no arguments; the pull request is fixed by the platform",
    );
  }
  return undefined;
}

export interface MergeInput {
  method?: (typeof mergeMethods)[number];
  commitTitle?: string;
  commitMessage?: string;
}

// validateMergeInput mirrors the gateway's PullRequestMergeContent.Validate
// (bex-api internal/agentsession) before any network call, and returns the
// request body to send.
export function validateMergeInput(
  args: unknown,
): { ok: false; error: ToolError } | { ok: true; input: MergeInput } {
  if (args === undefined || args === null) return { ok: true, input: {} };
  if (typeof args !== "object" || Array.isArray(args)) {
    return invalid("arguments must be an object");
  }
  const { method, commitTitle, commitMessage, ...rest } = args as Record<
    string,
    unknown
  >;
  const unknown = Object.keys(rest);
  if (unknown.length > 0) {
    return invalid(
      `unknown argument ${unknown[0]}: merge_pull_request takes only method, commitTitle and commitMessage; the pull request is fixed by the platform`,
    );
  }
  const input: MergeInput = {};
  if (method !== undefined) {
    if (
      typeof method !== "string" ||
      !(mergeMethods as readonly string[]).includes(method)
    ) {
      return invalid("method must be merge, squash or rebase");
    }
    input.method = method as MergeInput["method"];
  }
  if (commitTitle !== undefined) {
    if (typeof commitTitle !== "string" || /\p{Cs}/u.test(commitTitle)) {
      return invalid("commitTitle must be valid UTF-8 text");
    }
    const title = commitTitle.trim();
    if (/\p{Cc}/u.test(title))
      return invalid("commitTitle must be a single line");
    if ([...title].length > maxCommitTitleRunes) {
      return {
        ok: false,
        error: {
          code: "too_large",
          message: `commitTitle exceeds ${maxCommitTitleRunes} characters`,
          hint: "Shorten the commit title.",
        },
      };
    }
    if (title) input.commitTitle = title;
  }
  if (commitMessage !== undefined) {
    if (
      typeof commitMessage !== "string" ||
      /[\p{Cs}\0]/u.test(commitMessage)
    ) {
      return invalid("commitMessage must be valid UTF-8 text");
    }
    if (Buffer.byteLength(commitMessage, "utf8") > maxCommitMessageBytes) {
      return {
        ok: false,
        error: {
          code: "too_large",
          message: `commitMessage exceeds ${maxCommitMessageBytes} bytes`,
          hint: "Shorten the commit message.",
        },
      };
    }
    if (commitMessage) input.commitMessage = commitMessage;
  }
  return { ok: true, input };
}

async function post(
  endpoint: string,
  label: string,
  body: object,
): Promise<
  | { ok: true; payload: Record<string, unknown> }
  | { ok: false; error: ToolError }
> {
  if (!endpoint) {
    return upstreamError(`this session has no ${label} configured`);
  }
  const reply = await postToGateway(
    endpoint,
    {
      headers: { "content-type": "application/json" },
      body: JSON.stringify(body),
    },
    { label, fallbackCode: "upstream_error", timeoutMs: requestTimeoutMs },
  );
  if (!reply.ok) return reply;
  const payload = reply.payload;
  if (!payload || typeof payload !== "object") {
    return upstreamError(`${label} returned an unexpected response`);
  }
  return { ok: true, payload: payload as Record<string, unknown> };
}

export async function markPullRequestReady(
  args: unknown,
  env: NodeJS.ProcessEnv = process.env,
): Promise<Outcome<ReadyPullRequest>> {
  const rejected = validateReadyInput(args);
  if (rejected) return rejected;
  const label = "pull-request ready endpoint";
  const reply = await post(env.BEX_PR_READY_ENDPOINT || "", label, {});
  if (!reply.ok) return reply;
  const { url, number, alreadyReady } = reply.payload;
  if (typeof url !== "string" || !url || typeof number !== "number") {
    return upstreamError(`${label} returned an unexpected response`);
  }
  return {
    ok: true,
    result: {
      url,
      number,
      ...(alreadyReady === true ? { alreadyReady: true as const } : {}),
    },
  };
}

export async function mergePullRequest(
  args: unknown,
  env: NodeJS.ProcessEnv = process.env,
): Promise<Outcome<MergedPullRequest>> {
  const checked = validateMergeInput(args);
  if (!checked.ok) return checked;
  const label = "pull-request merge endpoint";
  const reply = await post(
    env.BEX_PR_MERGE_ENDPOINT || "",
    label,
    checked.input,
  );
  if (!reply.ok) return reply;
  const { url, number, merged, sha, method, headSha } = reply.payload;
  if (
    typeof url !== "string" ||
    !url ||
    typeof number !== "number" ||
    merged !== true ||
    typeof sha !== "string" ||
    !sha ||
    typeof method !== "string"
  ) {
    return upstreamError(`${label} returned an unexpected response`);
  }
  const result: MergedPullRequest = {
    url,
    number,
    merged: true,
    sha,
    method,
    headSha: typeof headSha === "string" ? headSha : "",
  };
  try {
    await recordMerge(env.BEX_PR_RECORD_PATH || "", result);
  } catch (error) {
    // The merge stands either way; the record only tells the driver's
    // post-turn delivery check what was merged.
    process.stderr.write(
      `bex mcp: could not record the merge: ${describeError(error)}\n`,
    );
  }
  return { ok: true, result };
}

// recordMerge marks the pull request record (the one open_or_update_draft_pr
// writes) merged at headSha. GitHub may delete a merged branch, and the
// delivery check then counts local commits from the merged head rather than
// asking the agent to push work that is already merged.
async function recordMerge(
  recordPath: string,
  result: MergedPullRequest,
): Promise<void> {
  if (!recordPath) return;
  let record: Record<string, unknown> = {};
  try {
    record = JSON.parse(await readFile(recordPath, "utf8")) as Record<
      string,
      unknown
    >;
  } catch {
    // No pull request recorded this turn: the merge record stands alone.
  }
  await writeJsonAtomic(recordPath, {
    ...record,
    url: result.url,
    number: result.number,
    ...(result.headSha ? { headSha: result.headSha } : {}),
    merged: true,
    mergeSha: result.sha,
    recordedAt: new Date().toISOString(),
  });
}
