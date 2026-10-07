import type { en } from "@/i18n";
import { ownValue } from "@/common/lib/own-value";
import { deriveStatus } from "@/features/databases/lib/status";
import type { DatabaseStatusKey } from "@/features/databases/types";

/**
 * Maps a resolved database status key to its i18n badge label. Shared by the
 * list rows and the detail header so the same status renders the same word
 * everywhere (mirrors services' STATUS_LABEL).
 */
export const STATUS_LABEL: Record<DatabaseStatusKey, keyof typeof en> = {
  available: "databases.statusAvailable",
  creating: "databases.statusCreating",
  restarting: "databases.statusRestarting",
  upgrading: "databases.statusUpgrading",
  unavailable: "databases.statusUnavailable",
  suspended: "databases.statusSuspended",
  unknown: "databases.statusUnknown",
};

/**
 * An unavailable database's reason by bex-api's statusReasonCode, the
 * operator's Ready-condition reason (w5/079). A code missing here shows
 * bex-api's English sentence instead.
 */
const STATUS_REASON_LABEL: Record<string, keyof typeof en> = {
  ClusterFailed: "databases.statusReasonClusterFailed",
  ClusterReadFailed: "databases.statusReasonClusterReadFailed",
  NetworkPolicyFailed: "databases.statusReasonNetworkPolicyFailed",
  DiskAutoscalingFailed: "databases.statusReasonDiskAutoscalingFailed",
  ExportFailed: "databases.statusReasonExportFailed",
  PoolerFailed: "databases.statusReasonPoolerFailed",
  PostUpgradeBackupFailed: "databases.statusReasonPostUpgradeBackupFailed",
  MajorVersionUpgradeFailed: "databases.statusReasonMajorVersionUpgradeFailed",
  StorageShrinkRejected: "databases.statusReasonStorageShrinkRejected",
  RecoveryUnavailable: "databases.statusReasonRecoveryUnavailable",
  BackupStoreUnavailable: "databases.statusReasonBackupStoreUnavailable",
  ScheduledBackupFailed: "databases.statusReasonScheduledBackupFailed",
  ScheduledBackupCleanupFailed:
    "databases.statusReasonScheduledBackupCleanupFailed",
};

/** The translation key of an unavailable database's reason, if this dashboard knows its code. */
export function statusReasonLabel(
  code: string | null | undefined,
): keyof typeof en | undefined {
  return ownValue(STATUS_REASON_LABEL, code);
}

/**
 * The i18n label for a database's displayed status — one composition shared by
 * the badge and the detail page's Details row, so a suspended instance can
 * never read "Suspended" in one and the raw "available" in the other (w1/m159).
 */
export function statusLabel(d: {
  status: string;
  suspended: string;
}): keyof typeof en {
  return STATUS_LABEL[deriveStatus(d).key];
}
