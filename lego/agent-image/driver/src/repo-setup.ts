/*
 * Copyright 2026.
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *     http://www.apache.org/licenses/LICENSE-2.0
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// The session repository's environment (ADR109): setup steps the driver runs
// from the repository root before the agent's first turn in this sandbox, and
// checks the overlay asks the agent to run before it delivers.
import { spawn } from "node:child_process";
import type { UIMessageChunk } from "ai";
import { appendTail, killProcessTree } from "./proc.js";

export interface RepoStep {
  name: string;
  run: string;
}

export interface RepoEnvironment {
  setup: RepoStep[];
  checks: RepoStep[];
}

// AGENT_SESSION_SETUP_FAILED is the one failure code bex-api accepts from the
// driver's status file (ADR109 D5).
export const SETUP_FAILED_CODE = "AGENT_SESSION_SETUP_FAILED";

export const SETUP_STEP_TIMEOUT_MS = 10 * 60_000;
export const SETUP_TOTAL_TIMEOUT_MS = 20 * 60_000;
const OUTPUT_TAIL_BYTES = 8 << 10;
// A step that succeeded keeps a shorter tail: setup runs every turn, so its
// output is recorded every turn.
const SUCCESS_TAIL_BYTES = 2 << 10;
const HEARTBEAT_MS = 15_000;

export class RepoSetupError extends Error {
  readonly code = SETUP_FAILED_CODE;
}

function isStep(value: unknown): value is RepoStep {
  const step = value as RepoStep;
  return (
    typeof step === "object" &&
    step !== null &&
    typeof step.name === "string" &&
    typeof step.run === "string"
  );
}

// parseRepoEnvironment reads BEX_REPO_ENVIRONMENT. bex-api validated it; a
// value that does not parse is treated as no environment rather than failing
// a turn the agent could still do.
export function parseRepoEnvironment(
  raw: string | undefined,
): RepoEnvironment | undefined {
  if (!raw) return undefined;
  try {
    const parsed = JSON.parse(raw) as Partial<RepoEnvironment>;
    const setup = Array.isArray(parsed.setup)
      ? parsed.setup.filter(isStep)
      : [];
    const checks = Array.isArray(parsed.checks)
      ? parsed.checks.filter(isStep)
      : [];
    return setup.length || checks.length ? { setup, checks } : undefined;
  } catch {
    return undefined;
  }
}

export interface RepoSetupOptions {
  cwd: string;
  env: Record<string, string>;
  // write emits one UI-message chunk into the turn's stream, so setup steps
  // pass the same redaction and publication path as the agent's tool calls.
  write: (chunk: UIMessageChunk) => void;
  // heartbeat tells the turn's idle bound that setup is still working.
  heartbeat: () => void;
  abortSignal?: AbortSignal;
  stepTimeoutMs?: number;
  totalTimeoutMs?: number;
}

// runRepoSetup runs each setup step in order as a tool call titled
// `setup: <name>`. The first step that exits non-zero or runs past its limit
// throws RepoSetupError naming it; the agent never starts.
export async function runRepoSetup(
  environment: RepoEnvironment,
  options: RepoSetupOptions,
): Promise<void> {
  const stepTimeout = options.stepTimeoutMs ?? SETUP_STEP_TIMEOUT_MS;
  const deadline =
    Date.now() + (options.totalTimeoutMs ?? SETUP_TOTAL_TIMEOUT_MS);
  for (const [index, step] of environment.setup.entries()) {
    options.abortSignal?.throwIfAborted();
    const toolCallId = `bex-setup-${index + 1}`;
    const title = `setup: ${step.name}`;
    // The tool name is the title, as acp-map.ts names the agent's tool calls.
    const tool = { toolCallId, toolName: title, dynamic: true as const };
    options.write({ type: "tool-input-start", ...tool, title });
    options.write({
      type: "tool-input-available",
      ...tool,
      title,
      input: { step: step.name, command: step.run },
    });
    const remaining = deadline - Date.now();
    const timeoutMs = Math.min(stepTimeout, remaining);
    const result =
      remaining <= 0
        ? {
            failure: "did not start: setup ran past its total limit",
            durationMs: 0,
            output: "",
          }
        : await runStep(step.run, {
            cwd: options.cwd,
            env: options.env,
            heartbeat: options.heartbeat,
            abortSignal: options.abortSignal,
            timeoutMs,
            timeoutFailure:
              remaining < stepTimeout
                ? "ran past setup's total limit"
                : `ran past its ${minutes(timeoutMs)} limit`,
          });
    if (result.failure) {
      options.write({
        type: "tool-output-error",
        toolCallId,
        dynamic: true,
        errorText: [result.failure, result.output].filter(Boolean).join("\n"),
      });
      throw new RepoSetupError(`setup step "${step.name}" ${result.failure}`);
    }
    options.write({
      type: "tool-output-available",
      toolCallId,
      dynamic: true,
      output: [
        `exited 0 after ${(result.durationMs / 1000).toFixed(1)}s`,
        lastBytes(result.output, SUCCESS_TAIL_BYTES),
      ]
        .filter(Boolean)
        .join("\n"),
    });
  }
}

// lastBytes keeps the end of text within limit bytes, starting at a
// character boundary.
function lastBytes(text: string, limit: number): string {
  const bytes = Buffer.from(text, "utf8");
  if (bytes.length <= limit) return text;
  return bytes
    .subarray(bytes.length - limit)
    .toString("utf8")
    .replace(/^\uFFFD+/, "");
}

function minutes(ms: number): string {
  const m = Math.round(ms / 60_000);
  return m >= 1
    ? `${m} minute${m === 1 ? "" : "s"}`
    : `${Math.round(ms / 1000)} seconds`;
}

interface StepResult {
  failure?: string;
  durationMs: number;
  output: string;
}

// After the step's shell exits, its stdio may still be held by something it
// backgrounded (a service started for the agent); wait this long for the last
// output, then move on and leave that process running.
const STDIO_GRACE_MS = 500;

function runStep(
  command: string,
  options: {
    cwd: string;
    env: Record<string, string>;
    heartbeat: () => void;
    abortSignal?: AbortSignal;
    timeoutMs: number;
    timeoutFailure: string;
  },
): Promise<StepResult> {
  const started = Date.now();
  return new Promise((resolve) => {
    // Its own process group, so a timeout kills everything the step started.
    const child = spawn("bash", ["-c", command], {
      cwd: options.cwd,
      env: options.env,
      stdio: ["ignore", "pipe", "pipe"],
      detached: true,
    });
    let tail: Buffer = Buffer.alloc(0);
    const keep = (data: Buffer) => {
      tail = appendTail(tail, data, OUTPUT_TAIL_BYTES);
      options.heartbeat();
    };
    child.stdout.on("data", keep);
    child.stderr.on("data", keep);
    let failure: string | undefined;
    const kill = (reason: string) => {
      failure ??= reason;
      killProcessTree(child);
    };
    const heartbeat = setInterval(options.heartbeat, HEARTBEAT_MS);
    const timer = setTimeout(
      () => kill(options.timeoutFailure),
      options.timeoutMs,
    );
    const onAbort = () => kill("was stopped: the turn ended");
    options.abortSignal?.addEventListener("abort", onAbort, { once: true });
    let settled = false;
    let grace: NodeJS.Timeout | undefined;
    const settle = (exitCode: number | null, signal: NodeJS.Signals | null) => {
      if (settled) return;
      settled = true;
      clearInterval(heartbeat);
      clearTimeout(timer);
      clearTimeout(grace);
      options.abortSignal?.removeEventListener("abort", onAbort);
      child.stdout.destroy();
      child.stderr.destroy();
      if (!failure && exitCode !== 0)
        failure =
          exitCode === null
            ? `was killed by ${signal ?? "a signal"}`
            : `exited with code ${exitCode}`;
      resolve({
        failure,
        durationMs: Date.now() - started,
        output: tail.toString("utf8").replace(/^\uFFFD+/, ""),
      });
    };
    // A spawn failure emits error and then close; only close settles.
    child.on("error", (error) => {
      failure ??= `could not start: ${error.message}`;
    });
    child.on("exit", (code, signal) => {
      grace = setTimeout(() => settle(code, signal), STDIO_GRACE_MS);
    });
    child.on("close", settle);
  });
}
