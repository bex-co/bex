/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// share_file (ADR095 D6): the agent hands the user a file that should not be
// committed — a report, a CSV, a screenshot. The file is posted to the
// session's Pod-bound share URL; the gateway authenticates the source Pod and
// bex-api stores it as an output attachment of the current turn. No secret is
// involved.

import { createHash } from "node:crypto";
import { promises as fs } from "node:fs";
import path from "node:path";
import { describeError } from "./errors.js";
import { postToGateway, type ToolError } from "./gateway.js";

export const shareFileToolName = "share_file";
// The gateway's MaxShareFileBytes / the attachment cap (ADR095 D8).
export const maxShareFileBytes = 25 * 1024 * 1024;
// The gateway's MaxShareFileNameBytes: one value on both sides of the hop.
export const maxShareFileNameBytes = 255;
const requestTimeoutMs = 5 * 60_000;

// The directories a shared file may live in, resolved through symlinks:
// /attachments links to the extracted input copy. This is agent ergonomics —
// it keeps the agent from sharing the image's own files by mistake — not a
// security boundary: the agent controls the sandbox and could copy anything
// into /tmp first. What the platform enforces is server-side: the Pod, the
// session, the size and the quota.
export const defaultShareRoots = ["/workspace", "/tmp", "/attachments"];

export const shareFileToolDefinition = {
  name: shareFileToolName,
  title: "Share a file with the user",
  description:
    "Hand the user a file that should not be committed to the repository: a report, a CSV, " +
    "a screenshot, a generated image or archive. The user sees it in the conversation and can download it. " +
    `The file must be a regular file under /workspace, /tmp or /attachments, at most ${maxShareFileBytes >> 20} MiB. ` +
    "Code changes belong in commits and the pull request, not here. " +
    "If it returns an error, report that error to the user.",
  inputSchema: {
    type: "object",
    properties: {
      path: {
        type: "string",
        description:
          "Path of the file to share, absolute or relative to the working directory.",
      },
      name: {
        type: "string",
        description:
          "File name the user sees; defaults to the file's own name.",
      },
    },
    required: ["path"],
    additionalProperties: false,
  },
  annotations: { openWorldHint: true },
};

export interface SharedFile {
  attachmentId: string;
  name: string;
  mediaType: string;
  size: number;
}

export type ShareFileOutcome =
  | { ok: true; result: SharedFile }
  | { ok: false; error: ToolError };

function refuse(
  code: string,
  message: string,
  hint?: string,
): ShareFileOutcome {
  return { ok: false, error: { code, message, ...(hint ? { hint } : {}) } };
}

// within reports whether real lies in one of the resolved roots.
function within(real: string, roots: string[]): boolean {
  return roots.some(
    (root) => real === root || real.startsWith(root + path.sep),
  );
}

async function resolvedRoots(roots: string[]): Promise<string[]> {
  const out: string[] = [];
  for (const root of roots) {
    try {
      out.push(await fs.realpath(root));
    } catch {
      // A root that does not exist in this sandbox simply admits nothing.
    }
  }
  return out;
}

function asSharedFile(value: unknown): SharedFile | undefined {
  if (!value || typeof value !== "object") return undefined;
  const { attachmentId, name, mediaType, size } = value as Record<
    string,
    unknown
  >;
  if (typeof attachmentId !== "string" || !attachmentId) return undefined;
  return {
    attachmentId,
    name: typeof name === "string" ? name : "",
    mediaType: typeof mediaType === "string" ? mediaType : "",
    size: typeof size === "number" ? size : 0,
  };
}

// readShareable resolves the path, then reads the file through ONE handle:
// fstat and read see the same inode, so a swap between check and read cannot
// change what is sent. Every failure is a typed refusal.
async function readShareable(
  rawPath: string,
  roots: string[],
): Promise<
  | { ok: true; real: string; bytes: Buffer }
  | { ok: false; outcome: ShareFileOutcome }
> {
  let real: string;
  try {
    real = await fs.realpath(path.resolve(process.cwd(), rawPath));
  } catch {
    return {
      ok: false,
      outcome: refuse("invalid_request", `no such file: ${rawPath}`),
    };
  }
  if (!within(real, await resolvedRoots(roots))) {
    return {
      ok: false,
      outcome: refuse(
        "invalid_request",
        `${rawPath} is outside /workspace, /tmp and /attachments`,
        "Copy the file under /tmp first if it lives elsewhere.",
      ),
    };
  }
  let handle: fs.FileHandle | undefined;
  try {
    handle = await fs.open(real, "r");
    const stat = await handle.stat();
    if (!stat.isFile()) {
      return {
        ok: false,
        outcome: refuse("invalid_request", `${rawPath} is not a regular file`),
      };
    }
    if (stat.size > maxShareFileBytes) {
      return { ok: false, outcome: tooLarge(rawPath, stat.size) };
    }
    const bytes = await handle.readFile();
    // The file may have grown or shrunk since fstat: judge what was read.
    if (bytes.length === 0) {
      return {
        ok: false,
        outcome: refuse("invalid_request", `${rawPath} is empty`),
      };
    }
    if (bytes.length > maxShareFileBytes) {
      return { ok: false, outcome: tooLarge(rawPath, bytes.length) };
    }
    return { ok: true, real, bytes };
  } catch (error) {
    return {
      ok: false,
      outcome: refuse(
        "invalid_request",
        `${rawPath} could not be read: ${describeError(error)}`,
      ),
    };
  } finally {
    await handle?.close().catch(() => undefined);
  }
}

function tooLarge(rawPath: string, size: number): ShareFileOutcome {
  return refuse(
    "too_large",
    `${rawPath} is ${size} bytes`,
    `Files are limited to ${maxShareFileBytes >> 20} MiB; share a smaller file or summarize it.`,
  );
}

export async function shareFile(
  args: unknown,
  env: NodeJS.ProcessEnv = process.env,
  roots: string[] = defaultShareRoots,
): Promise<ShareFileOutcome> {
  const endpoint = env.BEX_SHARE_ENDPOINT || "";
  if (!endpoint) {
    return refuse(
      "unavailable",
      "file sharing is not configured for this session",
    );
  }
  if (!args || typeof args !== "object" || Array.isArray(args)) {
    return refuse("invalid_request", "arguments must be an object with a path");
  }
  const { path: rawPath, name: rawName } = args as Record<string, unknown>;
  if (typeof rawPath !== "string" || !rawPath.trim()) {
    return refuse("invalid_request", "path is required");
  }
  if (rawName !== undefined && typeof rawName !== "string") {
    return refuse("invalid_request", "name must be a string");
  }
  const file = await readShareable(rawPath, roots);
  if (!file.ok) return file.outcome;
  const name = (rawName ?? "").trim() || path.basename(file.real);
  if (Buffer.byteLength(name, "utf8") > maxShareFileNameBytes) {
    return refuse(
      "invalid_request",
      `name exceeds ${maxShareFileNameBytes} bytes`,
    );
  }

  // The file is bounded at 25 MiB, so it is read once and hashed in memory:
  // the gateway signs the digest into the envelope bex-api verifies.
  const sha256 = createHash("sha256").update(file.bytes).digest("hex");
  const reply = await postToGateway(
    endpoint,
    {
      headers: {
        "content-type": "application/octet-stream",
        "x-bex-share-name": encodeURIComponent(name),
        "x-bex-share-sha256": sha256,
      },
      body: file.bytes,
    },
    {
      label: "share endpoint",
      fallbackCode: "unavailable",
      timeoutMs: requestTimeoutMs,
    },
  );
  if (!reply.ok) return reply;
  const result = asSharedFile(reply.payload);
  if (!result) {
    return refuse(
      "unavailable",
      "share endpoint returned an unexpected response",
    );
  }
  return { ok: true, result };
}
