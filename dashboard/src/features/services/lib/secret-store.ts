import { classifyRefusal } from "@/common/lib/graphql-error";

/**
 * Classifies a failed env-var, secret-file or env-group read: the secret store
 * not wired, an authorization denial, or anything else.
 */
export function classifySecretStoreError(error: unknown) {
  return classifyRefusal(error, "SECRETS_UNAVAILABLE");
}
