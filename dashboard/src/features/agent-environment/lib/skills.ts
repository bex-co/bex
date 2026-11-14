// Workspace skills for cloud agents (ADR099): the client mirror of the
// server's skill rules, so a bad name or an oversized body is caught while
// typing. The server stays the authority.

import { utf8ByteLength } from "@/common/lib/utils/encode";
import {
  graphQLErrorCodes,
  isForbiddenError,
} from "@/common/lib/graphql-error";

/** ADR099 D2: the Agent Skills name format, at most 64 characters. */
export const SKILL_NAME_PATTERN = /^[a-z0-9]+(-[a-z0-9]+)*$/;
export const MAX_SKILL_NAME_LENGTH = 64;
export const MAX_SKILL_DESCRIPTION_LENGTH = 1024;
export const MAX_SKILL_BODY_BYTES = 64 * 1024;
/** Skills one turn may invoke (maxTurnSkills). */
export const MAX_TURN_SKILLS = 5;
export const MAX_SKILL_ARGUMENT_BYTES = 4 * 1024;

const TRIGGER_USER = "user";
const TRIGGER_MODEL = "model";

/** A skill the agent applies only when a prompt or the skills field invokes it. */
export function isUserOnly(triggers: readonly string[]): boolean {
  return !triggers.includes(TRIGGER_MODEL);
}

export function triggersFor(userOnly: boolean): string[] {
  return userOnly ? [TRIGGER_USER] : [TRIGGER_USER, TRIGGER_MODEL];
}

// Control characters and the Unicode line/paragraph separators: a description
// or an argument is one line (ADR099 D2/D4).
function breaksLine(text: string): boolean {
  for (const ch of text) {
    const code = ch.codePointAt(0) ?? 0;
    if (
      code < 0x20 ||
      (code >= 0x7f && code <= 0x9f) ||
      code === 0x2028 ||
      code === 0x2029
    ) {
      return true;
    }
  }
  return false;
}

/** The locale key of what is wrong with a typed name, or null when valid. */
export function skillNameProblem(name: string): string | null {
  if (name === "") return "agentEnvironment.skills.nameRequired";
  if (name.length > MAX_SKILL_NAME_LENGTH || !SKILL_NAME_PATTERN.test(name))
    return "agentEnvironment.skills.nameInvalid";
  return null;
}

export function skillDescriptionProblem(description: string): string | null {
  if (description.trim() === "")
    return "agentEnvironment.skills.descriptionRequired";
  if (
    description.length > MAX_SKILL_DESCRIPTION_LENGTH ||
    breaksLine(description)
  ) {
    return "agentEnvironment.skills.descriptionInvalid";
  }
  return null;
}

export function skillBodyProblem(body: string): string | null {
  if (body.includes("\0") || utf8ByteLength(body) > MAX_SKILL_BODY_BYTES)
    return "agentEnvironment.skills.bodyInvalid";
  return null;
}

/** An invocation's arguments: one line, at most 4 KiB. */
export function skillArgumentsProblem(args: string): string | null {
  if (breaksLine(args) || utf8ByteLength(args) > MAX_SKILL_ARGUMENT_BYTES)
    return "agentSessions.skillsArgumentsInvalid";
  return null;
}

/** The file name an exported skill downloads as. */
export function skillFileName(name: string): string {
  return `${name}-SKILL.md`;
}

/** A starting point for a skill pasted or written as a whole SKILL.md. */
export function skillTemplate(): string {
  return [
    "---",
    "name: my-skill",
    "description: When to use this skill, in one line",
    "---",
    "",
    "Steps the agent follows.",
    "",
  ].join("\n");
}

const SKILL_ERROR_KEYS: Record<string, string> = {
  AGENT_SKILL_INVALID: "agentEnvironment.skills.invalid",
  AGENT_SKILL_LIMIT_EXCEEDED: "agentEnvironment.skills.limitExceeded",
  AGENT_SKILL_CONFLICT: "agentEnvironment.skills.conflict",
  AGENT_SKILL_NOT_FOUND: "agentEnvironment.skills.notFound",
  AGENT_SKILLS_UNAVAILABLE: "agentEnvironment.skills.unavailable",
};

/** The locale key for a refused skill write, by the server's stable code. */
export function skillErrorKey(err: unknown): string {
  for (const code of graphQLErrorCodes(err)) {
    const key = SKILL_ERROR_KEYS[code];
    if (key) return key;
  }
  return isForbiddenError(err)
    ? "agentEnvironment.skills.forbidden"
    : "agentEnvironment.skills.saveFailed";
}
