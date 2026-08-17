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

import { spawn, type ChildProcessByStdio } from "node:child_process";
import { Readable, Writable } from "node:stream";
import {
  ClientSideConnection,
  ndJsonStream,
  PROTOCOL_VERSION,
  type Client,
  type PromptResponse,
  type RequestPermissionRequest,
  type RequestPermissionResponse,
  type SessionNotification,
  type SessionUpdate,
} from "@agentclientprotocol/sdk";
import type { AgentDriverConfig } from "./config.js";

const maximumConnectTimeoutMs = 60_000;

type ACPChildProcess = ChildProcessByStdio<Writable, Readable, Readable>;

// SessionProvider is the driver's handle on one ACP turn: it owns the agent
// child process + JSON-RPC connection, and exposes exactly the verbs the turn
// runner needs. Replaces the old fake-LanguageModel provider — the driver now
// speaks ACP directly through the official SDK.
export interface SessionProvider {
  sessionId: string;
  resume: "new" | "loaded" | "unsupported";
  prompt(text: string): Promise<PromptResponse>;
  cancel(): Promise<void>;
  cleanup(): Promise<void>;
}

export interface CreateSessionOptions {
  // onUpdate receives every ACP session/update for mapping into UI-message
  // chunks. It runs on the connection's read path, so it must not throw.
  onUpdate: (update: SessionUpdate) => void;
  // abortSignal, when aborted (turn timeout), SIGKILLs the child even if we are
  // still mid-connect — so a hung `initialize` cannot leak the process.
  abortSignal?: AbortSignal;
}

function withTimeout<T>(
  promise: Promise<T>,
  ms: number,
  message: string,
): Promise<T> {
  let timer: NodeJS.Timeout | undefined;
  const timeout = new Promise<never>((_, reject) => {
    timer = setTimeout(() => reject(new Error(message)), ms);
    timer.unref();
  });
  return Promise.race([promise, timeout]).finally(() => clearTimeout(timer));
}

// autoApprove selects a permission option for a headless run. The sandbox is the
// security boundary (ADR047 D5 egress + fs confinement), so the driver grants
// the requested action rather than blocking a fire-and-forget turn on a prompt.
function autoApprove(
  request: RequestPermissionRequest,
): RequestPermissionResponse {
  const option =
    request.options.find((candidate) => candidate.kind === "allow_always") ??
    request.options.find((candidate) => candidate.kind === "allow_once") ??
    request.options.find((candidate) => candidate.kind.startsWith("allow")) ??
    request.options[0];
  if (!option) return { outcome: { outcome: "cancelled" } };
  return { outcome: { outcome: "selected", optionId: option.optionId } };
}

export async function createSessionProvider(
  config: AgentDriverConfig,
  agentEnv: Record<string, string>,
  options: CreateSessionOptions,
): Promise<SessionProvider> {
  const child: ACPChildProcess = spawn(config.command, config.args, {
    cwd: config.cwd,
    env: { ...process.env, ...agentEnv },
    stdio: ["pipe", "pipe", "pipe"],
  });
  const maxStderrBytes = 64 << 10;
  let stderr = Buffer.alloc(0);
  child.stderr.on("data", (chunk: Buffer) => {
    stderr = Buffer.concat([stderr, chunk]);
    if (stderr.length > maxStderrBytes) {
      stderr = stderr.subarray(stderr.length - maxStderrBytes);
    }
  });
  const stopped = new Promise<void>((resolve) => {
    if (child.exitCode !== null) resolve();
    else child.once("exit", () => resolve());
  });
  const cleanup = async () => {
    if (child.exitCode === null) child.kill("SIGKILL");
    await stopped;
  };
  // Guarantee the child dies on turn timeout even if we never returned from
  // connect below (a hung initialize).
  options.abortSignal?.addEventListener("abort", () => void cleanup(), {
    once: true,
  });

  const client: Client = {
    async sessionUpdate(params: SessionNotification): Promise<void> {
      options.onUpdate(params.update);
    },
    async requestPermission(
      params: RequestPermissionRequest,
    ): Promise<RequestPermissionResponse> {
      return autoApprove(params);
    },
  };

  const stream = ndJsonStream(
    Writable.toWeb(child.stdin) as WritableStream<Uint8Array>,
    Readable.toWeb(child.stdout) as ReadableStream<Uint8Array>,
  );
  const connection = new ClientSideConnection(() => client, stream);

  // If the child dies during setup, surface a diagnostic instead of a generic
  // stream-closed error.
  const exited = new Promise<never>((_, reject) => {
    child.once("exit", (code, signal) =>
      reject(
        new Error(
          `ACP agent exited (code=${code}, signal=${signal}): ${stderr.toString().trim()}`,
        ),
      ),
    );
    child.once("error", reject);
  });
  exited.catch(() => {}); // avoid an unhandled rejection when the turn ends first

  const connectTimeoutMs = Math.min(
    config.turnTimeoutMs,
    maximumConnectTimeoutMs,
  );
  try {
    const initialize = await withTimeout(
      connection.initialize({
        protocolVersion: PROTOCOL_VERSION,
        clientCapabilities: {
          fs: { readTextFile: false, writeTextFile: false },
          terminal: false,
        },
        clientInfo: { name: "bex-agent-driver", version: "0.1.0" },
      }),
      connectTimeoutMs,
      `ACP connect timed out after ${connectTimeoutMs}ms`,
    );

    let sessionId: string;
    let resume: SessionProvider["resume"] = "new";
    const loadSupported = initialize.agentCapabilities?.loadSession === true;
    if (config.existingSessionId && loadSupported) {
      await connection.loadSession({
        sessionId: config.existingSessionId,
        cwd: config.cwd,
        mcpServers: [],
      });
      sessionId = config.existingSessionId;
      resume = "loaded";
    } else {
      if (config.existingSessionId) resume = "unsupported";
      const created = await connection.newSession({
        cwd: config.cwd,
        mcpServers: [],
      });
      sessionId = created.sessionId;
    }

    return {
      sessionId,
      resume,
      prompt(text: string) {
        return Promise.race([
          connection.prompt({ sessionId, prompt: [{ type: "text", text }] }),
          exited,
        ]);
      },
      cancel() {
        return connection.cancel({ sessionId }).catch(() => {});
      },
      cleanup,
    };
  } catch (error) {
    await cleanup();
    throw error;
  }
}
