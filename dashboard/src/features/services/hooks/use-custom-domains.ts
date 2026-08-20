import { useCallback, useState } from "react";
import { useQuery, useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  CustomDomainsDocument,
  AddCustomDomainDocument,
  DeleteCustomDomainDocument,
  VerifyCustomDomainDocument,
  type CustomDomainFieldsFragment,
} from "@/graphql/definitions";
import {
  graphQLErrorMessage,
  hasGraphQLErrorCode,
  refusalReason,
} from "@/common/lib/graphql-error";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  pairedSibling,
  type CustomDomainView,
} from "@/features/services/types";

// Managed custom domains are durable ownership claims. Pending claims are
// intentionally absent from App.spec.host(s); an exact TXT match atomically
// promotes them, after which the operator converges routing and TLS.

type RawDomain = CustomDomainFieldsFragment | null;

function mapDNSRecord(
  record: CustomDomainFieldsFragment["dnsRecord"],
): CustomDomainView["dnsRecord"] {
  return record
    ? {
        type: record.type ?? "",
        name: record.name ?? "",
        value: record.value ?? "",
      }
    : null;
}

function mapDomain(
  d: CustomDomainFieldsFragment & { name: string },
): CustomDomainView {
  return {
    name: d.name,
    domainType: d.domainType ?? "subdomain",
    ownershipVerified: d.ownershipStatus === "verified",
    verified: d.verificationStatus === "verified",
    active: d.serverStatus === "active",
    redirectForName: d.redirectForName,
    dnsRecord: mapDNSRecord(d.dnsRecord),
    ownershipDnsRecord: mapDNSRecord(d.ownershipDnsRecord),
  };
}

function mapDomains(
  raw: Array<RawDomain> | null | undefined,
): CustomDomainView[] {
  return (raw ?? [])
    .filter(
      (d): d is CustomDomainFieldsFragment & { name: string } =>
        d?.name != null,
    )
    .map(mapDomain);
}

// mapRaw maps a single mutation result (add/verify return one domain or null) to a
// view, or null when the domain/name is absent — the shared shape both write verbs use.
function mapRaw(raw: RawDomain): CustomDomainView | null {
  return raw?.name
    ? mapDomain(raw as CustomDomainFieldsFragment & { name: string })
    : null;
}

// classifyAddError turns a failed add into what to show the user, so the dialog
// tells them *why* the add was refused rather than a generic failure. The two
// stable bex-api sentinels get a friendly localized line (host taken by another
// service — 409, Render's "already exists on another site" — or a reserved
// platform host — 400); every other refusal falls through to the server's own
// reason (shared `refusalReason`), e.g. "wildcard hostnames are not allowed", so
// a rejection the UI doesn't special-case is still explained instead of
// collapsing to "Couldn't add {name}". Same message-substring convention the
// env-vars/secret-files hooks use (bex-api sentinels are stable wire text);
// `key` resolves through t(), `detail` is the server's own text.
// Exported for unit testing the classification in isolation.
export function classifyAddError(error: unknown): {
  key?: string;
  detail?: string;
} {
  const lower = (graphQLErrorMessage(error) ?? "").toLowerCase();
  if (lower.includes("another site"))
    return { key: "services.domainAddConflict" };
  if (lower.includes("reserved platform"))
    return { key: "services.domainAddReserved" };
  const detail = refusalReason(error);
  return detail ? { detail } : {};
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
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
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
   *  DNS instructions for both immediately, or null on error. On error `addError`
   *  is set with the server's reason so the dialog can show it inline. */
  addDomain: (name: string) => Promise<AddedDomain | null>;
  /** The reason the last add was refused, for persistent inline display in the
   *  add dialog (a transient toast alone vanishes before the user reads it and
   *  leaves the dialog looking like nothing happened). Null when the last add
   *  succeeded or none has run. */
  addError: string | null;
  /** Clear `addError` (on input change and when the dialog closes). */
  clearAddError: () => void;
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
 * result. Add creates a pending ownership claim; verify promotes an exact TXT
 * match into serving intent; delete removes either pending or verified state.
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
  const [addError, setAddError] = useState<string | null>(null);
  const clearAddError = useCallback(() => setAddError(null), []);

  const addDomain = useCallback(
    async (name: string) => {
      setBusy(true);
      setAddError(null);
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
          description: t("services.domainOwnershipNote"),
        });
        return primary
          ? { primary, sibling: pairedSibling(primary, fresh) }
          : null;
      } catch (e) {
        // Show the reason inline in the still-open dialog (persistent) rather
        // than only a transient toast that vanishes before it's read.
        const { key, detail } = classifyAddError(e);
        setAddError(
          key
            ? t(key, { name })
            : (detail ?? t("services.domainAddError", { name })),
        );
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
        if (view?.ownershipVerified) {
          toast.success(t("services.domainVerifySuccess", { name }));
        } else {
          toast.info(t("services.domainVerifyPending", { name }));
        }
        return view;
      } catch (error) {
        if (hasGraphQLErrorCode(error, "DOMAIN_OWNERSHIP_PENDING")) {
          toast.info(t("services.domainVerifyPending", { name }));
        } else {
          toast.error(t("services.domainVerifyError", { name }));
        }
        return null;
      } finally {
        setBusy(false);
      }
    },
    [serviceId, verifyCustomDomain, refetch, t],
  );

  return {
    addDomain,
    addError,
    clearAddError,
    deleteDomain,
    verifyDomain,
    busy,
  };
}
