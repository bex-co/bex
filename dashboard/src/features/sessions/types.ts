// A `type`-only import is erased at compile time — this never pulls the
// server-only `kratos-sessions.ts` into the client bundle, exactly like
// `auth.consent.tsx`'s `ConsentView` import.
export type { SessionView } from "@/common/server-fn/kratos-sessions";
