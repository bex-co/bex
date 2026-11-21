import { useState } from "react";
import { toast } from "sonner";
import { useTranslations } from "@/common/hooks/use-translations";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import { useSetEnvironmentServices } from "@/features/environments/hooks/use-set-environment-services";
import { useSetEnvironmentDatabases } from "@/features/environments/hooks/use-set-environment-databases";
import { useSetEnvironmentKeyValues } from "@/features/environments/hooks/use-set-environment-keyvalues";
import { useSetEnvironmentEnvGroups } from "@/features/environments/hooks/use-set-environment-env-groups";
import { useStrandedLinkGuard } from "@/features/env-groups/hooks/use-stranded-link-guard";
import { strandedPromptItems } from "@/features/env-groups/lib/stranded";
import type { StrandedGroupPromptItem } from "@/features/environments/components/stranded-groups-confirm-dialog";
import {
  resourceSelectionKey,
  type ResourceRow,
} from "@/features/projects/types";
import type { ResourceSelection } from "@/features/projects/hooks/use-resource-selection";

export interface BulkMoveTarget {
  environment: EnvironmentView;
  /** What the picker shows: the name, or `project / name` across projects. */
  label: string;
}

export interface BulkMove {
  /** Move the selection into the environment with this id. */
  move: (targetId: string) => Promise<void>;
  /** True while a write into this environment is in flight. */
  movingTo: (targetId: string) => boolean;
  /** The stranded-group confirmation a move is waiting on, or null. */
  strandedPrompt: { targetId: string; items: StrandedGroupPromptItem[] } | null;
  confirmStranded: () => void;
  dismissStranded: () => void;
}

/**
 * The selection toolbar's Move, shared by a project environment's card and the
 * Overview's ungrouped list (w4/m550). `targets` are the environments the
 * selection may move into. Rows that moved leave the selection; rows whose
 * write failed stay selected, so a retry is one click.
 */
export function useBulkMove(
  selection: ResourceSelection,
  targets: EnvironmentView[],
): BulkMove {
  const { t } = useTranslations();
  const { setServices, busyId: servicesBusyId } = useSetEnvironmentServices();
  const { setDatabases, busyId: databasesBusyId } =
    useSetEnvironmentDatabases();
  const { setKeyValues, busyId: keyValuesBusyId } =
    useSetEnvironmentKeyValues();
  const { setEnvGroups, busyId: envGroupsBusyId } =
    useSetEnvironmentEnvGroups();
  const guard = useStrandedLinkGuard();
  const [strandedPrompt, setStrandedPrompt] =
    useState<BulkMove["strandedPrompt"]>(null);
  const busyIds = [
    servicesBusyId,
    databasesBusyId,
    keyValuesBusyId,
    envGroupsBusyId,
  ];

  async function move(targetId: string) {
    const target = targets.find((candidate) => candidate.id === targetId);
    const selectedRows = selection.selectedRows;
    if (!target || selectedRows.length === 0) return;

    // w4/m238: a move that would leave a service linked to a group scoped
    // elsewhere asks first; with no stranded link the move is exactly the
    // single write it is today.
    const movingServiceIds = selectedRows
      .filter((row) => row.kind === "service")
      .map((row) => row.id);
    if (movingServiceIds.length > 0) {
      if (!guard.ready) {
        toast.error(t("environments.strandedScopeUnknown"));
        return;
      }
      // A group selected with its services moves with them (w4/564).
      const movingGroupIds = selectedRows
        .filter((row) => row.kind === "envgroup")
        .map((row) => row.id);
      const stranded = guard.strandedFor(movingServiceIds, target.id, {
        groupIds: movingGroupIds,
        destinationServiceIds: target.serviceIds,
      });
      if (stranded.length > 0) {
        setStrandedPrompt({
          targetId: target.id,
          items: strandedPromptItems(stranded, guard.scopeNameFor),
        });
        return;
      }
    }
    await performMove(target, selectedRows);
  }

  async function performMove(
    target: EnvironmentView,
    selectedRows: ResourceRow[],
  ) {
    // One entry per resource kind: which of the target's ids that kind already
    // holds, and the setter that replaces them. All four setters share the same
    // (id, name, ids) => Promise<boolean> shape, so the move is one loop.
    const kinds: Array<{
      kind: ResourceRow["kind"];
      current: string[];
      set: (id: string, name: string, ids: string[]) => Promise<boolean>;
    }> = [
      { kind: "service", current: target.serviceIds, set: setServices },
      { kind: "database", current: target.databaseIds, set: setDatabases },
      { kind: "keyvalue", current: target.keyValueIds, set: setKeyValues },
      { kind: "envgroup", current: target.envGroupIds, set: setEnvGroups },
    ];
    const operations = kinds.flatMap(({ kind, current, set }) => {
      const moving = selectedRows.filter((row) => row.kind === kind);
      if (moving.length === 0) return [];
      return [
        {
          kind,
          keys: moving.map(resourceSelectionKey),
          run: () =>
            set(target.id, target.name, [
              ...new Set([...current, ...moving.map((row) => row.id)]),
            ]),
        },
      ];
    });

    // Groups move after the services: bex-api refuses a group whose linked
    // services are still elsewhere (ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES), so
    // a service moving with its group must land first, and a group whose
    // services did not move is not sent to be refused (w4/564).
    const settle = (batch: typeof operations) =>
      Promise.all(
        batch.map(async (operation) => ({
          keys: operation.keys,
          succeeded: await operation.run(),
        })),
      );
    const serviceResults = await settle(
      operations.filter((op) => op.kind === "service"),
    );
    const servicesMoved = serviceResults.every((result) => result.succeeded);
    const results = [
      ...serviceResults,
      ...(await settle(
        operations.filter(
          (op) =>
            op.kind !== "service" && (servicesMoved || op.kind !== "envgroup"),
        ),
      )),
    ];
    selection.deselect(
      new Set(
        results
          .filter((result) => result.succeeded)
          .flatMap((result) => result.keys),
      ),
    );
  }

  return {
    move,
    movingTo: (targetId) => busyIds.some((id) => id === targetId),
    strandedPrompt,
    confirmStranded: () => {
      const target = targets.find(
        (candidate) => candidate.id === strandedPrompt?.targetId,
      );
      setStrandedPrompt(null);
      if (target) void performMove(target, selection.selectedRows);
    },
    dismissStranded: () => setStrandedPrompt(null),
  };
}
