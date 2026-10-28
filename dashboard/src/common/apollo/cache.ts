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
  },
};
