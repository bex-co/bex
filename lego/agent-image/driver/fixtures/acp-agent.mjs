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
import { spawnSync } from "node:child_process";
import readline from "node:readline";

if (
  process.env.ACP_FIXTURE_REQUIRE_MODEL_KEY === "1" &&
  process.env.ANTHROPIC_API_KEY !== "test-model-key-never-log"
) {
  throw new Error("expected model credential was not isolated to the ACP child");
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

const lines = readline.createInterface({ input: process.stdin });
lines.on("line", async (line) => {
  const message = JSON.parse(line);
  record(message.method);
  switch (message.method) {
    case "initialize":
      if (process.env.ACP_FIXTURE_HANG_INITIALIZE === "1") break;
      if (process.env.ACP_FIXTURE_REQUIRE_TYPED_FAILURE === "1") {
        const capabilities =
          message.params.clientCapabilities?._meta?.jetbrains?.air?.capabilities;
        if (!Array.isArray(capabilities) || !capabilities.includes("sessionFailure")) {
          throw new Error("expected typed session-failure capability");
        }
      }
      result(message.id, {
        protocolVersion: 1,
        agentCapabilities: {
          loadSession: process.env.ACP_FIXTURE_LOAD_SESSION === "1",
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
      result(message.id, { sessionId: "fixture-session" });
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
      break;
    case "session/load":
      result(message.id, {});
      break;
    case "session/prompt": {
      if (process.env.ACP_FIXTURE_CRASH === "1") {
        if (process.env.ACP_FIXTURE_CRASH_WITH_CREDENTIAL === "1") {
          console.error(process.env.ANTHROPIC_API_KEY);
        }
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
      const delay = Number(process.env.ACP_FIXTURE_DELAY_MS || 0);
      if (delay) await new Promise((resolve) => setTimeout(resolve, delay));
      update(sessionId, {
        sessionUpdate: "plan",
        entries: [
          { content: "edit and commit", priority: "high", status: "in_progress" },
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
      if (process.env.ACP_FIXTURE_LEAK_CREDENTIAL === "1") {
        // Simulate an agent echoing its environment (e.g. a tool running
        // `printenv`): the model key appears in tool output AND in a plain
        // message chunk, exercising both the raw-ACP and UI stream paths.
        update(sessionId, {
          sessionUpdate: "tool_call_update",
          toolCallId: "tool-1",
          status: "in_progress",
          rawOutput: { stdout: `ANTHROPIC_API_KEY=${process.env.ANTHROPIC_API_KEY}` },
        });
        update(sessionId, {
          sessionUpdate: "agent_message_chunk",
          content: { type: "text", text: `the key is ${process.env.ANTHROPIC_API_KEY}` },
        });
      }
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
