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

// mark_pull_request_ready and merge_pull_request (ADR090 D3 amendments
// w10/m538, w10/m577 and w10/041): when the task or a person's message asks,
// the agent undrafts a pull request and merges it, as Devin does, now or, with
// auto, once its required checks pass (GitHub auto-merge, or the merge queue
// its base branch requires). Each posts to a gateway verb on the session's
// Pod-bound Git proxy path (pull-request/ready and pull-request/merge). The
// repository is fixed server-side from the session record. The pull request
// is the session's own, merged at the head bex recorded for it, unless the
// agent names another of the repository's by number, which merges at the
// head GitHub reports for it then. The merge says at most which pull request,
// which method, what commit text and whether to wait for the checks, or that
// GitHub should not merge it later after all. Branch protection decides, and
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
  "Use it only when the task or a person's message asks for it.";

// numberProperty is both tools' optional pull request number (w10/041).
const numberProperty = {
  type: "integer",
  minimum: 1,
  description:
    "The number of another pull request of this session's repository to act on, one the task or a person names (`gh pr list` lists them). Omit it for this session's own pull request.",
};

export const markPullRequestReadyToolDefinition = {
  name: markPullRequestReadyToolName,
  title: "Mark the pull request ready for review",
  description:
    "Mark this session's own draft pull request ready for review (undraft it), or another pull request of this repository when you pass its number; one already ready answers alreadyReady: true. " +
    onlyWhenAsked +
    " The repository is fixed by the platform. If it returns an error, report it to the user; do not use `gh pr ready` or another route.",
  inputSchema: {
    type: "object",
    properties: { number: numberProperty },
    additionalProperties: false,
  },
  annotations: { idempotentHint: true, openWorldHint: true },
};

export const mergePullRequestToolDefinition = {
  name: mergePullRequestToolName,
  title: "Merge the pull request",
  description:
    "Merge this session's own pull request into its base branch, now or once its required checks pass, or another pull request of this repository when you pass its number. " +
    onlyWhenAsked +
    " Mark a draft ready first. When asked to merge once the checks pass or CI is green, call it with auto: true as soon as the pull request is ready and end your turn, without waiting for or watching the checks: GitHub auto-merge merges it when they pass (the answer has autoMerge: true, and next says what to tell the user), " +
    "and a base branch that requires a merge queue takes it into the queue, with or without auto (mergeQueue: true). Without auto it merges now; a pull request GitHub would merge now is merged even with auto. " +
    "If a person asks not to merge after all, call it with only disableAuto: true. " +
    "The merge of this session's own pull request is pinned to the head bex recorded when you last called open_or_update_draft_pr, so push and call that tool first; another pull request merges at the head GitHub reports for it when you call (the answer has otherPullRequest: true). " +
    "The repository's branch protection decides: if GitHub refuses (checks pending or failing, a required review, a conflict), the error gives GitHub's reason and what to do; " +
    "report it and do not call this tool in a loop, and never use `gh pr merge`.",
  inputSchema: {
    type: "object",
    properties: {
      number: numberProperty,
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
      auto: {
        type: "boolean",
        description:
          "true: have GitHub merge the pull request once its required checks pass (GitHub auto-merge, or the merge queue), so you can end your turn. On a merge-queue branch the queue sets the method and commit text.",
      },
      disableAuto: {
        type: "boolean",
        description:
          "true, with no other argument but number: GitHub no longer merges the pull request later (its auto-merge is disabled, or it leaves the merge queue).",
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
  // A pull request of the repository other than the session's own (w10/041).
  otherPullRequest?: true;
}

// MergeAnswer is what the merge did, with exactly one outcome: merged now
// (sha, the merge commit, and method), autoMerge armed with method, joined the
// mergeQueue, or autoMergeDisabled. Only a merge is merged: GitHub merges an
// armed or queued pull request later.
export interface MergeAnswer {
  url: string;
  number: number;
  merged: boolean;
  sha?: string;
  method?: string;
  // The head bex merged or armed: the one open_or_update_draft_pr last
  // recorded, or for another pull request the one GitHub reported.
  headSha: string;
  autoMerge?: true;
  mergeQueue?: true;
  autoMergeDisabled?: true;
  // A pull request of the repository other than the session's own (w10/041),
  // which the session's pull request record never takes.
  otherPullRequest?: true;
  // What to do now that GitHub merges it later: end the turn (w10/m577 t010).
  next?: string;
}

// endTurnAfter is an armed or queued answer's next step. The overlay and the
// tool description already say to end the turn, yet on production the default
// model armed auto-merge and then watched the checks anyway, "to report
// accurately" (w10/m577 t005, 2026-11-23). The answer is read at that very
// decision, so it says what the final reply reports in place of a merge.
const endTurnAfter = {
  autoMerge:
    "Auto-merge is armed: GitHub merges this pull request once its required checks pass. End your turn now, without waiting for or watching the checks. Your final reply says auto-merge is armed, not that the pull request merged.",
  mergeQueue:
    "The pull request joined the merge queue: GitHub merges it once the queue's checks pass. End your turn now, without waiting for or watching the checks. Your final reply says it is in the merge queue, not that it merged.",
} as const;

// The outcomes an answer may carry besides a merge.
const laterOutcomes = ["autoMerge", "mergeQueue", "autoMergeDisabled"] as const;

type Outcome<T> = { ok: true; result: T } | { ok: false; error: ToolError };

function invalid(message: string): { ok: false; error: ToolError } {
  return { ok: false, error: { code: "invalid_request", message } };
}

function upstreamError(message: string): { ok: false; error: ToolError } {
  return { ok: false, error: { code: "upstream_error", message } };
}

// validNumber reads an optional pull request number, a positive integer, as
// the request body's part: {number}, or {} when it is absent.
function validNumber(
  number: unknown,
): { ok: false; error: ToolError } | { ok: true; input: { number?: number } } {
  if (number === undefined) return { ok: true, input: {} };
  if (
    typeof number !== "number" ||
    !Number.isSafeInteger(number) ||
    number < 1
  ) {
    return invalid("number must be a positive pull request number");
  }
  return { ok: true, input: { number } };
}

// validateReadyInput accepts no arguments, or at most a pull request number,
// and returns the request body to send.
export function validateReadyInput(
  args: unknown,
): { ok: false; error: ToolError } | { ok: true; input: { number?: number } } {
  if (args === undefined || args === null) return { ok: true, input: {} };
  if (typeof args !== "object" || Array.isArray(args)) {
    return invalid("arguments must be an object");
  }
  const { number, ...rest } = args as Record<string, unknown>;
  const unknown = Object.keys(rest);
  if (unknown.length > 0) {
    return invalid(
      `unknown argument ${unknown[0]}: mark_pull_request_ready takes only number; the repository is fixed by the platform`,
    );
  }
  return validNumber(number);
}

export interface MergeInput {
  number?: number;
  method?: (typeof mergeMethods)[number];
  commitTitle?: string;
  commitMessage?: string;
  auto?: true;
  disableAuto?: true;
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
  const {
    number,
    method,
    commitTitle,
    commitMessage,
    auto,
    disableAuto,
    ...rest
  } = args as Record<string, unknown>;
  const unknown = Object.keys(rest);
  if (unknown.length > 0) {
    return invalid(
      `unknown argument ${unknown[0]}: merge_pull_request takes only number, method, commitTitle, commitMessage and auto, or disableAuto with at most number; the repository is fixed by the platform`,
    );
  }
  const checked = validNumber(number);
  if (!checked.ok) return checked;
  if (auto !== undefined && typeof auto !== "boolean") {
    return invalid("auto must be true or false");
  }
  if (disableAuto !== undefined && typeof disableAuto !== "boolean") {
    return invalid("disableAuto must be true or false");
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
  const named = checked.input;
  if (disableAuto === true) {
    if (auto === true || Object.keys(input).length > 0) {
      return invalid("disableAuto takes no other argument but number");
    }
    return { ok: true, input: { ...named, disableAuto: true } };
  }
  if (auto === true) input.auto = true;
  return { ok: true, input: { ...named, ...input } };
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
  const checked = validateReadyInput(args);
  if (!checked.ok) return checked;
  const label = "pull-request ready endpoint";
  const reply = await post(
    env.BEX_PR_READY_ENDPOINT || "",
    label,
    checked.input,
  );
  if (!reply.ok) return reply;
  const { url, number, alreadyReady, otherPullRequest } = reply.payload;
  if (typeof url !== "string" || !url || typeof number !== "number") {
    return upstreamError(`${label} returned an unexpected response`);
  }
  return {
    ok: true,
    result: {
      url,
      number,
      ...(alreadyReady === true ? { alreadyReady: true as const } : {}),
      ...(otherPullRequest === true ? { otherPullRequest: true as const } : {}),
    },
  };
}

export async function mergePullRequest(
  args: unknown,
  env: NodeJS.ProcessEnv = process.env,
): Promise<Outcome<MergeAnswer>> {
  const checked = validateMergeInput(args);
  if (!checked.ok) return checked;
  const label = "pull-request merge endpoint";
  const reply = await post(
    env.BEX_PR_MERGE_ENDPOINT || "",
    label,
    checked.input,
  );
  if (!reply.ok) return reply;
  const result = mergeAnswer(reply.payload);
  if (!result) {
    return upstreamError(`${label} returned an unexpected response`);
  }
  // Only the session's own pull request is its record: another one merged is
  // not this session's delivery.
  if (!result.merged || result.otherPullRequest) return { ok: true, result };
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

// mergeAnswer reads the gateway's answer as the gateway client does
// (PullRequestMergeResponse.wellFormed): the pull request named, exactly one
// outcome, and a merge's commit and method.
function mergeAnswer(payload: Record<string, unknown>): MergeAnswer | null {
  const { url, number, merged, sha, method, headSha } = payload;
  if (typeof url !== "string" || !url || typeof number !== "number") {
    return null;
  }
  const later = laterOutcomes.filter((outcome) => payload[outcome] === true);
  if (merged === true) {
    if (
      later.length > 0 ||
      typeof sha !== "string" ||
      !sha ||
      typeof method !== "string"
    ) {
      return null;
    }
  } else if (merged !== false || later.length !== 1) {
    return null;
  }
  return {
    url,
    number,
    merged: merged === true,
    ...(typeof sha === "string" && sha ? { sha } : {}),
    ...(typeof method === "string" && method ? { method } : {}),
    headSha: typeof headSha === "string" ? headSha : "",
    ...Object.fromEntries(later.map((outcome) => [outcome, true as const])),
    ...(payload.otherPullRequest === true
      ? { otherPullRequest: true as const }
      : {}),
    ...(later[0] === "autoMerge" || later[0] === "mergeQueue"
      ? { next: endTurnAfter[later[0]] }
      : {}),
  };
}

// recordMerge marks the pull request record (the one open_or_update_draft_pr
// writes) merged at headSha. GitHub may delete a merged branch, and the
// delivery check then counts local commits from the merged head rather than
// asking the agent to push work that is already merged.
async function recordMerge(
  recordPath: string,
  result: MergeAnswer,
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
