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

// describeError is the driver's ONE way to turn a thrown value into the text
// that reaches a human — the `failed` status file the Completer copies into
// `agent_sessions.failure_reason`, the `error` UI part the dashboard renders in
// the conversation, the POST /turn body, and the process log.
//
// It exists because `String(error)` does not survive a rejected JSON-RPC call.
// The ACP SDK rejects a failed request with the RAW protocol error OBJECT
// (`{code, message, data}`, ClientSideConnection: `pendingResponse.reject(
// response.error)`), not an Error — so the ubiquitous
// `error instanceof Error ? error.message : String(error)` collapsed every
// agent-side failure to the literal string "[object Object]". A live session
// whose model call was refused stored `failure_reason = "[object Object]"` and
// the dashboard dutifully showed it: the one field whose entire job is to say
// what went wrong was the one field that never did.
const maxDescriptionLength = 2000;

interface ProtocolErrorShape {
  message?: unknown;
  code?: unknown;
  data?: unknown;
}

export function describeError(error: unknown): string {
  const described = describe(error);
  return described.length > maxDescriptionLength
    ? `${described.slice(0, maxDescriptionLength)}… (truncated)`
    : described;
}

function describe(error: unknown): string {
  if (error === undefined || error === null) return "unknown error";
  if (typeof error === "string") return error || "unknown error";
  if (error instanceof Error) return error.message || error.name || "Error";
  if (typeof error === "object") return describeObject(error as ProtocolErrorShape);
  return String(error);
}

// A JSON-RPC error carries its human text in `message`, a numeric `code`, and
// optional structured `data` (the ACP agent puts the underlying failure there —
// e.g. the model provider's own status text). Render all three: the code
// distinguishes a protocol fault from an agent-reported one, and `data` is
// usually the only place the real cause appears.
function describeObject(error: ProtocolErrorShape): string {
  const parts: string[] = [];
  if (typeof error.message === "string" && error.message !== "") {
    parts.push(error.message);
  }
  if (typeof error.code === "number" || typeof error.code === "string") {
    parts.push(`(code ${error.code})`);
  }
  if (error.data !== undefined && error.data !== null) {
    const data = typeof error.data === "string" ? error.data : stringify(error.data);
    if (data) parts.push(data);
  }
  if (parts.length > 0) return parts.join(" ");
  // Not a protocol error: fall back to the object's own JSON so the reader gets
  // the fields rather than "[object Object]". A value JSON cannot render at all
  // (circular, throwing getters) still names its shape — a useless-but-honest
  // description beats the useless-and-misleading default coercion.
  const json = stringify(error);
  if (json) return json;
  const keys = Object.keys(error);
  return keys.length > 0
    ? `unserializable error object (keys: ${keys.join(", ")})`
    : "unserializable error object";
}

function stringify(value: unknown): string {
  try {
    const json = JSON.stringify(value);
    return json === undefined || json === "{}" ? "" : json;
  } catch {
    return ""; // circular or non-serializable: the caller falls back
  }
}
