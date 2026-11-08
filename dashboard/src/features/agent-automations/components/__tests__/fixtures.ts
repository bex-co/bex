import type { AutomationView } from "@/features/agent-automations/hooks/use-agent-automations";

/** A stored automation as the list/detail queries return it. */
export function automation(over: Partial<AutomationView> = {}): AutomationView {
  return {
    __typename: "AgentAutomation",
    id: "aut-1",
    ownerId: "tea-1",
    name: "nightly-deps",
    enabled: true,
    createdBy: "usr-1",
    triggers: [
      {
        __typename: "AgentAutomationTrigger",
        type: "schedule",
        schedule: {
          __typename: "AgentAutomationSchedule",
          frequency: "day",
          minute: null,
          at: "09:00",
          weekdays: null,
          runAt: null,
          cron: null,
          timezone: "Europe/Berlin",
          endsAt: null,
        },
      },
    ],
    template: {
      __typename: "AgentAutomationTemplate",
      task: "Update the dependencies",
      repo: "acme/widgets",
      branch: null,
      title: null,
      agentConfig: {
        __typename: "AgentAutomationAgentConfig",
        agent: "claude",
        model: null,
        template: null,
      },
      maxSpendUSD: 5,
      secretNames: [],
      skills: [],
      egressAllowlist: [],
      tags: [],
      outputSchema: null,
      outputRequired: null,
    },
    nextFireAt: "2099-01-01T08:00:00Z",
    lastFireAt: null,
    lastSessionId: null,
    pausedReason: null,
    consecutiveFailures: 0,
    version: 1,
    createdAt: "2026-11-01T00:00:00Z",
    updatedAt: "2026-11-01T00:00:00Z",
    ...over,
  } as AutomationView;
}
