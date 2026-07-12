import { useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { ScaleServiceDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseScaleServiceResult {
  scaleService: (id: string, numInstances: number) => Promise<boolean>;
  busy: boolean;
}

export function useScaleService(): UseScaleServiceResult {
  const { t } = useTranslations();
  const [mutate, { loading: busy }] = useMutation(ScaleServiceDocument);

  const scaleService = useCallback(
    async (id: string, numInstances: number) => {
      try {
        await mutate({ variables: { id, numInstances } });
        toast.success(t("services.scaleSuccess", { count: numInstances }));
        return true;
      } catch {
        toast.error(t("services.scaleError"));
        return false;
      }
    },
    [mutate, t],
  );

  return { scaleService, busy };
}
