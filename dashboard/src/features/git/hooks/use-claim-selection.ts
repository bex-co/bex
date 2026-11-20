import { useCallback, useState } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  GitClaimSelectionDocument,
  SelectGitClaimDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface ClaimCandidate {
  installationId: number;
  accountLogin: string;
  /** Already connected to this workspace: shown marked, not offered (ADR078 §3b). */
  connected: boolean;
}

export interface UseClaimSelectionResult {
  candidates: ClaimCandidate[];
  /** True only while the first read is in flight — never during a refetch. */
  loading: boolean;
  /**
   * The selection is unknown, expired, or already spent. Distinct from a live
   * selection with no candidates, which the chooser renders as "nothing
   * installed yet" plus the install action (ADR078 §3b).
   */
  gone: boolean;
  /** Binds one already-proved option; resolves true when it landed. */
  select: (installationId: number) => Promise<boolean>;
  selecting: boolean;
}

/**
 * Reads and completes a claim's deferred choice (ADR078 §3a/§3b).
 *
 * The callback recorded the set it had ALREADY proved — every installed account
 * the user administers, possibly none — and sent the browser back with a
 * selection id. This hook renders that set and spends it.
 *
 * `ownerId` is threaded per ADR078 §6: the selection belongs to the workspace the
 * claim was started for, and the backend refuses a mismatch, so the query defers
 * while the workspace id is still resolving rather than asking about the default.
 */
export function useClaimSelection(
  selectionId: string | undefined,
): UseClaimSelectionResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [selecting, setSelecting] = useState(false);

  // errorPolicy "all" keeps a refused/expired selection as an empty result
  // rather than a thrown error: the picker's `gone` branch is the user-facing
  // recovery, and a bounded refusal must read the same as an expired one.
  const { data, loading } = useQuery(GitClaimSelectionDocument, {
    variables: { ownerId: currentWorkspaceId ?? "", id: selectionId ?? "" },
    skip: !selectionId || currentWorkspaceId == null,
    fetchPolicy: "no-cache",
    errorPolicy: "all",
  });

  const [mutate] = useMutation(SelectGitClaimDocument, {
    fetchPolicy: "no-cache",
  });

  const select = useCallback(
    async (installationId: number) => {
      if (!selectionId || currentWorkspaceId == null) return false;
      setSelecting(true);
      try {
        await mutate({
          variables: {
            ownerId: currentWorkspaceId,
            selectionId,
            installationId,
          },
        });
        return true;
      } catch (err) {
        toast.error(mutationErrorMessage(err, t("git.selectClaimError")));
        return false;
      } finally {
        setSelecting(false);
      }
    },
    [mutate, selectionId, currentWorkspaceId, t],
  );

  const selection = data?.gitClaimSelection;
  const candidates = (selection?.candidates ?? []).flatMap((c) =>
    c?.installationId != null && c.accountLogin
      ? [
          {
            installationId: c.installationId,
            accountLogin: c.accountLogin,
            connected: c.connected === true,
          },
        ]
      : [],
  );

  return {
    candidates,
    // Gate on "loading and no data yet" (dashboard/AGENTS.md § Polling) so a
    // refetch never unmounts a picker the user is already looking at.
    loading: loading && data === undefined,
    // A refused read resolves to a null selection (errorPolicy "all"); a live
    // one with an empty list is not gone.
    gone: Boolean(selectionId) && !loading && selection == null,
    select,
    selecting,
  };
}
