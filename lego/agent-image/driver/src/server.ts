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
import readline from "node:readline";
import { WebSocket, WebSocketServer, type RawData } from "ws";
import { spawnRawACP } from "./acp.js";
import type { AgentDriverConfig } from "./config.js";
import type { CredentialManager } from "./credentials.js";
import type { UIMessageStreamHub, UIMessagePart } from "./stream-hub.js";

export type RunTurn = (
  prompt: string,
  onPart: (part: UIMessagePart) => void,
) => Promise<unknown>;

export interface DriverServerOptions {
  runTurn?: RunTurn;
}

export interface DriverServer {
  server: http.Server;
  address: ReturnType<http.Server["address"]>;
  close(): Promise<void>;
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
  return address === "127.0.0.1" || address === "::1" || address === "::ffff:127.0.0.1";
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
  if (typeof body?.prompt === "string" && body.prompt.trim()) return body.prompt;
  const messages = Array.isArray(body?.messages) ? body.messages : [];
  for (let i = messages.length - 1; i >= 0; i -= 1) {
    const message = messages[i] as Record<string, unknown> | undefined;
    if (message?.role !== "user") continue;
    if (typeof message.content === "string" && message.content.trim()) return message.content;
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

function bridgeACP(
  socket: WebSocket,
  config: AgentDriverConfig,
  credentialManager: CredentialManager,
): void {
  const child = spawnRawACP(config, credentialManager.agentEnvironment());
  const lines = readline.createInterface({ input: child.stdout });

  lines.on("line", (line) => {
    if (socket.readyState === WebSocket.OPEN) {
      socket.send(credentialManager.redact(line));
    }
  });
  child.stderr.on("data", (chunk: Buffer) => {
    // Agent diagnostics stay local to the sandbox and never enter ACP framing.
    process.stderr.write(credentialManager.redact(chunk.toString()));
  });
  child.once("error", (error) => socket.close(1011, error.message.slice(0, 120)));
  child.once("exit", (code) => {
    if (socket.readyState === WebSocket.OPEN) {
      socket.close(code === 0 ? 1000 : 1011, `agent exited ${code}`);
    }
  });
  socket.on("message", (data: RawData, binary: boolean) => {
    if (binary) {
      socket.close(1003, "ACP transport accepts JSON text only");
      return;
    }
    const line = data.toString();
    try {
      JSON.parse(line);
    } catch {
      socket.close(1007, "invalid JSON-RPC message");
      return;
    }
    child.stdin.write(`${line}\n`);
  });
  socket.once("close", () => {
    lines.close();
    child.kill("SIGTERM");
  });
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
            response.write(`data: ${JSON.stringify(part)}\n\n`);
          });
          response.end("data: [DONE]\n\n");
        } catch (error) {
          if (!response.headersSent) {
            response.writeHead(500, { "content-type": "application/json" });
            response.end(
              `${JSON.stringify({ error: credentialManager.redact(error instanceof Error ? error.message : String(error)) })}\n`,
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
      void credentialManager
        .scrubPersistedState()
        .then((scrubbed) => {
          credentialManager.forget();
          response.writeHead(200, { "content-type": "application/json" });
          response.end(`${JSON.stringify({ scrubbedFiles: scrubbed.length })}\n`);
        })
        .catch((error) => {
          response.writeHead(500, { "content-type": "application/json" });
          response.end(`${JSON.stringify({ error: error instanceof Error ? error.message : String(error) })}\n`);
        });
      return;
    }
    response.writeHead(404, { "content-type": "application/json" });
    response.end('{"error":"not found"}\n');
  });
  const sockets = new WebSocketServer({ noServer: true });
  server.on("upgrade", (request, socket, head) => {
    const url = new URL(request.url || "/", "http://bex-agent-driver.invalid");
    if (url.pathname !== "/acp") {
      socket.destroy();
      return;
    }
    sockets.handleUpgrade(request, socket, head, (websocket) => {
      sockets.emit("connection", websocket, request);
    });
  });
  sockets.on("connection", (socket: WebSocket) => bridgeACP(socket, config, credentialManager));

  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(config.listenPort, config.listenHost, resolve);
  });
  return {
    server,
    address: server.address(),
    async close() {
      for (const socket of sockets.clients) socket.terminate();
      await new Promise<void>((resolve) => sockets.close(() => resolve()));
      await new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      );
    },
  };
}
