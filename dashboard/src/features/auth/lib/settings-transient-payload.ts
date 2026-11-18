import type { FormValues } from "@ory/elements-react";

/**
 * The account settings flow's `transient_payload` (w7/085): the provider an
 * oidc unlink removes, `{ unlink: "github" }`. Render refuses only
 * disconnecting Google while a member belongs to a workspace that requires
 * Google login; any other provider they disconnect freely. bex-api's settings
 * guard (`lego/backend/internal/settingsguard`) gets neither the request body
 * nor the identity's credentials from Kratos, only the flow's transient
 * payload (`scripts/lib/kratos-oidc-fragment.sh`), so an unlink that does not
 * name its provider is refused like Google's. Every other submit sends none.
 */
export function settingsTransientPayload(
  values: FormValues,
): Record<string, unknown> {
  const { method, unlink } = values;
  return method === "oidc" && typeof unlink === "string" && unlink !== ""
    ? { unlink }
    : {};
}
