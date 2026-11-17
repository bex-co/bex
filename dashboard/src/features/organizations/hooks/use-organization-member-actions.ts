import { useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import {
  ChangeOrganizationMemberRoleDocument,
  InviteOrganizationMemberDocument,
  RemoveOrganizationMemberDocument,
  RevokeOrganizationInviteDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useKeyedOrganizationAction,
  useOrganizationDialogAction,
} from "@/features/organizations/hooks/use-organization-action";
import {
  ORGANIZATION_ROLE_LABEL_KEYS,
  type OrganizationRole,
} from "@/features/organizations/types";

/** The member list and the pending invites, refetched where mounted. */
const MEMBER_QUERIES = {
  refetchQueries: ["OrganizationMembers", "OrganizationInvites"],
  awaitRefetchQueries: true,
};

export interface InviteOrganizationMemberResult {
  invite: (email: string, role: OrganizationRole) => Promise<boolean>;
  busy: boolean;
  refusal: string | null;
  clearRefusal: () => void;
}

/** An Owner invites someone to the org itself, as Owner or Member (ADR115 D4). */
export function useInviteOrganizationMember(
  organizationId: string,
): InviteOrganizationMemberResult {
  const { t } = useTranslations();
  const [mutate, { loading }] = useMutation(
    InviteOrganizationMemberDocument,
    MEMBER_QUERIES,
  );
  const { run, refusal, clearRefusal } = useOrganizationDialogAction();

  const invite = useCallback(
    (email: string, role: OrganizationRole) =>
      run(async () => {
        await mutate({ variables: { organizationId, email, role } });
        return t("organizations.invited", { email });
      }, t("organizations.inviteFailed")),
    [run, mutate, organizationId, t],
  );

  return { invite, busy: loading, refusal, clearRefusal };
}

export interface OrganizationMemberActionsResult {
  changeRole: (
    subject: string,
    identity: string,
    role: OrganizationRole,
  ) => Promise<boolean>;
  remove: (subject: string, identity: string) => Promise<boolean>;
  revokeInvite: (inviteId: string, email: string) => Promise<boolean>;
  /** The subject or invite id an action is in flight for, or null. */
  busy: string | null;
}

/**
 * An Owner's member controls: change someone's org role, remove them from the
 * org (and so from every org workspace), revoke a pending invite. Each refusal
 * is a toast in the user's words; ORGANIZATION_MEMBER_LAST_ADMIN names the
 * workspace through `workspaceName`.
 */
export function useOrganizationMemberActions(
  organizationId: string,
  workspaceName: (id: string) => string | undefined,
): OrganizationMemberActionsResult {
  const { t } = useTranslations();
  const [changeRoleMutation] = useMutation(
    ChangeOrganizationMemberRoleDocument,
    MEMBER_QUERIES,
  );
  const [removeMutation] = useMutation(
    RemoveOrganizationMemberDocument,
    MEMBER_QUERIES,
  );
  const [revokeMutation] = useMutation(
    RevokeOrganizationInviteDocument,
    MEMBER_QUERIES,
  );
  const { busy, run } = useKeyedOrganizationAction(workspaceName);

  const changeRole = useCallback(
    (subject: string, identity: string, role: OrganizationRole) =>
      run(
        subject,
        () =>
          changeRoleMutation({
            variables: { organizationId, subject, role },
          }),
        t("organizations.roleChanged", {
          identity,
          role: t(ORGANIZATION_ROLE_LABEL_KEYS[role]),
        }),
        t("organizations.roleChangeFailed"),
      ),
    [run, changeRoleMutation, organizationId, t],
  );

  const remove = useCallback(
    (subject: string, identity: string) =>
      run(
        subject,
        () => removeMutation({ variables: { organizationId, subject } }),
        t("organizations.removed", { identity }),
        t("organizations.removeFailed", { identity }),
      ),
    [run, removeMutation, organizationId, t],
  );

  const revokeInvite = useCallback(
    (inviteId: string, email: string) =>
      run(
        inviteId,
        () => revokeMutation({ variables: { organizationId, inviteId } }),
        t("organizations.inviteRevoked", { email }),
        t("organizations.revokeFailed"),
      ),
    [run, revokeMutation, organizationId, t],
  );

  return { changeRole, remove, revokeInvite, busy };
}
