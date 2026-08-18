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

import http, { type IncomingMessage, type ServerResponse } from "node:http";
import type { AgentDriverConfig } from "./config.js";
import type { CredentialManager } from "./credentials.js";
import { describeError } from "./errors.js";
import { DriverGrantVerifier } from "./grant.js";
import type { UIMessageStreamHub, UIMessagePart } from "./stream-hub.js";

export type RunTurn = (
  prompt: string,
  onPart: (part: UIMessagePart) => void,
) => Promise<unknown>;

export interface DriverServerOptions {
  runTurn?: RunTurn;
  // terminalize permanently closes turn authority, kills/reaps an active ACP
  // child, and waits for the turn pipeline to stop before snapshot scrubbing.
  terminalize?: () => Promise<void>;
}

export interface DriverServer {
  server: http.Server;
  address: ReturnType<http.Server["address"]>;
  close(): Promise<void>;
  // setTurnInFlight lets the caller (main.ts's initial headless turn) participate
  // in the same single-flight guard as POST /turn, so a live-turn request during
  // the initial turn gets 409 instead of starting a second agent (codex #9).
  setTurnInFlight(inFlight: boolean): void;
}

function streamHeaders(response: ServerResponse): void {
  response.writeHead(200, {
    "content-type": "text/event-stream",
    "cache-control": "no-cache, no-transform",
    connection: "keep-alive",
    "x-vercel-ai-ui-message-stream": "v1",
  });
  response.flushHeaders();
}

function isLoopback(address: string | undefined): boolean {
  return (
    address === "127.0.0.1" ||
    address === "::1" ||
    address === "::ffff:127.0.0.1"
  );
}

async function readJSONBody(
  request: IncomingMessage,
  limit = 1 << 20,
): Promise<Record<string, unknown>> {
  const chunks: Buffer[] = [];
  let size = 0;
  for await (const chunk of request) {
    size += (chunk as Buffer).length;
    if (size > limit) throw new Error("request body too large");
    chunks.push(chunk as Buffer);
  }
  if (chunks.length === 0) return {};
  return JSON.parse(Buffer.concat(chunks).toString("utf8"));
}

// promptFromBody extracts a turn's prompt from either a bare {prompt} or the
// Vercel AI SDK sendMessages body {messages:[...]} — the last user message's
// text parts, joined. useChat posts the latter; a plain client posts the former.
function promptFromBody(body: Record<string, unknown>): string {
  if (typeof body?.prompt === "string" && body.prompt.trim())
    return body.prompt;
  const messages = Array.isArray(body?.messages) ? body.messages : [];
  for (let i = messages.length - 1; i >= 0; i -= 1) {
    const message = messages[i] as Record<string, unknown> | undefined;
    if (message?.role !== "user") continue;
    if (typeof message.content === "string" && message.content.trim())
      return message.content;
    const parts = Array.isArray(message.parts) ? message.parts : [];
    const text = parts
      .filter(
        (part: Record<string, unknown>) =>
          part?.type === "text" && typeof part.text === "string",
      )
      .map((part: Record<string, unknown>) => part.text as string)
      .join("");
    if (text.trim()) return text;
  }
  return "";
}

export async function startDriverServer(
  config: AgentDriverConfig,
  credentialManager: CredentialManager,
  hub: UIMessageStreamHub,
  options: DriverServerOptions = {},
): Promise<DriverServer> {
  // runTurn (ADR047 D9 t004) runs one live prompt turn on the persistent
  // session, mirroring its parts to the given sink; when unset, POST /turn 501s
  // (the fire-and-forget path serves only GET /stream). turnInFlight enforces
  // single-flight: the agent runs one turn at a time.
  const runTurn = options.runTurn;
  const grants = new DriverGrantVerifier(
    config.grantPublicKey,
    config.sessionID,
  );
  let turnInFlight = false;
  const server = http.createServer((request, response) => {
    const url = new URL(request.url || "/", "http://bex-agent-driver.invalid");
    if (request.method === "GET" && url.pathname === "/healthz") {
      response.writeHead(200, { "content-type": "application/json" });
      response.end('{"ok":true}\n');
      return;
    }
    if (request.method === "GET" && url.pathname === "/stream") {
      streamHeaders(response);
      hub.attach(response);
      return;
    }
    if (request.method === "POST" && url.pathname === "/turn") {
      if (!grants.consume(request, "turn")) {
        response.writeHead(401, { "content-type": "application/json" });
        response.end('{"error":"valid single-use driver grant required"}\n');
        return;
      }
      if (!runTurn) {
        response.writeHead(501, { "content-type": "application/json" });
        response.end('{"error":"live turns not enabled"}\n');
        return;
      }
      if (turnInFlight) {
        response.writeHead(409, { "content-type": "application/json" });
        response.end('{"error":"a turn is already running"}\n');
        return;
      }
      turnInFlight = true;
      void (async () => {
        try {
          const body = await readJSONBody(request);
          const prompt = promptFromBody(body);
          if (!prompt.trim()) {
            response.writeHead(400, { "content-type": "application/json" });
            response.end('{"error":"prompt is required"}\n');
            return;
          }
          streamHeaders(response);
          // The turn publishes to the hub (attached GET clients) and mirrors
          // each part onto THIS response so the gateway tees and forwards it.
          await runTurn(prompt, (part) => {
            if (!response.write(`data: ${JSON.stringify(part)}\n\n`)) {
              response.destroy(new Error("turn stream client is not draining"));
            }
          });
          response.end("data: [DONE]\n\n");
        } catch (error) {
          if (!response.headersSent) {
            response.writeHead(500, { "content-type": "application/json" });
            response.end(
              `${JSON.stringify({ error: credentialManager.redact(describeError(error)) })}\n`,
            );
          } else {
            response.end("data: [DONE]\n\n");
          }
        } finally {
          turnInFlight = false;
        }
      })();
      return;
    }
    if (request.method === "POST" && url.pathname === "/snapshot/scrub") {
      if (!isLoopback(request.socket.remoteAddress)) {
        response.writeHead(403, { "content-type": "application/json" });
        response.end('{"error":"loopback only"}\n');
        return;
      }
      if (!grants.consume(request, "snapshot")) {
        response.writeHead(401, { "content-type": "application/json" });
        response.end('{"error":"valid single-use snapshot grant required"}\n');
        return;
      }
      if (!options.terminalize) {
        response.writeHead(503, { "content-type": "application/json" });
        response.end('{"error":"snapshot terminalization unavailable"}\n');
        return;
      }
      void (async () => {
        try {
          await options.terminalize!();
          const scrubbed = await credentialManager.scrubPersistedState();
          response.writeHead(200, { "content-type": "application/json" });
          response.end(
            `${JSON.stringify({ scrubbedFiles: scrubbed.length })}\n`,
          );
        } catch (error) {
          response.writeHead(500, { "content-type": "application/json" });
          response.end(
            `${JSON.stringify({ error: credentialManager.redact(describeError(error)) })}\n`,
          );
        } finally {
          credentialManager.forget();
        }
      })();
      return;
    }
    response.writeHead(404, { "content-type": "application/json" });
    response.end('{"error":"not found"}\n');
  });
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(config.listenPort, config.listenHost, resolve);
  });
  return {
    server,
    address: server.address(),
    async close() {
      await new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      );
    },
    setTurnInFlight(inFlight: boolean) {
      turnInFlight = inFlight;
    },
  };
}
