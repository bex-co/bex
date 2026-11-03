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

// The Pod-bound gateway verbs the in-sandbox bex MCP tools call (ADR090 PR
// verb, ADR095 share route) answer refusals in one shape; this is their one
// client.

import { describeError } from "./errors.js";

// ToolError is the gateway's typed refusal ({error:{code,message,hint}}), the
// shape every bex MCP tool reports to the agent verbatim.
export interface ToolError {
  code: string;
  message: string;
  hint?: string;
}

export function asToolError(value: unknown): ToolError | undefined {
  if (!value || typeof value !== "object") return undefined;
  const error = (value as Record<string, unknown>).error;
  if (!error || typeof error !== "object") return undefined;
  const { code, message, hint } = error as Record<string, unknown>;
  if (typeof code !== "string" || typeof message !== "string") return undefined;
  return { code, message, ...(typeof hint === "string" ? { hint } : {}) };
}

export type GatewayReply =
  | { ok: true; payload: unknown }
  | { ok: false; error: ToolError };

// postToGateway sends one request to a Pod-bound gateway verb and returns its
// JSON payload, or a typed error: the gateway's own refusal when it sent one,
// otherwise fallbackCode with what went wrong. It never throws.
export async function postToGateway(
  endpoint: string,
  init: { headers: Record<string, string>; body: string | Uint8Array },
  options: { label: string; fallbackCode: string; timeoutMs: number },
): Promise<GatewayReply> {
  let response: Response;
  let text: string;
  try {
    response = await fetch(endpoint, {
      method: "POST",
      headers: init.headers,
      body: init.body,
      signal: AbortSignal.timeout(options.timeoutMs),
    });
    text = await response.text();
  } catch (error) {
    return {
      ok: false,
      error: {
        code: options.fallbackCode,
        message: `${options.label} unreachable: ${describeError(error)}`,
      },
    };
  }
  let payload: unknown;
  try {
    payload = text ? JSON.parse(text) : undefined;
  } catch {
    payload = undefined;
  }
  if (!response.ok) {
    return {
      ok: false,
      error: asToolError(payload) ?? {
        code: options.fallbackCode,
        message: `${options.label} returned HTTP ${response.status}`,
      },
    };
  }
  return { ok: true, payload };
}
