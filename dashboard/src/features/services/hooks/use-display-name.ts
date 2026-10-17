import { useCallback, useEffect, useRef, useState } from "react";
import type { ApolloClient } from "@apollo/client";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { useRouter } from "@tanstack/react-router";
import { toast } from "sonner";
import {
  ServerDocument,
  ServicesDocument,
  SetDisplayNameDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";

export interface UseDisplayNameResult {
  /**
   * Fires setDisplayName exactly once. Resolves true when the server accepted
   * the label — even if the follow-up refresh failed, which is reported
   * separately and never renames again — and false when it refused.
   */
  setDisplayName: (id: string, displayName: string) => Promise<boolean>;
  /** True from submit until the saved label is published (or its refresh failed). */
  busy: boolean;
  /** The label was saved but re-reading it failed; editing waits for a retry. */
  unpublished: boolean;
}

/**
 * Re-read a renamed service so every view publishes the saved label (w4/m225):
 * an older Server or Services read can land after the write and restore the
 * previous label, so settle every active one first, then read the service
 * afresh. The project rename's sequence (w4/m186, refreshRenamedProject).
 */
async function refreshRenamedService(client: ApolloClient, id: string) {
  const reads = client.refetchQueries({
    include: [ServerDocument, ServicesDocument],
    // Only an in-flight read can land after the write; an idle view already
    // took the label from the mutation result. Services lists have no id.
    onQueryUpdated: (query) => {
      const { id: readId } = query.variables as { id?: string };
      return (
        query.getCurrentResult().loading &&
        (readId === undefined || readId === id)
      );
    },
  });
  reads.catch(() => undefined); // each result is settled below
  await Promise.allSettled(reads.results);
  await client.query({
    query: ServerDocument,
    variables: { id },
    fetchPolicy: "network-only",
    context: { queryDeduplication: false },
  });
}

/** Wires the Settings name control to the App's mutable spec.displayName. */
export function useDisplayName(): UseDisplayNameResult {
  const { t } = useTranslations();
  const client = useApolloClient();
  const router = useRouter();
  const [mutate] = useMutation(SetDisplayNameDocument);
  const [busy, setBusy] = useState(false);
  const [unpublished, setUnpublished] = useState(false);
  // A publication that finishes after the row unmounted (the customer left
  // for another service) still refreshes the cache, but toasts and refreshes
  // nothing on the page they moved to.
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  // The write already committed: a failed refresh is never a failed rename,
  // and its retry re-reads without renaming again. Until it succeeds the row
  // stays uneditable rather than offering edits over a stale label.
  const publish = useCallback(
    (id: string, displayName: string, label: string): Promise<void> => {
      const attempt = async () => {
        try {
          await refreshRenamedService(client, id);
        } catch {
          if (!mounted.current) return;
          setUnpublished(true);
          toast.warning(
            t("services.displayNameRefreshFailed", { name: label }),
            {
              action: {
                label: t("common.tryAgain"),
                onClick: () => void attempt(),
              },
            },
          );
          return;
        }
        if (!mounted.current) return;
        setUnpublished(false);
        await router.invalidate().catch(() => undefined);
        toast.success(
          displayName
            ? t("services.displayNameSuccess", { name: displayName })
            : t("services.displayNameCleared"),
        );
      };
      return attempt();
    },
    [client, router, t],
  );

  const setDisplayName = useCallback(
    async (id: string, displayName: string) => {
      setBusy(true);
      try {
        let saved: string | null | undefined;
        try {
          const result = await mutate({ variables: { id, displayName } });
          saved = result.data?.setDisplayName?.name;
        } catch (err) {
          toast.error(
            mutationErrorMessage(err, t("services.displayNameError")),
          );
          return false;
        }
        // A cleared label falls back to the immutable name, never the id.
        await publish(id, displayName, displayName || saved || id);
        return true;
      } finally {
        setBusy(false);
      }
    },
    [mutate, publish, t],
  );

  return { setDisplayName, busy, unpublished };
}
