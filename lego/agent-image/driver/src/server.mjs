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

import http from "node:http";
import readline from "node:readline";
import { WebSocket, WebSocketServer } from "ws";
import { spawnRawACP } from "./acp.mjs";

function streamHeaders(response) {
  response.writeHead(200, {
    "content-type": "text/event-stream",
    "cache-control": "no-cache, no-transform",
    connection: "keep-alive",
    "x-vercel-ai-ui-message-stream": "v1",
  });
  response.flushHeaders();
}

function isLoopback(address) {
  return address === "127.0.0.1" || address === "::1" || address === "::ffff:127.0.0.1";
}

function bridgeACP(socket, config, credentialManager) {
  const child = spawnRawACP(config, credentialManager.agentEnvironment());
  const lines = readline.createInterface({ input: child.stdout });

  lines.on("line", (line) => {
    if (socket.readyState === WebSocket.OPEN) {
      socket.send(credentialManager.redact(line));
    }
  });
  child.stderr.on("data", (chunk) => {
    // Agent diagnostics stay local to the sandbox and never enter ACP framing.
    process.stderr.write(credentialManager.redact(chunk.toString()));
  });
  child.once("error", (error) => socket.close(1011, error.message.slice(0, 120)));
  child.once("exit", (code) => {
    if (socket.readyState === WebSocket.OPEN) {
      socket.close(code === 0 ? 1000 : 1011, `agent exited ${code}`);
    }
  });
  socket.on("message", (data, binary) => {
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

export async function startDriverServer(config, credentialManager, hub) {
  const server = http.createServer((request, response) => {
    const url = new URL(request.url, "http://bex-agent-driver.invalid");
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
          response.end(`${JSON.stringify({ error: error.message })}\n`);
        });
      return;
    }
    response.writeHead(404, { "content-type": "application/json" });
    response.end('{"error":"not found"}\n');
  });
  const sockets = new WebSocketServer({ noServer: true });
  server.on("upgrade", (request, socket, head) => {
    const url = new URL(request.url, "http://bex-agent-driver.invalid");
    if (url.pathname !== "/acp") {
      socket.destroy();
      return;
    }
    sockets.handleUpgrade(request, socket, head, (websocket) => {
      sockets.emit("connection", websocket, request);
    });
  });
  sockets.on("connection", (socket) => bridgeACP(socket, config, credentialManager));

  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(config.listenPort, config.listenHost, resolve);
  });
  return {
    server,
    address: server.address(),
    async close() {
      for (const socket of sockets.clients) socket.terminate();
      await new Promise((resolve) => sockets.close(resolve));
      await new Promise((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      );
    },
  };
}
