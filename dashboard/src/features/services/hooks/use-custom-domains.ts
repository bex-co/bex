import { useCallback, useState } from "react";
import { useQuery, useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  CustomDomainsDocument,
  AddCustomDomainDocument,
  DeleteCustomDomainDocument,
  VerifyCustomDomainDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  pairedSibling,
  type CustomDomainView,
} from "@/features/services/types";

// bex-api's custom-domains GraphQL is a thin veneer over App.spec.hosts[]
// (docs/ADR006-bex-api.md): the operator reconciles Traefik + cert-manager per host, so
// an add/delete converges asynchronously — the toast says the change is
// propagating rather than implying an instant apply. The hostname is the opaque
// id (id === name), and verification/serving status is read live from TLS state.

type RawDomain = {
  name: string | null;
  domainType: string | null;
  verificationStatus: string | null;
  serverStatus: string | null;
  dnsRecord: {
    type: string | null;
    name: string | null;
    value: string | null;
  } | null;
} | null;

function mapDomain(d: RawDomain & { name: string }): CustomDomainView {
  return {
    name: d.name,
    domainType: d.domainType ?? "subdomain",
    verified: d.verificationStatus === "verified",
    active: d.serverStatus === "active",
    dnsRecord: d.dnsRecord
      ? {
          type: d.dnsRecord.type ?? "",
          name: d.dnsRecord.name ?? "",
          value: d.dnsRecord.value ?? "",
        }
      : null,
  };
}

function mapDomains(
  raw: Array<RawDomain> | null | undefined,
): CustomDomainView[] {
  return (raw ?? [])
    .filter((d): d is RawDomain & { name: string } => d?.name != null)
    .map(mapDomain);
}

// mapRaw maps a single mutation result (add/verify return one domain or null) to a
// view, or null when the domain/name is absent — the shared shape both write verbs use.
function mapRaw(raw: RawDomain): CustomDomainView | null {
  return raw?.name ? mapDomain(raw as RawDomain & { name: string }) : null;
}

// addDomainErrorKey classifies a failed add by bex-api's sentinel text so the
// toast tells the user *why* — the host is taken by another service (409,
// Render's "already exists on another site") or is a reserved platform host
// (400) — rather than a generic failure. Same message-substring convention the
// env-vars/secret-files hooks use (bex-api sentinels are stable wire text).
function addDomainErrorKey(error: unknown): string {
  const msg = error instanceof Error ? error.message.toLowerCase() : "";
  if (msg.includes("another site")) return "services.domainAddConflict";
  if (msg.includes("reserved platform")) return "services.domainAddReserved";
  return "services.domainAddError";
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

/** The result of a successful add: the domain the tenant asked for, plus its
 *  www<->apex auto-paired sibling (w6/m23) when one was added alongside it. */
export interface AddedDomain {
  primary: CustomDomainView;
  sibling: CustomDomainView | null;
}

export interface UseCustomDomainMutationsResult {
  /** Add a custom domain; resolves to the created domain (with its DNS record) —
   *  and its auto-paired sibling, if any — on success so the caller can show the
   *  DNS instructions for both immediately, or null on error. */
  addDomain: (name: string) => Promise<AddedDomain | null>;
  /** Remove a custom domain; resolves true on success. */
  deleteDomain: (name: string) => Promise<boolean>;
  /** Re-check a domain's DNS/cert state now; resolves to the fresh domain, or null on error. */
  verifyDomain: (name: string) => Promise<CustomDomainView | null>;
  /** A write is in flight (disable the form/actions while true). */
  busy: boolean;
}

/**
 * Wires the custom-domain write mutations (`addCustomDomain` / `deleteCustomDomain`
 * / `verifyCustomDomain`), refetching the list after each write and toasting the
 * result. Each add/delete patches App.spec.hosts[], which the operator reconciles
 * into Traefik + a TLS certificate, so the success toast says the change is
 * propagating; verify is an idempotent re-read of the current DNS/cert state.
 */
export function useCustomDomainMutations(
  serviceId: string,
  refetch: () => Promise<CustomDomainView[]>,
): UseCustomDomainMutationsResult {
  const { t } = useTranslations();
  const [addCustomDomain] = useMutation(AddCustomDomainDocument);
  const [deleteCustomDomain] = useMutation(DeleteCustomDomainDocument);
  const [verifyCustomDomain] = useMutation(VerifyCustomDomainDocument);
  const [busy, setBusy] = useState(false);

  const addDomain = useCallback(
    async (name: string) => {
      setBusy(true);
      try {
        const res = await addCustomDomain({
          variables: { id: serviceId, name },
        });
        // refetch() resolves to the freshly-mapped list (UseCustomDomainsResult),
        // so the auto-paired sibling (if any) is looked up from data guaranteed to
        // include it — no race against the parent's next render.
        const fresh = await refetch();
        const primary = mapRaw(res.data?.addCustomDomain ?? null);
        toast.success(t("services.domainAddSuccess", { name }), {
          description: t("services.domainPropagateNote"),
        });
        return primary
          ? { primary, sibling: pairedSibling(primary, fresh) }
          : null;
      } catch (e) {
        toast.error(t(addDomainErrorKey(e), { name }));
        return null;
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

  const verifyDomain = useCallback(
    async (name: string) => {
      setBusy(true);
      try {
        const res = await verifyCustomDomain({
          variables: { id: serviceId, name },
        });
        await refetch();
        const view = mapRaw(res.data?.verifyCustomDomain ?? null);
        if (view?.verified) {
          toast.success(t("services.domainVerifySuccess", { name }));
        } else {
          toast.info(t("services.domainVerifyPending", { name }));
        }
        return view;
      } catch {
        toast.error(t("services.domainVerifyError", { name }));
        return null;
      } finally {
        setBusy(false);
      }
    },
    [serviceId, verifyCustomDomain, refetch, t],
  );

  return { addDomain, deleteDomain, verifyDomain, busy };
}
