import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { CreateDatabaseDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

/** The create form's collected values (Render's create-form subset bex serves). */
export interface CreateDatabaseInput {
  name: string;
  plan: string;
  /** PostgreSQL major version, or "" to let the operator pick its default. */
  version: string;
  diskSizeGB: number;
  public: boolean;
}

export interface UseCreateDatabaseResult {
  /** Fires createDatabase; resolves the new id on success, null on failure. */
  create: (input: CreateDatabaseInput) => Promise<string | null>;
  busy: boolean;
}

/**
 * Wires the create dialog to bex-api's `createDatabase`. The mutation returns as
 * soon as the Database CR is written (status "creating"); the operator then
 * provisions the CNPG cluster, so the toast says the DB is being created rather
 * than implying it's instantly ready — the list/detail poll it to Available.
 * Empty optional fields are omitted so the operator applies its own defaults.
 */
export function useCreateDatabase(): UseCreateDatabaseResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(CreateDatabaseDocument);
  const [busy, setBusy] = useState(false);

  const create = useCallback(
    async (input: CreateDatabaseInput) => {
      setBusy(true);
      try {
        const res = await mutate({
          variables: {
            name: input.name,
            plan: input.plan || undefined,
            version: input.version || undefined,
            diskSizeGB: input.diskSizeGB > 0 ? input.diskSizeGB : undefined,
            public: input.public,
          },
        });
        const id = res.data?.createDatabase?.id ?? input.name;
        toast.success(t("databases.createSuccess", { name: input.name }));
        return id;
      } catch {
        toast.error(t("databases.createError", { name: input.name }));
        return null;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { create, busy };
}
