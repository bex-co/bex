import { useCallback, useEffect, useState } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  DeployHookDocument,
  RegenerateDeployHookDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseDeployHookResult {
  /** The copy-ready secret URL, or null while loading / on read failure. */
  url: string | null;
  loading: boolean;
  error: Error | undefined;
  /** Rotate the URL; resolves true only after the new value is in local state. */
  regenerate: () => Promise<boolean>;
  regenerating: boolean;
}

/**
 * Reads and rotates a service's deploy-hook credential through bex-api's one
 * GraphQL service layer. The mutation result replaces local state immediately,
 * so the masked field and copy button can never keep the invalidated old URL.
 */
export function useDeployHook(serviceId: string): UseDeployHookResult {
  const { t } = useTranslations();
  const { data, loading, error } = useQuery(DeployHookDocument, {
    variables: { serviceId },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  const [mutate] = useMutation(RegenerateDeployHookDocument);
  const [url, setURL] = useState<string | null>(null);
  const [regenerating, setRegenerating] = useState(false);

  useEffect(() => {
    if (data?.deployHook?.url) setURL(data.deployHook.url);
  }, [data]);

  const regenerate = useCallback(async () => {
    setRegenerating(true);
    try {
      const result = await mutate({ variables: { serviceId } });
      const next = result.data?.regenerateDeployHook?.url;
      if (!next) throw new Error("deploy hook mutation returned no URL");
      setURL(next);
      toast.success(t("services.deployHookRegenerated"));
      return true;
    } catch {
      toast.error(t("services.deployHookRegenerateError"));
      return false;
    } finally {
      setRegenerating(false);
    }
  }, [mutate, serviceId, t]);

  return { url, loading, error, regenerate, regenerating };
}
