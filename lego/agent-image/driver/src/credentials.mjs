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

const maxScannedFileBytes = 32 * 1024 * 1024;
const scanChunkBytes = 64 * 1024;

function processCouldWrite(info) {
  const uid = process.getuid?.();
  if (uid === 0 || (uid !== undefined && info.uid === uid)) return true;
  const groups = new Set(process.getgroups?.() || []);
  if (groups.has(info.gid)) return (info.mode & 0o020) !== 0;
  return (info.mode & 0o002) !== 0;
}

function ignoreInaccessibleImmutablePath(target, info, error) {
  if (error.code !== "EACCES" && error.code !== "EPERM") return false;
  if (processCouldWrite(info)) {
    throw new Error(`cannot inspect agent-writable persisted path: ${target}`);
  }
  return true;
}

function replaceAll(data, needle, replacement) {
  const chunks = [];
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

async function largeFileContains(target, needle) {
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

async function scrubPath(target, needle, findings) {
  let info;
  try {
    info = await lstat(target);
  } catch (error) {
    if (error.code === "ENOENT") return;
    throw error;
  }
  if (info.isSymbolicLink()) return;
  if (info.isDirectory()) {
    let dir;
    try {
      dir = await opendir(target);
    } catch (error) {
      if (ignoreInaccessibleImmutablePath(target, info, error)) return;
      throw error;
    }
    for await (const entry of dir) {
      await scrubPath(`${target}/${entry.name}`, needle, findings);
    }
    return;
  }
  if (!info.isFile()) return;
  if (info.size > maxScannedFileBytes) {
    let containsCredential;
    try {
      containsCredential = await largeFileContains(target, needle);
    } catch (error) {
      if (ignoreInaccessibleImmutablePath(target, info, error)) return;
      throw error;
    }
    if (containsCredential) {
      throw new Error(
        `model credential found in oversized persisted file: ${target}`,
      );
    }
    return;
  }

  let data;
  try {
    data = await readFile(target);
  } catch (error) {
    if (ignoreInaccessibleImmutablePath(target, info, error)) return;
    throw error;
  }
  if (!data.includes(needle)) return;
  const scrubbed = replaceAll(data, needle, Buffer.from("[REDACTED]"));
  await writeFile(target, scrubbed, { mode: info.mode });
  findings.push(target);
}

export function createCredentialManager(config, env = process.env) {
  let secret = config.modelCredential || "";
  const credentialEnvName = config.credentialEnvName;

  // The generic injection variable is for the driver only. The child receives
  // the agent-native name; neither value belongs in the driver's inherited env.
  delete env.BEX_AGENT_MODEL_API_KEY;
  if (credentialEnvName) delete env[credentialEnvName];

  return {
    agentEnvironment() {
      if (!secret) return { ...config.agentEnv };
      return { ...config.agentEnv, [credentialEnvName]: secret };
    },

    async scrubPersistedState(roots = config.scrubRoots) {
      if (!secret) return [];
      const findings = [];
      const needle = Buffer.from(secret);
      for (const root of roots) await scrubPath(root, needle, findings);
      return findings;
    },

    forget() {
      secret = "";
    },

    redact(value) {
      const text = String(value);
      return secret ? text.split(secret).join("[REDACTED]") : text;
    },

    configured() {
      return secret !== "";
    },
  };
}
