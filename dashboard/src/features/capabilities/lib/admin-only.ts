// A protected environment's refusal of a non-Admin (w3/m281, w4/m594): bex-api
// codes it PROTECTED_ENVIRONMENT_ADMIN_REQUIRED on every surface, so a raced
// click reads the same reason the disabled control shows.

import type { Translate } from "@/common/hooks/use-translations";
import {
  hasGraphQLErrorCode,
  mutationErrorMessage,
} from "@/common/lib/graphql-error";
import { PROTECTED_PLACEMENT_CODE } from "@/features/environments/lib/placement";

/** Whether err is bex-api's protected-environment refusal of a non-Admin. */
export function isAdminOnlyRefusal(err: unknown): boolean {
  return hasGraphQLErrorCode(err, PROTECTED_PLACEMENT_CODE);
}

/**
 * mutationErrorMessage, except that the protected-environment refusal reads
 * as Render's "This action is limited to admins."
 */
export function adminOnlyErrorMessage(
  err: unknown,
  generic: string,
  t: Translate,
): string {
  return isAdminOnlyRefusal(err)
    ? t("capabilities.actionAdminOnly")
    : mutationErrorMessage(err, generic);
}
