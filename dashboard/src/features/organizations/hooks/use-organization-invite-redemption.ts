import { useRef, useState } from "react";
import { useApolloClient, useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  AcceptOrganizationInviteDocument,
  OrganizationInvitePreviewDocument,
  OrganizationsDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { takePendingInvite } from "@/common/lib/invite-token";
import {
  classifyInviteRedemptionError,
  invitationErrorKey,
} from "@/features/team/hooks/invite-redemption-error";

const TERMINAL_ERRORS = new Set([
  "invites.orgInvalid",
  "invites.orgExpired",
  "invites.used",
]);

/**
 * An organization invite's emailed link (w7/083), reviewed first and accepted
 * only by an explicit action, for whoever is signed in. A committed acceptance
 * is never replayed: if the caller's organizations do not list the org yet,
 * retrying only opens it.
 */
export function useOrganizationInviteRedemption(
  token: string,
  onOpened: (organizationId: string) => void,
) {
  const { t } = useTranslations();
  const client = useApolloClient();
  const [acceptMut] = useMutation(AcceptOrganizationInviteDocument);
  const preview = useQuery(OrganizationInvitePreviewDocument, {
    variables: { token },
    fetchPolicy: "no-cache",
    notifyOnNetworkStatusChange: true,
  });
  const [busy, setBusy] = useState(false);
  const locked = useRef(false);
  const [committed, setCommitted] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const details = preview.data?.organizationInvitePreview;

  // Refreshing the caller's organizations also puts the new org in the
  // switcher before its pages load.
  async function openOrganization(organizationId: string) {
    const result = await client.query({
      query: OrganizationsDocument,
      fetchPolicy: "network-only",
    });
    if (!result.data?.organizations?.some((o) => o.id === organizationId)) {
      throw new Error("Organization access is not ready");
    }
    takePendingInvite();
    onOpened(organizationId);
  }

  async function acceptInvitation(): Promise<string | null> {
    try {
      const result = await acceptMut({ variables: { token } });
      const organizationId =
        result.data?.acceptOrganizationInvite?.organizationId;
      if (!organizationId) throw new Error("Missing invitation result");
      setCommitted(organizationId);
      toast.success(
        t("invites.orgAccepted", {
          organization: details?.organizationName ?? "",
        }),
      );
      return organizationId;
    } catch (error) {
      if (classifyInviteRedemptionError(error) !== "already-accepted") {
        setActionError(invitationErrorKey(error, "organization"));
        return null;
      }
    }

    // Signing in with the invited email, or another tab, may have joined
    // since the preview. Ask again before offering Open.
    try {
      const refreshed = await preview.refetch();
      const membership = refreshed.data?.organizationInvitePreview;
      if (!membership?.alreadyMember) {
        setActionError("invites.used");
        return null;
      }
      setCommitted(membership.organizationId);
      return membership.organizationId;
    } catch (error) {
      setActionError(invitationErrorKey(error, "organization"));
      return null;
    }
  }

  async function accept() {
    if (locked.current || !details?.organizationId) return;
    locked.current = true;
    setBusy(true);
    setActionError(null);
    try {
      let destination =
        committed ?? (details.alreadyMember ? details.organizationId : null);
      if (!destination) destination = await acceptInvitation();
      if (!destination) return;
      try {
        await openOrganization(destination);
      } catch {
        setActionError("invites.orgAccessPending");
      }
    } finally {
      locked.current = false;
      setBusy(false);
    }
  }

  const terminalAction =
    actionError !== null && TERMINAL_ERRORS.has(actionError);
  return {
    details: terminalAction ? null : details,
    loading: preview.loading,
    errorKey:
      actionError ??
      (preview.error
        ? invitationErrorKey(preview.error, "organization")
        : null),
    retryable:
      !terminalAction &&
      (!preview.error ||
        classifyInviteRedemptionError(preview.error) === "ambiguous"),
    busy,
    joined: Boolean(committed || details?.alreadyMember),
    accept,
    retry: () => {
      setActionError(null);
      void preview.refetch().catch(() => undefined);
    },
  };
}
