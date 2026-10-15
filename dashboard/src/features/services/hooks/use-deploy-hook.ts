import { useCallback, useState } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  DeployHookDocument,
  RegenerateDeployHookDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";

export interface UseDeployHookResult {
  /** The copy-ready secret URL, or null while loading / on read failure. */
  url: string | null;
  loading: boolean;
  error: Error | undefined;
  /** Rotate the URL; resolves true only after the new value is in local state. */
  regenerate: () => Promise<boolean>;
  regenerating: boolean;
}

/** The last good URL, and the read that delivered it, for one service. */
interface KeptURL {
  serviceId: string;
  read: string | null;
  url: string | null;
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
  const [mutate] = useMutation(RegenerateDeployHookDocument, {
    // Write the rotated URL through to the cached DeployHook query — without
    // this a remount's cache-and-network read briefly serves the revoked URL.
    update(cache, result) {
      const next = result.data?.regenerateDeployHook;
      if (!next?.url) return;
      cache.writeQuery({
        query: DeployHookDocument,
        variables: { serviceId },
        data: { deployHook: next },
      });
    },
  });
  const [kept, setKept] = useState<KeptURL>({
    serviceId,
    read: null,
    url: null,
  });
  const [regenerating, setRegenerating] = useState(false);

  // The URL is a credential for one service: another service starts empty
  // rather than showing this one's (w1/130). Within a service, adopt each URL
  // the read delivers; a read that delivers none (a failed refresh) leaves the
  // last good one in place.
  const queried = data?.deployHook?.url || null;
  let current =
    kept.serviceId === serviceId ? kept : { serviceId, read: null, url: null };
  if (queried && queried !== current.read) {
    current = { serviceId, read: queried, url: queried };
  }
  if (current !== kept) setKept(current);

  const regenerate = useCallback(async () => {
    setRegenerating(true);
    try {
      const result = await mutate({ variables: { serviceId } });
      const next = result.data?.regenerateDeployHook?.url;
      if (!next) throw new Error("deploy hook mutation returned no URL");
      setKept((k) => (k.serviceId === serviceId ? { ...k, url: next } : k));
      toast.success(t("services.deployHookRegenerated"));
      return true;
    } catch (err) {
      toast.error(
        mutationErrorMessage(err, t("services.deployHookRegenerateError")),
      );
      return false;
    } finally {
      setRegenerating(false);
    }
  }, [mutate, serviceId, t]);

  return { url: current.url, loading, error, regenerate, regenerating };
}
