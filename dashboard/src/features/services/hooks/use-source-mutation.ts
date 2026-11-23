import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import type { TypedDocumentNode } from "@graphql-typed-document-node/core";
import { toast } from "sonner";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useAskForProtectedConfirmation } from "@/common/providers/protected-retry-context";
import {
  ProtectedConfirmationDismissed,
  withProtectedRetry,
} from "@/features/services/lib/protected-confirmation";

export type SourceMutationResult =
  | { ok: true; deployId: string | null }
  | { ok: false };

/**
 * Shared mutation runner for Settings → Update Source (w4/m593): executes a
 * source-update mutation (setRepo, setImage, linkBuildSource), extracts the
 * optional deploy id returned when the backend immediately triggered one, and
 * toasts either the deploying or saved-only outcome. Keeps the dialog open on
 * refusal and exposes `lastError` for inline rendering.
 */
export function useSourceMutation<
  TData,
  TVariables extends Record<string, unknown>,
  TArgs extends unknown[],
>(
  document: TypedDocumentNode<TData, TVariables>,
  toVariables: (...args: TArgs) => TVariables,
  extractDeployId: (data: TData | null | undefined) => string | null,
  options?: {
    withConfirm?: boolean;
  },
): {
  run: (...args: TArgs) => Promise<SourceMutationResult>;
  busy: boolean;
  lastError: string | null;
} {
  const { t } = useTranslations();
  const [mutate] = useMutation(document);
  const [busy, setBusy] = useState(false);
  const [lastError, setLastError] = useState<string | null>(null);
  const askForConfirmation = useAskForProtectedConfirmation();
  const withConfirm = options?.withConfirm ?? true;

  const run = useCallback(
    async (...args: TArgs): Promise<SourceMutationResult> => {
      const variables = toVariables(...args);
      setBusy(true);
      setLastError(null);
      try {
        const res = withConfirm
          ? await withProtectedRetry(askForConfirmation, (confirm) =>
              mutate({ variables: { ...variables, confirm } }),
            )
          : await mutate({ variables });
        const deployId = extractDeployId(res.data);
        toast.success(
          t(
            deployId
              ? "services.sourceUpdateDeploying"
              : "services.sourceUpdateSuccess",
          ),
        );
        return { ok: true, deployId };
      } catch (err) {
        if (err instanceof ProtectedConfirmationDismissed) return { ok: false };
        const fallback = t("services.sourceUpdateError");
        setLastError(mutationErrorMessage(err, fallback));
        toast.error(mutationErrorMessage(err, fallback));
        return { ok: false };
      } finally {
        setBusy(false);
      }
    },
    // toVariables and extractDeployId are fixed call-site definitions.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [mutate, t, askForConfirmation, withConfirm],
  );

  return { run, busy, lastError };
}
