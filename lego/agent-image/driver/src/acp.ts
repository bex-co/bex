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

import { spawn, type ChildProcessByStdio, type SpawnOptionsWithStdioTuple, type StdioPipe } from "node:child_process";
import readline from "node:readline";
import type { Readable, Writable } from "node:stream";
import { PROTOCOL_VERSION, type AgentCapabilities } from "@agentclientprotocol/sdk";
import { createACPProvider, type ACPProvider } from "@mcpc-tech/acp-ai-provider";
import type { AgentDriverConfig } from "./config.js";

export type { ACPProvider };

const probeTimeoutMs = 10_000;
const maximumConnectTimeoutMs = 60_000;

export type ACPChildProcess = ChildProcessByStdio<Writable, Readable, Readable>;

export interface SessionProvider {
  provider: ACPProvider;
  resume: "new" | "loaded" | "unsupported";
}

function processOptions(
  config: AgentDriverConfig,
  agentEnv: Record<string, string>,
): SpawnOptionsWithStdioTuple<StdioPipe, StdioPipe, StdioPipe> {
  return {
    cwd: config.cwd,
    env: { ...process.env, ...agentEnv },
    stdio: ["pipe", "pipe", "pipe"],
  };
}

export async function probeAgentCapabilities(
  config: AgentDriverConfig,
  agentEnv: Record<string, string>,
): Promise<AgentCapabilities> {
  const child: ACPChildProcess = spawn(
    config.command,
    config.args,
    processOptions(config, agentEnv),
  );
  const lines = readline.createInterface({ input: child.stdout });
  const stderr: string[] = [];
  child.stderr.on("data", (chunk: Buffer) => stderr.push(chunk.toString()));

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
    return await new Promise<AgentCapabilities>((resolve, reject) => {
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
        let message: Record<string, unknown>;
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
        const result = message.result as { agentCapabilities?: AgentCapabilities } | undefined;
        resolve(result?.agentCapabilities || {});
      });
      child.stdin.write(`${JSON.stringify(request)}\n`);
    });
  } finally {
    lines.close();
    child.kill("SIGTERM");
  }
}

export async function createSessionProvider(
  config: AgentDriverConfig,
  agentEnv: Record<string, string>,
): Promise<SessionProvider> {
  let existingSessionId: string | undefined;
  let resume: SessionProvider["resume"] = "new";
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
  let timer: NodeJS.Timeout | undefined;
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

export function spawnRawACP(
  config: AgentDriverConfig,
  agentEnv: Record<string, string>,
): ACPChildProcess {
  return spawn(config.command, config.args, processOptions(config, agentEnv));
}
