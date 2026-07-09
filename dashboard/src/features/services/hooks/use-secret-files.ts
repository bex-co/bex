import { useCallback, useState } from "react";
import { useQuery, useMutation, useApolloClient } from "@apollo/client/react";
import { toast } from "sonner";
import {
  SecretFileNamesDocument,
  SecretFileContentDocument,
  SetSecretFileDocument,
  DeleteSecretFileDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import type { SecretFileName } from "@/features/services/types";

// bex-api's secret-files GraphQL mirrors the env-vars shape (docs/bex-api.md):
// secret files nest under the service, `secretFileNames` lists names only (a
// file's content is fetched per name via `secretFile(name)`, "Show"), and every
// write rolls the pods — there is no separate deploy step, so the toast says the
// service is redeploying.

type RawName = { id: string | null; name: string | null } | null;

function mapNames(raw: Array<RawName> | null | undefined): SecretFileName[] {
  return (raw ?? [])
    .filter((f): f is { id: string | null; name: string } => f?.name != null)
    .map((f) => ({ id: f.id ?? f.name, name: f.name }));
}

export interface UseSecretFileNamesResult {
  names: SecretFileName[];
  loading: boolean;
  error: Error | undefined;
  /** Re-run the names query, resolving to the fresh (name-sorted) list. */
  refetch: () => Promise<SecretFileName[]>;
}

/**
 * Reads a service's secret-file names (`service(id){ secretFileNames{ id name }
 * }`). Names only — no file content is returned in the list.
 */
export function useSecretFileNames(serviceId: string): UseSecretFileNamesResult {
  const { data, loading, error, refetch } = useQuery(SecretFileNamesDocument, {
    variables: { id: serviceId },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const refetchNames = useCallback(async () => {
    const res = await refetch();
    return mapNames(res.data?.service?.secretFileNames);
  }, [refetch]);

  return {
    names: mapNames(data?.service?.secretFileNames),
    loading,
    error,
    refetch: refetchNames,
  };
}

/**
 * Returns a function that fetches a single file's content on demand (the
 * dashboard's "Show"): `service(id){ secretFile(name){ content } }`. Network-only
 * so a revealed body is always fresh; throws on an authz/store error so the row
 * can surface it.
 */
export function useRevealSecretFile(serviceId: string) {
  const client = useApolloClient();
  return useCallback(
    async (name: string): Promise<string> => {
      const res = await client.query({
        query: SecretFileContentDocument,
        variables: { id: serviceId, name },
        fetchPolicy: "network-only",
        errorPolicy: "none",
      });
      return res.data?.service?.secretFile?.content ?? "";
    },
    [client, serviceId],
  );
}

export interface UseSecretFileMutationsResult {
  /** Add or update one file (merges into the set); resolves true on success. */
  setFile: (name: string, content: string) => Promise<boolean>;
  /** Remove one file; resolves true on success. */
  deleteFile: (name: string) => Promise<boolean>;
  /** A write is in flight (disable the form while true). */
  busy: boolean;
}

/**
 * Wires the secret-file write mutations (`setSecretFile` / `deleteSecretFile`),
 * refetching the names after each write and toasting the result. Every write
 * rolls the service's pods, so the success toast says the change is redeploying
 * rather than implying an instant apply.
 */
export function useSecretFileMutations(
  serviceId: string,
  refetch: () => Promise<SecretFileName[]>,
): UseSecretFileMutationsResult {
  const { t } = useTranslations();
  const [setSecretFile] = useMutation(SetSecretFileDocument);
  const [deleteSecretFile] = useMutation(DeleteSecretFileDocument);
  const [busy, setBusy] = useState(false);

  const setFile = useCallback(
    async (name: string, content: string) => {
      setBusy(true);
      try {
        await setSecretFile({ variables: { serviceId, name, content } });
        await refetch();
        toast.success(t("services.secretFileSaveSuccess", { name }), {
          description: t("services.envRolloutNote"),
        });
        return true;
      } catch {
        toast.error(t("services.secretFileSaveError", { name }));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [serviceId, setSecretFile, refetch, t],
  );

  const deleteFile = useCallback(
    async (name: string) => {
      setBusy(true);
      try {
        await deleteSecretFile({ variables: { serviceId, name } });
        await refetch();
        toast.success(t("services.secretFileDeleteSuccess", { name }), {
          description: t("services.envRolloutNote"),
        });
        return true;
      } catch {
        toast.error(t("services.secretFileDeleteError", { name }));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [serviceId, deleteSecretFile, refetch, t],
  );

  return { setFile, deleteFile, busy };
}

/**
 * Classifies a secret-files GraphQL error into the states the tab renders
 * differently: the store being unconfigured (503-equivalent) and a permission
 * denial (403-equivalent) both come back as GraphQL errors whose message carries
 * bex-api's sentinel text. Same logic as classifyEnvVarError.
 */
export type SecretFileErrorKind = "unavailable" | "forbidden" | "generic";

export function classifySecretFileError(
  error: Error | undefined,
): SecretFileErrorKind | null {
  if (!error) return null;
  const msg = error.message.toLowerCase();
  if (msg.includes("secret store")) return "unavailable";
  if (msg.includes("forbidden")) return "forbidden";
  return "generic";
}
