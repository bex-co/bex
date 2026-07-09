import type { InMemoryCacheConfig } from "@apollo/client";

// Shared InMemoryCache configuration for both CSR and SSR Apollo clients.
// Types without a standard `id` field use keyFields: false so Apollo stores
// them inline under their parent field rather than attempting normalization.
export const apolloCacheConfig: InMemoryCacheConfig = {
  typePolicies: {
    UsageSummary: { keyFields: false },
    ServiceUsage: { keyFields: false },
    UsageRow: { keyFields: false },
  },
};
