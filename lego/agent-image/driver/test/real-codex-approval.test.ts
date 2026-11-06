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

// Approval mode against the REAL codex-acp (ADR100, w9/081): opt-in, because
// it needs the pinned adapter and its codex binary installed. Run it with
//   BEX_REAL_CODEX_ACP=/path/to/node_modules/.bin/codex-acp npm test
// A fake OpenAI Responses API asks for one shell call; the driver must open a
// decision for a risky command and pass the denial back.

import assert from "node:assert/strict";
import { appendFileSync, existsSync, mkdirSync } from "node:fs";
import { createServer, type IncomingMessage, type Server } from "node:http";
import type { AddressInfo } from "node:net";
import test from "node:test";
import { runHeadlessTurn } from "../src/session.js";
import { UIMessageStreamHub } from "../src/stream-hub.js";
import { childEnv, tempConfig } from "./helpers.js";

const adapter = process.env.BEX_REAL_CODEX_ACP;
const probeLog = process.env.BEX_REAL_CODEX_PROBE_LOG;

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
  try {
    return raw ? (JSON.parse(raw) as Record<string, unknown>) : {};
  } catch {
    return { raw };
  }
}

type Item = {
  type?: string;
  name?: string;
  output?: unknown;
  call_id?: string;
};

// fakeModel answers the first turn request with one shell function call for
// `command` (shaped for whichever shell tool codex offers), every later
// request with "DONE", and records the function call outputs it receives.
function fakeModel(command: string) {
  let asked = false;
  const outputs: unknown[] = [];
  const server = createServer(async (req, res) => {
    const request = await body(req);
    if (probeLog)
      appendFileSync(
        probeLog,
        JSON.stringify({
          method: req.method,
          url: req.url,
          tools: ((request.tools ?? []) as Item[]).map((t) => t.name ?? t.type),
          input: request.input,
        }) + "\n",
      );
    if (!req.url?.endsWith("/responses")) {
      res.statusCode = 404;
      res.end("{}");
      return;
    }
    const input = (request.input ?? []) as Item[];
    for (const item of input)
      if (
        item.type === "function_call_output" ||
        item.type === "custom_tool_call_output"
      )
        outputs.push(item.output);
    // codex 0.147 offers its shell through the `exec` code-mode tool
    // (`tools.exec_command`) inside an `additional_tools` input item.
    const offered = JSON.stringify(input).includes('"name":"exec"');
    const call = !asked && outputs.length === 0 && offered;
    if (call) asked = true;
    const item = call
      ? {
          type: "custom_tool_call",
          id: "ctc_1",
          call_id: "call_1",
          name: "exec",
          input: `const r = await tools.exec_command({ cmd: ${JSON.stringify(command)} });\ntext(JSON.stringify(r));`,
        }
      : {
          type: "message",
          role: "assistant",
          id: "msg_1",
          content: [{ type: "output_text", text: "DONE", annotations: [] }],
        };
    res.setHeader("content-type", "text/event-stream");
    const events: Array<Record<string, unknown>> = [
      { type: "response.created", response: { id: "resp_1" } },
      { type: "response.output_item.done", output_index: 0, item },
      {
        type: "response.completed",
        response: {
          id: "resp_1",
          usage: {
            input_tokens: 1,
            input_tokens_details: null,
            output_tokens: 1,
            output_tokens_details: null,
            total_tokens: 2,
          },
        },
      },
    ];
    for (const event of events)
      res.write(
        `event: ${String(event.type)}\ndata: ${JSON.stringify(event)}\n\n`,
      );
    res.end();
  });
  return { server, outputs };
}

// fakeDecisions opens every decision and answers it on the first await.
function fakeDecisions(allow: boolean) {
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
        state: allow ? "allowed" : "denied",
        answer: allow ? { allow: true, scope: "once" } : { allow: false },
      }),
    );
  });
  return { server, opened };
}

export async function codexTurn(
  mode: "autonomous" | "risky" | "all",
  command: string,
  allow = false,
) {
  const model = fakeModel(command);
  const decisions = fakeDecisions(allow);
  const modelURL = await listen(model.server);
  const decisionsURL = await listen(decisions.server);
  try {
    const config = await tempConfig({
      command: adapter!,
      args: [],
      profile: "codex",
      approvalMode: mode,
      decisionUrl:
        mode === "autonomous" ? "" : `${decisionsURL}/agent-decisions/YQ/Yg`,
      turnTimeoutMs: 120_000,
    });
    mkdirSync(`${config.root}/.codex`, { recursive: true });
    // The driver binds codex's OpenAI provider slot to the model route
    // (acp.ts providers.set); point it at the fake.
    config.modelBaseUrl = `${modelURL}/v1`;
    config.modelBaseUrlEnvName = "OPENAI_BASE_URL";
    config.credentialEnvName = "OPENAI_API_KEY";
    config.agentEnv = {
      ...config.agentEnv,
      OPENAI_BASE_URL: `${modelURL}/v1`,
      OPENAI_API_KEY: "sk-probe",
      DEFAULT_AUTH_REQUEST: JSON.stringify({ methodId: "api-key" }),
      NO_BROWSER: "1",
      HOME: config.root, // never the developer's own ~/.codex
      CODEX_HOME: `${config.root}/.codex`,
    };
    const result = await runHeadlessTurn(
      config,
      childEnv(config),
      new UIMessageStreamHub(),
    );
    return {
      result,
      opened: decisions.opened,
      outputs: model.outputs,
      workspace: config.cwd,
    };
  } finally {
    model.server.close();
    decisions.server.close();
  }
}

const skip = !adapter && "set BEX_REAL_CODEX_ACP";
const opts = { skip, timeout: 180_000 };

test(
  "the real codex adapter's risky command opens a decision and its denial holds",
  opts,
  async () => {
    const { result, opened, outputs } = await codexTurn("risky", "sudo true");
    assert.equal(result.state, "succeeded", JSON.stringify(result));
    assert.equal((opened[0]?.detail as Record<string, unknown>)?.rule, "sudo");
    assert.match(JSON.stringify(outputs), /rejected by user/i);
  },
);

test(
  "the real codex adapter gates an absolute-path sudo too",
  opts,
  async () => {
    const { opened } = await codexTurn("risky", "/usr/bin/sudo true");
    assert.equal((opened[0]?.detail as Record<string, unknown>)?.rule, "sudo");
  },
);

test(
  "the real codex adapter's ordinary command runs ungated in risky mode",
  opts,
  async () => {
    const { opened, outputs, workspace } = await codexTurn(
      "risky",
      "echo ok > ok.txt",
    );
    assert.equal(opened.length, 0, JSON.stringify(opened));
    assert.ok(outputs.length > 0);
    assert.ok(existsSync(`${workspace}/ok.txt`), JSON.stringify(outputs));
  },
);

test(
  "the real codex adapter asks before every command in all mode, and an allow writes the workspace",
  opts,
  async () => {
    const { opened, workspace, outputs } = await codexTurn(
      "all",
      "touch made.txt",
      true,
    );
    assert.equal(opened.length, 1, JSON.stringify(opened));
    assert.ok(existsSync(`${workspace}/made.txt`), JSON.stringify(outputs));
  },
);

test(
  "the real codex adapter asks nothing in an autonomous session",
  opts,
  async () => {
    const { opened, outputs } = await codexTurn("autonomous", "sudo true");
    assert.equal(opened.length, 0);
    assert.doesNotMatch(JSON.stringify(outputs), /rejected by user/i);
  },
);
