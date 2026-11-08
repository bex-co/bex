import type { InMemoryCacheConfig } from "@apollo/client";

// Shared InMemoryCache configuration for both CSR and SSR Apollo clients.
// Types without a standard `id` field use keyFields: false so Apollo stores
// them inline under their parent field rather than attempting normalization.
export const apolloCacheConfig: InMemoryCacheConfig = {
  typePolicies: {
    UsageSummary: { keyFields: false },
    ServiceUsage: { keyFields: false },
    UsageRow: { keyFields: false },
    // A log entry's id is a record identity for the reader's dedupe (w4/m217),
    // not a cache identity: entries stay embedded in their query's result, as
    // they were before the id was selected.
    LogEntry: { keyFields: false },
    // One agent environment per workspace (w10/m17): keyed by its owner so
    // updateAgentEnvironment's result refreshes the settings query in place.
    AgentEnvironment: { keyFields: ["ownerId"] },
    // One notification policy per workspace (w3/m271): keyed by its owner so a
    // save refreshes the settings card and every service page's default label.
    OwnerNotificationSettings: { keyFields: ["ownerId"] },
    AgentSession: {
      fields: { outcome: { merge: mergeAgentSessionOutcome } },
    },
  },
};

type OutcomeRecord = Record<string, unknown> | null | undefined;

// Only a single-session read fills an outcome's costly fields (ADR094: the
// final summary, shared files, structured result and cost). A list poll of the
// same session then writes them as null, which would blank a detail page that
// already showed them. While the turn and state are unchanged those fields
// cannot have been cleared server-side, so a null incoming value keeps the
// existing one; any change of turn or state takes the incoming outcome whole.
const outcomeDetailFields = ["summary", "files", "result", "cost"] as const;

export function mergeAgentSessionOutcome(
  existing: OutcomeRecord,
  incoming: OutcomeRecord,
): OutcomeRecord {
  if (
    !existing ||
    !incoming ||
    existing.turn !== incoming.turn ||
    existing.state !== incoming.state
  ) {
    return incoming;
  }
  const merged = { ...incoming };
  for (const field of outcomeDetailFields) {
    if (merged[field] == null && existing[field] != null) {
      merged[field] = existing[field];
    }
  }
  return merged;
}
