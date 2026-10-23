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

// Post-turn delivery check (ADR090). The agent owns delivery;
// the driver only observes. Before the provider stops it reads Git state with
// read-only commands, and if work is left undelivered it asks the agent once,
// on the same ACP session, to finish. The facts land in the status file as
// `git` for the Completer, with `undelivered` naming what is still missing. Nothing here commits, pushes or rewrites refs.

import { readFile } from "node:fs/promises";
import { describeError } from "./errors.js";
import {
  git,
  hasCommit,
  remoteOid,
  type DeliveryBaseline,
} from "./git-setup.js";
import { toolName } from "./mcp-bex.js";

export interface GitFacts {
  dirty: boolean;
  // Local commits on HEAD that are not on the remote session branch.
  unpushedCommits: number;
  // The remote session branch's head ("" when it does not exist or the
  // probe failed).
  remoteHead: string;
  baseBranch: string;
  // Whether the bex MCP tool recorded a pull request for this session.
  prRecorded: boolean;
  prUrl: string;
  prNumber: number | null;
}

// DeliveryStatus is the status file's `git` object: the facts after the final
// (post-nudge) read plus the plain-language reasons work is still undelivered
// (empty when delivered).
export interface DeliveryStatus extends GitFacts {
  undelivered: string[];
}

export interface DeliveryCheckConfig {
  cwd: string;
  branch: string;
  repoUrl: string;
  baseBranch: string;
  prRecordPath: string;
  existingPrUrl?: string;
  existingPrNumber?: number | null;
  deliveryBaseline?: DeliveryBaseline;
}

async function readPrRecord(
  recordPath: string,
): Promise<{ url: string; number: number | null }> {
  try {
    const record = JSON.parse(await readFile(recordPath, "utf8")) as Record<
      string,
      unknown
    >;
    return {
      url: typeof record.url === "string" ? record.url : "",
      number: typeof record.number === "number" ? record.number : null,
    };
  } catch {
    return { url: "", number: null };
  }
}

async function countCommits(cwd: string, range: string[]): Promise<number> {
  try {
    return Number(await git(cwd, ["rev-list", "--count", ...range])) || 0;
  } catch {
    return 0;
  }
}

export async function readGitFacts(
  config: DeliveryCheckConfig,
): Promise<GitFacts> {
  const cwd = config.cwd;
  const baseline = config.deliveryBaseline;
  const [dirty, remoteHead, recorded] = await Promise.all([
    git(cwd, ["status", "--porcelain"]).then(
      (output) => output !== "",
      (error) => {
        console.warn(
          `delivery check: git status failed: ${describeError(error)}`,
        );
        return false;
      },
    ),
    remoteOid(cwd, config.repoUrl, config.branch, { retry: false }).catch(
      (error) => {
        console.warn(
          `delivery check: remote branch probe failed: ${describeError(error)}`,
        );
        return "";
      },
    ),
    readPrRecord(config.prRecordPath),
  ]);
  // Commits the remote branch does not have yet. Measured against the remote
  // head when it is present locally, otherwise against the setup-time base.
  let exclude = "";
  if (await hasCommit(cwd, remoteHead)) {
    exclude = remoteHead;
  } else if (baseline && (await hasCommit(cwd, baseline.baseOid))) {
    exclude = baseline.baseOid;
  }
  const unpushedCommits = exclude
    ? await countCommits(cwd, ["HEAD", `^${exclude}`])
    : 0;
  // This turn's tool result wins; otherwise the PR an earlier turn opened.
  const record = recorded.url
    ? recorded
    : {
        url: config.existingPrUrl ?? "",
        number: config.existingPrNumber ?? null,
      };
  return {
    dirty,
    unpushedCommits,
    remoteHead,
    baseBranch: baseline?.baseBranch || config.baseBranch,
    prRecorded: record.url !== "",
    prUrl: record.url,
    prNumber: record.number,
  };
}

// undeliveredFacts lists, in plain words, what is still undelivered; empty
// means nothing needs a nudge.
export function undeliveredFacts(
  facts: GitFacts,
  config: Pick<DeliveryCheckConfig, "branch" | "deliveryBaseline">,
): string[] {
  const reasons: string[] = [];
  if (facts.dirty) reasons.push("the working tree has uncommitted changes");
  if (facts.unpushedCommits > 0) {
    reasons.push(
      `${facts.unpushedCommits} local commit${facts.unpushedCommits === 1 ? " is" : "s are"} not on origin/${config.branch}`,
    );
  }
  const baseOid = config.deliveryBaseline?.baseOid ?? "";
  if (facts.remoteHead && facts.remoteHead !== baseOid && !facts.prRecorded) {
    reasons.push(
      `origin/${config.branch} has pushed commits but no draft pull request was recorded`,
    );
  }
  return reasons;
}

export function deliveryNudge(reasons: string[], branch: string): string {
  return (
    `Your changes are not fully delivered: ${reasons.join("; ")}. ` +
    `Commit, push with \`git push origin HEAD:${branch}\` and open or update the draft pull request with \`${toolName}\`, or explain why not.`
  );
}
