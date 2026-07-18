import { useCallback, useEffect } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  DatabaseRecoveryInfoDocument,
  DatabaseExportsDocument,
  CreateDatabaseExportDocument,
  RecoverDatabaseDocument,
  type DatabaseBackup,
  type DatabaseExport,
} from "@/features/databases/api/operations";
import { useTranslations } from "@/common/hooks/use-translations";

export interface BackupItem {
  id: string;
  status: string;
  createdAt: string | null;
}

export interface ExportItem extends BackupItem {
  url: string | null;
  urlExpiresAt: string | null;
  expiresAt: string | null;
  filename: string | null;
  failureReason: string | null;
}

/** Normalize a nullable list of backup nodes (base backups or exports) onto
 * the non-null BackupItem view — shared by the recovery-info and exports reads. */
function mapBackups(
  nodes: Array<DatabaseBackup | null> | null | undefined,
): BackupItem[] {
  return (nodes ?? [])
    .filter((b): b is DatabaseBackup => b != null)
    .map((b) => ({
      id: b.id ?? "",
      status: b.status ?? "",
      createdAt: b.createdAt ?? null,
    }));
}

function mapExports(
  nodes: Array<DatabaseExport | null> | null | undefined,
): ExportItem[] {
  return (nodes ?? [])
    .filter((item): item is DatabaseExport => item != null)
    .map((item) => ({
      id: item.id ?? "",
      status: item.status ?? "",
      createdAt: item.createdAt ?? null,
      url: item.url ?? null,
      urlExpiresAt: item.urlExpiresAt ?? null,
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
  });
  const raw = infoQuery.data?.databaseRecoveryInfo;
  const enabled = raw?.enabled ?? false;
  // Exports only exist for backed-up databases — skip the round-trip otherwise
  // (the panel hides the exports list when recovery is disabled anyway).
  const exportsQuery = useQuery(DatabaseExportsDocument, {
    variables: { id },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    skip: !enabled,
  });
  const {
    data: exportsData,
    refetch: refetchExports,
    startPolling,
    stopPolling,
  } = exportsQuery;
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
    if (exportInProgress) {
      startPolling(5_000);
    } else {
      stopPolling();
    }
    return stopPolling;
  }, [exportInProgress, startPolling, stopPolling]);

  const createExport = useCallback(async () => {
    try {
      await createExportMut({ variables: { id } });
      toast.success(t("databases.recoveryExportStarted"));
      void refetchExports();
    } catch {
      toast.error(t("databases.recoveryExportError"));
    }
  }, [createExportMut, id, refetchExports, t]);

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
          t("databases.recoveryRestoreError", {
            error: (e as Error).message,
          }),
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
    exporting,
    recovering,
    createExport,
    recover,
  };
}
