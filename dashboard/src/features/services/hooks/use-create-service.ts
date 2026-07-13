import { useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { CreateServiceDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface CreateServiceInput {
  name: string;
  type?: string;
  repo?: string;
  image?: string;
  branch?: string;
  rootDir?: string;
  plan?: string;
  autoDeploy?: boolean;
  schedule?: string;
  command?: string;
  publishPath?: string;
}

export interface UseCreateServiceResult {
  create: (input: CreateServiceInput) => Promise<string | null>;
  busy: boolean;
}

/**
 * Wires the create wizard to bex-api's `createService`, scoped to the switcher's
 * selected workspace (w6/m14): `ownerId` names the workspace the service is
 * created in, mirroring `useServices`'s list read. As there the selection is the
 * gate — a create is refused (never sent with a null ownerId, which the backend
 * would silently route to the caller's default workspace) until the workspace
 * list resolves; that's the list hooks' `skip` in mutation form.
 */
export function useCreateService(): UseCreateServiceResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate, { loading: busy }] = useMutation(CreateServiceDocument);

  const create = useCallback(
    async (input: CreateServiceInput) => {
      if (currentWorkspaceId == null) {
        toast.error(t("services.createError", { name: input.name }));
        return null;
      }
      try {
        const res = await mutate({
          variables: {
            name: input.name,
            ownerId: currentWorkspaceId,
            type: input.type,
            repo: input.repo,
            image: input.image,
            branch: input.branch,
            rootDir: input.rootDir,
            plan: input.plan,
            autoDeploy: input.autoDeploy,
            schedule: input.schedule,
            command: input.command,
            publishPath: input.publishPath,
          },
        });
        const id = res.data?.createService?.id ?? input.name;
        toast.success(t("services.createSuccess", { name: input.name }));
        return id;
      } catch {
        toast.error(t("services.createError", { name: input.name }));
        return null;
      }
    },
    [mutate, t, currentWorkspaceId],
  );

  return { create, busy };
}
