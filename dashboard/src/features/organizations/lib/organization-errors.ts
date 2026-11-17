import type { Translate } from "@/common/hooks/use-translations";
import {
  graphQLErrorCodes,
  graphQLErrorExtensions,
  mutationErrorMessage,
} from "@/common/lib/graphql-error";

/**
 * bex-api's org refusals (ADR115), each in the user's words. The server's
 * English message is the fallback only for a code this table does not know.
 */
const CODE_KEYS: Record<string, string> = {
  ORGANIZATION_NOT_FOUND: "organizations.errorNotFound",
  FORBIDDEN: "organizations.errorForbidden",
  ORGANIZATION_LAST_OWNER: "organizations.errorLastOwner",
  ORGANIZATION_ALREADY_MEMBER: "organizations.errorAlreadyMember",
  ORGANIZATION_GUEST_SINGLE_WORKSPACE:
    "organizations.errorGuestSingleWorkspace",
  CANNOT_CHANGE_OWN_ROLE: "organizations.errorOwnRole",
  CANNOT_REMOVE_SELF: "organizations.errorRemoveSelf",
  INVALID_PRIVACY: "organizations.errorInvalidPrivacy",
};

/**
 * The sentence an org mutation's failure shows. ORGANIZATION_MEMBER_LAST_ADMIN
 * names the workspace by its id (`extensions.workspaceId`), which
 * `workspaceName` turns into the name the org pages show.
 */
export function organizationErrorMessage(
  err: unknown,
  t: Translate,
  fallback: string,
  workspaceName: (id: string) => string | undefined = () => undefined,
): string {
  const lastAdmin = graphQLErrorExtensions(
    err,
    "ORGANIZATION_MEMBER_LAST_ADMIN",
  );
  if (lastAdmin) {
    const id =
      typeof lastAdmin["workspaceId"] === "string"
        ? lastAdmin["workspaceId"]
        : "";
    return t("organizations.errorMemberLastAdmin", {
      workspace: workspaceName(id) ?? id,
    });
  }
  const known = graphQLErrorCodes(err).find((code) => code in CODE_KEYS);
  return known ? t(CODE_KEYS[known]) : mutationErrorMessage(err, fallback);
}
