// Repository environments for agent sessions (ADR109): the client mirror of
// the server's shape rules and limits (ADR109 D3), so a bad name, a duplicate
// or an oversized command is caught while typing. The server stays the
// authority.

import { ownValue } from "@/common/lib/own-value";
import { utf8ByteLength } from "@/common/lib/utils/encode";
import {
  graphQLErrorCodes,
  graphQLErrorExtensions,
  isForbiddenError,
} from "@/common/lib/graphql-error";
import type { RepoEnvironmentBuildView } from "@/features/agent-environment/lib/repo-environment-builds";

/**
 * The settings card's element id: the session page's setup-failure callout
 * links to /workspace/settings#repository-environments.
 */
export const REPO_ENVIRONMENTS_ANCHOR = "repository-environments";

/** Repositories with an environment per workspace (maxRepoEnvironments). */
export const MAX_REPO_ENVIRONMENTS = 50;

/** ADR109 D3: a step or check name, unique within its list. */
export const STEP_NAME_PATTERN = /^[A-Za-z0-9][A-Za-z0-9 ._-]{0,63}$/;

/** The two ordered lists an environment holds. */
export type RepoStepKind = "setup" | "checks";

/** Per list: how many entries, and how long each command may be in bytes. */
export const REPO_STEP_LIMITS: Record<
  RepoStepKind,
  { maxSteps: number; maxRunBytes: number }
> = {
  setup: { maxSteps: 10, maxRunBytes: 8 * 1024 },
  checks: { maxSteps: 10, maxRunBytes: 2 * 1024 },
};

/** A setup step or check: a name and a shell command run from the repository root. */
export interface RepoStep {
  name: string;
  run: string;
}

/** What an environment write sends: both ordered lists, whole. */
export interface RepoEnvironmentInput {
  setup: RepoStep[];
  checks: RepoStep[];
}

/** A repository's environment as every read shows it. */
export interface RepoEnvironmentView extends RepoEnvironmentInput {
  /** owner/name, lowercased by the server. */
  repo: string;
  /** The SHA-256 of the setup steps (ADR111 D7). */
  version: string;
  /** The newest build (ADR111 D10); null for a repository that never built. */
  latestBuild: RepoEnvironmentBuildView | null;
}

/** The locale keys of what is wrong with one row's name and command. */
export interface RepoStepProblems {
  name: string | null;
  run: string | null;
}

/** A row with neither a name nor a command: the card drops it rather than refuse it. */
function isBlankStep(step: RepoStep): boolean {
  return step.name.trim() === "" && step.run.trim() === "";
}

/**
 * What is wrong with each row of one list, as the server would judge the
 * trimmed values the card sends. A blank row has no problem (it is not sent).
 * A repeated name is flagged on its second and later rows, so the first stays
 * the one to keep.
 */
export function repoStepProblems(
  kind: RepoStepKind,
  steps: readonly RepoStep[],
): RepoStepProblems[] {
  const seen = new Set<string>();
  return steps.map((step) => {
    if (isBlankStep(step)) return { name: null, run: null };
    const name = step.name.trim();
    const run = step.run.trim();
    let nameProblem: string | null = null;
    if (name === "") nameProblem = "agentEnvironment.repoEnvs.nameRequired";
    else if (!STEP_NAME_PATTERN.test(name))
      nameProblem = "agentEnvironment.repoEnvs.nameInvalid";
    else if (seen.has(name))
      nameProblem = "agentEnvironment.repoEnvs.nameDuplicate";
    seen.add(name);
    let runProblem: string | null = null;
    if (run === "") runProblem = "agentEnvironment.repoEnvs.runRequired";
    else if (
      run.includes("\0") ||
      utf8ByteLength(run) > REPO_STEP_LIMITS[kind].maxRunBytes
    ) {
      runProblem =
        kind === "setup"
          ? "agentEnvironment.repoEnvs.setupRunTooLong"
          : "agentEnvironment.repoEnvs.checkRunTooLong";
    }
    return { name: nameProblem, run: runProblem };
  });
}

/**
 * What is wrong with the environment as a whole, or null: the per-list caps,
 * and that an environment holds at least one setup step or check (an empty
 * one is deleted instead).
 */
export function repoEnvironmentProblem(
  setup: readonly RepoStep[],
  checks: readonly RepoStep[],
): string | null {
  if (setup.length > REPO_STEP_LIMITS.setup.maxSteps)
    return "agentEnvironment.repoEnvs.setupTooMany";
  if (checks.length > REPO_STEP_LIMITS.checks.maxSteps)
    return "agentEnvironment.repoEnvs.checksTooMany";
  if (setup.length === 0 && checks.length === 0)
    return "agentEnvironment.repoEnvs.environmentEmpty";
  return null;
}

/** The rows as the card sends them: in order, blank rows dropped, names and commands trimmed. */
export function trimRepoSteps(steps: readonly RepoStep[]): RepoStep[] {
  return steps
    .filter((step) => !isBlankStep(step))
    .map((step) => ({ name: step.name.trim(), run: step.run.trim() }));
}

/** AGENT_REPO_ENVIRONMENT_INVALID's `field` names the part it refused. */
const INVALID_FIELD_KEYS: Record<string, string> = {
  repo: "agentEnvironment.repoEnvs.repoInvalid",
  setup: "agentEnvironment.repoEnvs.setupRefused",
  checks: "agentEnvironment.repoEnvs.checksRefused",
};

const REPO_ENVIRONMENT_ERROR_KEYS: Record<string, string> = {
  AGENT_REPO_ENVIRONMENT_LIMIT_EXCEEDED:
    "agentEnvironment.repoEnvs.limitExceeded",
  AGENT_REPO_ENVIRONMENT_NOT_FOUND: "agentEnvironment.repoEnvs.notFound",
  AGENT_REPO_ENVIRONMENTS_UNAVAILABLE: "agentEnvironment.repoEnvs.unavailable",
};

/** The locale key for a refused environment write, by the server's stable code. */
export function repoEnvironmentErrorKey(err: unknown): string {
  const invalid = graphQLErrorExtensions(err, "AGENT_REPO_ENVIRONMENT_INVALID");
  if (invalid) {
    const field = invalid["field"];
    return (
      (typeof field === "string"
        ? ownValue(INVALID_FIELD_KEYS, field)
        : undefined) ?? "agentEnvironment.repoEnvs.invalid"
    );
  }
  for (const code of graphQLErrorCodes(err)) {
    const key = ownValue(REPO_ENVIRONMENT_ERROR_KEYS, code);
    if (key) return key;
  }
  return isForbiddenError(err)
    ? "agentEnvironment.repoEnvs.forbidden"
    : "agentEnvironment.repoEnvs.saveFailed";
}
