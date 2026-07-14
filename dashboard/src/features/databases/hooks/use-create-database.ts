import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { CreateDatabaseDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { graphQLErrorMessage } from "@/common/lib/graphql-error";

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
  /** Workspace Postgres cap hit — show inline with upgrade CTA (w7/m9). */
  capLimit: string | null;
}

/**
 * Wires the create dialog to bex-api's `createDatabase`. The mutation returns as
 * soon as the Database CR is written (status "creating"); the operator then
 * provisions the CNPG cluster, so the toast says the DB is being created rather
 * than implying it's instantly ready — the list/detail poll it to Available.
 * Empty optional fields are omitted so the operator applies its own defaults.
 *
 * Scoped to the switcher's selected workspace (w6/m14): `ownerId` names the
 * workspace the database is created in, mirroring `useDatabases`'s list read.
 * A create is refused (never sent with a null ownerId, which the backend would
 * silently route to the caller's default workspace) until the workspace list
 * resolves — the list hooks' `skip` in mutation form.
 */
export function useCreateDatabase(): UseCreateDatabaseResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(CreateDatabaseDocument);
  const [busy, setBusy] = useState(false);
  const [capLimit, setCapLimit] = useState<string | null>(null);

  const create = useCallback(
    async (input: CreateDatabaseInput) => {
      if (currentWorkspaceId == null) {
        toast.error(t("databases.createError", { name: input.name }));
        return null;
      }
      setBusy(true);
      setCapLimit(null);
      try {
        const res = await mutate({
          variables: {
            name: input.name,
            ownerId: currentWorkspaceId,
            plan: input.plan || undefined,
            version: input.version || undefined,
            diskSizeGB: input.diskSizeGB > 0 ? input.diskSizeGB : undefined,
            public: input.public,
          },
        });
        const id = res.data?.createDatabase?.id ?? input.name;
        toast.success(t("databases.createSuccess", { name: input.name }));
        return id;
      } catch (err) {
        const msg = graphQLErrorMessage(err) ?? "";
        if (msg.toLowerCase().includes("workspace is limited")) {
          setCapLimit(msg);
        } else {
          toast.error(t("databases.createError", { name: input.name }));
        }
        return null;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t, currentWorkspaceId],
  );

  return { create, busy, capLimit };
}
