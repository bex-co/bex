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

// ask_user (ADR100 D4): in a session with an approval mode the agent can ask
// the person who started it a question and wait for the answer, instead of
// guessing or stopping. The question becomes a decision on the session's
// Pod-bound decision URL; the session shows needs_input (waitingFor user)
// until a member answers, or until it expires after 24 hours, when the tool
// says so and the agent continues on its best assumption.

import type { ToolError } from "./gateway.js";
import { oneLine, requestDecision } from "./approval.js";
import { SecretRedactor, secretValuesFromEnv } from "./redact.js";

export const askUserToolName = "ask_user";
export const maxQuestionChars = 2000;
const maxOptions = 10;

export const askUserToolDefinition = {
  name: askUserToolName,
  title: "Ask the user",
  description:
    "Ask the person who started this session a question and wait for the answer. Use it only when you are genuinely blocked " +
    "on a choice only they can make (a missing credential, an ambiguous requirement, a destructive step); finish everything that " +
    "does not depend on the answer first. The session pauses until they reply, which can take hours. If no answer arrives, the " +
    "tool says so: continue with your best assumption and state it in your reply.",
  inputSchema: {
    type: "object",
    properties: {
      question: {
        type: "string",
        description: `The question, at most ${maxQuestionChars} characters.`,
      },
      options: {
        type: "array",
        items: { type: "string" },
        description: `Optional suggested answers (at most ${maxOptions}); the person may answer anything.`,
      },
    },
    required: ["question"],
    additionalProperties: false,
  },
  annotations: { openWorldHint: true },
};

export type AskUserOutcome =
  | { ok: true; result: { answered: boolean; answer: string } }
  | { ok: false; error: ToolError };

function invalid(message: string): AskUserOutcome {
  return { ok: false, error: { code: "invalid_request", message } };
}

export async function askUser(
  args: unknown,
  env: NodeJS.ProcessEnv = process.env,
  decide: typeof requestDecision = requestDecision,
): Promise<AskUserOutcome> {
  const endpoint = env.BEX_DECISION_ENDPOINT || "";
  if (!endpoint) {
    return {
      ok: false,
      error: {
        code: "not_expected",
        message: "this session cannot ask questions",
        hint: "Make a reasonable assumption and state it in your reply.",
      },
    };
  }
  if (!args || typeof args !== "object" || Array.isArray(args)) {
    return invalid("arguments must be an object with question");
  }
  const { question, options } = args as Record<string, unknown>;
  if (typeof question !== "string" || !question.trim())
    return invalid("question is required");
  if ([...question].length > maxQuestionChars)
    return invalid(`question exceeds ${maxQuestionChars} characters`);
  if (
    options !== undefined &&
    (!Array.isArray(options) ||
      options.length > maxOptions ||
      options.some((o) => typeof o !== "string"))
  ) {
    return invalid(`options must be at most ${maxOptions} strings`);
  }
  const redactor = new SecretRedactor(secretValuesFromEnv(env));
  const text = question.trim();
  const answer = await decide(
    endpoint,
    {
      kind: "question",
      title: oneLine(text, 200),
      detail: { question: text, ...(options ? { options } : {}) },
    },
    {
      clockPath: env.BEX_DECISION_CLOCK_PATH || "",
      scrub: (value) => redactor.redactString(value),
    },
  );
  if (answer.state === "answered" && answer.text) {
    return { ok: true, result: { answered: true, answer: answer.text } };
  }
  return {
    ok: true,
    result: {
      answered: false,
      answer:
        "No answer arrived. Continue with your best assumption and state it in your reply.",
    },
  };
}
