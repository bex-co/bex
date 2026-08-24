import { useMutation, useQuery } from "@apollo/client/react";
import { useCallback, useState } from "react";
import { toast } from "sonner";

import {
  AddDiskDocument,
  DeleteDiskDocument,
  DiskSnapshotsDocument,
  DisksDocument,
  RestoreDiskSnapshotDocument,
  UpdateDiskDocument,
} from "@/graphql/definitions";
import { graphQLErrorMessage } from "@/common/lib/graphql-error";
import { RESOURCE_POLL_INTERVAL_MS, skipPollWhenHidden } from "@/common/lib/polling";
import { useTranslations } from "@/common/hooks/use-translations";

/** A service's persistent disk, as the Disk tab presents it. */
export interface DiskView {
  id: string;
  name: string;
  mountPath: string;
  sizeGB: number;
  serviceId: string;
}

/** One stored snapshot. `snapshotKey` expires 24 hours after it is listed. */
export interface DiskSnapshotView {
  createdAt: string;
  snapshotKey: string;
  instanceId: string;
}

export interface UseDiskResult {
  /** The service's disk, or null when it has none. */
  disk: DiskView | null;
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<void>;
}

/**
 * Reads the one disk a service may have. Render allows at most one per service,
 * so the list query is collapsed to a single value here rather than making every
 * caller reason about an array that can only ever hold zero or one entry.
 */
export function useDisk(serviceId: string): UseDiskResult {
  const { data, loading, error, refetch } = useQuery(DisksDocument, {
    variables: { serviceId },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });

  const refetchDisk = useCallback(async () => {
    await refetch();
  }, [refetch]);

  // The generated types are nullable field-by-field (GraphQL's schema allows
  // it); normalize once here so no component has to defend against a disk with
  // a null mount path.
  const first = data?.disks?.[0];
  return {
    disk: first
      ? {
          id: first.id ?? "",
          name: first.name ?? "",
          mountPath: first.mountPath ?? "",
          sizeGB: first.sizeGB ?? 0,
          serviceId: first.serviceId ?? "",
        }
      : null,
    // Only the FIRST load counts as loading. This query is cache-and-network and
    // polls, so Apollo raises `loading` again on every background refetch — and
    // a diskless service has nothing else to render, so the caller would swap
    // its add form for a skeleton every few seconds and destroy the mount path
    // the tenant was halfway through typing. Verified against production
    // 2026-08-24: the form vanished ~2s after opening, and the skeleton
    // appeared in the same frame. Once `data` has arrived, an empty list is a
    // real answer ("no disk"), not a pending one.
    loading: loading && data === undefined,
    error,
    refetch: refetchDisk,
  };
}

export interface UseDiskSnapshotsResult {
  snapshots: DiskSnapshotView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<void>;
}

/**
 * Lists a disk's snapshots. Skipped entirely when the service has no disk, so a
 * diskless service never asks the API for snapshots that cannot exist.
 */
export function useDiskSnapshots(diskId: string | null): UseDiskSnapshotsResult {
  const { data, loading, error, refetch } = useQuery(DiskSnapshotsDocument, {
    variables: { id: diskId ?? "" },
    skip: !diskId,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const refetchSnapshots = useCallback(async () => {
    await refetch();
  }, [refetch]);

  return {
    snapshots: (data?.diskSnapshots ?? []).flatMap((s) =>
      s?.snapshotKey
        ? [{ createdAt: s.createdAt ?? "", snapshotKey: s.snapshotKey, instanceId: s.instanceId ?? "" }]
        : [],
    ),
    // First load only, for the same reason as useDisk above: otherwise the
    // snapshots card flashes a skeleton over the list on every refetch.
    loading: loading && data === undefined,
    error,
    refetch: refetchSnapshots,
  };
}

export interface UseDiskMutationsResult {
  addDisk: (input: { mountPath: string; sizeGB: number }) => Promise<boolean>;
  growDisk: (diskId: string, sizeGB: number) => Promise<boolean>;
  deleteDisk: (diskId: string) => Promise<boolean>;
  restoreSnapshot: (diskId: string, snapshotKey: string) => Promise<boolean>;
  /** The server's own reason for the last failed add, kept visible beside the toast. */
  addError: string | null;
  clearAddError: () => void;
  busy: boolean;
}

/**
 * The disk write verbs. Each one refetches before resolving, so the caller's UI
 * reflects the server rather than an optimistic guess — attaching a disk
 * redeploys the service, and a restore stops it, so what the API decided is the
 * only thing worth showing.
 */
export function useDiskMutations(serviceId: string, refetch: () => Promise<void>): UseDiskMutationsResult {
  const { t } = useTranslations();
  const [busy, setBusy] = useState(false);
  const [addError, setAddError] = useState<string | null>(null);
  const [addDiskMutation] = useMutation(AddDiskDocument);
  const [updateDiskMutation] = useMutation(UpdateDiskDocument);
  const [deleteDiskMutation] = useMutation(DeleteDiskDocument);
  const [restoreMutation] = useMutation(RestoreDiskSnapshotDocument);

  const run = useCallback(
    async (action: () => Promise<unknown>, successKey: string, onError?: (message: string) => void) => {
      setBusy(true);
      try {
        await action();
        await refetch();
        toast.success(t(successKey));
        return true;
      } catch (error) {
        const message = graphQLErrorMessage(error) ?? t("common.errorDefaultMessage");
        onError?.(message);
        toast.error(message);
        return false;
      } finally {
        setBusy(false);
      }
    },
    [refetch, t],
  );

  const addDisk = useCallback(
    async ({ mountPath, sizeGB }: { mountPath: string; sizeGB: number }) => {
      setAddError(null);
      return run(
        () =>
          addDiskMutation({
            // Render's dashboard has no name field; the disk is named after the
            // path it serves so the API-level name is never empty.
            variables: { serviceId, name: "data", mountPath, sizeGB },
          }),
        "services.diskAddSuccess",
        setAddError,
      );
    },
    [addDiskMutation, run, serviceId],
  );

  const growDisk = useCallback(
    (diskId: string, sizeGB: number) =>
      run(() => updateDiskMutation({ variables: { id: diskId, sizeGB } }), "services.diskResizeSuccess"),
    [run, updateDiskMutation],
  );

  const deleteDisk = useCallback(
    (diskId: string) => run(() => deleteDiskMutation({ variables: { id: diskId } }), "services.diskDeleteSuccess"),
    [deleteDiskMutation, run],
  );

  const restoreSnapshot = useCallback(
    (diskId: string, snapshotKey: string) =>
      run(() => restoreMutation({ variables: { id: diskId, snapshotKey } }), "services.diskRestoreStarted"),
    [restoreMutation, run],
  );

  return {
    addDisk,
    growDisk,
    deleteDisk,
    restoreSnapshot,
    addError,
    clearAddError: useCallback(() => setAddError(null), []),
    busy,
  };
}
