// A session's or an automation's own `gh` binding (w10/m561, Devin's security
// profile bound per session and per automation): inherit, on or off. Shared by
// the new-session composer, the automation form and its settings card, and
// Settings → Agents' automations default. Who may choose what is bex-api's
// rule (AGENT_*_GITHUB_CLI_LOCKED); these helpers only mirror it so a choice
// the server would refuse is shown disabled with its reason.

import type { Translate } from "@/common/hooks/use-translations";
import type { AgentEnvironmentView } from "@/features/agent-environment/lib/environment";

export const GITHUB_CLI_CHOICES = ["inherit", "on", "off"] as const;
export type GithubCliChoice = (typeof GITHUB_CLI_CHOICES)[number];

/** The environment fields the binding chain reads. */
export type GithubCliEnvironment = Pick<
  AgentEnvironmentView,
  "githubCliEnabled" | "githubCliMandatory" | "automationsGithubCliEnabled"
>;

/** A stored binding as a choice: null (or absent) inherits. */
export function githubCliChoice(
  binding: boolean | null | undefined,
): GithubCliChoice {
  if (binding == null) return "inherit";
  return binding ? "on" : "off";
}

/** The binding a choice sends: undefined (the field omitted) inherits. */
export function githubCliBinding(choice: GithubCliChoice): boolean | undefined {
  return choice === "inherit" ? undefined : choice === "on";
}

/**
 * Whether the workspace's `gh` is off and its admins made that mandatory: then
 * no automations default, automation or session turns it back on.
 */
export function githubCliMandatoryOff(
  env: GithubCliEnvironment | null,
): boolean {
  return env != null && !env.githubCliEnabled && env.githubCliMandatory;
}

/**
 * What inherit resolves to, null while the environment is unknown. A session
 * started from the composer inherits the workspace's switch; an automation's
 * sessions inherit the automations default, else that switch, and a
 * mandatory off wins over both.
 */
export function inheritedGithubCli(
  env: GithubCliEnvironment | null,
  level: "session" | "automation",
): boolean | null {
  if (!env) return null;
  if (level === "session" || githubCliMandatoryOff(env)) {
    return env.githubCliEnabled;
  }
  return env.automationsGithubCliEnabled ?? env.githubCliEnabled;
}

/** Why "on" cannot be chosen: a mandatory off, or a caller who is no admin. */
export type GithubCliOnLock = "mandatory" | "admin";

/**
 * Why a session or an automation may not choose "on", or null when it may:
 * nobody may under a mandatory off, and only a workspace admin (can_manage)
 * may otherwise, since an on outlives a later off. `kept` is an automation
 * that already stores on: the server keeps an unchanged on, so it stays
 * choosable.
 */
export function githubCliOnLock({
  env,
  canManage,
  kept = false,
}: {
  env: GithubCliEnvironment | null;
  canManage: boolean;
  kept?: boolean;
}): GithubCliOnLock | null {
  if (kept) return null;
  if (githubCliMandatoryOff(env)) return "mandatory";
  return canManage ? null : "admin";
}

/** The locale key of a lock's reason. */
export function githubCliOnLockKey(lock: GithubCliOnLock): string {
  return lock === "mandatory"
    ? "agentSessions.githubCliOnMandatory"
    : "agentSessions.githubCliOnAdminOnly";
}

/** A choice's label; inherit names what it resolves to when that is known. */
export function githubCliChoiceLabel(
  choice: GithubCliChoice,
  inherited: boolean | null,
  t: Translate,
): string {
  if (choice === "on") return t("agentSessions.githubCliOn");
  if (choice === "off") return t("agentSessions.githubCliOff");
  if (inherited == null) return t("agentSessions.githubCliInherit");
  return t(
    inherited
      ? "agentSessions.githubCliInheritOn"
      : "agentSessions.githubCliInheritOff",
  );
}
