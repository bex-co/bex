import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { CreateKeyValueDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  conflictOrGenericMessage,
  graphQLErrorMessage,
} from "@/common/lib/graphql-error";
import { usePaymentRequiredGate } from "@/features/usage/context/payment-required-context";
import { isPaymentOnboardingCancelled } from "@/features/usage/context/payment-required-error";

/** The create form's collected values (Render's `/new/redis` subset bex serves). */
export interface CreateKeyValueInput {
  name: string;
  plan: string;
  /** Valkey version, or "" to let the operator pick its default. */
  version: string;
  public: boolean;
  /** Eviction policy at the memory budget (Render's Maxmemory Policy). */
  maxmemoryPolicy: string;
  /** Persistence mode (Render's Persistence Mode): journal-snapshot|snapshot|off. */
  persistenceMode: string;
  /** Optional Environment; the server also joins its parent Project. */
  environmentId?: string;
}

export interface UseCreateKeyValueResult {
  /** Fires createKeyValue; resolves the new id on success, null on failure. */
  create: (input: CreateKeyValueInput) => Promise<string | null>;
  busy: boolean;
  /** Workspace key-value cap hit — show inline with upgrade CTA (w7/m9). */
  capLimit: string | null;
}

/**
 * Wires the create form to bex-api's `createKeyValue`. The mutation returns as
 * soon as the KeyValue CR is written (status "creating"); the operator then
 * provisions the Valkey StatefulSet, so the toast says the store is being
 * created rather than implying it's instantly ready — the list/detail poll it
 * to Available. Empty optional fields are omitted so the operator applies its
 * own defaults. Mirrors databases' `useCreateDatabase`, including the workspace
 * scoping (w6/m14): `ownerId` names the workspace the store is created in — the
 * switcher's selection, as in `useKeyValues`'s list read — and a create is
 * refused (never sent with a null ownerId, which the backend would silently
 * route to the caller's default workspace) until the workspace list resolves.
 */
export function useCreateKeyValue(): UseCreateKeyValueResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(CreateKeyValueDocument);
  const [busy, setBusy] = useState(false);
  const [capLimit, setCapLimit] = useState<string | null>(null);
  const paymentGate = usePaymentRequiredGate();

  const create = useCallback(
    async (input: CreateKeyValueInput) => {
      if (currentWorkspaceId == null) {
        toast.error(t("keyvalue.createError", { name: input.name }));
        return null;
      }
      setBusy(true);
      setCapLimit(null);
      try {
        const res = await paymentGate.run(() =>
          mutate({
            variables: {
              name: input.name,
              ownerId: currentWorkspaceId,
              environmentId: input.environmentId,
              plan: input.plan || undefined,
              version: input.version || undefined,
              public: input.public,
              maxmemoryPolicy: input.maxmemoryPolicy || undefined,
              persistenceMode: input.persistenceMode || undefined,
            },
          }),
        );
        const id = res.data?.createKeyValue?.id;
        if (!id) throw new Error("createKeyValue returned no id");
        toast.success(t("keyvalue.createSuccess", { name: input.name }));
        return id;
      } catch (err) {
        if (isPaymentOnboardingCancelled(err)) return null;
        const msg = graphQLErrorMessage(err) ?? "";
        if (msg.toLowerCase().includes("workspace is limited")) {
          setCapLimit(msg);
        } else {
          toast.error(
            conflictOrGenericMessage(
              err,
              t("keyvalue.createError", { name: input.name }),
            ),
          );
        }
        return null;
      } finally {
        setBusy(false);
      }
    },
    [mutate, paymentGate, t, currentWorkspaceId],
  );

  return { create, busy, capLimit };
}
