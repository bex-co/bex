import { useState } from "react";
import { useApolloClient } from "@apollo/client/react";
import type { DocumentNode } from "graphql";
import { Check, Loader2, RotateCcw } from "lucide-react";
import { toast } from "sonner";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { adminOnlyErrorMessage } from "@/features/capabilities/lib/admin-only";
import {
  ResumeDatabaseDocument,
  ResumeKeyValueDocument,
  ResumeServiceDocument,
  SuspendDatabaseDocument,
  SuspendKeyValueDocument,
  SuspendServiceDocument,
  ViewerCapabilitiesDocument,
} from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  toSnapshot,
  unavailableCapabilities,
  type CapabilityState,
} from "@/features/capabilities/lib/capability-policy";
import { ResourceTypeBadge } from "@/features/projects/components/resource-type-badge";
import {
  BULK_CONCURRENCY,
  bulkLifecycleReason,
  capabilitiesFromState,
  runBounded,
  suspendPhrase,
  type BulkLifecycleAction,
  type LifecycleTarget,
} from "@/features/projects/lib/bulk-lifecycle";

// One existing per-resource mutation per kind: Render's dashboard calls
// suspendService / suspendDatabase / suspendRedis once per resource, never a
// batch operation (docs/render-artifacts/service-list-bulk-actions.md).
const MUTATION: Record<
  BulkLifecycleAction,
  Record<LifecycleTarget["kind"], DocumentNode>
> = {
  suspend: {
    service: SuspendServiceDocument,
    database: SuspendDatabaseDocument,
    keyvalue: SuspendKeyValueDocument,
  },
  resume: {
    service: ResumeServiceDocument,
    database: ResumeDatabaseDocument,
    keyvalue: ResumeKeyValueDocument,
  },
};

const COPY = {
  suspend: {
    title: "projects.bulkSuspendTitle",
    body: "projects.bulkSuspendBody",
    confirm: "projects.bulkSuspendConfirm",
    running: "projects.bulkRowSuspending",
    done: "projects.bulkRowSuspended",
  },
  resume: {
    title: "projects.bulkResumeTitle",
    body: "projects.bulkResumeBody",
    confirm: "projects.bulkResumeConfirm",
    running: "projects.bulkRowResuming",
    done: "projects.bulkRowResumed",
  },
} as const;

type RowState =
  | { status: "pending" }
  | { status: "running" }
  | { status: "done" }
  | { status: "error"; message: string };

export interface BulkLifecycleDialogProps {
  action: BulkLifecycleAction;
  /** The rows the action changes (`targetsFor`), fixed while it is open. */
  targets: LifecycleTarget[];
  /** The selection belongs to a protected environment: Admins only. */
  protectedEnvironment: boolean;
  /** Closed; `succeeded` holds the keys of the rows that changed. */
  onClose: (succeeded: ReadonlySet<string>) => void;
}

/**
 * Render's **Suspend services** / **Resume services** modal (w4/m550): the
 * rows it changes, each with its own state, one call per row with at most 10
 * in flight, a Retry on a row that failed, and **Close** once all succeeded.
 * Suspend asks for Render's typed `sudo suspend …` phrase. The workspace grant
 * is read fresh before anything is sent, so a role that changed since the
 * toolbar enabled the action sends nothing (the w6/m146 freshness rule).
 *
 * Mounted only while open: it is the piece that talks to bex-api.
 */
export function BulkLifecycleDialog({
  action,
  targets,
  protectedEnvironment,
  onClose,
}: BulkLifecycleDialogProps) {
  const { t } = useTranslations();
  const client = useApolloClient();
  const { currentWorkspaceId } = useWorkspace();
  const [rows, setRows] = useState<Record<string, RowState>>(() =>
    Object.fromEntries(
      targets.map((target) => [target.key, { status: "pending" }]),
    ),
  );
  const [running, setRunning] = useState(false);
  const copy = COPY[action];
  const stateOf = (key: string): RowState => rows[key] ?? { status: "pending" };
  const done = targets.every((target) => stateOf(target.key).status === "done");
  const setRow = (key: string, state: RowState) =>
    setRows((current) => ({ ...current, [key]: state }));

  function close() {
    onClose(
      new Set(
        targets
          .filter((target) => stateOf(target.key).status === "done")
          .map((target) => target.key),
      ),
    );
  }

  async function refusal(): Promise<string | undefined> {
    let state: CapabilityState = unavailableCapabilities;
    if (currentWorkspaceId) {
      try {
        const result = await client.query({
          query: ViewerCapabilitiesDocument,
          variables: { ownerId: currentWorkspaceId, fresh: true },
          fetchPolicy: "no-cache",
          errorPolicy: "none",
          context: { queryDeduplication: false },
        });
        const payload = result.data?.viewerCapabilities;
        if (payload) {
          state = {
            status: "ready",
            snapshot: toSnapshot(
              currentWorkspaceId,
              payload.grants ?? [],
              payload.role ?? null,
            ),
          };
        }
      } catch {
        // An unanswerable check sends nothing, like a refusal.
      }
    }
    return bulkLifecycleReason(
      capabilitiesFromState(state, currentWorkspaceId),
      protectedEnvironment,
      t,
    );
  }

  async function runOne(target: LifecycleTarget) {
    setRow(target.key, { status: "running" });
    try {
      await client.mutate({
        mutation: MUTATION[action][target.kind],
        variables: { id: target.id },
      });
      setRow(target.key, { status: "done" });
    } catch (err) {
      setRow(target.key, {
        status: "error",
        message: adminOnlyErrorMessage(err, t("projects.bulkRowError"), t),
      });
    }
  }

  async function run(batch: LifecycleTarget[]) {
    if (running || batch.length === 0) return;
    setRunning(true);
    try {
      const reason = await refusal();
      if (reason) {
        toast.error(reason);
        return;
      }
      await runBounded(batch, BULK_CONCURRENCY, runOne);
      // Render refetches its service list once the batch settles.
      await client
        .refetchQueries({ include: ["Services", "Databases", "KeyValues"] })
        .catch(() => undefined);
    } finally {
      setRunning(false);
    }
  }

  const label = t(copy.confirm);
  return (
    <ConfirmDialog
      open
      onOpenChange={(next) => {
        if (!next && !running) close();
      }}
      title={t(copy.title)}
      description={t(copy.body)}
      phrase={
        action === "suspend" && !done ? suspendPhrase(targets) : undefined
      }
      destructive={action === "suspend" && !done}
      pending={running}
      closeOnConfirm={false}
      confirmLabel={
        done ? (
          t("common.close")
        ) : (
          <>
            {running ? <Loader2 className="animate-spin" /> : null}
            {label}
          </>
        )
      }
      onConfirm={() =>
        done
          ? close()
          : void run(
              targets.filter((target) => stateOf(target.key).status !== "done"),
            )
      }
    >
      <ul className="max-h-52 divide-y overflow-y-auto rounded-md border">
        {targets.map((target) => {
          const state = stateOf(target.key);
          return (
            <li
              key={target.key}
              className="flex items-center gap-3 px-3 py-2 text-sm"
            >
              <div className="min-w-0 flex-1">
                <div className="flex min-w-0 items-center gap-2">
                  <span className="truncate font-medium" title={target.name}>
                    {target.name}
                  </span>
                  <ResourceTypeBadge kind={target.kind} />
                </div>
                {state.status === "error" ? (
                  <p className="mt-1 text-xs text-destructive">
                    {state.message}
                  </p>
                ) : null}
              </div>
              {state.status === "running" ? (
                <span className="flex shrink-0 items-center">
                  <Loader2
                    aria-hidden
                    className="size-4 animate-spin text-muted-foreground"
                  />
                  <span className="sr-only">{t(copy.running)}</span>
                </span>
              ) : state.status === "done" ? (
                <span className="flex shrink-0 items-center">
                  <Check aria-hidden className="size-4 text-emerald-600" />
                  <span className="sr-only">{t(copy.done)}</span>
                </span>
              ) : state.status === "error" ? (
                <Button
                  variant="ghost"
                  size="sm"
                  disabled={running}
                  aria-label={t("projects.bulkRetryRow", {
                    name: target.name,
                  })}
                  onClick={() => void run([target])}
                >
                  <RotateCcw />
                  {t("projects.bulkRetry")}
                </Button>
              ) : null}
            </li>
          );
        })}
      </ul>
    </ConfirmDialog>
  );
}
