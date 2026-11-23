// A session's or an automation's own `gh` binding (w10/m561, Devin's security
// profile bound per session and per automation): inherit, on or off. Shared by
// the new-session composer, a session's settings (w10/m599), the automation
// form and its settings card, and Settings → Agents' automations default.
// Who may choose what is bex-api's rule (AGENT_*_GITHUB_CLI_LOCKED); these
// helpers only mirror it so a choice the server would refuse is shown
// disabled with its reason.

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

/** The sessions a session's `gh` inherits through, as bex-api walks them. */
export interface SessionGithubCliChain {
  /** The bindings of the sessions above it, its tree's root first. */
  ancestors: readonly (boolean | null)[];
  /** The automation that started the tree's root, if one did. */
  rootAutomationId: string | null;
}

/** The tree-walk shape of a session: what githubCliChainOf reads. */
type ChainSession = {
  id: string;
  automationId: string | null;
  parentSessionId: string | null;
  agentConfig: { githubCli: boolean | null };
};

/** bex-api's walk up a session's tree stops after three ancestors (ADR102). */
const MAX_TREE_WALK = 3;

/**
 * The chain a started session inherits through (w10/m599), as
 * `agentsession.LoadGitHubCLIChain` reads it: the sessions above it up to
 * its tree's root, and the automation that started that root. A child needs
 * its tree (null while it loads); an ancestor missing from a loaded tree was
 * deleted, which makes the session below it the root.
 */
export function githubCliChainOf(
  session: ChainSession,
  tree: readonly ChainSession[] | null,
): SessionGithubCliChain | null {
  if (!session.parentSessionId) {
    return { ancestors: [], rootAutomationId: session.automationId };
  }
  if (!tree) return null;
  const byId = new Map(tree.map((s) => [s.id, s]));
  const ancestors: (boolean | null)[] = [];
  let root = session;
  for (let i = 0; i < MAX_TREE_WALK && root.parentSessionId; i++) {
    const parent = byId.get(root.parentSessionId);
    if (!parent) break;
    ancestors.unshift(parent.agentConfig.githubCli);
    root = parent;
  }
  return { ancestors, rootAutomationId: root.automationId };
}

/**
 * What a started session's inherit resolves to (w10/m599), null while that
 * is unknown, as `GitHubCLIChain.Resolve` decides without the session's own
 * binding: a mandatory off wins; then the workspace's switch, or for a tree
 * an automation started its automation's own binding, else the automations
 * default, else that switch (`automationBinding` is undefined until the
 * automation is read); then the root's own binding; then any off below it.
 */
export function inheritedSessionGithubCli(
  env: GithubCliEnvironment | null,
  chain: SessionGithubCliChain | null,
  automationBinding?: boolean | null,
): boolean | null {
  if (!env) return null;
  if (githubCliMandatoryOff(env)) return false;
  if (!chain) return null;
  let on = env.githubCliEnabled;
  if (chain.rootAutomationId) {
    if (automationBinding === undefined) return null;
    on = automationBinding ?? env.automationsGithubCliEnabled ?? on;
  }
  const [root, ...below] = chain.ancestors;
  if (root != null) on = root;
  return on && !below.includes(false);
}

/** Why a choice cannot be made: a mandatory off, or a caller who is no admin. */
export type GithubCliLock = "mandatory" | "admin";

/**
 * Why a session or an automation may not choose `choice`, or null when it
 * may. `stored` is what the server keeps unchanged: inherit on a create, the
 * saved binding on an edit, and keeping it is never refused. Only a workspace
 * admin (can_manage) chooses or changes the binding (w10/044), as Devin's
 * "Members without the permission can't enumerate, select, or change
 * profiles": a member's sessions and automations follow the resolved
 * default, which the control still shows. Nobody chooses "on" under a
 * mandatory off.
 */
export function githubCliLock({
  choice,
  stored = "inherit",
  env,
  canManage,
}: {
  choice: GithubCliChoice;
  stored?: GithubCliChoice;
  env: GithubCliEnvironment | null;
  canManage: boolean;
}): GithubCliLock | null {
  if (choice === stored) return null;
  if (!canManage) return "admin";
  return choice === "on" && githubCliMandatoryOff(env) ? "mandatory" : null;
}

/** The locale key of a lock's reason. */
export function githubCliLockKey(lock: GithubCliLock): string {
  return lock === "mandatory"
    ? "agentSessions.githubCliOnMandatory"
    : "agentSessions.githubCliAdminOnly";
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
