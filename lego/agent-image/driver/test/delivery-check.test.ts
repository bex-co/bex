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
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { ensureRepo } from "../src/git-setup.js";
import { runHeadlessTurn } from "../src/session.js";
import { UIMessageStreamHub } from "../src/stream-hub.js";
import {
  childEnv,
  git,
  promptsSent,
  seedBareRemote,
  tempConfig,
  type TestConfig,
} from "./helpers.js";

const branch = "bex-agent/ags-test";

// repoSession builds a hermetic origin plus a workspace set up exactly as the
// driver's setup phase does, then returns a repo-session config.
async function repoSession(mode: string): Promise<{
  config: TestConfig;
  remote: string;
  baseOid: string;
  promptsLog: string;
}> {
  const config = await tempConfig({
    deliver: true,
    branch,
    baseBranch: "main",
    turnTimeoutMs: 20_000,
  });
  const remote = await seedBareRemote(config.root);
  config.repoUrl = remote;
  await ensureRepo(config);
  const promptsLog = path.join(config.root, "prompts.log");
  config.agentEnv = {
    ACP_FIXTURE_DELIVERY_REPO: config.cwd,
    ACP_FIXTURE_DELIVERY: mode,
    ACP_FIXTURE_PROMPTS_LOG: promptsLog,
  };
  return {
    config,
    remote,
    baseOid: git(remote, ["rev-parse", "main"]),
    promptsLog,
  };
}

function remoteBranch(remote: string): string {
  try {
    return git(remote, ["rev-parse", "--verify", "-q", `refs/heads/${branch}`]);
  } catch {
    return "";
  }
}

test("an agent that commits, pushes and opens its PR gets no nudge", async () => {
  const { config, remote, promptsLog } = await repoSession("deliver");
  const result = await runHeadlessTurn(
    config,
    childEnv(config),
    new UIMessageStreamHub(),
  );
  assert.equal((await promptsSent(promptsLog)).length, 1);
  const head = remoteBranch(remote);
  assert.match(head, /^[0-9a-f]{40}$/);
  const expected = {
    dirty: false,
    unpushedCommits: 0,
    remoteHead: head,
    baseBranch: "main",
    prRecorded: true,
    prUrl: "https://github.com/acme/widgets/pull/7",
    prNumber: 7,
    undelivered: [],
  };
  assert.deepEqual(result.git, expected);
  const status = JSON.parse(await readFile(config.statusPath, "utf8"));
  assert.equal(status.state, "succeeded");
  assert.deepEqual(status.git, expected);
  for (const removed of ["delivery", "evidence", "scrubbedFiles"]) {
    assert.equal(status[removed], undefined);
  }
  // The commit is the agent's, under the repo-local identity from setup.
  assert.equal(
    git(remote, ["log", "-1", "--format=%an|%s", head]),
    "bex agent|fixture: agent-authored change",
  );
});

test("a dirty tree gets exactly one nudge on the same session", async () => {
  const { config, remote, promptsLog } =
    await repoSession("dirty-then-deliver");
  const hub = new UIMessageStreamHub();
  const result = await runHeadlessTurn(config, childEnv(config), hub);
  const sent = await promptsSent(promptsLog);
  assert.equal(sent.length, 2);
  assert.match(
    sent[1],
    /^Your changes are not fully delivered: the working tree has uncommitted changes\. Commit, push with `git push origin HEAD:bex-agent\/ags-test` and open or update the draft pull request with `open_or_update_draft_pr`, or explain why not\.$/,
  );
  // The overlay preamble rode only the first prompt.
  assert.match(sent[0], /bex platform context/);
  assert.doesNotMatch(sent[1], /bex platform context/);
  // The nudge reply streams as part of the same turn; the nudge text does not.
  const history = JSON.stringify(hub.history);
  assert.match(history, /Follow-up handled/);
  assert.doesNotMatch(history, /not fully delivered/);
  assert.equal(result.git?.dirty, false);
  assert.equal(result.git?.prRecorded, true);
  assert.equal(result.git?.remoteHead, remoteBranch(remote));
  // The reasons reflect the post-nudge read, not the first one.
  assert.deepEqual(result.git?.undelivered, []);
});

test("an agent that ignores the nudge ends succeeded with git.dirty and no driver commit", async () => {
  const { config, remote, baseOid, promptsLog } = await repoSession("dirty");
  const result = await runHeadlessTurn(
    config,
    childEnv(config),
    new UIMessageStreamHub(),
  );
  assert.equal(result.state, "succeeded");
  assert.equal((await promptsSent(promptsLog)).length, 2, "exactly one nudge");
  assert.deepEqual(result.git, {
    dirty: true,
    unpushedCommits: 0,
    remoteHead: "",
    baseBranch: "main",
    prRecorded: false,
    prUrl: "",
    prNumber: null,
    undelivered: ["the working tree has uncommitted changes"],
  });
  const status = JSON.parse(await readFile(config.statusPath, "utf8"));
  assert.deepEqual(status.git.undelivered, [
    "the working tree has uncommitted changes",
  ]);
  // The driver never committed or pushed on the agent's behalf.
  assert.equal(git(config.cwd, ["rev-parse", "HEAD"]), baseOid);
  assert.equal(remoteBranch(remote), "");
  assert.match(git(config.cwd, ["status", "--porcelain"]), /left-dirty\.txt/);
});

test("pushed commits without a recorded PR are named in the nudge", async () => {
  const { config, promptsLog } = await repoSession("push-no-pr");
  const result = await runHeadlessTurn(
    config,
    childEnv(config),
    new UIMessageStreamHub(),
  );
  const sent = await promptsSent(promptsLog);
  assert.equal(sent.length, 2);
  assert.match(
    sent[1],
    /origin\/bex-agent\/ags-test has pushed commits but no draft pull request was recorded/,
  );
  assert.equal(result.git?.unpushedCommits, 0);
  assert.equal(result.git?.prRecorded, false);
});

test("unpushed local commits are counted against the remote branch", async () => {
  const { config, promptsLog } = await repoSession("none");
  config.agentEnv = {
    ...config.agentEnv,
    ACP_FIXTURE_COMMIT_REPO: config.cwd,
  };
  const result = await runHeadlessTurn(
    config,
    childEnv(config),
    new UIMessageStreamHub(),
  );
  const sent = await promptsSent(promptsLog);
  assert.match(sent[1], /1 local commit is not on origin\/bex-agent\/ags-test/);
  assert.equal(result.git?.unpushedCommits, 1);
});

test("a steering turn's pushed commits count as delivered when an earlier turn opened the PR", async () => {
  const { config, promptsLog } = await repoSession("push-no-pr");
  config.existingPrUrl = "https://github.com/acme/widgets/pull/7";
  config.existingPrNumber = 7;
  const result = await runHeadlessTurn(
    config,
    childEnv(config),
    new UIMessageStreamHub(),
  );
  assert.equal((await promptsSent(promptsLog)).length, 1);
  assert.equal(result.git?.prRecorded, true);
  assert.equal(result.git?.prUrl, "https://github.com/acme/widgets/pull/7");
  assert.equal(result.git?.prNumber, 7);
});
