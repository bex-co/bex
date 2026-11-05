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

// submit_result (ADR094 D5): when a turn carries an outputSchema, the agent
// hands its structured answer back through this tool instead of prose. The
// driver validates the result against the turn's schema first and returns the
// problems to the agent so it can fix and resubmit; a result that passes is
// posted to the session's Pod-bound result URL, where bex-api validates it
// again against the schema it stored (that check is the authority). The last
// valid submission wins; `final: true` is the agent's statement that this is
// the answer. No secret is involved: the gateway authenticates the source Pod.

import { readFile } from "node:fs/promises";
import { Ajv2020, type ErrorObject } from "ajv/dist/2020.js";
import { describeError } from "./errors.js";
import { writeJsonAtomic } from "./fs-util.js";
import { postToGateway, type ToolError } from "./gateway.js";

export const submitResultToolName = "submit_result";
// The gateway's MaxResultBytes and the schema cap (ADR094 D5): one value on
// both sides of the hop.
export const maxResultBytes = 256 * 1024;
export const maxOutputSchemaBytes = 64 * 1024;
const maxProblems = 8;
const requestTimeoutMs = 60_000;

export const submitResultToolDefinition = {
  name: submitResultToolName,
  title: "Submit the structured result",
  description:
    "Hand back the machine-readable answer this task asks for. `result` must be a JSON value that satisfies the output schema in your instructions. " +
    "If it does not, the validation errors come back to you: fix the result and call again. The last valid call wins. " +
    "Call it with `final: true` when the result is complete, before you end the turn; `final: false` saves a partial value without finishing. " +
    "If it returns another error, report that error in your reply.",
  inputSchema: {
    type: "object",
    properties: {
      result: {
        description:
          "The result: any JSON value that satisfies the task's output schema.",
      },
      final: {
        type: "boolean",
        description:
          "true when this is the complete result; false for a partial value.",
      },
    },
    required: ["result", "final"],
    additionalProperties: false,
  },
  annotations: { idempotentHint: true, openWorldHint: true },
};

export interface SubmittedResult {
  accepted: true;
  final: boolean;
}

export type SubmitResultOutcome =
  | { ok: true; result: SubmittedResult }
  | { ok: false; error: ToolError };

function refuse(
  code: string,
  message: string,
  hint?: string,
): SubmitResultOutcome {
  return { ok: false, error: { code, message, ...(hint ? { hint } : {}) } };
}

// describeProblems turns Ajv's errors into the sentences the agent reads.
function describeProblems(errors: ErrorObject[] | null | undefined): string {
  const lines = (errors ?? []).slice(0, maxProblems).map((error) => {
    const where = error.instancePath || "(result)";
    return `${where}: ${error.message ?? "is invalid"}`;
  });
  return lines.join("; ") || "the result does not match the output schema";
}

// compileSchema compiles the turn's schema as JSON Schema 2020-12. Nothing
// loads a remote schema: a reference that leaves the document fails to compile.
export function compileSchema(schemaJson: string) {
  const ajv = new Ajv2020({ allErrors: true, strict: false });
  return ajv.compile(JSON.parse(schemaJson));
}

export async function submitResult(
  args: unknown,
  env: NodeJS.ProcessEnv = process.env,
): Promise<SubmitResultOutcome> {
  const endpoint = env.BEX_RESULT_ENDPOINT || "";
  const schemaJson = env.BEX_RESULT_SCHEMA || "";
  if (!endpoint || !schemaJson) {
    return refuse(
      "not_expected",
      "this turn did not ask for a structured result",
      "Put your answer in your reply.",
    );
  }
  if (!args || typeof args !== "object" || Array.isArray(args)) {
    return refuse("invalid_request", "arguments must be an object with result and final");
  }
  const { result, final } = args as Record<string, unknown>;
  if (result === undefined) {
    return refuse("invalid_request", "result is required");
  }
  if (typeof final !== "boolean") {
    return refuse("invalid_request", "final must be true or false");
  }
  const body = JSON.stringify({ result, final });
  // The wrapper adds a few bytes; judge the result's own size like the gateway.
  if (Buffer.byteLength(JSON.stringify(result), "utf8") > maxResultBytes) {
    return refuse(
      "too_large",
      `the result is larger than ${maxResultBytes >> 10} KiB`,
      "Submit a smaller result; link to or summarize bulky detail instead of pasting it.",
    );
  }
  let validate: ReturnType<typeof compileSchema>;
  try {
    validate = compileSchema(schemaJson);
  } catch (error) {
    return refuse(
      "unavailable",
      `the output schema could not be compiled: ${describeError(error)}`,
    );
  }
  if (!validate(result)) {
    return refuse(
      "schema_mismatch",
      describeProblems(validate.errors),
      "Fix the result so it matches the output schema, then call submit_result again.",
    );
  }
  const reply = await postToGateway(
    endpoint,
    { headers: { "content-type": "application/json" }, body },
    { label: "result endpoint", fallbackCode: "unavailable", timeoutMs: requestTimeoutMs },
  );
  if (!reply.ok) return reply;
  const payload = reply.payload as Record<string, unknown> | undefined;
  if (!payload || payload.accepted !== true) {
    return refuse("unavailable", "result endpoint returned an unexpected response");
  }
  try {
    await recordSubmission(env.BEX_RESULT_RECORD_PATH || "", final);
  } catch (error) {
    // The result is stored either way; the record only informs the driver's
    // reminder, so a write failure must not hide the acceptance.
    process.stderr.write(
      `bex mcp: could not record result: ${describeError(error)}\n`,
    );
  }
  return { ok: true, result: { accepted: true, final } };
}

async function recordSubmission(recordPath: string, final: boolean): Promise<void> {
  if (!recordPath) return;
  // A partial submission never downgrades a final one the agent already made.
  if (!final && (await readResultRecord(recordPath)).final) return;
  await writeJsonAtomic(recordPath, {
    final,
    recordedAt: new Date().toISOString(),
  });
}

// readResultRecord reports whether the bex MCP server recorded a final result
// for this turn. A missing or unreadable record means none.
export async function readResultRecord(
  recordPath: string,
): Promise<{ final: boolean }> {
  try {
    const record = JSON.parse(await readFile(recordPath, "utf8")) as Record<
      string,
      unknown
    >;
    return { final: record.final === true };
  } catch {
    return { final: false };
  }
}

// resultReminder is the one bounded follow-up prompt a turn gets when it ends
// without a final result (ADR094 D5).
export function resultReminder(): string {
  return (
    `This task requires a structured result and you have not submitted a final one. ` +
    `Call ${submitResultToolName} now with \`final: true\` and a result that satisfies the output schema, then end your turn. ` +
    `Do not do other work.`
  );
}
