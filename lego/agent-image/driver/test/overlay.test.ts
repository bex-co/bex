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
  assert.match(text, /Leave the pull request as a draft/);
  assert.match(text, /once the tool has returned its URL/);
  assert.match(text, /no code changes .* written report and make no commit/);
  assert.match(text, /\.github\/pull_request_template\.md/);
  assert.match(text, /`authUrl`/);
  assert.match(
    text,
    /Committing, pushing `bex-agent\/ags-abc` and opening or updating its draft pull request are therefore their explicit request/,
  );
  assert.match(text, /only commit or open pull requests when asked/);
  assert.match(text, /same branch and the same pull request/);
  // Direct guidance, not shouted imperatives (they make models over-trigger).
  assert.doesNotMatch(text, /\b(MUST|NEVER|ALWAYS|IMPORTANT|CRITICAL)\b/);
});

test("a steering turn's overlay names the existing PR and asks for a whole-branch body", () => {
  const first = platformOverlay(repoContext);
  assert.doesNotMatch(first, /already opened draft pull request/);
  assert.match(first, /describe every commit since `main`/);
  const later = platformOverlay({
    ...repoContext,
    existingPrUrl: "https://github.com/acme/widgets/pull/7",
    existingPrNumber: 7,
  });
  assert.match(
    later,
    /already opened draft pull request #7 \(https:\/\/github\.com\/acme\/widgets\/pull\/7\)/,
  );
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
  assert.doesNotMatch(text, /open_or_update_draft_pr|git push/);
});

test("only the full network tier explains the egress proxy, in both overlays", () => {
  const proxyLine = /bex egress proxy .*non-HTTP protocols are not available/;
  const scratch = { ...repoContext, deliver: false, repoUrl: "", branch: "" };
  for (const context of [repoContext, scratch]) {
    assert.match(
      platformOverlay({ ...context, networkTier: "full" }),
      proxyLine,
    );
    for (const networkTier of [undefined, "", "registries", "custom"]) {
      assert.doesNotMatch(
        platformOverlay({ ...context, networkTier }),
        /egress proxy/,
      );
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
    ]);
  }
  assert.deepEqual(
    bexMcpServers({ deliver: false, repoUrl: "", prRecordPath: "/x" }),
    [],
  );
});
