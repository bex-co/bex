// The composer's invoked skills (ADR099 D4): workspace skills this turn must
// follow, with optional one-line arguments, held until the create or steer
// call sends them.

import type { AgentSessionSkillInput } from "@/graphql/schema-types";
import { skillArgumentsProblem } from "@/features/agent-environment/lib/skills";

export type ComposerSkill = AgentSessionSkillInput;

/** The wire form: blank arguments are omitted. */
export function composerSkillsInput(
  skills: readonly ComposerSkill[],
): AgentSessionSkillInput[] {
  return skills.map((skill) => {
    const args = skill.arguments?.trim();
    return args ? { name: skill.name, arguments: args } : { name: skill.name };
  });
}

/** A stable, order-independent form for the idempotency fingerprint. */
export function composerSkillsFingerprint(
  skills: readonly ComposerSkill[],
): AgentSessionSkillInput[] {
  return composerSkillsInput(skills).sort((a, b) =>
    a.name.localeCompare(b.name),
  );
}

/** True when every chosen skill's arguments are valid, so the composer may send. */
export function composerSkillsValid(skills: readonly ComposerSkill[]): boolean {
  return skills.every(
    (skill) => skillArgumentsProblem(skill.arguments ?? "") == null,
  );
}
