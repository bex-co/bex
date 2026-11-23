import { classifyRefusal, type RefusalKind } from "@/common/lib/graphql-error";
import { isAdminOnlyRefusal } from "@/features/capabilities/lib/admin-only";

/**
 * Classifies a failed env-var, secret-file or env-group read: the secret store
 * not wired, an authorization denial, or anything else. A protected
 * environment's refusal of a non-Admin is a denial too (w8/193, coded since
 * w4/m594).
 */
export function classifySecretStoreError(error: unknown): RefusalKind | null {
  if (isAdminOnlyRefusal(error)) return "forbidden";
  return classifyRefusal(error, "SECRETS_UNAVAILABLE");
}
