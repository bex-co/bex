import { useCallback, useState } from "react";
import { useQuery, useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  AutoscalingConfigDocument,
  SetAutoscalingDocument,
  DisableAutoscalingDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface AutoscalingFormValues {
  minInstances: number;
  maxInstances: number;
  targetCPUPercent?: number | null;
  targetMemoryPercent?: number | null;
}

export interface UseAutoscalingResult {
  enabled: boolean;
  loading: boolean;
  saving: boolean;
  minInstances: number;
  maxInstances: number;
  targetCPUPercent: number | null;
  targetMemoryPercent: number | null;
  save: (values: AutoscalingFormValues) => Promise<boolean>;
  disable: () => Promise<boolean>;
}

export function useAutoscaling(serviceId: string): UseAutoscalingResult {
  const { t } = useTranslations();
  const [saving, setSaving] = useState(false);

  const { data, loading } = useQuery(AutoscalingConfigDocument, {
    variables: { id: serviceId },
  });

  const [mutateSet] = useMutation(SetAutoscalingDocument);
  const [mutateDisable] = useMutation(DisableAutoscalingDocument);

  const cfg = data?.autoscalingConfig;

  const save = useCallback(
    async (values: AutoscalingFormValues) => {
      setSaving(true);
      try {
        await mutateSet({
          variables: {
            id: serviceId,
            minInstances: values.minInstances,
            maxInstances: values.maxInstances,
            targetCPUPercent: values.targetCPUPercent ?? undefined,
            targetMemoryPercent: values.targetMemoryPercent ?? undefined,
          },
          refetchQueries: [{ query: AutoscalingConfigDocument, variables: { id: serviceId } }],
        });
        toast.success(t("services.scalingSaved"));
        return true;
      } catch {
        toast.error(t("services.scalingError"));
        return false;
      } finally {
        setSaving(false);
      }
    },
    [mutateSet, serviceId, t],
  );

  const disable = useCallback(async () => {
    setSaving(true);
    try {
      await mutateDisable({
        variables: { id: serviceId },
        refetchQueries: [{ query: AutoscalingConfigDocument, variables: { id: serviceId } }],
      });
      toast.success(t("services.scalingDisabled"));
      return true;
    } catch {
      toast.error(t("services.scalingError"));
      return false;
    } finally {
      setSaving(false);
    }
  }, [mutateDisable, serviceId, t]);

  return {
    enabled: cfg?.enabled ?? false,
    loading,
    saving,
    minInstances: cfg?.minInstances ?? 1,
    maxInstances: cfg?.maxInstances ?? 3,
    targetCPUPercent: cfg?.targetCPUPercent ?? null,
    targetMemoryPercent: cfg?.targetMemoryPercent ?? null,
    save,
    disable,
  };
}
