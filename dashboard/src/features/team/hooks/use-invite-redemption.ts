import { useEffect, useRef, useState } from "react";
import { useApolloClient, useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  AcceptWorkspaceInviteDocument,
  WorkspaceInvitePreviewDocument,
  WorkspacesDocument,
  ViewerCapabilitiesDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { takePendingInviteToken } from "@/common/lib/invite-token";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { classifyInviteRedemptionError } from "./invite-redemption-error";

/** Token acceptance is always an explicit action. A committed acceptance is
 * never replayed if refreshing workspace access fails; retry only opens it. */
export function useInviteRedemption(token: string, onOpened: () => void) {
  const { t } = useTranslations();
  const client = useApolloClient();
  const { workspaces, setCurrentWorkspaceId } = useWorkspace();
  const [acceptMut] = useMutation(AcceptWorkspaceInviteDocument);
  const preview = useQuery(WorkspaceInvitePreviewDocument, {
    variables: { token },
    fetchPolicy: "no-cache",
    notifyOnNetworkStatusChange: true,
  });
  const [busy, setBusy] = useState(false);
  const locked = useRef(false);
  const [committedWorkspace, setCommittedWorkspace] = useState<string | null>(
    null,
  );
  const [readyToOpen, setReadyToOpen] = useState<string | null>(null);
  const opened = useRef(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const details = preview.data?.workspaceInvitePreview;

  async function openWorkspace(workspaceId: string) {
    const result = await client.query({
      query: WorkspacesDocument,
      fetchPolicy: "network-only",
    });
    if (!result.data?.workspaces?.some((w) => w?.id === workspaceId)) {
      throw new Error("Workspace access is not ready");
    }
    const access = await client.query({
      query: ViewerCapabilitiesDocument,
      variables: { ownerId: workspaceId },
      fetchPolicy: "network-only",
    });
    if (!access.data?.viewerCapabilities?.canView) {
      throw new Error("Workspace authorization is not ready");
    }
    setReadyToOpen(workspaceId);
  }

  // The query promise resolves before WorkspaceProvider necessarily receives
  // its cache notification. Selecting sooner lets its stale-list fallback
  // immediately switch back to the personal workspace. Wait for that provider
  // to observe the new membership before selecting and leaving review.
  useEffect(() => {
    if (
      !readyToOpen ||
      opened.current ||
      !workspaces.some((w) => w.id === readyToOpen)
    )
      return;
    opened.current = true;
    setCurrentWorkspaceId(readyToOpen);
    takePendingInviteToken();
    onOpened();
  }, [readyToOpen, workspaces, setCurrentWorkspaceId, onOpened]);

  async function accept() {
    if (locked.current || !details?.workspaceId) return;
    locked.current = true;
    setBusy(true);
    setActionError(null);
    let destination =
      committedWorkspace ??
      (details.alreadyMember ? details.workspaceId : null);
    try {
      if (!destination) {
        try {
          const result = await acceptMut({ variables: { token } });
          destination = result.data?.acceptWorkspaceInvite?.workspaceId ?? null;
          if (!destination) throw new Error("Missing invitation result");
          setCommittedWorkspace(destination);
          toast.success(
            t("team.inviteAccepted", {
              workspace: details.workspaceName ?? "",
            }),
          );
        } catch (error) {
          // Authentication's email-match path or another tab may have joined
          // since preview. Refresh authoritative membership before offering Open.
          if (classifyInviteRedemptionError(error) !== "already-accepted") {
            setActionError(invitationErrorKey(error));
            return;
          }
          try {
            const refreshed = await preview.refetch();
            const membership = refreshed.data?.workspaceInvitePreview;
            if (!membership?.alreadyMember || !membership.workspaceId) {
              setActionError("invites.used");
              return;
            }
            destination = membership.workspaceId;
            setCommittedWorkspace(destination);
          } catch (refreshError) {
            setActionError(invitationErrorKey(refreshError));
            return;
          }
        }
      }
      try {
        await openWorkspace(destination);
      } catch {
        setActionError("invites.accessPending");
      }
    } finally {
      locked.current = false;
      setBusy(false);
    }
  }

  const terminalAction =
    actionError === "invites.invalid" ||
    actionError === "invites.expired" ||
    actionError === "invites.used";
  return {
    details: terminalAction ? null : details,
    loading: preview.loading,
    errorKey:
      actionError ?? (preview.error ? invitationErrorKey(preview.error) : null),
    retryable:
      !terminalAction &&
      (!preview.error ||
        classifyInviteRedemptionError(preview.error) === "ambiguous"),
    busy,
    joined: Boolean(committedWorkspace || details?.alreadyMember),
    accept,
    retry: () => {
      setActionError(null);
      void preview.refetch().catch(() => undefined);
    },
  };
}

function invitationErrorKey(error: unknown): string {
  switch (classifyInviteRedemptionError(error)) {
    case "already-accepted":
      return "invites.used";
    case "expired":
      return "invites.expired";
    case "plan-limit":
      return "invites.planLimit";
    case "terminal":
      return "invites.invalid";
    default:
      return "invites.retryError";
  }
}
