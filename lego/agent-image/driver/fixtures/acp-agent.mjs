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

import { appendFileSync, writeFileSync } from "node:fs";
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
      result(message.id, {
        protocolVersion: 1,
        agentCapabilities: {
          loadSession: process.env.ACP_FIXTURE_LOAD_SESSION === "1",
        },
        agentInfo: { name: "bex-test-acp-agent", version: "1" },
      });
      break;
    case "session/new":
      result(message.id, { sessionId: "fixture-session" });
      break;
    case "session/load":
      result(message.id, {});
      break;
    case "session/prompt": {
      if (process.env.ACP_FIXTURE_CRASH === "1") process.exit(23);
      const sessionId = message.params.sessionId;
      const delay = Number(process.env.ACP_FIXTURE_DELAY_MS || 0);
      if (delay) await new Promise((resolve) => setTimeout(resolve, delay));
      update(sessionId, {
        sessionUpdate: "plan",
        entries: [
          { content: "edit and commit", priority: "high", status: "in_progress" },
        ],
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
