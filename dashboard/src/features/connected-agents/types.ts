// A `type`-only import is erased at compile time — this never pulls the
// server-only `hydra-connected-agents.ts` (or its Hydra admin client) into the
// client bundle, exactly like `auth.consent.tsx`'s `ConsentView` import.
export type { ConnectedAgentView } from "@/common/server-fn/hydra-connected-agents";
