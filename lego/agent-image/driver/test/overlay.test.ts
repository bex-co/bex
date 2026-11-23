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
import { bexMcpServers, createSessionProvider } from "../src/acp.js";
import { platformOverlay } from "../src/overlay.js";
import { RepoSetupError } from "../src/repo-setup.js";
import { runHeadlessTurn } from "../src/session.js";
import { UIMessageStreamHub } from "../src/stream-hub.js";
import {
  childEnv,
  promptsSent,
  tempConfig,
  type TestConfig,
} from "./helpers.js";

const repoContext = {
  cwd: "/workspace",
  deliver: true,
  repoUrl: "http://gw:8082/git/bnM/c2lk/cmVwbw/YnJhbmNo",
  branch: "bex-agent/ags-abc",
  baseBranch: "",
  repository: "acme/widgets",
  deliveryBaseline: { baseBranch: "main" },
};

async function sessionCalls(
  file: string,
): Promise<{ method: string; params: Record<string, any> }[]> {
  return (await readFile(file, "utf8"))
    .trim()
    .split("\n")
    .map((line) => JSON.parse(line));
}

async function logged(config: TestConfig) {
  const sessionLog = path.join(config.root, "sessions.log");
  const promptsLog = path.join(config.root, "prompts.log");
  return { sessionLog, promptsLog };
}

test("the repo overlay names the branch, base, tool and authorization", () => {
  const text = platformOverlay(repoContext);
  assert.match(text, /bex platform context \(overlay v1\)/);
  assert.match(text, /`bex-agent\/ags-abc`/);
  assert.match(text, /into `main`/);
  assert.match(text, /`acme\/widgets`/);
  assert.match(text, /git push origin HEAD:bex-agent\/ags-abc/);
  assert.match(text, /git pull --rebase origin bex-agent\/ags-abc/);
  assert.match(text, /open_or_update_draft_pr/);
  assert.match(text, /Do not force-push/);
  // w10/040: Devin's default, a pull request ready for review; a draft only
  // when someone asks for one.
  assert.match(
    text,
    /The deliverable is a pull request from `bex-agent\/ags-abc` into `main`\./,
  );
  assert.match(
    text,
    /The pull request opens ready for review; pass `draft: true` only when the task, a person's message or the repository's instructions ask for a draft pull request\./,
  );
  assert.doesNotMatch(text, /draft pull request from/);
  assert.match(
    text,
    /started by pull request feedback, reply on the pull request with `reply_on_pull_request`/,
  );
  assert.match(text, /once the tool has returned its URL/);
  assert.match(text, /no code changes .* written report and make no commit/);
  assert.match(text, /\.github\/pull_request_template\.md/);
  assert.match(text, /`authUrl`/);
  assert.match(
    text,
    /Committing, pushing `bex-agent\/ags-abc` and opening or updating its pull request are therefore their explicit request/,
  );
  assert.match(text, /only commit or open pull requests when asked/);
  assert.match(text, /same branch and the same pull request/);
  // w10/m538: marking ready and merging happen only when asked, as Devin
  // does, after the checks pass, and a refusal is reported, not retried.
  assert.match(
    text,
    /Leave the merge to the requester, and a draft as a draft, unless the task or a person's message asks you to merge it or mark it ready\. When asked, call `mark_pull_request_ready` to mark a draft ready for review/,
  );
  assert.match(
    text,
    /To merge, mark a draft ready first, then call `merge_pull_request`; it acts on the head you last pushed and recorded with `open_or_update_draft_pr`/,
  );
  // w10/m577: asked to merge once the checks pass, it arms auto-merge or the
  // merge queue and ends its turn instead of watching them.
  assert.match(
    text,
    /When asked to merge once the checks pass, call it with `auto: true` and end your turn instead of waiting for the checks: GitHub merges the pull request when its required checks pass/,
  );
  assert.match(
    text,
    /if a person asks you not to merge after all, call `merge_pull_request` with `disableAuto: true`/,
  );
  // Nothing says to wait with a gh pr checks that names no pull request,
  // which fails in a session (w10/042).
  assert.doesNotMatch(text, /gh pr checks --watch/);
  // w10/041: asked about another pull request of the repository, it passes
  // that one's number, merged at the head GitHub reports when it calls.
  assert.match(
    text,
    /When the task or a person asks you to mark ready or merge another pull request of this repository, pass its `number` to either tool; that merge is pinned to the head GitHub reports for it when you call/,
  );
  assert.match(
    text,
    /you cannot approve your own pull request: when GitHub refuses, report its reason as given and do not call the tool in a loop/,
  );
  assert.match(text, /include the merge commit in your final reply/);
  assert.match(
    text,
    /Marking the pull request ready or merging it is not part of that request unless the task or a later message asks for it/,
  );
  assert.doesNotMatch(text, /requester's decision/);
  // Direct guidance, not shouted imperatives (they make models over-trigger).
  assert.doesNotMatch(text, /\b(MUST|NEVER|ALWAYS|IMPORTANT|CRITICAL)\b/);
});

test("a steering turn's overlay names the existing PR and asks for a whole-branch body", () => {
  const first = platformOverlay(repoContext);
  assert.doesNotMatch(first, /already opened pull request/);
  assert.match(first, /describe every commit since `main`/);
  const later = platformOverlay({
    ...repoContext,
    existingPrUrl: "https://github.com/acme/widgets/pull/7",
    existingPrNumber: 7,
  });
  assert.match(
    later,
    /already opened pull request #7 \(https:\/\/github\.com\/acme\/widgets\/pull\/7\)/,
  );
});

test("a takeover pushes its own branch and fast-forwards the person's PR with the tool", () => {
  const takeover = {
    ...repoContext,
    deliveryBaseline: { baseBranch: "feature/fix-login" },
    existingPrUrl: "https://github.com/acme/widgets/pull/7",
    existingPrNumber: 7,
    prTakeover: true,
  };
  const text = platformOverlay(takeover);
  assert.match(
    text,
    /You took over pull request #7 \(https:\/\/github\.com\/acme\/widgets\/pull\/7\), which a person opened\. Your branch `bex-agent\/ags-abc` started from its branch `feature\/fix-login`\./,
  );
  assert.match(
    text,
    /Pushing `bex-agent\/ags-abc` does not change the pull request: `open_or_update_draft_pr` brings it up to date by fast-forwarding `feature\/fix-login`/,
  );
  assert.match(text, /git push origin HEAD:bex-agent\/ags-abc/);
  assert.match(
    text,
    /Do not force-push, and do not rebase or amend pushed commits/,
  );
  assert.match(
    text,
    /`pr_branch_diverged`.*`git fetch origin feature\/fix-login && git merge FETCH_HEAD`/,
  );
  assert.match(
    text,
    /leaves the pull request's title, description and draft or ready state as they are/,
  );
  assert.match(text, /call `reply_on_pull_request` with what you changed/);
  assert.match(text, /a `\/bex` comment on pull request #7/);
  // The person's pull request is marked ready or merged only when asked.
  assert.match(
    text,
    /6\. Its draft or ready state and whether it merges belong to the person who opened it, so leave them as they are unless the `\/bex` comment or a later message asks you to mark it ready or merge it\. When asked, call `mark_pull_request_ready`/,
  );
  assert.match(
    text,
    /Marking the pull request ready or merging it is not part of that request unless the comment or a later message asks for it/,
  );
  assert.doesNotMatch(text, /The deliverable is a pull request/);
  assert.doesNotMatch(text, /pass `draft: true`/);
  assert.doesNotMatch(text, /already opened pull request/);
  assert.doesNotMatch(text, /\b(MUST|NEVER|ALWAYS|IMPORTANT|CRITICAL)\b/);
  // Without a recorded PR the flag changes nothing: the PR overlay stays.
  const unrecorded = platformOverlay({
    ...takeover,
    existingPrUrl: "",
    existingPrNumber: null,
  });
  assert.match(unrecorded, /The deliverable is a pull request/);
});

test("a repo-less session gets the scratch overlay without Git delivery", () => {
  const text = platformOverlay({
    ...repoContext,
    deliver: false,
    repoUrl: "",
    branch: "",
  });
  assert.match(text, /No repository is attached/);
  assert.match(text, /The deliverable is your reply/);
  assert.doesNotMatch(
    text,
    /open_or_update_draft_pr|reply_on_pull_request|git push/,
  );
});

test("only the full network tier explains the egress proxy, in both overlays", () => {
  const proxyLine = /bex egress proxy .*non-HTTP protocols are not available/;
  const scratch = { ...repoContext, deliver: false, repoUrl: "", branch: "" };
  for (const context of [repoContext, scratch]) {
    assert.match(
      platformOverlay({ ...context, networkTier: "full" }),
      proxyLine,
    );
    assert.match(
      platformOverlay({ ...context, networkTier: "full" }),
      /Java tools use it too: `JAVA_TOOL_OPTIONS` .*`java\.net\.http\.HttpClient`.* with no `Authenticator`.*every `mvn` gets Maven settings naming it/,
    );
    for (const networkTier of [undefined, "", "registries", "custom"]) {
      assert.doesNotMatch(
        platformOverlay({ ...context, networkTier }),
        /egress proxy|JAVA_TOOL_OPTIONS/,
      );
    }
  }
});

// w4/m544: the agent is told which Python it has and why, in both overlays;
// a session that selected none (or whose request was refused) is told none.
test("the overlay names the session's Python and its source", () => {
  const scratch = { ...repoContext, deliver: false, repoUrl: "", branch: "" };
  for (const context of [repoContext, scratch]) {
    const fromFile = platformOverlay({
      ...context,
      python: { version: "3.13.18", source: ".python-version", narration: "" },
    });
    assert.match(
      fromFile,
      /- `python`, `python3`, `pip` and `pip3` are Python 3\.13\.18, chosen as a Render deploy of this code would choose it: from `\.python-version`\. .*`\/usr\/bin\/python3` is Debian's 3\.11\./,
    );
    assert.match(
      platformOverlay({
        ...context,
        python: { version: "3.14.10", source: "default", narration: "" },
      }),
      /are Python 3\.14\.10, chosen as a Render deploy of this code would choose it: Render's default for a new service, since neither `PYTHON_VERSION` nor a `\.python-version` names one\./,
    );
    for (const python of [undefined, { error: new RepoSetupError("no") }]) {
      assert.doesNotMatch(platformOverlay({ ...context, python }), /`python3`/);
    }
  }
});

test("an advertised systemPrompt capability carries the overlay on session/new and session/load", async () => {
  for (const load of [false, true]) {
    const config = await tempConfig({
      existingSessionId: load ? "prior-session" : "",
    });
    const { sessionLog, promptsLog } = await logged(config);
    config.agentEnv = {
      ACP_FIXTURE_SYSTEM_PROMPT: "1",
      ACP_FIXTURE_LOAD_SESSION: load ? "1" : "0",
      ACP_FIXTURE_SESSION_LOG: sessionLog,
      ACP_FIXTURE_PROMPTS_LOG: promptsLog,
    };
    const hub = new UIMessageStreamHub();
    await runHeadlessTurn(config, childEnv(config), hub);
    const [call] = await sessionCalls(sessionLog);
    assert.equal(call.method, load ? "session/load" : "session/new");
    assert.equal(call.params.systemPrompt.mode, "append");
    assert.deepEqual(call.params.systemPrompt.content, [
      { type: "text", text: platformOverlay(config) },
    ]);
    const [prompt] = await promptsSent(promptsLog);
    assert.equal(prompt, config.prompt, "no preamble when the field is used");
    assert.doesNotMatch(JSON.stringify(hub.history), /bex platform context/);
  }
});

test("Codex receives the overlay as developer_instructions through CODEX_CONFIG", async () => {
  const config = await tempConfig({ profile: "codex" });
  const { sessionLog, promptsLog } = await logged(config);
  const envLog = path.join(config.root, "env.json");
  config.agentEnv = {
    ACP_FIXTURE_ENV_LOG: envLog,
    ACP_FIXTURE_SESSION_LOG: sessionLog,
    ACP_FIXTURE_PROMPTS_LOG: promptsLog,
  };
  await runHeadlessTurn(config, childEnv(config), new UIMessageStreamHub());
  const env = JSON.parse(await readFile(envLog, "utf8"));
  assert.equal(
    JSON.parse(env.CODEX_CONFIG).developer_instructions,
    platformOverlay(config),
  );
  const [call] = await sessionCalls(sessionLog);
  assert.equal(call.params.systemPrompt, undefined);
  assert.equal((await promptsSent(promptsLog))[0], config.prompt);
});

test("other adapters get the overlay prepended once to the first prompt, never streamed", async () => {
  const config = await tempConfig();
  const { sessionLog, promptsLog } = await logged(config);
  config.agentEnv = {
    ACP_FIXTURE_SESSION_LOG: sessionLog,
    ACP_FIXTURE_PROMPTS_LOG: promptsLog,
    ACP_FIXTURE_LOAD_SESSION: "1",
  };
  const hub = new UIMessageStreamHub();
  const mirror: string[] = [];
  const first = await runHeadlessTurn(config, childEnv(config), hub, {
    closeHub: false,
    onPart: (part) => mirror.push(JSON.stringify(part)),
  });
  const [prompt] = await promptsSent(promptsLog);
  assert.ok(
    prompt.startsWith("<bex-platform-context>\n# bex platform context"),
  );
  assert.equal(prompt.split("bex platform context (overlay").length, 2);
  assert.ok(prompt.endsWith(config.prompt));
  const [call] = await sessionCalls(sessionLog);
  assert.equal(call.params.systemPrompt, undefined);
  for (const body of [
    JSON.stringify(hub.history),
    mirror.join("\n"),
    await readFile(config.sessionLogPath, "utf8"),
  ]) {
    assert.doesNotMatch(body, /bex platform context/);
  }
  // A later turn that natively reloads the session already carries it.
  config.existingSessionId = first.sessionId!;
  await runHeadlessTurn(config, childEnv(config), hub, {
    prompt: "follow-up",
  });
  assert.equal((await promptsSent(promptsLog))[1], "follow-up");
});

test("an invalid_params reply to the overlay fails the turn with a named reason", async () => {
  const config = await tempConfig();
  config.agentEnv = {
    ACP_FIXTURE_SYSTEM_PROMPT: "1",
    ACP_FIXTURE_REJECT_SYSTEM_PROMPT: "1",
  };
  await assert.rejects(
    runHeadlessTurn(config, childEnv(config), new UIMessageStreamHub()),
    /ACP agent rejected the bex platform overlay \(systemPrompt invalid_params\)/,
  );
  const status = JSON.parse(await readFile(config.statusPath, "utf8"));
  assert.equal(status.state, "failed");
  assert.match(status.error, /rejected the bex platform overlay/);
});

test("Claude sessions disable built-in git instructions and attribution", async () => {
  const config = await tempConfig({ profile: "claude" });
  const { sessionLog } = await logged(config);
  config.agentEnv = {
    ACP_FIXTURE_SYSTEM_PROMPT: "1",
    ACP_FIXTURE_SESSION_LOG: sessionLog,
  };
  await runHeadlessTurn(config, childEnv(config), new UIMessageStreamHub());
  const [call] = await sessionCalls(sessionLog);
  assert.deepEqual(call.params._meta.claudeCode.options, {
    settings: {
      includeGitInstructions: false,
      attribution: { commit: "", pr: "" },
    },
    env: { CLAUDE_CODE_DISABLE_GIT_INSTRUCTIONS: "1" },
  });
});

test("repo sessions register the bex stdio MCP server on session/new and session/load", async () => {
  for (const load of [false, true]) {
    const config = await tempConfig({
      deliver: true,
      branch: "bex-agent/ags-test",
      repoUrl: "http://gw:8082/git/bnM/c2lk/cmVwbw/YnJhbmNo/",
      existingSessionId: load ? "prior-session" : "",
    });
    const { sessionLog } = await logged(config);
    config.agentEnv = {
      ACP_FIXTURE_LOAD_SESSION: load ? "1" : "0",
      ACP_FIXTURE_SESSION_LOG: sessionLog,
    };
    const provider = await createSessionProvider(config, childEnv(config), {
      onUpdate: () => {},
    });
    await provider.cleanup();
    const [call] = await sessionCalls(sessionLog);
    assert.equal(call.method, load ? "session/load" : "session/new");
    assert.equal(call.params.mcpServers.length, 1);
    const [server] = call.params.mcpServers;
    assert.equal(server.name, "bex");
    assert.equal(server.command, process.execPath);
    assert.equal(server.args.length, 1);
    assert.ok(path.isAbsolute(server.args[0]));
    assert.match(server.args[0], /mcp-bex\.(js|ts)$/);
    assert.deepEqual(server.env, [
      {
        name: "BEX_PR_ENDPOINT",
        value: "http://gw:8082/git/bnM/c2lk/cmVwbw/YnJhbmNo/pull-request",
      },
      { name: "BEX_PR_RECORD_PATH", value: config.prRecordPath },
      {
        name: "BEX_PR_COMMENT_ENDPOINT",
        value:
          "http://gw:8082/git/bnM/c2lk/cmVwbw/YnJhbmNo/pull-request/comment",
      },
      {
        name: "BEX_PR_READY_ENDPOINT",
        value: "http://gw:8082/git/bnM/c2lk/cmVwbw/YnJhbmNo/pull-request/ready",
      },
      {
        name: "BEX_PR_MERGE_ENDPOINT",
        value: "http://gw:8082/git/bnM/c2lk/cmVwbw/YnJhbmNo/pull-request/merge",
      },
    ]);
  }
  assert.deepEqual(
    bexMcpServers({
      deliver: false,
      repoUrl: "",
      prRecordPath: "/x",
      shareUrl: "",
    }),
    [],
  );
  // A repo-less session still gets the server when it can share files, with
  // only the share endpoint.
  const [scratch] = bexMcpServers({
    deliver: false,
    repoUrl: "",
    prRecordPath: "/x",
    shareUrl: "http://gw:8082/agent-attachments/bnM/c2lk",
  });
  assert.deepEqual(scratch.env, [
    {
      name: "BEX_SHARE_ENDPOINT",
      value: "http://gw:8082/agent-attachments/bnM/c2lk",
    },
  ]);
});

test("the overlay explains share_file only when the session can share", () => {
  const base = {
    cwd: "/workspace",
    deliver: false,
    repoUrl: "",
    branch: "",
    baseBranch: "",
    repository: "",
  };
  assert.doesNotMatch(platformOverlay(base), /share_file/);
  assert.match(
    platformOverlay({ ...base, shareUrl: "http://gw/agent-attachments/a/b" }),
    /## Sharing files[\s\S]*`share_file`/,
  );
});
