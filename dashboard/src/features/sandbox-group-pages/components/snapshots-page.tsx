import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { toast } from "sonner";
import { Button } from "@/common/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/common/components/ui/sheet";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { CodeBlock } from "@/common/components/code-block";
import { LocalDateTime } from "@/common/components/local-time";
import { formatBytes } from "@/common/lib/format/format-bytes";
import { refusalReason } from "@/common/lib/graphql-error";
import { useDebounce } from "@/common/hooks/use-debounce";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useCursorPages,
  useDeleteSnapshot,
  useGroupSnapshots,
  useSandboxesStartedFrom,
  useSnapshotDetails,
  type SnapshotRow,
} from "@/features/sandbox-group-pages/hooks/use-group-pages";
import { useSandboxGroupDetail } from "@/features/sandbox-group-pages/lib/group-detail";
import {
  snapshotFailureKey,
  snapshotNeverExpires,
  type TimeRange,
} from "@/features/sandbox-group-pages/lib/sandbox-view";
import {
  API_BASE_URL,
  cliSnapshot,
  cliStartFromSnapshot,
  SNAPSHOT_SDK_FILE,
  sdkRestoreSnapshot,
  sdkSnapshot,
  type SdkLanguage,
} from "@/features/sandbox-group-pages/lib/starter-code";
import {
  DetailRow,
  IdWithCopy,
  LoadError,
  PagingControls,
  SdkLanguageToggle,
  SearchBox,
  SnapshotStatusBadge,
  StarterPanel,
  StartedSandboxesList,
  TimeRangeSelect,
} from "./parts";
import {
  SnapshotsListSkeleton,
  DrawerBodySkeleton,
} from "./group-page-skeletons";

/**
 * A sandbox group's Snapshots page, Render's (w1/m658): its snapshots with
 * their statuses and failure texts, a search and a time range, and a
 * snapshot's drawer with its details, the sandboxes started from it, starter
 * code to restore it, and Delete snapshot.
 */
export function SnapshotsPage({
  openSnapshotId,
  onOpen,
  onClose,
}: {
  openSnapshotId: string | undefined;
  onOpen: (snapshotId: string) => void;
  onClose: () => void;
}) {
  const { t } = useTranslations();
  const { group } = useSandboxGroupDetail();
  const [search, setSearch] = useState("");
  const [range, setRange] = useState<TimeRange>("all");
  const pages = useCursorPages();
  const query = useDebounce(search);
  const list = useGroupSnapshots({
    groupId: group.id,
    ownerId: group.ownerId,
    search: query,
    range,
    cursor: pages.cursor,
  });
  // A new question starts again at the first page.
  const onSearch = (value: string) => {
    setSearch(value);
    pages.reset();
  };
  const onRange = (value: TimeRange) => {
    setRange(value);
    pages.reset();
  };
  const filtered = search.trim() !== "" || range !== "all";
  const empty = !list.loading && !list.error && list.snapshots.length === 0;

  return (
    <div className="space-y-6">
      <div
        data-skeleton-region="snapshots-toolbar"
        className="flex flex-col gap-2 sm:flex-row sm:items-center"
      >
        <SearchBox
          value={search}
          onChange={onSearch}
          placeholder={t("sandboxGroupPages.searchSnapshots")}
        />
        <TimeRangeSelect value={range} onChange={onRange} />
      </div>
      {list.loading ? (
        <SnapshotsListSkeleton />
      ) : list.error ? (
        <LoadError
          title={t("sandboxGroupPages.loadSnapshotsError")}
          onRetry={() => void list.refetch()}
        />
      ) : empty && filtered ? (
        <div className="flex flex-col items-center gap-2 rounded-lg border py-10 text-center">
          <p className="text-sm font-medium">
            {t("sandboxGroupPages.noMatchingSnapshots")}
          </p>
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              onSearch("");
              onRange("all");
            }}
          >
            {t("sandboxGroupPages.clearFilters")}
          </Button>
        </div>
      ) : empty ? (
        <StarterPanel
          title={t("sandboxGroupPages.firstSnapshotTitle")}
          description={t("sandboxGroupPages.firstSnapshotDescription")}
          workspaceId={group.ownerId}
          cli={[
            {
              title: t("sandboxGroupPages.starterSnapshotSandbox"),
              command: cliSnapshot(),
            },
            {
              title: t("sandboxGroupPages.starterStartFromSnapshot"),
              command: cliStartFromSnapshot(group.id),
            },
          ]}
          sdk={{
            title: t("sandboxGroupPages.starterSnapshotCode"),
            description: t("sandboxGroupPages.firstSnapshotNeedsRunning"),
            file: SNAPSHOT_SDK_FILE,
            code: (language, apiBaseUrl) =>
              sdkSnapshot(language, {
                apiBaseUrl,
                workspaceId: group.ownerId,
                sandboxGroupId: group.id,
              }),
          }}
        />
      ) : (
        <SnapshotTable rows={list.snapshots} onOpen={onOpen} />
      )}
      <PagingControls
        page={pages.page}
        hasMore={list.hasMore}
        onPrevious={pages.previous}
        onNext={() => pages.next(list.cursor)}
      />
      {openSnapshotId ? (
        <SnapshotDrawer
          snapshotId={openSnapshotId}
          rows={list.snapshots}
          onClose={onClose}
          onDeleted={() => {
            onClose();
            void list.refetch();
          }}
        />
      ) : null}
    </div>
  );
}

function Expires({ row }: { row: SnapshotRow }) {
  const { t } = useTranslations();
  return snapshotNeverExpires(row) ? (
    <>{t("sandboxGroupPages.never")}</>
  ) : (
    <LocalDateTime value={row.expiresAt} />
  );
}

function SnapshotTable({
  rows,
  onOpen,
}: {
  rows: SnapshotRow[];
  onOpen: (id: string) => void;
}) {
  const { t } = useTranslations();
  return (
    <>
      <div className="hidden overflow-hidden rounded-lg border md:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("sandboxGroupPages.colSnapshot")}</TableHead>
              <TableHead>{t("sandboxGroupPages.colStatus")}</TableHead>
              <TableHead>{t("sandboxGroupPages.colSourceSandbox")}</TableHead>
              <TableHead>{t("sandboxGroupPages.colSize")}</TableHead>
              <TableHead>{t("sandboxGroupPages.colCaptured")}</TableHead>
              <TableHead>{t("sandboxGroupPages.colExpires")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow
                key={row.id}
                className="cursor-pointer"
                onClick={() => onOpen(row.id)}
              >
                <TableCell className="max-w-64">
                  <button
                    type="button"
                    className="block max-w-full truncate text-left font-medium hover:underline"
                    onClick={(e) => {
                      e.stopPropagation();
                      onOpen(row.id);
                    }}
                  >
                    {row.name ?? row.id}
                  </button>
                  {row.name ? (
                    <span className="text-muted-foreground block truncate font-mono text-xs">
                      {row.id}
                    </span>
                  ) : null}
                </TableCell>
                <TableCell>
                  <SnapshotStatusBadge status={row.status} />
                  {row.status === "failed" ? (
                    <span className="text-muted-foreground block text-xs">
                      {t(snapshotFailureKey(row.failureCode))}
                    </span>
                  ) : null}
                </TableCell>
                <TableCell className="font-mono text-xs">
                  {row.sourceSandboxId}
                </TableCell>
                <TableCell className="text-sm tabular-nums">
                  {row.sizeBytes != null ? formatBytes(row.sizeBytes, 1) : "—"}
                </TableCell>
                <TableCell className="text-muted-foreground text-sm">
                  <LocalDateTime value={row.capturedAt} />
                </TableCell>
                <TableCell className="text-muted-foreground text-sm">
                  <Expires row={row} />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <ul className="space-y-2 md:hidden">
        {rows.map((row) => (
          <li key={row.id}>
            <button
              type="button"
              className="w-full space-y-1 rounded-lg border p-3 text-left"
              onClick={() => onOpen(row.id)}
            >
              <span className="flex items-center justify-between gap-2">
                <span className="truncate text-sm font-medium">
                  {row.name ?? row.id}
                </span>
                <SnapshotStatusBadge status={row.status} />
              </span>
              <span className="text-muted-foreground block truncate font-mono text-xs">
                {row.id}
              </span>
              <span className="text-muted-foreground block text-xs">
                <LocalDateTime value={row.requestedAt} />
              </span>
            </button>
          </li>
        ))}
      </ul>
    </>
  );
}

function SnapshotDrawer({
  snapshotId,
  rows,
  onClose,
  onDeleted,
}: {
  snapshotId: string;
  rows: SnapshotRow[];
  onClose: () => void;
  onDeleted: () => void;
}) {
  const { t } = useTranslations();
  const { group } = useSandboxGroupDetail();
  const listed = rows.find((r) => r.id === snapshotId);
  // A link to a snapshot not on the open page reads it on its own.
  const byId = useSnapshotDetails(
    group.id,
    group.ownerId,
    snapshotId,
    Boolean(listed),
  );
  const snapshot = listed ?? byId.snapshot;
  const started = useSandboxesStartedFrom(
    group.id,
    group.ownerId,
    snapshot ? { snapshotId: snapshot.id } : null,
  );
  const { remove, pending } = useDeleteSnapshot(group.id, group.ownerId);
  const [confirming, setConfirming] = useState(false);
  const [language, setLanguage] = useState<SdkLanguage>("typescript");
  const name = snapshot?.name ?? snapshotId;
  const phrase = `sudo delete snapshot ${name}`;

  const runDelete = async () => {
    try {
      await remove(snapshotId);
      setConfirming(false);
      toast.success(t("sandboxGroupPages.snapshotDeleted", { name }));
      onDeleted();
    } catch (err) {
      setConfirming(false);
      toast.error(
        snapshot?.status === "creating"
          ? refusalReason(err)
          : t("sandboxGroupPages.deleteSnapshotError"),
      );
    }
  };

  return (
    <Sheet open onOpenChange={(open) => !open && onClose()}>
      <SheetContent
        side="right"
        className="w-full gap-0 overflow-y-auto sm:max-w-xl"
      >
        {!snapshot && byId.loading ? (
          <DrawerBodySkeleton />
        ) : !snapshot && byId.error ? (
          <div className="p-4 pr-12">
            <SheetTitle className="sr-only">{snapshotId}</SheetTitle>
            <LoadError
              title={t("sandboxGroupPages.loadSnapshotsError")}
              onRetry={() => void byId.refetch()}
            />
          </div>
        ) : !snapshot ? (
          <SheetHeader>
            <SheetTitle>{t("sandboxGroupPages.snapshotGoneTitle")}</SheetTitle>
            <SheetDescription>
              {t("sandboxGroupPages.snapshotGoneDescription", {
                id: snapshotId,
              })}
            </SheetDescription>
          </SheetHeader>
        ) : (
          <>
            <SheetHeader className="border-b pr-12">
              <div className="flex items-center gap-2">
                <SheetTitle className="truncate">{name}</SheetTitle>
                <SnapshotStatusBadge status={snapshot.status} />
              </div>
              <SheetDescription asChild>
                <div>
                  <IdWithCopy
                    id={snapshot.id}
                    label={t("sandboxGroupPages.copySnapshotId")}
                  />
                </div>
              </SheetDescription>
            </SheetHeader>
            <div className="space-y-6 p-4">
              {snapshot.status === "failed" ? (
                <p className="text-sm">
                  {t(snapshotFailureKey(snapshot.failureCode))}{" "}
                  {t("sandboxGroupPages.snapshotFailedHelp")}
                </p>
              ) : null}
              <section className="space-y-2">
                <h3 className="text-sm font-semibold">
                  {t("sandboxGroupPages.tabDetails")}
                </h3>
                <dl className="divide-y">
                  <DetailRow label={t("sandboxGroupPages.detailType")}>
                    {snapshot.kind === "runtime"
                      ? t("sandboxGroupPages.kindRuntime")
                      : t("sandboxGroupPages.kindFilesystem")}
                  </DetailRow>
                  <DetailRow
                    label={t("sandboxGroupPages.detailResourceConstraint")}
                  >
                    {snapshot.planSpec
                      ? t("sandboxGroupPages.planSize", {
                          plan: snapshot.planSpec.name,
                          cpus: snapshot.planSpec.cpus ?? "—",
                          memory: snapshot.planSpec.memoryGb ?? "—",
                        })
                      : "—"}
                  </DetailRow>
                  <DetailRow label={t("sandboxGroupPages.colSourceSandbox")}>
                    <Link
                      to="/sandbox-groups/$sandboxGroupId"
                      params={{ sandboxGroupId: group.id }}
                      search={{ sandbox: snapshot.sourceSandboxId }}
                      className="text-primary font-mono text-xs hover:underline"
                    >
                      {snapshot.sourceSandboxId}
                    </Link>
                  </DetailRow>
                  <DetailRow label={t("sandboxGroupPages.colSize")}>
                    {snapshot.sizeBytes != null
                      ? formatBytes(snapshot.sizeBytes, 1)
                      : "—"}
                  </DetailRow>
                  <DetailRow label={t("sandboxGroupPages.colCaptured")}>
                    <LocalDateTime value={snapshot.capturedAt} />
                  </DetailRow>
                  <DetailRow label={t("sandboxGroupPages.detailLifecycle")}>
                    {snapshot.ttlSeconds
                      ? t("sandboxGroupPages.lifecycleAfterUse", {
                          days: Math.round(snapshot.ttlSeconds / 86_400),
                        })
                      : t("sandboxGroupPages.lifecycleKept")}
                  </DetailRow>
                  <DetailRow label={t("sandboxGroupPages.colExpires")}>
                    <Expires row={snapshot} />
                  </DetailRow>
                </dl>
              </section>
              <section className="space-y-2">
                <h3 className="text-sm font-semibold">
                  {t("sandboxGroupPages.navSandboxes")}
                </h3>
                <StartedSandboxesList
                  groupId={group.id}
                  sandboxes={started.sandboxes}
                  loading={started.loading}
                  empty={t("sandboxGroupPages.noSandboxesFromSnapshot")}
                />
              </section>
              {snapshot.status === "available" ? (
                <section className="space-y-2">
                  <h3 className="text-sm font-semibold">
                    {t("sandboxGroupPages.restoreTitle")}
                  </h3>
                  <SdkLanguageToggle value={language} onChange={setLanguage} />
                  <CodeBlock
                    code={sdkRestoreSnapshot(language, {
                      apiBaseUrl: API_BASE_URL,
                      workspaceId: group.ownerId,
                      sandboxGroupId: group.id,
                      snapshotId: snapshot.id,
                    })}
                    language={SNAPSHOT_SDK_FILE[language]}
                  />
                </section>
              ) : null}
              <Button
                variant="destructive"
                size="sm"
                onClick={() => setConfirming(true)}
              >
                {t("sandboxGroupPages.deleteSnapshot")}
              </Button>
            </div>
          </>
        )}
        <ConfirmDialog
          open={confirming}
          onOpenChange={setConfirming}
          title={t("sandboxGroupPages.deleteSnapshotTitle")}
          description={t("sandboxGroupPages.deleteSnapshotDescription", {
            name,
          })}
          confirmLabel={t("sandboxGroupPages.deleteSnapshot")}
          destructive
          pending={pending}
          phrase={phrase}
          closeOnConfirm={false}
          onConfirm={() => void runDelete()}
        />
      </SheetContent>
    </Sheet>
  );
}
