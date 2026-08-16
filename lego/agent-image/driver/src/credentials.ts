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

import { lstat, open, opendir, readFile, writeFile } from "node:fs/promises";
import type { Stats } from "node:fs";
import type { AgentDriverConfig } from "./config.js";

const maxScannedFileBytes = 32 * 1024 * 1024;
const scanChunkBytes = 64 * 1024;

export interface CredentialManager {
  agentEnvironment(): Record<string, string>;
  scrubPersistedState(roots?: string[]): Promise<string[]>;
  containsSecret(value: string): boolean;
  forget(): void;
  redact(value: unknown): string;
  redactPart<T>(value: T): T;
  configured(): boolean;
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

async function largeFileContains(target: string, needle: Buffer): Promise<boolean> {
  const file = await open(target, "r");
  const buffer = Buffer.allocUnsafe(scanChunkBytes);
  let tail = Buffer.alloc(0);
  try {
    for (;;) {
      const { bytesRead } = await file.read(buffer, 0, buffer.length, null);
      if (bytesRead === 0) return false;
      const window = Buffer.concat([tail, buffer.subarray(0, bytesRead)]);
      if (window.includes(needle)) return true;
      const overlap = Math.min(needle.length - 1, window.length);
      tail = Buffer.from(window.subarray(window.length - overlap));
    }
  } finally {
    await file.close();
  }
}

async function scrubPath(
  target: string,
  needle: Buffer,
  findings: string[],
): Promise<void> {
  let info: Stats;
  try {
    info = await lstat(target);
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") return;
    throw error;
  }
  if (info.isSymbolicLink()) return;
  if (info.isDirectory()) {
    let dir;
    try {
      dir = await opendir(target);
    } catch (error) {
      if (ignoreInaccessibleImmutablePath(target, info, error as NodeJS.ErrnoException)) return;
      throw error;
    }
    for await (const entry of dir) {
      await scrubPath(`${target}/${entry.name}`, needle, findings);
    }
    return;
  }
  if (!info.isFile()) return;
  if (info.size > maxScannedFileBytes) {
    let containsCredential: boolean;
    try {
      containsCredential = await largeFileContains(target, needle);
    } catch (error) {
      if (ignoreInaccessibleImmutablePath(target, info, error as NodeJS.ErrnoException)) return;
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
  } catch (error) {
    if (ignoreInaccessibleImmutablePath(target, info, error as NodeJS.ErrnoException)) return;
    throw error;
  }
  if (!data.includes(needle)) return;
  const scrubbed = replaceAll(data, needle, Buffer.from("[REDACTED]"));
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
  const credentialEnvName = config.credentialEnvName;

  // The generic injection variable is for the driver only. The child receives
  // the agent-native name; neither value belongs in the driver's inherited env.
  delete env.BEX_AGENT_MODEL_API_KEY;
  if (credentialEnvName) delete env[credentialEnvName];

  // scrub is the one string-level redactor behind redact and redactPart, so
  // escaped-form handling can never live in only one of them.
  const scrub = (text: string): string => {
    let out = text.split(secret).join("[REDACTED]");
    if (escapedSecret !== secret) out = out.split(escapedSecret).join("[REDACTED]");
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
      return { ...config.agentEnv, ...modelProxyEnv, [credentialEnvName]: secret };
    },

    async scrubPersistedState(roots = config.scrubRoots) {
      if (!secret) return [];
      const findings: string[] = [];
      const needle = Buffer.from(secret);
      for (const root of roots) await scrubPath(root, needle, findings);
      return findings;
    },

    // containsSecret reports whether text carries the raw model credential (or
    // its JSON-escaped rendering). It is the pre-push fail-closed check's needle
    // (round-5 finding 6) — matching what redactPart scrubs — without exposing
    // the closed-over secret to the caller.
    containsSecret(value: string) {
      if (!secret) return false;
      return value.includes(secret) || (escapedSecret !== secret && value.includes(escapedSecret));
    },

    forget() {
      secret = "";
      escapedSecret = "";
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
