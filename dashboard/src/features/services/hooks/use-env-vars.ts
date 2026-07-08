import { useCallback, useState } from "react";
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

// bex-api's env-vars GraphQL is Render dashboard-shaped (docs/bex-api.md#env-vars):
// env vars nest under the service, `envVarKeys` lists keys only (values fetched
// per key via `envVar(key)`, "Show secret"), and every write rolls the pods —
// there is no separate deploy step, so the toast says the service is redeploying.

type RawKey = { id: string | null; key: string | null } | null;

function mapKeys(raw: Array<RawKey> | null | undefined): EnvVarKey[] {
  return (raw ?? [])
    .filter((k): k is { id: string | null; key: string } => k?.key != null)
    .map((k) => ({ id: k.id ?? k.key, key: k.key }));
}

export interface UseEnvVarKeysResult {
  keys: EnvVarKey[];
  loading: boolean;
  error: Error | undefined;
  /** Re-run the keys query, resolving to the fresh (key-sorted) list. */
  refetch: () => Promise<EnvVarKey[]>;
}

/**
 * Reads a service's env-var keys (Render's `serviceEnvVarKeys`: `service(id){
 * envVarKeys{ id key } }`). Keys only — no values are returned in the list.
 */
export function useEnvVarKeys(serviceId: string): UseEnvVarKeysResult {
  const { data, loading, error, refetch } = useQuery(EnvVarKeysDocument, {
    variables: { id: serviceId },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const refetchKeys = useCallback(async () => {
    const res = await refetch();
    return mapKeys(res.data?.service?.envVarKeys);
  }, [refetch]);

  return {
    keys: mapKeys(data?.service?.envVarKeys),
    loading,
    error,
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
  setVar: (key: string, value: string) => Promise<boolean>;
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
    async (key: string, value: string) => {
      setBusy(true);
      try {
        await setEnvVar({ variables: { serviceId, key, value } });
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
