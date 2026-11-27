import { useEffect, useMemo, useState } from "react";
import { AlertTriangle, Columns3, Pause, Play } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Checkbox } from "@/common/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/common/components/ui/tooltip";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { LocalDateTime } from "@/common/components/local-time";
import { useDebounce } from "@/common/hooks/use-debounce";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useCursorPages,
  useGroupSandboxes,
  useRowSelection,
  useTerminateSandboxes,
  type SandboxRow,
} from "@/features/sandbox-group-pages/hooks/use-group-pages";
import { useSandboxGroupDetail } from "@/features/sandbox-group-pages/lib/group-detail";
import {
  DEFAULT_TIME_RANGE,
  erroredShare,
  isEndedSandbox,
  statusFromSearch,
  type TimeRange,
} from "@/features/sandbox-group-pages/lib/sandbox-view";
import {
  cliCreateSandbox,
  SDK_FILE,
  sdkCreateSandbox,
} from "@/features/sandbox-group-pages/lib/starter-code";
import {
  LoadError,
  PagingControls,
  SandboxDuration,
  SandboxStatusBadge,
  SearchBox,
  StarterPanel,
  TimeRangeSelect,
} from "./parts";
import { SandboxDrawer } from "./sandbox-drawer";
import type { SandboxDrawerTab } from "@/features/sandbox-group-pages/lib/drawer-tabs";
import { SandboxesListSkeleton } from "./group-page-skeletons";

const OPTIONAL_COLUMNS = ["created", "duration", "plan", "source"] as const;
type OptionalColumn = (typeof OPTIONAL_COLUMNS)[number];

const COLUMN_LABEL = {
  created: "sandboxGroupPages.colCreated",
  duration: "sandboxGroupPages.colDuration",
  plan: "sandboxGroupPages.colPlan",
  source: "sandboxGroupPages.colSource",
} as const;

export interface SandboxesPageProps {
  /** The sandbox whose drawer is open, from the URL. */
  openSandboxId: string | undefined;
  openTab: SandboxDrawerTab;
  onOpen: (sandboxId: string, tab?: SandboxDrawerTab) => void;
  onClose: () => void;
}

/**
 * A sandbox group's index, Render's Sandboxes list (w1/m658): search by name,
 * id or status, a time range, Total, Errored and Running over the range above
 * a chart of the runs that ended, live updates that Pause holds still, and
 * Terminate on the selected rows. A row opens the sandbox's drawer.
 */
export function SandboxesPage({
  openSandboxId,
  openTab,
  onOpen,
  onClose,
}: SandboxesPageProps) {
  const { t } = useTranslations();
  const { group } = useSandboxGroupDetail();
  const [search, setSearch] = useState("");
  const [range, setRange] = useState<TimeRange>(DEFAULT_TIME_RANGE);
  const [live, setLive] = useState(true);
  const pages = useCursorPages();
  const [hidden, setHidden] = useState<Set<OptionalColumn>>(new Set());
  const [confirming, setConfirming] = useState(false);
  const query = useDebounce(search);
  const status = statusFromSearch(query, t);
  const list = useGroupSandboxes({
    groupId: group.id,
    ownerId: group.ownerId,
    search: query,
    status,
    range,
    cursor: pages.cursor,
    live,
  });
  const rowIds = useMemo(
    () => list.sandboxes.map((s) => s.id),
    [list.sandboxes],
  );
  const selection = useRowSelection(rowIds);
  const { terminate, pending } = useTerminateSandboxes(group.ownerId);
  const filtered = search.trim() !== "" || range !== DEFAULT_TIME_RANGE;

  // A new question starts again at the first page.
  const onSearch = (value: string) => {
    setSearch(value);
    pages.reset();
  };
  const onRange = (value: TimeRange) => {
    setRange(value);
    pages.reset();
  };

  // Render: while the list shows only running sandboxes, a drawer whose
  // sandbox stops moves to a running one.
  useEffect(() => {
    if (status !== "running" || !openSandboxId) return;
    const open = list.sandboxes.find((s) => s.id === openSandboxId);
    if (open || list.loading) return;
    const next = list.sandboxes[0];
    if (next) {
      toast.info(t("sandboxGroupPages.switchedToRunning"));
      onOpen(next.id, openTab);
    }
  }, [list.loading, list.sandboxes, onOpen, openSandboxId, openTab, status, t]);

  const show = (column: OptionalColumn) => !hidden.has(column);
  const selectedIds = [...selection.selected];
  const runTerminate = async () => {
    const failed = await terminate(selectedIds);
    selection.clear();
    setConfirming(false);
    void list.refetch();
    if (failed.length > 0) {
      toast.error(
        t("sandboxGroupPages.terminateFailed", { count: failed.length }),
      );
    } else {
      toast.success(
        t("sandboxGroupPages.terminateStarted", { count: selectedIds.length }),
      );
    }
  };

  const empty = !list.loading && !list.error && list.sandboxes.length === 0;
  return (
    <div className="space-y-6">
      <div
        data-skeleton-region="sandboxes-toolbar"
        className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center"
      >
        <SearchBox
          value={search}
          onChange={onSearch}
          placeholder={t("sandboxGroupPages.searchSandboxes")}
        />
        <TimeRangeSelect value={range} onChange={onRange} />
        {/* The live-updates label shrinks to an ellipsis rather than push
            the columns button off a narrow screen (Russian's is 383 px, past
            390 px's column; w1/m658 t009). */}
        <div className="flex min-w-0 items-center gap-2 sm:ml-auto">
          <Button
            size="sm"
            variant="outline"
            className="min-w-0 shrink"
            onClick={() => setLive((v) => !v)}
          >
            {live ? (
              <Pause className="size-4" aria-hidden="true" />
            ) : (
              <Play className="size-4" aria-hidden="true" />
            )}
            <span className="truncate">
              {live
                ? t("sandboxGroupPages.pauseLive")
                : t("sandboxGroupPages.resumeLive")}
            </span>
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                size="icon-sm"
                variant="ghost"
                aria-label={t("sandboxGroupPages.showHideColumns")}
              >
                <Columns3 className="size-4" aria-hidden="true" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              {OPTIONAL_COLUMNS.map((column) => (
                <DropdownMenuCheckboxItem
                  key={column}
                  checked={show(column)}
                  onCheckedChange={(on) =>
                    setHidden((prev) => {
                      const next = new Set(prev);
                      if (on) next.delete(column);
                      else next.add(column);
                      return next;
                    })
                  }
                >
                  {t(COLUMN_LABEL[column])}
                </DropdownMenuCheckboxItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      <RunSummary
        runs={list.runs}
        running={list.running}
        loading={list.loading}
      />

      {selection.selected.size > 0 ? (
        <div className="bg-muted/50 flex flex-wrap items-center justify-between gap-2 rounded-md border px-3 py-2">
          <span className="text-sm">
            {t("sandboxGroupPages.selectedCount", {
              count: selection.selected.size,
            })}
          </span>
          <Button
            size="sm"
            variant="destructive"
            onClick={() => setConfirming(true)}
            disabled={pending}
          >
            {t("sandboxGroupPages.terminateSelected")}
          </Button>
        </div>
      ) : null}

      {list.loading ? (
        <SandboxesListSkeleton />
      ) : list.error ? (
        <LoadError
          title={t("sandboxGroupPages.loadSandboxesError")}
          onRetry={() => void list.refetch()}
        />
      ) : empty && filtered ? (
        <div className="flex flex-col items-center gap-2 rounded-lg border py-10 text-center">
          <p className="text-sm font-medium">
            {t("sandboxGroupPages.noMatchingSandboxes")}
          </p>
          <p className="text-muted-foreground text-sm">
            {t("sandboxGroupPages.noMatchingSandboxesHelp")}
          </p>
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              onSearch("");
              onRange(DEFAULT_TIME_RANGE);
            }}
          >
            {t("sandboxGroupPages.clearFilters")}
          </Button>
        </div>
      ) : empty ? (
        <StarterPanel
          title={t("sandboxGroupPages.firstSandboxTitle")}
          description={t("sandboxGroupPages.firstSandboxDescription")}
          workspaceId={group.ownerId}
          cli={[
            {
              title: t("sandboxGroupPages.starterCreateSandbox"),
              command: cliCreateSandbox(group.id),
            },
          ]}
          sdk={{
            title: t("sandboxGroupPages.starterFirstSandboxFile"),
            file: SDK_FILE,
            code: (language, apiBaseUrl) =>
              sdkCreateSandbox(language, {
                apiBaseUrl,
                workspaceId: group.ownerId,
                sandboxGroupId: group.id,
              }),
            run: true,
          }}
        />
      ) : (
        <SandboxTable
          rows={list.sandboxes}
          show={show}
          selected={selection.selected}
          onToggle={selection.toggle}
          onToggleAll={selection.toggleAll}
          onOpen={(id) => onOpen(id)}
        />
      )}
      {/* Outside the branches: a page that came back empty can go back. */}
      <PagingControls
        page={pages.page}
        hasMore={list.hasMore}
        onPrevious={pages.previous}
        onNext={() => pages.next(list.cursor)}
      />

      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={t("sandboxGroupPages.terminateTitle", {
          count: selectedIds.length,
        })}
        description={t("sandboxGroupPages.terminateDescription", {
          count: selectedIds.length,
        })}
        confirmLabel={t("sandboxGroupPages.terminateConfirm")}
        destructive
        pending={pending}
        closeOnConfirm={false}
        onConfirm={() => void runTerminate()}
      />

      {openSandboxId ? (
        <SandboxDrawer
          key={openSandboxId}
          sandboxId={openSandboxId}
          tab={openTab}
          onTabChange={(tab) => onOpen(openSandboxId, tab)}
          onClose={onClose}
          onTerminated={() => void list.refetch()}
        />
      ) : null}
    </div>
  );
}

function RunSummary({
  runs,
  running,
  loading,
}: {
  runs: {
    total: number;
    errored: number;
    succeeded: number;
    buckets: { time: string | null; succeeded: number; errored: number }[];
  } | null;
  running: number | null;
  loading: boolean;
}) {
  const { t, i18n } = useTranslations();
  const buckets = runs?.buckets ?? [];
  const peak = Math.max(1, ...buckets.map((b) => b.succeeded + b.errored));
  const share = runs
    ? erroredShare(runs.total, runs.errored, i18n.language)
    : null;
  const cards = [
    {
      label: t("sandboxGroupPages.summaryTotal"),
      value: String(runs?.total ?? "—"),
    },
    {
      label: t("sandboxGroupPages.summaryErrored"),
      value: share ?? "—",
      tip: t("sandboxGroupPages.summaryErroredTip"),
    },
    {
      label: t("sandboxGroupPages.summaryRunning"),
      value: running == null ? "—" : String(running),
      tip: t("sandboxGroupPages.summaryRunningTip"),
    },
  ];
  return (
    <div data-skeleton-region="run-summary" className="space-y-3">
      <div className="grid grid-cols-3 gap-2 sm:gap-4">
        {cards.map((card) => (
          <Card key={card.label} className="gap-1 py-3">
            <CardHeader className="px-3 py-0 sm:px-4">
              <CardDescription className="truncate">
                {card.tip ? (
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <button
                        type="button"
                        className="cursor-help underline decoration-dotted underline-offset-2"
                      >
                        {card.label}
                      </button>
                    </TooltipTrigger>
                    <TooltipContent className="max-w-64">
                      {card.tip}
                    </TooltipContent>
                  </Tooltip>
                ) : (
                  card.label
                )}
              </CardDescription>
              <CardTitle className="text-base font-semibold tabular-nums">
                {loading ? "—" : card.value}
              </CardTitle>
            </CardHeader>
          </Card>
        ))}
      </div>
      <div className="rounded-lg border p-3">
        {!loading && !runs ? (
          <div className="flex h-[96px] items-center justify-center gap-2 text-sm">
            <AlertTriangle
              className="text-muted-foreground size-4"
              aria-hidden="true"
            />
            {t("sandboxGroupPages.loadRunsError")}
          </div>
        ) : (
          <>
            <div
              className="flex h-[72px] items-end gap-px"
              role="img"
              aria-label={t("sandboxGroupPages.runsChartLabel", {
                succeeded: runs?.succeeded ?? 0,
                errored: runs?.errored ?? 0,
              })}
            >
              {buckets.map((b, i) => (
                <div
                  key={b.time ?? i}
                  className="flex h-full min-w-0 flex-1 flex-col-reverse"
                >
                  <div
                    className="bg-emerald-600/70"
                    style={{ height: `${(b.succeeded / peak) * 100}%` }}
                  />
                  <div
                    className="bg-destructive/70"
                    style={{ height: `${(b.errored / peak) * 100}%` }}
                  />
                </div>
              ))}
            </div>
            <div className="text-muted-foreground mt-2 flex gap-4 text-xs">
              <span className="inline-flex items-center gap-1">
                <span className="inline-block size-2 rounded-sm bg-emerald-600/70" />
                {t("sandboxGroupPages.runsSucceeded")}
              </span>
              <span className="inline-flex items-center gap-1">
                <span className="bg-destructive/70 inline-block size-2 rounded-sm" />
                {t("sandboxGroupPages.runsErrored")}
              </span>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

function SandboxSource({ row }: { row: SandboxRow }) {
  const { t } = useTranslations();
  if (row.sourceSnapshotId) {
    return <span className="font-mono text-xs">{row.sourceSnapshotId}</span>;
  }
  if (row.sourceBuildId) {
    return <span>{row.sourceBuild?.name ?? row.sourceBuildId}</span>;
  }
  return (
    <span className="text-muted-foreground">
      {t("sandboxGroupPages.baseImage")}
    </span>
  );
}

function SandboxTable({
  rows,
  show,
  selected,
  onToggle,
  onToggleAll,
  onOpen,
}: {
  rows: SandboxRow[];
  show: (column: OptionalColumn) => boolean;
  selected: Set<string>;
  onToggle: (id: string, on: boolean) => void;
  onToggleAll: (ids: string[]) => void;
  onOpen: (id: string) => void;
}) {
  const { t } = useTranslations();
  const selectable = rows.filter((r) => !isEndedSandbox(r.status));
  const allSelected =
    selectable.length > 0 && selectable.every((r) => selected.has(r.id));
  return (
    <>
      <div className="hidden overflow-hidden rounded-lg border md:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">
                <Checkbox
                  checked={allSelected}
                  onCheckedChange={(on) =>
                    onToggleAll(on === true ? selectable.map((r) => r.id) : [])
                  }
                  aria-label={t("sandboxGroupPages.selectAll")}
                  disabled={selectable.length === 0}
                />
              </TableHead>
              <TableHead>{t("sandboxGroupPages.colSandbox")}</TableHead>
              <TableHead>{t("sandboxGroupPages.colStatus")}</TableHead>
              {OPTIONAL_COLUMNS.filter(show).map((column) => (
                <TableHead key={column}>{t(COLUMN_LABEL[column])}</TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow
                key={row.id}
                className="cursor-pointer"
                onClick={() => onOpen(row.id)}
              >
                <TableCell onClick={(e) => e.stopPropagation()}>
                  <Checkbox
                    checked={selected.has(row.id)}
                    onCheckedChange={(on) => onToggle(row.id, on === true)}
                    disabled={isEndedSandbox(row.status)}
                    aria-label={t("sandboxGroupPages.selectSandbox", {
                      name: row.name ?? row.id,
                    })}
                  />
                </TableCell>
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
                  <SandboxStatusBadge status={row.status} />
                </TableCell>
                {show("created") ? (
                  <TableCell className="text-muted-foreground text-sm">
                    <LocalDateTime value={row.createdAt} />
                  </TableCell>
                ) : null}
                {show("duration") ? (
                  <TableCell className="text-sm tabular-nums">
                    <SandboxDuration sandbox={row} />
                  </TableCell>
                ) : null}
                {show("plan") ? (
                  <TableCell className="text-sm">{row.plan ?? "—"}</TableCell>
                ) : null}
                {show("source") ? (
                  <TableCell className="max-w-48 truncate text-sm">
                    <SandboxSource row={row} />
                  </TableCell>
                ) : null}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <ul className="space-y-2 md:hidden">
        {rows.map((row) => (
          <li
            key={row.id}
            className="flex items-start gap-3 rounded-lg border p-3"
          >
            <Checkbox
              className="mt-0.5"
              checked={selected.has(row.id)}
              onCheckedChange={(on) => onToggle(row.id, on === true)}
              disabled={isEndedSandbox(row.status)}
              aria-label={t("sandboxGroupPages.selectSandbox", {
                name: row.name ?? row.id,
              })}
            />
            <button
              type="button"
              className="min-w-0 flex-1 space-y-1 text-left"
              onClick={() => onOpen(row.id)}
            >
              <span className="flex items-center justify-between gap-2">
                <span className="truncate text-sm font-medium">
                  {row.name ?? row.id}
                </span>
                <SandboxStatusBadge status={row.status} />
              </span>
              <span className="text-muted-foreground block truncate font-mono text-xs">
                {row.id}
              </span>
              <span className="text-muted-foreground block text-xs tabular-nums">
                <SandboxDuration sandbox={row} />
              </span>
            </button>
          </li>
        ))}
      </ul>
    </>
  );
}
