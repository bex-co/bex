import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import type { TypedDocumentNode } from "@graphql-typed-document-node/core";
import { toast } from "sonner";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * The shape every single-field service setting shares: fire one mutation, toast
 * the outcome either way, expose a `busy` flag while it is in flight, and
 * resolve true only on success.
 *
 * Each setting still gets its own named hook — the name and its doc comment are
 * what a reader looks for — but the body lives here once instead of being
 * retyped per field, which is how the toast keys and the `finally { setBusy }`
 * discipline stay identical across the Settings surface.
 *
 * Settings whose messaging is conditional (auto-deploy's on/off wording,
 * maintenance mode, a cleared-vs-set credential) deliberately keep their own
 * bodies: forcing them through here would mean passing a key-picking callback,
 * which is longer than the code it replaces.
 */
export function useFieldMutation<
  TData,
  TVariables extends Record<string, unknown>,
  TArgs extends unknown[],
>(
  document: TypedDocumentNode<TData, TVariables>,
  toVariables: (...args: TArgs) => TVariables,
  keys: { success: string; error: string },
): { run: (...args: TArgs) => Promise<boolean>; busy: boolean } {
  const { t } = useTranslations();
  const [mutate] = useMutation(document);
  const [busy, setBusy] = useState(false);

  const run = useCallback(
    async (...args: TArgs) => {
      setBusy(true);
      try {
        await mutate({ variables: toVariables(...args) });
        toast.success(t(keys.success));
        return true;
      } catch {
        toast.error(t(keys.error));
        return false;
      } finally {
        setBusy(false);
      }
    },
    // toVariables and keys are call-site literals, stable per hook instance.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [mutate, t],
  );

  return { run, busy };
}
