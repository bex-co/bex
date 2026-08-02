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

import { spawn } from "node:child_process";
import readline from "node:readline";
import { PROTOCOL_VERSION } from "@agentclientprotocol/sdk";
import { createACPProvider } from "@mcpc-tech/acp-ai-provider";

const probeTimeoutMs = 10_000;
const maximumConnectTimeoutMs = 60_000;

function processOptions(config, agentEnv) {
  return {
    cwd: config.cwd,
    env: { ...process.env, ...agentEnv },
    stdio: ["pipe", "pipe", "pipe"],
  };
}

export async function probeAgentCapabilities(config, agentEnv) {
  const child = spawn(config.command, config.args, processOptions(config, agentEnv));
  const lines = readline.createInterface({ input: child.stdout });
  const stderr = [];
  child.stderr.on("data", (chunk) => stderr.push(chunk.toString()));

  const request = {
    jsonrpc: "2.0",
    id: 1,
    method: "initialize",
    params: {
      protocolVersion: PROTOCOL_VERSION,
      clientCapabilities: {
        fs: { readTextFile: false, writeTextFile: false },
        terminal: false,
      },
      clientInfo: { name: "bex-agent-driver", version: "0.1.0" },
    },
  };

  try {
    return await new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        reject(new Error(`ACP capability probe timed out after ${probeTimeoutMs}ms`));
      }, probeTimeoutMs);
      timer.unref();

      child.once("error", (error) => {
        clearTimeout(timer);
        reject(error);
      });
      child.once("exit", (code, signal) => {
        clearTimeout(timer);
        reject(
          new Error(
            `ACP agent exited during capability probe (code=${code}, signal=${signal}): ${stderr.join("").trim()}`,
          ),
        );
      });
      lines.on("line", (line) => {
        let message;
        try {
          message = JSON.parse(line);
        } catch {
          return;
        }
        if (message.id !== request.id) return;
        clearTimeout(timer);
        if (message.error) {
          reject(new Error(`ACP initialize failed: ${JSON.stringify(message.error)}`));
          return;
        }
        resolve(message.result?.agentCapabilities || {});
      });
      child.stdin.write(`${JSON.stringify(request)}\n`);
    });
  } finally {
    lines.close();
    child.kill("SIGTERM");
  }
}

export async function createSessionProvider(config, agentEnv) {
  let existingSessionId;
  let resume = "new";
  if (config.existingSessionId) {
    const capabilities = await probeAgentCapabilities(config, agentEnv);
    if (capabilities.loadSession === true) {
      existingSessionId = config.existingSessionId;
      resume = "loaded";
    } else {
      resume = "unsupported";
    }
  }

  const provider = createACPProvider({
    command: config.command,
    args: config.args,
    env: agentEnv,
    session: { cwd: config.cwd, mcpServers: [] },
    persistSession: config.persistSession,
    ...(existingSessionId ? { existingSessionId } : {}),
  });
  // Spawn before the caller drops its copy of the model credential. Session
  // creation remains lazy, but the already-running child owns the only env copy.
  const connectTimeoutMs = Math.min(config.turnTimeoutMs, maximumConnectTimeoutMs);
  let timer;
  try {
    await Promise.race([
      provider.connect(),
      new Promise((_, reject) => {
        timer = setTimeout(
          () => reject(new Error(`ACP provider connect timed out after ${connectTimeoutMs}ms`)),
          connectTimeoutMs,
        );
        timer.unref();
      }),
    ]);
  } catch (error) {
    provider.cleanup();
    throw error;
  } finally {
    clearTimeout(timer);
  }
  return { provider, resume };
}

export function spawnRawACP(config, agentEnv) {
  return spawn(config.command, config.args, processOptions(config, agentEnv));
}
