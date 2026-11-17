import { useCallback, useEffect, useState } from "react";
import { useApolloClient, useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  DatabaseRecoveryInfoDocument,
  DatabaseExportsDocument,
  DatabaseExportLinksDocument,
  CreateDatabaseExportDocument,
  RecoverDatabaseDocument,
  type DatabaseExportsQuery,
  type DatabaseRecoveryInfoQuery,
} from "@/graphql/definitions";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { useTranslations } from "@/common/hooks/use-translations";
import { nonNull } from "@/common/lib/non-null";
import { openDownloadURL } from "@/common/lib/download-file";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { placementErrorMessage } from "@/features/environments/lib/placement";

export interface BackupItem {
  id: string;
  status: string;
  createdAt: string | null;
}

/**
 * One export as the status poll reads it. It carries no download URL: bex-api
 * mints one only for a request that asks, and each such request is one recorded
 * download of the database (w21/105), so `downloadExport` asks when a person
 * presses Download and the 25-second poll never does.
 */
export interface ExportItem extends BackupItem {
  expiresAt: string | null;
  filename: string | null;
  failureReason: string | null;
}

type BackupNodes = NonNullable<
  DatabaseRecoveryInfoQuery["databaseRecoveryInfo"]
>["backups"];

type ExportNodes = DatabaseExportsQuery["databaseExports"];

/** Normalize a nullable list of backup nodes (base backups or exports) onto
 * the non-null BackupItem view — shared by the recovery-info and exports reads. */
function mapBackups(nodes: BackupNodes | undefined): BackupItem[] {
  return (nodes ?? []).filter(nonNull).map((b) => ({
    id: b.id ?? "",
    status: b.status ?? "",
    createdAt: b.createdAt ?? null,
  }));
}

function mapExports(nodes: ExportNodes | undefined): ExportItem[] {
  return (nodes ?? []).filter(nonNull).map((item) => ({
    id: item.id ?? "",
    status: item.status ?? "",
    createdAt: item.createdAt ?? null,
    expiresAt: item.expiresAt ?? null,
    filename: item.filename ?? null,
    failureReason: item.failureReason ?? null,
  }));
}

export interface RecoveryInfo {
  enabled: boolean;
  earliestRecoveryTime: string | null;
  latestRecoveryTime: string | null;
  backups: BackupItem[];
}

export interface RecoverInput {
  name: string;
  targetTime?: string;
}

/**
 * Reads a database's recovery info (PITR window + backup list) and export
 * history, and drives restore-to-new-instance + create-export. Recovery info is
 * `errorPolicy: all` so a no-backup plan (which returns `enabled:false`, not an
 * error) still renders. Restore always creates a NEW database — the source is
 * untouched (docs/ADR009-postgresql-management.md).
 */
export function useRecovery(id: string) {
  const { t } = useTranslations();
  const infoQuery = useQuery(DatabaseRecoveryInfoDocument, {
    variables: { id },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });
  const raw = infoQuery.data?.databaseRecoveryInfo;
  const enabled = raw?.enabled ?? false;
  const recoveryInfoUnavailable = Boolean(infoQuery.error);
  // Exports only exist for backed-up databases — skip the round-trip otherwise
  // (the panel hides the exports list when recovery is disabled or unreadable).
  const exportsQuery = useQuery(DatabaseExportsDocument, {
    variables: { id },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    skip: !enabled || recoveryInfoUnavailable,
  });
  const {
    data: exportsData,
    refetch: refetchExports,
    startPolling,
    stopPolling,
  } = exportsQuery;
  const client = useApolloClient();
  const [downloadingExportId, setDownloadingExportId] = useState<string | null>(
    null,
  );
  const [createExportMut, { loading: exporting }] = useMutation(
    CreateDatabaseExportDocument,
  );
  const [recoverMut, { loading: recovering }] = useMutation(
    RecoverDatabaseDocument,
  );

  const info: RecoveryInfo = {
    enabled,
    earliestRecoveryTime: raw?.earliestRecoveryTime ?? null,
    latestRecoveryTime: raw?.latestRecoveryTime ?? null,
    backups: mapBackups(raw?.backups),
  };

  const exports = mapExports(exportsData?.databaseExports);
  const exportInProgress = exports.some(
    (item) => item.status === "created" || item.status === "running",
  );

  useEffect(() => {
    if (!enabled || recoveryInfoUnavailable) {
      stopPolling();
      return stopPolling;
    }
    // Fast while an export is running, baseline cadence otherwise so the list
    // still reflects exports started elsewhere.
    startPolling(exportInProgress ? 5_000 : RESOURCE_POLL_INTERVAL_MS);
    return stopPolling;
  }, [
    enabled,
    exportInProgress,
    recoveryInfoUnavailable,
    startPolling,
    stopPolling,
  ]);

  const createExport = useCallback(async () => {
    try {
      await createExportMut({ variables: { id } });
      toast.success(t("databases.recoveryExportStarted"));
      void refetchExports();
    } catch (err) {
      toast.error(
        mutationErrorMessage(err, t("databases.recoveryExportError")),
      );
    }
  }, [createExportMut, id, refetchExports, t]);

  // Mints this export's fresh 15-minute URL and hands it to the browser. Every
  // press is its own request (network-only), so it is one recorded download and
  // a URL that has expired is never reused from the cache.
  const downloadExport = useCallback(
    async (item: ExportItem) => {
      setDownloadingExportId(item.id);
      try {
        const { data } = await client.query({
          query: DatabaseExportLinksDocument,
          variables: { id },
          fetchPolicy: "network-only",
          errorPolicy: "none",
        });
        const url = data?.databaseExports?.find((e) => e?.id === item.id)?.url;
        if (!url) {
          toast.error(t("databases.recoveryDownloadUnavailable"));
          void refetchExports();
          return;
        }
        openDownloadURL(url, item.filename);
      } catch (err) {
        toast.error(
          mutationErrorMessage(err, t("databases.recoveryDownloadError")),
        );
      } finally {
        setDownloadingExportId(null);
      }
    },
    [client, id, refetchExports, t],
  );

  const recover = useCallback(
    async (input: RecoverInput): Promise<string | null> => {
      try {
        const result = await recoverMut({
          variables: {
            id,
            name: input.name,
            targetTime: input.targetTime || null,
          },
        });
        const recoveredID = result.data?.recoverDatabase?.id;
        if (!recoveredID) throw new Error("recoverDatabase returned no id");
        toast.success(
          t("databases.recoveryRestoreStarted", { name: input.name }),
        );
        return recoveredID;
      } catch (e) {
        toast.error(
          placementErrorMessage(e, t("databases.recoveryRestoreError"), t),
        );
        return null;
      }
    },
    [recoverMut, id, t],
  );

  return {
    info,
    exports,
    exportInProgress,
    loading: infoQuery.loading,
    error: infoQuery.error,
    exporting,
    recovering,
    createExport,
    downloadExport,
    downloadingExportId,
    recover,
  };
}
