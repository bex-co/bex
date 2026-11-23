import { AgentModelSource, AgentNetworkTier } from "@/graphql/definitions";
import type { AgentEnvironmentView } from "@/features/agent-environment/lib/environment";

/**
 * A workspace agent environment on the platform model route with no spend
 * fields read (ADR091 D5, D10.1); tests override what they exercise.
 */
export function agentEnvironment(
  over: Partial<AgentEnvironmentView> = {},
): AgentEnvironmentView {
  return {
    __typename: "AgentEnvironment",
    ownerId: "tea-1",
    networkTier: AgentNetworkTier.Registries,
    customHosts: [],
    defaultModel: null,
    models: [],
    // Devin's "Allow all models" (w10/m592), every workspace's default:
    // enabledModels is the whole catalog and nothing is new in it. A test
    // that narrows the list by model sets it false.
    allowAllModels: true,
    // The catalog models members may choose (w10/m527): empty with the
    // catalog, so Settings draws no model list unless a test lists models.
    enabledModels: [],
    // The ones off because they are new in the catalog (w10/m568): none.
    newModels: [],
    // The catalog's providers and the filter by provider (w10/m573): none,
    // and the list is by model.
    providers: [],
    enabledProviders: null,
    // The sandbox templates (w10/m519): none listed, so Settings draws no
    // default-sandbox control unless a test lists some.
    defaultTemplate: "agent",
    templates: [],
    modelKeySet: false,
    modelSource: AgentModelSource.Platform,
    registryHosts: [],
    // The sessions' gh (w10/m524): on, as a workspace that never saved an
    // environment reads.
    githubCliEnabled: true,
    // Its narrower bindings (w10/m561): automations inherit it, and its off
    // is not mandatory.
    automationsGithubCliEnabled: null,
    githubCliMandatory: false,
    // The idle close (w6/m563): the platform's 5 minutes, on a plan that
    // cannot change it, so Settings draws no idle timeout section unless a
    // test puts the workspace on Enterprise.
    idleTimeoutMinutes: 5,
    idleTimeoutConfigurable: false,
    enabledAgents: ["claude", "codex", "gemini"],
    agentDataUse: [
      {
        __typename: "AgentDataUse",
        agent: "muse",
        acknowledgedAt: null,
        acknowledgedBy: null,
      },
    ],
    updatedAt: null,
    agentModelMonthlySpendCapUsd: null,
    agentModelPlanSpendCapUsd: null,
    agentModelSpendCapOverrideUsd: null,
    agentModelMaxSpendCapUsd: null,
    agentModelSpendMonthToDateUsd: null,
    agentModelSpendCapTierLimited: null,
    trustTier: null,
    trustTierLocked: null,
    nextChargeThresholdUsd: null,
    agentModelCreditUsd: null,
    agentModelUnbilledUsd: null,
    billingBalanceDue: null,
    agentModelSpendCapReached: null,
    agentModelChargesAtThreshold: null,
    ...over,
  };
}
