import { useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { CreateServiceDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

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

export function useCreateService(): UseCreateServiceResult {
  const { t } = useTranslations();
  const [mutate, { loading: busy }] = useMutation(CreateServiceDocument);

  const create = useCallback(
    async (input: CreateServiceInput) => {
      try {
        const res = await mutate({
          variables: {
            name: input.name,
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
    [mutate, t],
  );

  return { create, busy };
}
