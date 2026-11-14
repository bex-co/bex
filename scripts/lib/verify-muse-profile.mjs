// Runs inside a live agent sandbox Pod (scripts/verify-sandbox-isolation.sh,
// BEX_VERIFY_AGENT_DRIVER=1): one headless muse turn (ADR108) through the
// image's real bex-agent-driver, against a loopback stand-in for the gateway
// model proxy. Under the real sandbox runtime it proves that the driver starts
// the pinned Muse binary, switches Muse's own sandbox off (a shell tool call
// runs), and that every model request is POST /v1/responses with only the
// session placeholder and the pinned model. No credential or model is used.
// Prints one JSON verdict line; exits non-zero on any failure.
import { execFile } from "node:child_process";
import { generateKeyPairSync } from "node:crypto";
import { mkdtempSync, readFileSync } from "node:fs";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import path from "node:path";
import { promisify } from "node:util";

const sessionId = "ags-verify-muse";
const placeholder = `bex-model-proxy-placeholder-${sessionId}`;
const pinned = JSON.parse(
  readFileSync("/opt/bex-agent-driver/agent-profiles.json", "utf8"),
).profiles.find((p) => p.id === "muse")?.model;
const work = mkdtempSync(path.join(tmpdir(), "muse-verify-"));
const marker = path.join(work, "marker.txt");
const requests = [];
let mainTurns = 0;

const sse = (value) => `data: ${JSON.stringify(value)}\n\n`;
const response = (status) => ({
  id: "r",
  object: "response",
  model: pinned,
  status,
  output: [],
});
const server = createServer((req, res) => {
  const chunks = [];
  req.on("data", (chunk) => chunks.push(chunk));
  req.on("end", () => {
    let body = {};
    try {
      body = JSON.parse(Buffer.concat(chunks).toString());
    } catch {}
    requests.push({
      method: req.method,
      url: req.url,
      auth: req.headers.authorization,
      model: body.model,
    });
    if (req.method !== "POST" || req.url !== "/v1/responses") {
      res.writeHead(404).end();
      return;
    }
    const tools = new Set(
      (body.tools || []).flatMap((t) => [
        t.name,
        ...(t.tools || []).map((x) => x.name),
      ]),
    );
    let call;
    if (tools.has("submit_reminder_decision")) {
      call = {
        name: "submit_reminder_decision",
        arguments: {
          decision: "none",
          advisory_text: null,
          confidence: "high",
          priority: null,
          reason: "verify",
          skill_id: null,
          visible_for_steps: null,
        },
      };
    } else if (mainTurns++ === 0 && tools.has("bash")) {
      call = {
        name: "bash",
        arguments: {
          command: `echo muse-verify > ${marker}`,
          description: "verify",
        },
      };
    }
    res.writeHead(200, { "content-type": "text/event-stream" });
    res.write(
      sse({
        type: "response.created",
        sequence_number: 1,
        response: response("in_progress"),
      }),
    );
    res.write(
      call
        ? sse({
            type: "response.function_call_arguments.done",
            sequence_number: 2,
            output_index: 0,
            item_id: "f",
            call_id: `c${requests.length}`,
            name: call.name,
            arguments: JSON.stringify(call.arguments),
          })
        : sse({
            type: "response.output_text.delta",
            sequence_number: 2,
            output_index: 0,
            item_id: "m",
            content_index: 0,
            delta: "ok",
          }),
    );
    res.end(
      sse({
        type: "response.completed",
        sequence_number: 3,
        response: {
          ...response("completed"),
          usage: { input_tokens: 1, output_tokens: 1, total_tokens: 2 },
        },
      }),
    );
  });
});
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));

const statusFile = path.join(work, "status.json");
let verdict;
try {
  await promisify(execFile)("bex-agent-driver", [], {
    timeout: 180_000,
    env: {
      PATH: process.env.PATH,
      HOME: path.join(work, "home"),
      BEX_AGENT_PROFILE: "muse",
      BEX_AGENT_SESSION_ID: sessionId,
      BEX_AGENT_CWD: work,
      BEX_AGENT_GRANT_PUBLIC_KEY: generateKeyPairSync(
        "ed25519",
      ).publicKey.export({ format: "jwk" }).x,
      BEX_AGENT_LISTEN_HOST: "127.0.0.1",
      BEX_AGENT_LISTEN_PORT: "8790",
      BEX_AGENT_MODEL_PROXY_URL: `http://127.0.0.1:${server.address().port}`,
      BEX_AGENT_MODEL_API_KEY: placeholder,
      BEX_AGENT_PROMPT: "Write the verification marker.",
      BEX_AGENT_EXIT_AFTER_TURN: "1",
      BEX_AGENT_STATUS_FILE: statusFile,
      BEX_AGENT_SESSION_LOG: path.join(work, "session.jsonl"),
    },
  });
  const status = JSON.parse(readFileSync(statusFile, "utf8")).state;
  const wrote = readFileSync(marker, "utf8").trim() === "muse-verify";
  const proxied =
    requests.length > 0 &&
    requests.every(
      (r) =>
        r.method === "POST" &&
        r.url === "/v1/responses" &&
        r.auth === `Bearer ${placeholder}` &&
        r.model === pinned,
    );
  verdict = {
    ok: status === "succeeded" && wrote && proxied,
    status,
    wrote,
    proxied,
    requests: requests.length,
  };
} catch (error) {
  verdict = {
    ok: false,
    error: String(error).slice(0, 2000),
    requests: requests.length,
  };
}
server.close();
console.log(JSON.stringify(verdict));
process.exit(verdict.ok ? 0 : 1);
