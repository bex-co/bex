#!/usr/bin/env node
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

import { appendFileSync, closeSync, writeFileSync } from "node:fs";
import { spawn, spawnSync } from "node:child_process";
import readline from "node:readline";

if (
  process.env.ACP_FIXTURE_REQUIRE_MODEL_KEY === "1" &&
  process.env.ANTHROPIC_API_KEY !== "bex-model-proxy-placeholder-ags-test"
) {
  throw new Error(
    "expected model credential was not isolated to the ACP child",
  );
}

if (process.env.ACP_FIXTURE_ORPHAN_STDIO === "1") {
  const descendant = spawn(
    process.execPath,
    ["-e", "setInterval(() => {}, 1000)"],
    { stdio: "inherit" },
  );
  writeFileSync(
    process.env.ACP_FIXTURE_PID_LOG,
    `${process.pid}\n${descendant.pid}\n`,
  );
  process.exit(23);
}

function record(method) {
  if (process.env.ACP_FIXTURE_LOG) {
    appendFileSync(process.env.ACP_FIXTURE_LOG, `${method}\n`);
  }
}

function send(message) {
  process.stdout.write(`${JSON.stringify(message)}\n`);
}

function result(id, value) {
  send({ jsonrpc: "2.0", id, result: value });
}

function update(sessionId, value) {
  send({
    jsonrpc: "2.0",
    method: "session/update",
    params: { sessionId, update: value },
  });
}

function git(repo, args) {
  const result = spawnSync("git", args, { cwd: repo });
  if (result.status !== 0) throw new Error(result.stderr.toString());
  return result.stdout.toString().trim();
}

// bexServer is the `bex` MCP server registration the driver sent, if any.
let bexServer;
function rememberSession(method, params) {
  if (process.env.ACP_FIXTURE_SESSION_LOG) {
    appendFileSync(
      process.env.ACP_FIXTURE_SESSION_LOG,
      `${JSON.stringify({ method, params })}\n`,
    );
  }
  bexServer = (params.mcpServers || []).find((server) => server.name === "bex");
}

function bexEnv(name) {
  return bexServer?.env?.find((entry) => entry.name === name)?.value;
}

// Agent-owned delivery stand-ins (ADR090): the fixture behaves like an agent
// that edits, commits, pushes its branch and calls the bex PR tool (simulated
// by writing the tool's driver-owned record file).
function editFixture(repo, name) {
  writeFileSync(`${repo}/${name}`, `${name} by the ACP fixture\n`);
}
function deliverFixture(repo) {
  git(repo, ["add", "-A"]);
  if (git(repo, ["status", "--porcelain"])) {
    git(repo, ["commit", "-m", "fixture: agent-authored change"]);
  }
  git(repo, ["push", "origin", "HEAD"]);
  const record = bexEnv("BEX_PR_RECORD_PATH");
  if (!record) throw new Error("bex MCP server was not registered");
  writeFileSync(
    record,
    JSON.stringify({
      url: "https://github.com/acme/widgets/pull/7",
      number: 7,
      created: true,
      headSha: git(repo, ["rev-parse", "HEAD"]),
    }),
  );
}
function deliveryBehavior(promptIndex) {
  const repo = process.env.ACP_FIXTURE_DELIVERY_REPO;
  const mode = process.env.ACP_FIXTURE_DELIVERY;
  if (!repo || !mode) return;
  if (promptIndex === 0) {
    if (mode === "deliver") {
      editFixture(repo, "delivered.txt");
      deliverFixture(repo);
    } else if (mode === "push-no-pr") {
      editFixture(repo, "pushed.txt");
      git(repo, ["add", "-A"]);
      git(repo, ["commit", "-m", "fixture: pushed without a PR"]);
      git(repo, ["push", "origin", "HEAD"]);
    } else {
      editFixture(repo, "left-dirty.txt");
    }
  } else if (mode === "dirty-then-deliver") {
    deliverFixture(repo);
  }
}

let promptCount = 0;

function commitFixture() {
  const repo = process.env.ACP_FIXTURE_COMMIT_REPO;
  if (!repo) return;
  writeFileSync(`${repo}/agent-result.txt`, "committed by the ACP fixture\n");
  const added = spawnSync("git", ["add", "agent-result.txt"], { cwd: repo });
  if (added.status !== 0) throw new Error(added.stderr.toString());
  const committed = spawnSync("git", ["commit", "-m", "agent: complete task"], {
    cwd: repo,
  });
  if (committed.status !== 0) throw new Error(committed.stderr.toString());
}

let rejectedLoad = false;
const lines = readline.createInterface({ input: process.stdin });
lines.on("line", async (line) => {
  const message = JSON.parse(line);
  record(message.method);
  switch (message.method) {
    case "initialize":
      if (process.env.ACP_FIXTURE_PID_LOG)
        appendFileSync(process.env.ACP_FIXTURE_PID_LOG, `${process.pid}\n`);
      if (process.env.ACP_FIXTURE_HANG_INITIALIZE === "1") break;
      if (process.env.ACP_FIXTURE_REQUIRE_TYPED_FAILURE === "1") {
        const capabilities =
          message.params.clientCapabilities?._meta?.jetbrains?.air
            ?.capabilities;
        if (
          !Array.isArray(capabilities) ||
          !capabilities.includes("sessionFailure")
        ) {
          throw new Error("expected typed session-failure capability");
        }
      }
      if (process.env.ACP_FIXTURE_ENV_LOG) {
        writeFileSync(
          process.env.ACP_FIXTURE_ENV_LOG,
          JSON.stringify({ CODEX_CONFIG: process.env.CODEX_CONFIG ?? null }),
        );
      }
      result(message.id, {
        protocolVersion: 1,
        agentCapabilities: {
          loadSession: process.env.ACP_FIXTURE_LOAD_SESSION === "1",
          ...(process.env.ACP_FIXTURE_SYSTEM_PROMPT === "1"
            ? { sessionCapabilities: { systemPrompt: {} } }
            : {}),
        },
        agentInfo: { name: "bex-test-acp-agent", version: "1" },
      });
      break;
    case "providers/set": {
      const expected = process.env.ACP_FIXTURE_PROVIDER_BASE_URL;
      if (expected && message.params.baseUrl !== expected) {
        throw new Error("provider base URL was not bound to the model proxy");
      }
      if (
        expected &&
        message.params.headers?.Authorization !==
          `Bearer ${process.env.OPENAI_API_KEY}`
      ) {
        throw new Error("provider did not receive the session placeholder");
      }
      result(message.id, {});
      break;
    }
    case "session/new":
      rememberSession("session/new", message.params);
      if (
        process.env.ACP_FIXTURE_REJECT_SYSTEM_PROMPT === "1" &&
        message.params.systemPrompt
      ) {
        send({
          jsonrpc: "2.0",
          id: message.id,
          error: { code: -32602, message: "Invalid params: systemPrompt" },
        });
        break;
      }
      if (process.env.ACP_FIXTURE_SETUP_METADATA === "1") {
        update("fixture-session", {
          sessionUpdate: "available_commands_update",
          availableCommands: [
            { name: "setup-command", description: "Current session command" },
          ],
        });
      }
      if (rejectedLoad)
        throw new Error(
          "failed load poisoned the adapter; a fresh process is required",
        );
      if (process.env.ACP_FIXTURE_FAIL_NEW === "1") {
        send({
          jsonrpc: "2.0",
          id: message.id,
          error: { code: -32603, message: "fresh session creation failed" },
        });
        break;
      }
      if (process.env.ACP_FIXTURE_CLOSE_INPUT_AFTER_SESSION === "1") {
        // Reproduce an adapter that loses its ACP input while staying alive.
        // fd 0 alone leaves libuv's already-open Pipe handle writable on Linux,
        // so close every view there. On macOS, readline and libuv already close
        // the descriptor after closeSync(0); closing those views too aborts the
        // fixture as a double close. In both cases the next JSON-RPC write fails
        // while the child remains alive, exercising the input-error lifecycle.
        if (process.platform === "linux") {
          const inputHandle = process.stdin._handle;
          lines.close();
          closeSync(0);
          process.stdin.destroy();
          inputHandle?.close();
        } else {
          closeSync(0);
        }
        setInterval(() => {}, 1_000);
      }
      // A response makes the parent issue session/prompt immediately. Send it
      // only after the fixture has closed every stdin view so the prompt write
      // deterministically observes EPIPE instead of racing into the pipe buffer.
      result(message.id, { sessionId: "fixture-session" });
      break;
    case "session/load": {
      rememberSession("session/load", message.params);
      if (process.env.ACP_FIXTURE_REPLAY_LOAD === "1") {
        update(message.params.sessionId, {
          sessionUpdate: "agent_message_chunk",
          content: { type: "text", text: "historical-answer-from-prior-turn" },
        });
        update(message.params.sessionId, {
          sessionUpdate: "tool_call",
          toolCallId: "historical-tool",
          title: "historical-tool-result",
          kind: "read",
          status: "completed",
          rawInput: { path: "old.txt" },
          rawOutput: { text: "historical-tool-result" },
        });
      }
      const mode = process.env.ACP_FIXTURE_LOAD_ERROR;
      if (mode === "timeout") break;
      if (mode) {
        rejectedLoad = true;
        const errors = {
          missing: {
            code: -32002,
            message: "Resource not found",
            data: { uri: "session:prior-session" },
          },
          missingLocalized: { code: -32002, message: "Sitzung nicht gefunden" },
          corrupt: {
            code: -32603,
            message: "Saved session state is corrupt",
            data: "near the saved transcript",
          },
          closed: {
            code: -32603,
            message: "Query closed before response received",
          },
          auth: { code: -32000, message: "Authentication required" },
          wrappedAuth: {
            code: -32603,
            message: "Query closed before response received",
            data: "provider authentication failed",
          },
          routing: { code: -32603, message: "model proxy routing failed" },
          unknown: { code: -32603, message: "unknown session failure" },
          cancelled: { code: -32800, message: "Request cancelled" },
        };
        send({ jsonrpc: "2.0", id: message.id, error: errors[mode] });
        break;
      }
      result(message.id, {});
      break;
    }
    case "session/cancel":
      break; // Notification: no JSON-RPC response.
    case "session/prompt": {
      const promptIndex = promptCount++;
      if (process.env.ACP_FIXTURE_PROMPTS_LOG) {
        appendFileSync(
          process.env.ACP_FIXTURE_PROMPTS_LOG,
          `${JSON.stringify(message.params.prompt)}\n`,
        );
      }
      if (process.env.ACP_FIXTURE_PROMPT_LOG) {
        writeFileSync(
          process.env.ACP_FIXTURE_PROMPT_LOG,
          JSON.stringify(message.params.prompt),
        );
      }
      if (process.env.ACP_FIXTURE_CRASH === "1") {
        console.error("fixture crashed on purpose");
        process.exit(23);
      }
      const sessionId = message.params.sessionId;
      if (process.env.ACP_FIXTURE_TYPED_FAILURE === "1") {
        update(sessionId, {
          sessionUpdate: "session_info_update",
          _meta: {
            jetbrains: {
              air: {
                version: 1,
                sessionFailure: {
                  id: "fixture-retry",
                  revision: 1,
                  category: "transport_lost",
                  severity: "warning",
                  title: "model proxy transport retrying",
                  actions: [],
                },
              },
            },
          },
        });
        result(message.id, {
          stopReason: "end_turn",
          _meta: {
            jetbrains: {
              air: {
                version: 1,
                sessionFailure: {
                  id: "fixture-failure",
                  revision: 1,
                  category: "transport_lost",
                  severity: "error",
                  title: "model proxy transport failed",
                  details: `near ${process.env.OPENAI_API_KEY}`,
                  actions: [],
                },
              },
            },
          },
        });
        break;
      }
      if (promptIndex > 0) {
        // A follow-up prompt on the same session (the driver's delivery nudge).
        deliveryBehavior(promptIndex);
        update(sessionId, {
          sessionUpdate: "agent_message_chunk",
          content: { type: "text", text: "Follow-up handled." },
        });
        result(message.id, { stopReason: "end_turn" });
        break;
      }
      const delay = Number(process.env.ACP_FIXTURE_DELAY_MS || 0);
      if (delay) await new Promise((resolve) => setTimeout(resolve, delay));
      update(sessionId, {
        sessionUpdate: "available_commands_update",
        availableCommands: [
          { name: "review", description: "Review the changes" },
        ],
      });
      update(sessionId, {
        sessionUpdate: "current_mode_update",
        currentModeId: "code",
      });
      update(sessionId, {
        sessionUpdate: "plan",
        entries: [
          {
            content: "edit and commit",
            priority: "high",
            status: "in_progress",
          },
        ],
      });
      update(sessionId, {
        sessionUpdate: "agent_thought_chunk",
        content: { type: "text", text: "I'll edit the file and commit." },
      });
      update(sessionId, {
        sessionUpdate: "tool_call",
        toolCallId: "tool-1",
        title: "Edit fixture",
        kind: "edit",
        status: "in_progress",
        rawInput: { path: "agent-result.txt" },
      });
      update(sessionId, {
        sessionUpdate: "tool_call_update",
        toolCallId: "tool-1",
        status: "in_progress",
        content: [
          {
            type: "diff",
            path: "agent-result.txt",
            oldText: "",
            newText: "committed by the ACP fixture\n",
          },
          { type: "terminal", terminalId: "terminal-1" },
        ],
      });
      commitFixture();
      deliveryBehavior(promptIndex);
      update(sessionId, {
        sessionUpdate: "agent_message_chunk",
        content: { type: "text", text: "Task committed." },
      });
      update(sessionId, {
        sessionUpdate: "tool_call_update",
        toolCallId: "tool-1",
        status: "completed",
      });
      result(message.id, { stopReason: "end_turn" });
      break;
    }
    default:
      send({
        jsonrpc: "2.0",
        id: message.id,
        error: { code: -32601, message: `unknown method ${message.method}` },
      });
  }
});
