import { useCallback, useEffect, useMemo, useState } from "react";
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

// bex-api's secret-files GraphQL mirrors the env-vars shape (docs/ADR006-bex-api.md):
// the paged `secretFiles` query lists names only (a file's content is fetched
// per name via `secretFile(name)`, "Show"), and every write rolls the pods —
// there is no separate deploy step, so the toast says the service is
// redeploying. This hook walks every page exactly like useEnvVarKeys.

const PAGE_SIZE = 100;

type RawPageItem = {
  secretFile: { id: string | null; name: string | null } | null;
  cursor: string | null;
} | null;

function mapPage(raw: Array<RawPageItem> | null | undefined) {
  const items = (raw ?? []).filter(
    (item): item is NonNullable<RawPageItem> =>
      Boolean(item?.secretFile?.name),
  );
  const names = items
    .map((item) => item.secretFile)
    .filter(
      (file): file is { id: string | null; name: string } =>
        file?.name != null,
    )
    .map((file) => ({ id: file.id ?? file.name, name: file.name }));
  return { items, names };
}

function uniqueNames(names: SecretFileName[]): SecretFileName[] {
  return [...new Map(names.map((name) => [name.id, name])).values()];
}

function pageIdentity(
  serviceId: string,
  items: Array<NonNullable<RawPageItem>>,
) {
  return `${serviceId}\0${items.map((item) => item.cursor).join("\0")}`;
}

export interface UseSecretFileNamesResult {
  names: SecretFileName[];
  loading: boolean;
  error: Error | undefined;
  /** Re-run the names query, resolving to the fresh (name-sorted) list. */
  refetch: () => Promise<SecretFileName[]>;
}

/**
 * Reads every page of a service's secret-file names (`secretFiles(serviceId,
 * cursor, limit)`). Names only — no file content is returned in the list. The
 * page walk is identical to useEnvVarKeys: fetch the first page, then follow the
 * trailing cursor until a short page ends the list.
 */
export function useSecretFileNames(
  serviceId: string,
): UseSecretFileNamesResult {
  const client = useApolloClient();
  const { data, loading, error, refetch } = useQuery(SecretFileNamesDocument, {
    variables: { serviceId, limit: PAGE_SIZE },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  const first = useMemo(() => mapPage(data?.secretFiles), [data?.secretFiles]);
  const firstPageID = pageIdentity(serviceId, first.items);
  const [tail, setTail] = useState<{
    pageID: string;
    names: SecretFileName[];
    error?: Error;
  }>({ pageID: "", names: [] });

  const fetchTail = useCallback(
    async (items: Array<NonNullable<RawPageItem>>) => {
      const tail: SecretFileName[] = [];
      let page = items;
      let priorCursor = "";
      while (page.length === PAGE_SIZE) {
        const cursor = page.at(-1)?.cursor ?? "";
        if (!cursor || cursor === priorCursor) break;
        priorCursor = cursor;
        const result = await client.query({
          query: SecretFileNamesDocument,
          variables: { serviceId, cursor, limit: PAGE_SIZE },
          fetchPolicy: "network-only",
          errorPolicy: "none",
        });
        const mapped = mapPage(result.data?.secretFiles);
        tail.push(...mapped.names);
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
      .then((names) => {
        if (active) setTail({ pageID: firstPageID, names });
      })
      .catch((err: unknown) => {
        if (active)
          setTail({
            pageID: firstPageID,
            names: [],
            error: err instanceof Error ? err : new Error(String(err)),
          });
      });
    return () => {
      active = false;
    };
  }, [fetchTail, first.items, firstPageID]);

  const refetchNames = useCallback(async () => {
    const res = await refetch({
      serviceId,
      cursor: undefined,
      limit: PAGE_SIZE,
    });
    const fresh = mapPage(res.data?.secretFiles);
    const names = await fetchTail(fresh.items);
    setTail({ pageID: pageIdentity(serviceId, fresh.items), names });
    return uniqueNames([...fresh.names, ...names]);
  }, [fetchTail, refetch, serviceId]);

  const currentTail = tail.pageID === firstPageID ? tail : undefined;
  const paging = first.items.length === PAGE_SIZE && currentTail == null;

  return {
    names: uniqueNames([...first.names, ...(currentTail?.names ?? [])]),
    loading: loading || paging,
    error: error ?? currentTail?.error,
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
