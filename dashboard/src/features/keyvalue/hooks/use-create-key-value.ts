import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { CreateKeyValueDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

/** The create form's collected values (Render's `/new/redis` subset bex serves). */
export interface CreateKeyValueInput {
  name: string;
  plan: string;
  /** Valkey version, or "" to let the operator pick its default. */
  version: string;
  public: boolean;
}

export interface UseCreateKeyValueResult {
  /** Fires createKeyValue; resolves the new id on success, null on failure. */
  create: (input: CreateKeyValueInput) => Promise<string | null>;
  busy: boolean;
}

/**
 * Wires the create form to bex-api's `createKeyValue`. The mutation returns as
 * soon as the KeyValue CR is written (status "creating"); the operator then
 * provisions the Valkey StatefulSet, so the toast says the store is being
 * created rather than implying it's instantly ready — the list/detail poll it
 * to Available. Empty optional fields are omitted so the operator applies its
 * own defaults. Mirrors databases' `useCreateDatabase`.
 */
export function useCreateKeyValue(): UseCreateKeyValueResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(CreateKeyValueDocument);
  const [busy, setBusy] = useState(false);

  const create = useCallback(
    async (input: CreateKeyValueInput) => {
      setBusy(true);
      try {
        const res = await mutate({
          variables: {
            name: input.name,
            plan: input.plan || undefined,
            version: input.version || undefined,
            public: input.public,
          },
        });
        const id = res.data?.createKeyValue?.id ?? input.name;
        toast.success(t("keyvalue.createSuccess", { name: input.name }));
        return id;
      } catch {
        toast.error(t("keyvalue.createError", { name: input.name }));
        return null;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { create, busy };
}
