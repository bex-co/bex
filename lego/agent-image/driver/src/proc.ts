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

// Child-process helpers shared by the ACP launcher, repository setup and the
// session's Python selection.
import { spawn, type ChildProcess } from "node:child_process";

// killProcessTree kills a child spawned detached and everything it started:
// the group can outlive its leader and keep inherited pipes open.
export function killProcessTree(child: ChildProcess): void {
  if (process.platform !== "win32" && child.pid !== undefined) {
    try {
      process.kill(-child.pid, "SIGKILL");
      return;
    } catch {
      // A spawn failure has no process group; fall back to the child handle.
    }
  }
  if (child.exitCode === null && child.signalCode === null)
    child.kill("SIGKILL");
}

// appendTail keeps the last max bytes of a stream, copying out of a large
// chunk so the kept tail never pins a bigger buffer.
export function appendTail(tail: Buffer, chunk: Buffer, max: number): Buffer {
  if (chunk.length >= max)
    return Buffer.from(chunk.subarray(chunk.length - max));
  const joined = Buffer.concat([tail, chunk]);
  return joined.length > max
    ? Buffer.from(joined.subarray(joined.length - max))
    : joined;
}

// run resolves with a command's stdout, or rejects with a CommandError
// carrying its exit code and stderr. A spawn failure (a missing command)
// rejects with the system error, whose code is ENOENT.
export function run(
  command: string,
  args: string[],
  options: {
    env?: Record<string, string>;
    cwd?: string;
    timeoutMs?: number;
    signal?: AbortSignal;
  } = {},
): Promise<string> {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      cwd: options.cwd,
      env: options.env,
      stdio: ["ignore", "pipe", "pipe"],
      timeout: options.timeoutMs,
      signal: options.signal,
    });
    const out: Buffer[] = [];
    const err: Buffer[] = [];
    child.stdout.on("data", (chunk: Buffer) => out.push(chunk));
    child.stderr.on("data", (chunk: Buffer) => err.push(chunk));
    child.on("error", reject);
    child.on("close", (code, signal) => {
      if (code === 0) return resolve(Buffer.concat(out).toString("utf8"));
      const reason =
        Buffer.concat(err).toString("utf8").trim() ||
        (signal ? `killed by ${signal}` : `exited with code ${code}`);
      reject(new CommandError(code, reason));
    });
  });
}

export class CommandError extends Error {
  constructor(
    readonly exitCode: number | null,
    message: string,
  ) {
    super(message);
  }
}
