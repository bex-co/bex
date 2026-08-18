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
  // Child stderr can contain the injected model credential. Sanitize it while
  // the credential manager still holds its needles; the caller deliberately
  // forgets them after persisting a terminal verdict.
  redact: (value: unknown) => string;
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
    // ACP adapters may launch their provider CLI as a child. A distinct Unix
    // process group lets cleanup terminate the whole tree so descendants cannot
    // retain the model placeholder or keep stdio open after the adapter dies.
    detached: process.platform !== "win32",
  });
  const maxStderrBytes = 64 << 10;
  let stderr = Buffer.alloc(0);
  child.stderr.on("data", (chunk: Buffer) => {
    stderr = Buffer.concat([stderr, chunk]);
    if (stderr.length > maxStderrBytes) {
      stderr = stderr.subarray(stderr.length - maxStderrBytes);
    }
  });

  // Install one terminal signal before constructing the protocol stream and
  // race every ACP operation against it. The SDK's write path can log an EPIPE
  // without rejecting the pending JSON-RPC request; the old lifecycle promise
  // did not watch stdio, and only prompt raced child exit. Either case could
  // leave status at `running` until the four-hour turn deadline.
  let terminalSettled = false;
  let rejectTerminal!: (error: Error) => void;
  const terminal = new Promise<never>((_, reject) => {
    rejectTerminal = reject;
  });
  terminal.catch(() => {}); // cleanup also closes stdio after a successful turn

  const failTerminal = (error: Error) => {
    if (terminalSettled) return;
    terminalSettled = true;
    rejectTerminal(error);
  };
  const exitError = (code: number | null, signal: NodeJS.Signals | null) => {
    const detail = options.redact(stderr.toString().trim());
    return new Error(
      `ACP agent exited (code=${code}, signal=${signal})${detail ? `: ${detail}` : ""}`,
    );
  };
  const onExit = (code: number | null, signal: NodeJS.Signals | null) =>
    failTerminal(exitError(code, signal));
  const onStreamError = (stream: "stdin" | "stdout", error: Error) => {
    const failure = new Error(`ACP agent ${stream} failed: ${error.message}`);
    failTerminal(failure);
    return failure;
  };

  child.once("exit", onExit);
  child.once("error", failTerminal);
  child.stdin.once("error", (error) => onStreamError("stdin", error));
  child.stdout.once("error", (error) => onStreamError("stdout", error));
  // Cover an exit observed between spawn() and listener installation.
  if (child.exitCode !== null || child.signalCode !== null) {
    onExit(child.exitCode, child.signalCode);
  }

  const stopped = new Promise<void>((resolve) => {
    if (child.exitCode !== null || child.signalCode !== null) resolve();
    else child.once("close", () => resolve());
  });
  const killProcessTree = () => {
    if (child.exitCode !== null || child.signalCode !== null) return;
    if (process.platform !== "win32" && child.pid !== undefined) {
      try {
        process.kill(-child.pid, "SIGKILL");
        return;
      } catch {
        // A spawn failure has no process group; fall back to the child handle.
      }
    }
    child.kill("SIGKILL");
  };
  const cleanup = async () => {
    killProcessTree();
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

  // Writable.toWeb logs some pipe-write failures without propagating them to
  // the ChildProcess stream's `error` event. Own each write callback so an
  // EPIPE rejects the shared terminal signal even when the ACP SDK leaves its
  // JSON-RPC request pending.
  const protocolInput = new WritableStream<Uint8Array>({
    write(chunk) {
      return new Promise<void>((resolve, reject) => {
        try {
          child.stdin.write(chunk, (error) => {
            if (error) {
              reject(onStreamError("stdin", error));
              return;
            }
            resolve();
          });
        } catch (error) {
          reject(
            onStreamError(
              "stdin",
              error instanceof Error ? error : new Error(String(error)),
            ),
          );
        }
      });
    },
    close() {
      child.stdin.end();
    },
    abort(reason) {
      child.stdin.destroy(reason instanceof Error ? reason : undefined);
    },
  });
  const stream = ndJsonStream(
    protocolInput,
    Readable.toWeb(child.stdout) as ReadableStream<Uint8Array>,
  );
  const connection = new ClientSideConnection(() => client, stream);

  const connectTimeoutMs = Math.min(
    config.turnTimeoutMs,
    maximumConnectTimeoutMs,
  );
  const setupCall = <T>(
    operation: Promise<T>,
    phase: "connect" | "session load" | "session create",
  ) =>
    withTimeout(
      Promise.race([operation, terminal]),
      connectTimeoutMs,
      `ACP ${phase} timed out after ${connectTimeoutMs}ms`,
    );
  try {
    const initialize = await setupCall(
      connection.initialize({
        protocolVersion: PROTOCOL_VERSION,
        clientCapabilities: {
          fs: { readTextFile: false, writeTextFile: false },
          terminal: false,
        },
        clientInfo: { name: "bex-agent-driver", version: "0.1.0" },
      }),
      "connect",
    );

    let sessionId: string;
    let resume: SessionProvider["resume"] = "new";
    const loadSupported = initialize.agentCapabilities?.loadSession === true;
    if (config.existingSessionId && loadSupported) {
      await setupCall(
        connection.loadSession({
          sessionId: config.existingSessionId,
          cwd: config.cwd,
          mcpServers: [],
        }),
        "session load",
      );
      sessionId = config.existingSessionId;
      resume = "loaded";
    } else {
      if (config.existingSessionId) resume = "unsupported";
      const created = await setupCall(
        connection.newSession({
          cwd: config.cwd,
          mcpServers: [],
        }),
        "session create",
      );
      sessionId = created.sessionId;
    }

    return {
      sessionId,
      resume,
      prompt(text: string) {
        return Promise.race([
          connection.prompt({ sessionId, prompt: [{ type: "text", text }] }),
          terminal,
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
