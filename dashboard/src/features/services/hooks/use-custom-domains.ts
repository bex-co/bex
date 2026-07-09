import { useCallback, useState } from "react";
import { useQuery, useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  CustomDomainsDocument,
  AddCustomDomainDocument,
  DeleteCustomDomainDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import type { CustomDomainView } from "@/features/services/types";

// bex-api's custom-domains GraphQL is a thin veneer over App.spec.hosts[]
// (docs/bex-api.md): the operator reconciles Traefik + cert-manager per host, so
// an add/delete converges asynchronously — the toast says the change is
// propagating rather than implying an instant apply. The hostname is the opaque
// id (id === name), and verification/serving status is read live from TLS state.

type RawDomain = {
  name: string | null;
  verificationStatus: string | null;
  serverStatus: string | null;
} | null;

function mapDomains(
  raw: Array<RawDomain> | null | undefined,
): CustomDomainView[] {
  return (raw ?? [])
    .filter((d): d is RawDomain & { name: string } => d?.name != null)
    .map((d) => ({
      name: d.name,
      verified: d.verificationStatus === "verified",
      active: d.serverStatus === "active",
    }));
}

export interface UseCustomDomainsResult {
  domains: CustomDomainView[];
  loading: boolean;
  error: Error | undefined;
  /** Re-run the query, resolving to the fresh domain list. */
  refetch: () => Promise<CustomDomainView[]>;
}

/**
 * Reads a service's custom domains (`customDomains(id)`), mapping bex-api's
 * Render-shaped domain objects onto the normalized CustomDomainView. Presentation
 * only — the same shared Core the REST/MCP surfaces use.
 */
export function useCustomDomains(serviceId: string): UseCustomDomainsResult {
  const { data, loading, error, refetch } = useQuery(CustomDomainsDocument, {
    variables: { id: serviceId },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const refetchDomains = useCallback(async () => {
    const res = await refetch();
    return mapDomains(res.data?.customDomains);
  }, [refetch]);

  return {
    domains: mapDomains(data?.customDomains),
    loading,
    error,
    refetch: refetchDomains,
  };
}

export interface UseCustomDomainMutationsResult {
  /** Add a custom domain; resolves true on success. */
  addDomain: (name: string) => Promise<boolean>;
  /** Remove a custom domain; resolves true on success. */
  deleteDomain: (name: string) => Promise<boolean>;
  /** A write is in flight (disable the form/actions while true). */
  busy: boolean;
}

/**
 * Wires the custom-domain write mutations (`addCustomDomain` / `deleteCustomDomain`),
 * refetching the list after each write and toasting the result. Each write patches
 * App.spec.hosts[], which the operator reconciles into Traefik + a TLS certificate,
 * so the success toast says the change is propagating.
 */
export function useCustomDomainMutations(
  serviceId: string,
  refetch: () => Promise<CustomDomainView[]>,
): UseCustomDomainMutationsResult {
  const { t } = useTranslations();
  const [addCustomDomain] = useMutation(AddCustomDomainDocument);
  const [deleteCustomDomain] = useMutation(DeleteCustomDomainDocument);
  const [busy, setBusy] = useState(false);

  const addDomain = useCallback(
    async (name: string) => {
      setBusy(true);
      try {
        await addCustomDomain({ variables: { id: serviceId, name } });
        await refetch();
        toast.success(t("services.domainAddSuccess", { name }), {
          description: t("services.domainPropagateNote"),
        });
        return true;
      } catch {
        toast.error(t("services.domainAddError", { name }));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [serviceId, addCustomDomain, refetch, t],
  );

  const deleteDomain = useCallback(
    async (name: string) => {
      setBusy(true);
      try {
        await deleteCustomDomain({ variables: { id: serviceId, name } });
        await refetch();
        toast.success(t("services.domainDeleteSuccess", { name }));
        return true;
      } catch {
        toast.error(t("services.domainDeleteError", { name }));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [serviceId, deleteCustomDomain, refetch, t],
  );

  return { addDomain, deleteDomain, busy };
}
