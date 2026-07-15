import { useCallback, useEffect, useMemo, useState } from "react";
import { useQuery, useMutation, useApolloClient } from "@apollo/client/react";
import { toast } from "sonner";
import {
  EnvVarKeysDocument,
  EnvVarValueDocument,
  SetEnvVarDocument,
  DeleteEnvVarDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import type { EnvVarKey } from "@/features/services/types";

// bex-api's env-vars GraphQL is Render dashboard-shaped (docs/ADR006-bex-api.md#env-vars):
// the paged `envVars` query selects keys only (values are fetched per key via
// service.envVar(key), "Show secret"), and every write rolls the pods — there is
// no separate deploy step, so the toast says the service is redeploying.

const PAGE_SIZE = 100;

type RawPageItem = {
  envVar: { id: string | null; key: string | null } | null;
  cursor: string | null;
} | null;

function mapPage(raw: Array<RawPageItem> | null | undefined) {
  const items = (raw ?? []).filter((item): item is NonNullable<RawPageItem> =>
    Boolean(item?.envVar?.key),
  );
  const keys = items
    .map((item) => item.envVar)
    .filter(
      (envVar): envVar is { id: string | null; key: string } =>
        envVar?.key != null,
    )
    .map((envVar) => ({ id: envVar.id ?? envVar.key, key: envVar.key }));
  return { items, keys };
}

function uniqueKeys(keys: EnvVarKey[]): EnvVarKey[] {
  return [...new Map(keys.map((key) => [key.id, key])).values()];
}

function pageIdentity(
  serviceId: string,
  items: Array<NonNullable<RawPageItem>>,
) {
  return `${serviceId}\0${items.map((item) => item.cursor).join("\0")}`;
}

export interface UseEnvVarKeysResult {
  keys: EnvVarKey[];
  loading: boolean;
  error: Error | undefined;
  /** Re-run the keys query, resolving to the fresh (key-sorted) list. */
  refetch: () => Promise<EnvVarKey[]>;
}

/**
 * Reads every page of a service's env-var keys. Keys only — no values are
 * selected by the list query.
 */
export function useEnvVarKeys(serviceId: string): UseEnvVarKeysResult {
  const client = useApolloClient();
  const { data, loading, error, refetch } = useQuery(EnvVarKeysDocument, {
    variables: { serviceId, limit: PAGE_SIZE },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  const first = useMemo(() => mapPage(data?.envVars), [data?.envVars]);
  const firstPageID = pageIdentity(serviceId, first.items);
  const [tail, setTail] = useState<{
    pageID: string;
    keys: EnvVarKey[];
    error?: Error;
  }>({ pageID: "", keys: [] });

  const fetchTail = useCallback(
    async (items: Array<NonNullable<RawPageItem>>) => {
      const tail: EnvVarKey[] = [];
      let page = items;
      let priorCursor = "";
      while (page.length === PAGE_SIZE) {
        const cursor = page.at(-1)?.cursor ?? "";
        if (!cursor || cursor === priorCursor) break;
        priorCursor = cursor;
        const result = await client.query({
          query: EnvVarKeysDocument,
          variables: { serviceId, cursor, limit: PAGE_SIZE },
          fetchPolicy: "network-only",
          errorPolicy: "none",
        });
        const mapped = mapPage(result.data?.envVars);
        tail.push(...mapped.keys);
        page = mapped.items;
      }
      return tail;
    },
    [client, serviceId],
  );

  useEffect(() => {
    let active = true;
    if (first.items.length < PAGE_SIZE) return () => undefined;
    void fetchTail(first.items)
      .then((keys) => {
        if (active) setTail({ pageID: firstPageID, keys });
      })
      .catch((err: unknown) => {
        if (active)
          setTail({
            pageID: firstPageID,
            keys: [],
            error: err instanceof Error ? err : new Error(String(err)),
          });
      });
    return () => {
      active = false;
    };
  }, [fetchTail, first.items, firstPageID]);

  const refetchKeys = useCallback(async () => {
    const res = await refetch({
      serviceId,
      cursor: undefined,
      limit: PAGE_SIZE,
    });
    const fresh = mapPage(res.data?.envVars);
    const keys = await fetchTail(fresh.items);
    setTail({ pageID: pageIdentity(serviceId, fresh.items), keys });
    return uniqueKeys([...fresh.keys, ...keys]);
  }, [fetchTail, refetch, serviceId]);

  const currentTail = tail.pageID === firstPageID ? tail : undefined;
  const paging = first.items.length === PAGE_SIZE && currentTail == null;

  return {
    keys: uniqueKeys([...first.keys, ...(currentTail?.keys ?? [])]),
    loading: loading || paging,
    error: error ?? currentTail?.error,
    refetch: refetchKeys,
  };
}

/**
 * Returns a function that fetches a single variable's value on demand (the
 * dashboard's "Show secret"): `service(id){ envVar(key){ value } }`. Network-only
 * so a revealed value is always fresh; throws on an authz/store error so the row
 * can surface it.
 */
export function useRevealEnvVar(serviceId: string) {
  const client = useApolloClient();
  return useCallback(
    async (key: string): Promise<string> => {
      const res = await client.query({
        query: EnvVarValueDocument,
        variables: { id: serviceId, key },
        fetchPolicy: "network-only",
        errorPolicy: "none",
      });
      return res.data?.service?.envVar?.value ?? "";
    },
    [client, serviceId],
  );
}

export interface UseEnvVarMutationsResult {
  /** Add or update one variable (merges into the set); resolves true on success. */
  setVar: (
    key: string,
    value: string,
    generateValue?: boolean,
  ) => Promise<boolean>;
  /** Remove one variable; resolves true on success. */
  deleteVar: (key: string) => Promise<boolean>;
  /** A write is in flight (disable the form while true). */
  busy: boolean;
}

/**
 * Wires the env-var write mutations (`setEnvVar` / `deleteEnvVar`), refetching
 * the keys after each write and toasting the result. Every write rolls the
 * service's pods, so the success toast says the change is redeploying rather than
 * implying an instant apply.
 */
export function useEnvVarMutations(
  serviceId: string,
  refetch: () => Promise<EnvVarKey[]>,
): UseEnvVarMutationsResult {
  const { t } = useTranslations();
  const [setEnvVar] = useMutation(SetEnvVarDocument);
  const [deleteEnvVar] = useMutation(DeleteEnvVarDocument);
  const [busy, setBusy] = useState(false);

  const setVar = useCallback(
    async (key: string, value: string, generateValue = false) => {
      setBusy(true);
      try {
        await setEnvVar({
          variables: {
            serviceId,
            key,
            value: generateValue ? undefined : value,
            generateValue: generateValue || undefined,
          },
        });
        await refetch();
        toast.success(t("services.envSaveSuccess", { key }), {
          description: t("services.envRolloutNote"),
        });
        return true;
      } catch {
        toast.error(t("services.envSaveError", { key }));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [serviceId, setEnvVar, refetch, t],
  );

  const deleteVar = useCallback(
    async (key: string) => {
      setBusy(true);
      try {
        await deleteEnvVar({ variables: { serviceId, key } });
        await refetch();
        toast.success(t("services.envDeleteSuccess", { key }), {
          description: t("services.envRolloutNote"),
        });
        return true;
      } catch {
        toast.error(t("services.envDeleteError", { key }));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [serviceId, deleteEnvVar, refetch, t],
  );

  return { setVar, deleteVar, busy };
}

/**
 * Classifies an env-vars GraphQL error into the states the tab renders
 * differently: the store being unconfigured (503-equivalent) and a permission
 * denial (403-equivalent) both come back as GraphQL errors whose message carries
 * bex-api's sentinel text.
 */
export type EnvVarErrorKind = "unavailable" | "forbidden" | "generic";

export function classifyEnvVarError(
  error: Error | undefined,
): EnvVarErrorKind | null {
  if (!error) return null;
  const msg = error.message.toLowerCase();
  if (msg.includes("secret store")) return "unavailable";
  if (msg.includes("forbidden")) return "forbidden";
  return "generic";
}
