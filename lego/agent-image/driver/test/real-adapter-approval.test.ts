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

// Approval mode against the REAL claude-agent-acp (ADR100): opt-in, because
// it needs the pinned adapter installed. Run it with
//   BEX_REAL_CLAUDE_ACP=/path/to/node_modules/.bin/claude-agent-acp npm test
// A fake Anthropic Messages API asks for one Bash call; the driver must open
// a decision for it and pass the denial back. The fixture agent missed the
// production bug this pins (w9/m176, 2026-11-06): the adapter's AIR-shaped
// permission request carries no kind, so `sudo true` ran ungated.

import assert from "node:assert/strict";
import { createServer, type IncomingMessage, type Server } from "node:http";
import type { AddressInfo } from "node:net";
import test from "node:test";
import { runHeadlessTurn } from "../src/session.js";
import { UIMessageStreamHub } from "../src/stream-hub.js";
import { childEnv, tempConfig } from "./helpers.js";

const adapter = process.env.BEX_REAL_CLAUDE_ACP;

type Block = { type: string; [key: string]: unknown };

function listen(server: Server): Promise<string> {
  return new Promise((resolve) =>
    server.listen(0, "127.0.0.1", () =>
      resolve(`http://127.0.0.1:${(server.address() as AddressInfo).port}`),
    ),
  );
}

async function body(req: IncomingMessage): Promise<Record<string, unknown>> {
  let raw = "";
  for await (const chunk of req) raw += chunk;
  return raw ? (JSON.parse(raw) as Record<string, unknown>) : {};
}

// fakeModel answers the first request that offers Bash with a tool_use for
// `command`, everything else with "DONE", and records the tool result.
function fakeModel(command: string) {
  let asked = false;
  const results: Block[] = [];
  const server = createServer(async (req, res) => {
    const request = await body(req);
    const messages = (request.messages ?? []) as Array<{ content?: unknown }>;
    const blocks = messages.flatMap((m) =>
      Array.isArray(m.content) ? (m.content as Block[]) : [],
    );
    const result = blocks.find((b) => b.type === "tool_result");
    if (result) results.push(result);
    const tools = (request.tools ?? []) as Array<{ name: string }>;
    const toolUse = !asked && !result && tools.some((t) => t.name === "Bash");
    if (toolUse) asked = true;
    const message = (content: unknown[], stop: string | null) => ({
      id: "msg",
      type: "message",
      role: "assistant",
      model: "probe",
      content,
      stop_reason: stop,
      stop_sequence: null,
      usage: { input_tokens: 1, output_tokens: 1 },
    });
    const input = { command, description: "probe" };
    const final = toolUse
      ? [{ type: "tool_use", id: "toolu_1", name: "Bash", input }]
      : [{ type: "text", text: "DONE" }];
    const stop = toolUse ? "tool_use" : "end_turn";
    if (request.stream !== true) {
      res.setHeader("content-type", "application/json");
      res.end(JSON.stringify(message(final, stop)));
      return;
    }
    res.setHeader("content-type", "text/event-stream");
    const events: Array<[string, Record<string, unknown>]> = [
      ["message_start", { message: message([], null) }],
      toolUse
        ? [
            "content_block_start",
            {
              index: 0,
              content_block: {
                type: "tool_use",
                id: "toolu_1",
                name: "Bash",
                input: {},
              },
            },
          ]
        : [
            "content_block_start",
            { index: 0, content_block: { type: "text", text: "" } },
          ],
      toolUse
        ? [
            "content_block_delta",
            {
              index: 0,
              delta: {
                type: "input_json_delta",
                partial_json: JSON.stringify(input),
              },
            },
          ]
        : [
            "content_block_delta",
            { index: 0, delta: { type: "text_delta", text: "DONE" } },
          ],
      ["content_block_stop", { index: 0 }],
      [
        "message_delta",
        {
          delta: { stop_reason: stop, stop_sequence: null },
          usage: { output_tokens: 1 },
        },
      ],
      ["message_stop", {}],
    ];
    for (const [type, data] of events)
      res.write(
        `event: ${type}\ndata: ${JSON.stringify({ type, ...data })}\n\n`,
      );
    res.end();
  });
  return { server, results };
}

// fakeDecisions opens every decision and denies it on the first await.
function fakeDecisions() {
  const opened: Array<Record<string, unknown>> = [];
  const server = createServer(async (req, res) => {
    const request = await body(req);
    res.setHeader("content-type", "application/json");
    if (!req.url?.includes("/dcs-")) {
      opened.push(request);
      res.end(
        JSON.stringify({ id: "dcs-00000000000000000001", state: "pending" }),
      );
      return;
    }
    res.end(
      JSON.stringify({
        id: "dcs-00000000000000000001",
        state: "denied",
        answer: { allow: false },
      }),
    );
  });
  return { server, opened };
}

async function turn(mode: "risky" | "all", command: string) {
  const model = fakeModel(command);
  const decisions = fakeDecisions();
  const modelURL = await listen(model.server);
  const decisionsURL = await listen(decisions.server);
  try {
    const config = await tempConfig({
      command: adapter!,
      args: [],
      profile: "claude",
      approvalMode: mode,
      decisionUrl: `${decisionsURL}/agent-decisions/YQ/Yg`,
      turnTimeoutMs: 90_000,
    });
    config.agentEnv = {
      ...config.agentEnv,
      ANTHROPIC_BASE_URL: modelURL,
      ANTHROPIC_API_KEY: "sk-ant-probe",
      HOME: config.root, // never the developer's own ~/.claude rules
    };
    const result = await runHeadlessTurn(
      config,
      childEnv(config),
      new UIMessageStreamHub(),
    );
    return { result, opened: decisions.opened, results: model.results };
  } finally {
    model.server.close();
    decisions.server.close();
  }
}

test(
  "the real claude adapter's risky command opens a decision and its denial holds",
  { skip: !adapter && "set BEX_REAL_CLAUDE_ACP", timeout: 120_000 },
  async () => {
    const { result, opened, results } = await turn("risky", "sudo true");
    assert.equal(result.state, "succeeded", JSON.stringify(result));
    assert.equal((opened[0]?.detail as Record<string, unknown>)?.rule, "sudo");
    assert.match(JSON.stringify(results[0]), /refused permission/i);
  },
);

test(
  "the real claude adapter's ordinary command runs ungated in risky mode",
  { skip: !adapter && "set BEX_REAL_CLAUDE_ACP", timeout: 120_000 },
  async () => {
    const { opened, results } = await turn("risky", "true");
    assert.equal(opened.length, 0);
    assert.doesNotMatch(JSON.stringify(results[0]), /refused permission/i);
  },
);
