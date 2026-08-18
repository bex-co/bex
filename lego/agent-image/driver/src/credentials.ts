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
import { lstat, open, opendir, readFile, writeFile } from "node:fs/promises";
import type { Stats } from "node:fs";
import path from "node:path";
import type { AgentDriverConfig } from "./config.js";

const maxScannedFileBytes = 32 * 1024 * 1024;
const scanChunkBytes = 64 * 1024;
const defaultMaxScannedFiles = 100_000;
const defaultMaxScanDepth = 64;
const defaultScanTimeoutMs = 120_000;

export interface ScrubOptions {
  signal?: AbortSignal;
  maxFiles?: number;
  maxEntries?: number;
  maxBytes?: number;
  maxDepth?: number;
  timeoutMs?: number;
}

export interface CredentialManager {
  agentEnvironment(): Record<string, string>;
  scrubPersistedState(
    roots?: string[],
    options?: ScrubOptions,
  ): Promise<string[]>;
  containsSecret(value: string): boolean;
  // secretNeedles returns every byte form the credential can plausibly be
  // re-encoded into — the raw and JSON-escaped renderings plus the common
  // reversible encodings (base64 std/url, padded and not, hex both cases).
  // Delivery's object-level scan (codex round-9 #1) searches every newly
  // pushed git blob for each, so a base64- or hex-encoded credential, or one
  // hiding in a binary blob that `git log -p` omits, still fails the push.
  secretNeedles(): Buffer[];
  forget(): void;
  redact(value: unknown): string;
  redactPart<T>(value: T): T;
  configured(): boolean;
}

// minEncodedNeedleBytes gates the reversible-encoding needles: encoding a very
// short string produces a short needle that can appear in unrelated base64 or
// hex content by chance. Real model credentials are long API keys; below this
// length only the literal renderings are matched.
const minEncodedNeedleBytes = 16;

function encodedForms(value: string): string[] {
  if (!value) return [];
  const forms: string[] = [];
  const raw = Buffer.from(value, "utf8");
  if (raw.length >= minEncodedNeedleBytes) {
    const b64 = raw.toString("base64");
    const b64url = raw.toString("base64url");
    forms.push(
      b64,
      b64.replace(/=+$/, ""),
      b64url,
      b64url.replace(/=+$/, ""),
      raw.toString("hex"),
      raw.toString("hex").toUpperCase(),
    );
  }
  return forms;
}

function processCouldWrite(info: Stats): boolean {
  const uid = process.getuid?.();
  if (uid === 0 || (uid !== undefined && info.uid === uid)) return true;
  const groups = new Set(process.getgroups?.() || []);
  if (groups.has(info.gid)) return (info.mode & 0o020) !== 0;
  return (info.mode & 0o002) !== 0;
}

function ignoreInaccessibleImmutablePath(
  target: string,
  info: Stats,
  error: NodeJS.ErrnoException,
): boolean {
  if (error.code !== "EACCES" && error.code !== "EPERM") return false;
  if (processCouldWrite(info)) {
    throw new Error(`cannot inspect agent-writable persisted path: ${target}`);
  }
  return true;
}

function replaceAll(data: Buffer, needle: Buffer, replacement: Buffer): Buffer {
  const chunks: Buffer[] = [];
  let cursor = 0;
  for (;;) {
    const index = data.indexOf(needle, cursor);
    if (index === -1) break;
    chunks.push(data.subarray(cursor, index), replacement);
    cursor = index + needle.length;
  }
  chunks.push(data.subarray(cursor));
  return Buffer.concat(chunks);
}

interface ScrubBudget {
  files: number;
  entries: number;
  bytes: number;
  deadline: number;
  maxFiles: number;
  maxEntries: number;
  maxBytes: number;
  maxDepth: number;
  overlap: number;
  signal?: AbortSignal;
  seen: Set<string>;
}

function checkBudget(budget: ScrubBudget, depth: number): void {
  if (budget.signal?.aborted)
    throw new Error("persisted credential scan aborted");
  if (Date.now() > budget.deadline)
    throw new Error("persisted credential scan timed out");
  if (depth > budget.maxDepth)
    throw new Error("persisted credential scan exceeded depth limit");
}

function accountBytes(budget: ScrubBudget, bytes: number): void {
  budget.bytes += bytes;
  if (budget.bytes > budget.maxBytes) {
    throw new Error("persisted credential scan exceeded byte limit");
  }
}

function scanWindow(
  chunk: Buffer,
  tail: Buffer,
  needles: Buffer[],
  overlap: number,
): { found: boolean; tail: Buffer } {
  const window = Buffer.concat([tail, chunk]);
  return {
    found: needles.some((needle) => window.includes(needle)),
    tail: Buffer.from(
      window.subarray(window.length - Math.min(overlap, window.length)),
    ),
  };
}

async function largeFileContains(
  target: string,
  needles: Buffer[],
  budget: ScrubBudget,
  depth: number,
): Promise<boolean> {
  const file = await open(target, "r");
  const buffer = Buffer.allocUnsafe(scanChunkBytes);
  let tail: Buffer<ArrayBufferLike> = Buffer.alloc(0);
  try {
    for (;;) {
      checkBudget(budget, depth);
      const { bytesRead } = await file.read(buffer, 0, buffer.length, null);
      if (bytesRead === 0) return false;
      accountBytes(budget, bytesRead);
      const result = scanWindow(
        buffer.subarray(0, bytesRead),
        tail,
        needles,
        budget.overlap,
      );
      if (result.found) return true;
      tail = result.tail;
    }
  } finally {
    await file.close();
  }
}

// Git's loose and packed objects are compressed, so scanning .git/objects as
// ordinary files cannot prove that a credential is absent. Ask Git to stream
// every object (reachable and unreachable) in decompressed form and inspect the
// stream with fixed memory. An object cannot be byte-rewritten without changing
// repository identity, so a match fails the turn closed before delivery.
async function scanGitObjects(
  gitDir: string,
  needles: Buffer[],
  budget: ScrubBudget,
  depth: number,
): Promise<void> {
  checkBudget(budget, depth);
  budget.files += 1;
  if (budget.files > budget.maxFiles) {
    throw new Error("persisted credential scan exceeded file limit");
  }
  await new Promise<void>((resolve, reject) => {
    const child = spawn(
      "git",
      [
        "--git-dir",
        gitDir,
        "cat-file",
        "--batch-all-objects",
        "--unordered",
        "--batch",
      ],
      { stdio: ["ignore", "pipe", "ignore"] },
    );
    let tail: Buffer<ArrayBufferLike> = Buffer.alloc(0);
    let settled = false;
    const finish = (error?: Error) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      budget.signal?.removeEventListener("abort", abort);
      if (error) reject(error);
      else resolve();
    };
    const stop = (error: Error) => {
      child.kill("SIGKILL");
      finish(error);
    };
    const abort = () => stop(new Error("persisted credential scan aborted"));
    const timer = setTimeout(
      () => stop(new Error("persisted credential scan timed out")),
      Math.max(1, budget.deadline - Date.now()),
    );
    budget.signal?.addEventListener("abort", abort, { once: true });
    child.once("error", () =>
      finish(new Error(`cannot inspect Git object database: ${gitDir}`)),
    );
    child.stdout.on("data", (raw: Buffer) => {
      if (settled) return;
      try {
        checkBudget(budget, depth);
        accountBytes(budget, raw.length);
        const result = scanWindow(raw, tail, needles, budget.overlap);
        if (result.found) {
          stop(
            new Error(
              `model credential found in persisted Git object database: ${gitDir}`,
            ),
          );
          return;
        }
        tail = result.tail;
      } catch (error) {
        stop(error as Error);
      }
    });
    child.once("close", (code) => {
      if (settled) return;
      if (code !== 0) {
        finish(new Error(`cannot inspect Git object database: ${gitDir}`));
        return;
      }
      finish();
    });
  });
}

async function scrubPath(
  target: string,
  needles: Buffer[],
  findings: string[],
  budget: ScrubBudget,
  depth: number,
): Promise<void> {
  checkBudget(budget, depth);
  let info: Stats;
  try {
    info = await lstat(target);
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") return;
    throw error;
  }
  if (info.isSymbolicLink()) return;
  const inode = `${info.dev}:${info.ino}`;
  if (budget.seen.has(inode)) return;
  budget.seen.add(inode);
  budget.entries += 1;
  if (budget.entries > budget.maxEntries) {
    throw new Error("persisted credential scan exceeded entry limit");
  }
  if (info.isDirectory()) {
    let dir;
    try {
      dir = await opendir(target);
    } catch (error) {
      if (
        ignoreInaccessibleImmutablePath(
          target,
          info,
          error as NodeJS.ErrnoException,
        )
      )
        return;
      throw error;
    }
    const isGitDir = path.basename(target) === ".git";
    if (isGitDir) await scanGitObjects(target, needles, budget, depth + 1);
    for await (const entry of dir) {
      checkBudget(budget, depth);
      // scanGitObjects inspected the decompressed contents. Reading packfiles a
      // second time is both expensive and ineffective against compressed data.
      if (isGitDir && entry.name === "objects") continue;
      await scrubPath(
        path.join(target, entry.name),
        needles,
        findings,
        budget,
        depth + 1,
      );
    }
    return;
  }
  if (!info.isFile()) return;
  budget.files += 1;
  if (budget.files > budget.maxFiles) {
    throw new Error("persisted credential scan exceeded file limit");
  }
  if (info.size > maxScannedFileBytes) {
    let containsCredential: boolean;
    try {
      containsCredential = await largeFileContains(
        target,
        needles,
        budget,
        depth,
      );
    } catch (error) {
      if (
        ignoreInaccessibleImmutablePath(
          target,
          info,
          error as NodeJS.ErrnoException,
        )
      )
        return;
      throw error;
    }
    if (containsCredential) {
      throw new Error(
        `model credential found in oversized persisted file: ${target}`,
      );
    }
    return;
  }

  let data: Buffer;
  try {
    data = await readFile(target);
    accountBytes(budget, data.length);
  } catch (error) {
    if (
      ignoreInaccessibleImmutablePath(
        target,
        info,
        error as NodeJS.ErrnoException,
      )
    )
      return;
    throw error;
  }
  if (!needles.some((needle) => data.includes(needle))) return;
  let scrubbed = data;
  for (const needle of needles) {
    scrubbed = replaceAll(scrubbed, needle, Buffer.from("[REDACTED]"));
  }
  await writeFile(target, scrubbed, { mode: info.mode });
  findings.push(target);
}

export function createCredentialManager(
  config: AgentDriverConfig,
  env: NodeJS.ProcessEnv = process.env,
): CredentialManager {
  let secret = config.modelCredential || "";
  // The JSON-escaped rendering of the secret, precomputed once (it only
  // changes on forget): a credential containing `"` or `\` appears escaped
  // inside serialized parts, so both forms must be scrubbed everywhere.
  let escapedSecret = secret ? JSON.stringify(secret).slice(1, -1) : "";
  // Every string form of the credential: the literal renderings plus the
  // reversible encodings (codex round-9 #1) — an agent that cannot print the
  // raw key can still print base64(key) or hex(key), and a byte-scan of git
  // objects must catch those as reliably as the literals. Recomputed on forget.
  let secretForms: string[] = [];
  const resetForms = () => {
    secretForms = secret ? [secret, ...encodedForms(secret)] : [];
    if (secret && escapedSecret !== secret) secretForms.push(escapedSecret);
  };
  resetForms();
  const credentialEnvName = config.credentialEnvName;

  // The generic injection variable is for the driver only. The child receives
  // the agent-native name; neither value belongs in the driver's inherited env.
  delete env.BEX_AGENT_MODEL_API_KEY;
  if (credentialEnvName) delete env[credentialEnvName];

  // scrub is the one string-level redactor behind redact and redactPart, so
  // escaped- and encoded-form handling can never live in only one of them.
  const scrub = (text: string): string => {
    let out = text;
    for (const form of secretForms) out = out.split(form).join("[REDACTED]");
    return out;
  };

  // ADR062: when the model proxy is active, the agent is pointed at the gateway
  // proxy via its provider base-URL env; the credential the agent carries is only
  // the placeholder (config.modelCredential), which the proxy strips and replaces
  // with the real key on the vendor hop.
  const modelProxyEnv: Record<string, string> =
    config.modelBaseUrl && config.modelBaseUrlEnvName
      ? { [config.modelBaseUrlEnvName]: config.modelBaseUrl }
      : {};

  return {
    agentEnvironment() {
      if (!secret) return { ...config.agentEnv, ...modelProxyEnv };
      return {
        ...config.agentEnv,
        ...modelProxyEnv,
        [credentialEnvName]: secret,
      };
    },

    async scrubPersistedState(roots = config.scrubRoots, options = {}) {
      if (!secret) return [];
      const findings: string[] = [];
      const needles = secretForms.map((form) => Buffer.from(form, "utf8"));
      const budget: ScrubBudget = {
        files: 0,
        entries: 0,
        bytes: 0,
        deadline: Date.now() + (options.timeoutMs ?? defaultScanTimeoutMs),
        maxFiles: options.maxFiles ?? defaultMaxScannedFiles,
        // Directory inodes count separately from files so a very broad empty
        // tree cannot grow the deduplication set without an explicit bound.
        maxEntries: options.maxEntries ?? 2 * defaultMaxScannedFiles,
        // Runtime is bounded by timeout/file/depth controls. There is no default
        // aggregate-byte rejection: that rejected legitimate large repositories
        // even though the scanner's memory stays fixed. An explicit byte budget
        // remains available for narrower callers.
        maxBytes: options.maxBytes ?? Number.POSITIVE_INFINITY,
        maxDepth: options.maxDepth ?? defaultMaxScanDepth,
        overlap: needles.reduce(
          (max, needle) => Math.max(max, needle.length - 1),
          0,
        ),
        signal: options.signal,
        seen: new Set<string>(),
      };
      for (const root of roots) {
        await scrubPath(root, needles, findings, budget, 0);
      }
      return findings;
    },

    // containsSecret reports whether text carries any form of the model
    // credential: the raw and JSON-escaped renderings plus the reversible
    // encodings (codex round-9 #1). It is the pre-push fail-closed check's
    // needle set (round-5 finding 6) — matching what redactPart scrubs —
    // without exposing the closed-over secret to the caller.
    containsSecret(value: string) {
      if (!secret) return false;
      return secretForms.some((form) => value.includes(form));
    },

    secretNeedles() {
      return secretForms.map((form) => Buffer.from(form, "utf8"));
    },

    forget() {
      secret = "";
      escapedSecret = "";
      resetForms();
    },

    redact(value: unknown) {
      const text = String(value);
      return secret ? scrub(text) : text;
    },

    // redactPart sanitizes a structured stream part BEFORE its first fan-out,
    // so the hub history, attached SSE clients, the POST /turn mirror (the
    // gateway's durable transcript tee), and the session log all carry one
    // sanitized representation. Matches on the serialized JSON, covering the
    // secret nested at any depth and its JSON-escaped rendering; fails closed
    // if the substitution would corrupt the document.
    redactPart<T>(value: T): T {
      if (!secret) return value;
      const text = JSON.stringify(value);
      if (text === undefined) return value;
      const sanitized = scrub(text);
      if (sanitized === text) return value;
      try {
        return JSON.parse(sanitized) as T;
      } catch {
        return { type: "data-redacted" } as unknown as T;
      }
    },

    configured() {
      return secret !== "";
    },
  };
}
