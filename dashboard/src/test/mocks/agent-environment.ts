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
    modelPinned: false,
    pinnedModel: null,
    modelKeySet: false,
    modelSource: AgentModelSource.Platform,
    registryHosts: [],
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
