import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { MoreHorizontal } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/common/components/ui/sheet";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/common/components/ui/tabs";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { LocalDateTime } from "@/common/components/local-time";
import { formatBytes } from "@/common/lib/format/format-bytes";
import { regionLabel } from "@/common/lib/region-label";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useGroupSnapshots,
  useSandboxDetails,
  useSandboxEvents,
  useTerminateSandboxes,
  type SandboxEventRow,
  type SandboxRow,
} from "@/features/sandbox-group-pages/hooks/use-group-pages";
import { useSandboxGroupDetail } from "@/features/sandbox-group-pages/lib/group-detail";
import {
  formatSeconds,
  isEndedSandbox,
  LIFECYCLE_LABEL,
  snapshotFailureKey,
  TRIGGER_LABEL,
} from "@/features/sandbox-group-pages/lib/sandbox-view";
import {
  DetailRow,
  IdWithCopy,
  LoadError,
  SandboxDuration,
  SandboxStatusBadge,
  SnapshotStatusBadge,
} from "./parts";
import {
  EventsFrame,
  SandboxDrawerSkeleton,
  SnapshotRowsFrame,
} from "./drawer-frames";
import {
  SANDBOX_DRAWER_TABS,
  type SandboxDrawerTab,
} from "@/features/sandbox-group-pages/lib/drawer-tabs";

const TAB_LABEL = {
  events: "sandboxGroupPages.tabEvents",
  snapshots: "sandboxGroupPages.tabSnapshots",
  details: "sandboxGroupPages.tabDetails",
} as const;

/**
 * Render's sandbox details drawer (w1/m658): the sandbox's name and id, its
 * status, Terminate sandbox in its menu, and the Events, Snapshots and
 * Details tabs. Render's Logs tab waits on the logs API's `sandbox` filter
 * (w1/162), and its Shell tab is behind a flag Render documents nothing of.
 */
export function SandboxDrawer({
  sandboxId,
  tab,
  onTabChange,
  onClose,
  onTerminated,
}: {
  sandboxId: string;
  tab: SandboxDrawerTab;
  onTabChange: (tab: SandboxDrawerTab) => void;
  onClose: () => void;
  onTerminated: () => void;
}) {
  const { t } = useTranslations();
  const { group } = useSandboxGroupDetail();
  const { sandbox, loading, notFound, error, refetch } = useSandboxDetails(
    sandboxId,
    group.ownerId,
  );
  const { terminate, pending } = useTerminateSandboxes(group.ownerId);
  const [confirming, setConfirming] = useState(false);
  const inGroup = sandbox && sandbox.sandboxGroupId === group.id;

  const runTerminate = async () => {
    const failed = await terminate([sandboxId]);
    setConfirming(false);
    if (failed.length > 0) {
      toast.error(t("sandboxGroupPages.terminateFailed", { count: 1 }));
      return;
    }
    toast.success(t("sandboxGroupPages.terminateStarted", { count: 1 }));
    void refetch();
    onTerminated();
  };

  return (
    <Sheet open onOpenChange={(open) => !open && onClose()}>
      <SheetContent
        side="right"
        className="w-full gap-0 overflow-y-auto sm:max-w-xl"
        data-route-skeleton={!sandbox ? "sandbox-drawer" : undefined}
      >
        {loading ? (
          <>
            <SheetTitle className="sr-only">{sandboxId}</SheetTitle>
            <SandboxDrawerSkeleton tab={tab} />
          </>
        ) : error ? (
          <div className="p-4 pr-12">
            <SheetTitle className="sr-only">{sandboxId}</SheetTitle>
            <LoadError
              title={t("sandboxGroupPages.loadSandboxError")}
              onRetry={() => void refetch()}
            />
          </div>
        ) : notFound || !inGroup ? (
          <SheetHeader>
            <SheetTitle>{t("sandboxGroupPages.sandboxGoneTitle")}</SheetTitle>
            <SheetDescription>
              {t("sandboxGroupPages.sandboxGoneDescription", { id: sandboxId })}
            </SheetDescription>
          </SheetHeader>
        ) : (
          <>
            <SheetHeader className="border-b pr-12">
              {/* The menu's 32 px whether or not the sandbox still has one,
                  so the header keeps its height when the sandbox ends. */}
              <div className="flex min-h-8 items-center gap-2">
                <SheetTitle className="truncate">
                  {sandbox.name ?? sandbox.id}
                </SheetTitle>
                <SandboxStatusBadge status={sandbox.status} />
                {!isEndedSandbox(sandbox.status) ? (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button
                        size="icon-sm"
                        variant="ghost"
                        className="ml-auto"
                        aria-label={t("sandboxGroupPages.sandboxActions")}
                      >
                        <MoreHorizontal className="size-4" aria-hidden="true" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem
                        variant="destructive"
                        onSelect={() => setConfirming(true)}
                      >
                        {t("sandboxGroupPages.terminateSandbox")}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                ) : null}
              </div>
              <SheetDescription asChild>
                <div>
                  <IdWithCopy
                    id={sandbox.id}
                    label={t("sandboxGroupPages.copySandboxId")}
                  />
                </div>
              </SheetDescription>
            </SheetHeader>
            <Tabs
              value={tab}
              onValueChange={(value) => onTabChange(value as SandboxDrawerTab)}
              className="gap-0"
            >
              <TabsList variant="underline" className="justify-start px-2">
                {SANDBOX_DRAWER_TABS.map((key) => (
                  <TabsTrigger
                    key={key}
                    value={key}
                    className="px-3 py-2 text-sm"
                  >
                    {t(TAB_LABEL[key])}
                  </TabsTrigger>
                ))}
              </TabsList>
              <TabsContent value="events" className="p-4">
                <EventsTab sandbox={sandbox} ownerId={group.ownerId} />
              </TabsContent>
              <TabsContent value="snapshots" className="p-4">
                <SnapshotsTab sandbox={sandbox} />
              </TabsContent>
              <TabsContent value="details" className="p-4">
                <DetailsTab sandbox={sandbox} />
              </TabsContent>
            </Tabs>
          </>
        )}
        <ConfirmDialog
          open={confirming}
          onOpenChange={setConfirming}
          title={t("sandboxGroupPages.terminateTitle", { count: 1 })}
          description={t("sandboxGroupPages.terminateDescription", {
            count: 1,
          })}
          confirmLabel={t("sandboxGroupPages.terminateConfirm")}
          destructive
          pending={pending}
          closeOnConfirm={false}
          onConfirm={() => void runTerminate()}
        />
      </SheetContent>
    </Sheet>
  );
}

const OUTCOME_DOT: Record<string, string> = {
  SUCCESS: "bg-emerald-600",
  ERROR: "bg-destructive",
  IN_PROGRESS: "bg-primary animate-pulse",
};

function EventsTab({
  sandbox,
  ownerId,
}: {
  sandbox: SandboxRow;
  ownerId: string;
}) {
  const { t } = useTranslations();
  const events = useSandboxEvents(sandbox.id, ownerId, sandbox.status);
  if (events.loading) return <EventsFrame />;
  if (events.error) {
    return <p className="text-sm">{t("sandboxGroupPages.loadEventsError")}</p>;
  }
  if (events.events.length === 0) {
    return (
      <p className="text-muted-foreground text-sm">
        {t("sandboxGroupPages.noEvents")}
      </p>
    );
  }
  return (
    <div className="space-y-3">
      <ol className="space-y-3">
        {events.events.map((event) => (
          <li key={event.id} className="flex gap-3">
            <span
              aria-hidden="true"
              className={`mt-1.5 size-2 shrink-0 rounded-full ${OUTCOME_DOT[event.outcome] ?? "bg-muted-foreground"}`}
            />
            <EventBody event={event} />
          </li>
        ))}
      </ol>
      {events.hasMore ? (
        <Button
          size="sm"
          variant="outline"
          disabled={events.loadingEarlier}
          onClick={() => void events.loadEarlier()}
        >
          {events.loadingEarlier
            ? t("sandboxGroupPages.loadingEarlierEvents")
            : t("sandboxGroupPages.loadEarlierEvents")}
        </Button>
      ) : null}
      {events.earlierError ? (
        <p className="text-sm">{t("sandboxGroupPages.loadEventsError")}</p>
      ) : null}
    </div>
  );
}

function EventBody({ event }: { event: SandboxEventRow }) {
  const { t } = useTranslations();
  const { group } = useSandboxGroupDetail();
  const time = (
    <span className="text-muted-foreground text-xs">
      <LocalDateTime value={event.timestamp} />
    </span>
  );
  if (event.__typename === "SandboxLifecycleEvent") {
    const label = LIFECYCLE_LABEL[event.lifecycleType];
    const trigger = event.trigger ? TRIGGER_LABEL[event.trigger] : undefined;
    return (
      <div className="min-w-0 space-y-0.5">
        <p className="text-sm font-medium">{label ? t(label) : event.label}</p>
        {trigger || event.userEmail ? (
          <p className="text-muted-foreground text-xs">
            {[trigger ? t(trigger) : null, event.userEmail]
              .filter(Boolean)
              .join(" · ")}
          </p>
        ) : null}
        {time}
      </div>
    );
  }
  if (event.__typename === "SandboxExecutionEvent") {
    return (
      <div className="min-w-0 space-y-0.5">
        <p
          className="truncate font-mono text-sm"
          title={event.fullCommand ?? undefined}
        >
          {event.fullCommand || event.summary}
        </p>
        <p className="text-muted-foreground text-xs">
          {event.executionType === "files"
            ? t("sandboxGroupPages.eventFileTransfer", {
                operation: event.operation,
              })
            : t("sandboxGroupPages.eventCommand", {
                operation: event.operation,
              })}
          {event.exitCode != null
            ? ` · ${t("sandboxGroupPages.exitCode", { code: event.exitCode })}`
            : ""}
        </p>
        <p className="text-muted-foreground font-mono text-xs">
          {event.executionId}
        </p>
        {time}
      </div>
    );
  }
  if (event.__typename === "SandboxSnapshotEvent") {
    return (
      <div className="min-w-0 space-y-0.5">
        <p className="flex items-center gap-2 text-sm font-medium">
          {t("sandboxGroupPages.eventSnapshot")}
          <SnapshotStatusBadge status={event.snapshot.status} />
        </p>
        {event.snapshot.status === "failed" ? (
          <p className="text-muted-foreground text-xs">
            {t(snapshotFailureKey(event.snapshot.failureCode))}
          </p>
        ) : null}
        <Link
          to="/sandbox-groups/$sandboxGroupId/snapshots"
          params={{ sandboxGroupId: group.id }}
          search={{ snapshot: event.snapshot.id }}
          className="text-primary text-xs hover:underline"
        >
          {t("sandboxGroupPages.viewSnapshot")}
        </Link>
        <div>{time}</div>
      </div>
    );
  }
  return null;
}

function SnapshotsTab({ sandbox }: { sandbox: SandboxRow }) {
  const { t } = useTranslations();
  const { group } = useSandboxGroupDetail();
  const { snapshots, loading, error } = useGroupSnapshots({
    groupId: group.id,
    ownerId: group.ownerId,
    sourceSandboxId: sandbox.id,
  });
  if (loading) return <SnapshotRowsFrame />;
  if (error)
    return (
      <p className="text-sm">{t("sandboxGroupPages.loadSnapshotsError")}</p>
    );
  if (snapshots.length === 0) {
    return (
      <p className="text-muted-foreground text-sm">
        {isEndedSandbox(sandbox.status)
          ? t("sandboxGroupPages.noSnapshotsEnded")
          : t("sandboxGroupPages.noSnapshotsRunning")}
      </p>
    );
  }
  return (
    <ul className="divide-y rounded-lg border">
      {snapshots.map((snap) => (
        // The badge shares the name's line so the date and size get the
        // row's whole width on one line: beside the badge, Spanish and
        // Portuguese wrapped them at 390 px and the row outgrew its frame
        // (w1/m658 t009).
        <li key={snap.id} className="space-y-1 p-3">
          <div className="flex items-center justify-between gap-2">
            <Link
              to="/sandbox-groups/$sandboxGroupId/snapshots"
              params={{ sandboxGroupId: group.id }}
              search={{ snapshot: snap.id }}
              className="truncate text-sm font-medium hover:underline"
            >
              {snap.name ?? snap.id}
            </Link>
            <SnapshotStatusBadge status={snap.status} />
          </div>
          <span className="text-muted-foreground block truncate text-xs">
            <LocalDateTime value={snap.requestedAt} />
            {snap.sizeBytes != null
              ? ` · ${formatBytes(snap.sizeBytes, 1)}`
              : ""}
          </span>
        </li>
      ))}
    </ul>
  );
}

function DetailsTab({ sandbox }: { sandbox: SandboxRow }) {
  const { t } = useTranslations();
  const { group } = useSandboxGroupDetail();
  const off = t("sandboxGroupPages.off");
  const policy = sandbox.networkPolicy?.type;
  const policyKey =
    policy === "deny-all"
      ? "sandboxGroupPages.networkDenyAll"
      : policy === "allow-list"
        ? "sandboxGroupPages.networkAllowList"
        : "sandboxGroupPages.networkAllowAll";
  const trigger = sandbox.trigger ? TRIGGER_LABEL[sandbox.trigger] : undefined;
  return (
    <dl className="divide-y">
      <DetailRow label={t("sandboxGroupPages.detailSandboxId")}>
        <IdWithCopy
          id={sandbox.id}
          label={t("sandboxGroupPages.copySandboxId")}
        />
      </DetailRow>
      <DetailRow label={t("sandboxGroupPages.detailGroup")}>
        {group.name}
      </DetailRow>
      <DetailRow label={t("sandboxGroupPages.detailPlan")}>
        {sandbox.planSpec
          ? t("sandboxGroupPages.planSize", {
              plan: sandbox.planSpec.name,
              cpus: sandbox.planSpec.cpus ?? "—",
              memory: sandbox.planSpec.memoryGb ?? "—",
            })
          : (sandbox.plan ?? "—")}
      </DetailRow>
      <DetailRow label={t("sandboxGroupPages.detailCreated")}>
        <LocalDateTime value={sandbox.createdAt} />
      </DetailRow>
      {sandbox.endedAt ? (
        <DetailRow label={t("sandboxGroupPages.detailEnded")}>
          <LocalDateTime value={sandbox.endedAt} />
        </DetailRow>
      ) : null}
      <DetailRow label={t("sandboxGroupPages.detailDuration")}>
        <SandboxDuration sandbox={sandbox} />
      </DetailRow>
      <DetailRow label={t("sandboxGroupPages.detailTimeout")}>
        {formatSeconds(sandbox.timeoutSeconds, off)}
        {sandbox.timeoutAt ? (
          <span className="text-muted-foreground block text-xs">
            <LocalDateTime value={sandbox.timeoutAt} />
          </span>
        ) : null}
      </DetailRow>
      <DetailRow label={t("sandboxGroupPages.detailIdleTimeout")}>
        {formatSeconds(sandbox.idleTimeoutSeconds, off)}
      </DetailRow>
      <DetailRow label={t("sandboxGroupPages.detailNetwork")}>
        {t(policyKey)}
        {policy === "allow-list" && sandbox.networkPolicy?.rules?.length ? (
          <span className="text-muted-foreground block font-mono text-xs">
            {sandbox.networkPolicy.rules.map((r) => r.domain).join(", ")}
          </span>
        ) : null}
      </DetailRow>
      <DetailRow label={t("sandboxGroupPages.detailRegion")}>
        {sandbox.region ? regionLabel(sandbox.region, t) : "—"}
      </DetailRow>
      <DetailRow label={t("sandboxGroupPages.detailSource")}>
        {sandbox.sourceSnapshotId ? (
          <Link
            to="/sandbox-groups/$sandboxGroupId/snapshots"
            params={{ sandboxGroupId: group.id }}
            search={{ snapshot: sandbox.sourceSnapshotId }}
            className="text-primary hover:underline"
          >
            {t("sandboxGroupPages.goToSnapshot")}
          </Link>
        ) : sandbox.sourceBuildId ? (
          <span>
            {t("sandboxGroupPages.importedImage")}{" "}
            {sandbox.sourceBuild?.name ? (
              <Link
                to="/sandbox-groups/$sandboxGroupId/images"
                params={{ sandboxGroupId: group.id }}
                search={{ image: sandbox.sourceBuild.name }}
                className="text-primary hover:underline"
              >
                {t("sandboxGroupPages.goToImage")}
              </Link>
            ) : (
              <span className="font-mono text-xs">{sandbox.sourceBuildId}</span>
            )}
          </span>
        ) : (
          t("sandboxGroupPages.baseImage")
        )}
      </DetailRow>
      <DetailRow label={t("sandboxGroupPages.detailTrigger")}>
        {trigger ? t(trigger) : "—"}
      </DetailRow>
      {sandbox.source?.__typename === "SandboxEmailSource" ? (
        <DetailRow label={t("sandboxGroupPages.detailUser")}>
          {sandbox.source.email}
        </DetailRow>
      ) : null}
      {sandbox.errorReason ? (
        // bex reports UNKNOWN alone: it does not record why a sandbox errored.
        <DetailRow label={t("sandboxGroupPages.detailError")}>
          {t("sandboxGroupPages.errorUnknown")}
        </DetailRow>
      ) : null}
    </dl>
  );
}
