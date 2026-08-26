import { Link } from "@tanstack/react-router";
import { Archive, ArchiveRestore, GitPullRequest } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/common/components/ui/tooltip";
import { useArchiveToggle } from "@/features/agent-sessions/hooks/use-archive-toggle";
import { Card, CardContent } from "@/common/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { Skeleton } from "@/common/components/ui/skeleton";
import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";
import { RelativeAge } from "@/common/components/relative-time";
import type {
  AgentSessionArchivedFilter,
  AgentSessionPhase,
  AgentSessionView,
} from "@/features/agent-sessions/types";
import { AgentSessionsEmptyState } from "@/features/agent-sessions/components/empty-state";
import {
  agentSessionStatusPhraseKey,
  sessionTitleShort,
} from "@/features/agent-sessions/lib/mapper";

type ChipVariant = "success" | "destructive" | "secondary" | "outline";

const ACTIVE_CHIP_CLASS =
  "border-transparent bg-blue-600/15 text-blue-700 dark:text-blue-400";

const PHASE_CHIP: Record<
  AgentSessionPhase,
  { variant: ChipVariant; className?: string }
> = {
  completed: { variant: "success" },
  failed: { variant: "destructive" },
  canceled: { variant: "secondary" },
  canceling: { variant: "secondary" },
  hibernated: { variant: "secondary" },
  hibernating: { variant: "outline", className: ACTIVE_CHIP_CLASS },
  creating: { variant: "outline", className: ACTIVE_CHIP_CLASS },
  running: { variant: "outline", className: ACTIVE_CHIP_CLASS },
  resuming: { variant: "outline", className: ACTIVE_CHIP_CLASS },
  redispatching: { variant: "outline", className: ACTIVE_CHIP_CLASS },
};

/**
 * The lifecycle chip: color-keyed on `phase`. Shared by denser list rows and
 * the detail header so the color mapping can't drift.
 */
export function AgentSessionPhaseChip({ phase }: { phase: AgentSessionPhase }) {
  const { t } = useTranslations();
  const chip = PHASE_CHIP[phase] ?? { variant: "outline" as ChipVariant };
  return (
    <Badge variant={chip.variant} className={chip.className}>
      {t(`agentSessions.phase.${phase}`)}
    </Badge>
  );
}

function PrBadge({
  session,
  hideEmpty,
}: {
  session: AgentSessionView;
  hideEmpty?: boolean;
}) {
  const { t } = useTranslations();
  if (session.prNumber == null || session.prNumber === 0) {
    if (hideEmpty) return null;
    return <span className="text-muted-foreground">—</span>;
  }
  const label = t("agentSessions.prBadge", { number: session.prNumber });
  if (!session.prUrl) {
    return (
      <Badge variant="secondary" className="gap-1">
        <GitPullRequest />
        {label}
      </Badge>
    );
  }
  return (
    <a
      href={session.prUrl}
      target="_blank"
      rel="noreferrer"
      onClick={(e) => e.stopPropagation()}
      className="inline-flex"
    >
      <Badge variant="secondary" className="gap-1 hover:bg-secondary/70">
        <GitPullRequest />
        {label}
      </Badge>
    </a>
  );
}

export interface SessionListProps {
  sessions: AgentSessionView[];
  loading: boolean;
  error?: Error;
  onChanged?: () => void | Promise<unknown>;
  onRetry?: () => void;
  onClearFilters?: () => void;
  archiveFilter?: AgentSessionArchivedFilter;
  phase?: AgentSessionPhase;
}

function ArchiveRowAction({
  session,
  busy,
  onToggle,
}: {
  session: AgentSessionView;
  busy: boolean;
  onToggle: (session: AgentSessionView) => void;
}) {
  const { t } = useTranslations();
  const label = session.isArchived
    ? t("agentSessions.unarchive")
    : t("agentSessions.archive");

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          size="icon"
          variant="ghost"
          aria-label={label}
          disabled={busy}
          className="size-7"
          onClick={(e) => {
            e.stopPropagation();
            onToggle(session);
          }}
        >
          {session.isArchived ? (
            <ArchiveRestore className="size-4" />
          ) : (
            <Archive className="size-4" />
          )}
        </Button>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}

export function RecentsRowsSkeleton({ rows = 5 }: { rows?: number }) {
  return (
    <div
      className="space-y-2"
      data-testid="agent-sessions-recents-skeleton"
    >
      {Array.from({ length: rows }).map((_, i) => (
        <Skeleton key={i} className="h-14 w-full rounded-lg" />
      ))}
    </div>
  );
}

/**
 * Default `/agents` history: full-row recents. Archived / All keep the denser
 * table. Phase chips stay on the table and on detail, not on recents.
 */
export function SessionList({
  sessions,
  loading,
  error,
  onChanged,
  onRetry,
  onClearFilters,
  archiveFilter,
  phase,
}: SessionListProps) {
  const { t } = useTranslations();
  const { toggle, busyId } = useArchiveToggle(onChanged);
  const dense = archiveFilter === "archived" || archiveFilter === "all";

  if (loading && sessions.length === 0) {
    return dense ? (
      <Skeleton className="h-40 w-full" />
    ) : (
      <RecentsRowsSkeleton />
    );
  }
  if (error && sessions.length === 0) {
    return (
      <div className="flex flex-wrap items-center gap-3 py-2">
        <p className="text-sm">{t("agentSessions.errorTitle")}</p>
        {onRetry ? (
          <Button size="sm" variant="outline" onClick={onRetry}>
            {t("agentSessions.retry")}
          </Button>
        ) : null}
      </div>
    );
  }
  if (sessions.length === 0) {
    return (
      <AgentSessionsEmptyState
        mode={
          phase ? "filtered" : archiveFilter === "archived" ? "archived" : "default"
        }
        onClearFilters={onClearFilters}
      />
    );
  }

  if (!dense) {
    return (
      <ul className="divide-y" data-testid="agent-sessions-recents">
        {sessions.map((s) => (
          <li key={s.id} className="group flex items-start gap-1">
            <Link
              to="/agents/$agentSessionId"
              params={{ agentSessionId: s.id }}
              search={{
                fromArchived: archiveFilter,
                fromPhase: phase,
              }}
              className="hover:bg-muted/40 min-w-0 flex-1 rounded-md px-2 py-3"
              title={sessionTitleShort(s)}
            >
              <span className="block truncate font-medium">
                {sessionTitleShort(s)}
              </span>
              <span className="text-muted-foreground mt-1 block truncate text-xs">
                {t(agentSessionStatusPhraseKey(s))}
                {" · "}
                {s.repo}
                {" · "}
                <RelativeAge value={s.createdAt} />
              </span>
            </Link>
            <div className="flex shrink-0 items-center gap-1 py-3 pr-1">
              <PrBadge session={s} hideEmpty />
              <ArchiveRowAction
                session={s}
                busy={busyId === s.id}
                onToggle={(session) => void toggle(session)}
              />
            </div>
          </li>
        ))}
      </ul>
    );
  }

  return (
    <Card className="gap-0 overflow-hidden py-0 shadow-none">
      <CardContent className="p-0">
        <Table>
          <TableHeader className="bg-muted/30">
            <TableRow>
              <TableHead className="pl-4">
                {t("agentSessions.colTask")}
              </TableHead>
              <TableHead>{t("agentSessions.colPhase")}</TableHead>
              <TableHead>{t("agentSessions.colPr")}</TableHead>
              <TableHead className="hidden sm:table-cell">
                {t("agentSessions.colCreated")}
              </TableHead>
              <TableHead>
                <span className="sr-only">{t("agentSessions.colActions")}</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sessions.map((s) => (
              <TableRow key={s.id} className="group">
                <TableCell className="max-w-[420px] py-3 pl-4 font-medium">
                  <Link
                    to="/agents/$agentSessionId"
                    params={{ agentSessionId: s.id }}
                    search={{
                      fromArchived: archiveFilter,
                      fromPhase: phase,
                    }}
                    className={cn(
                      "block truncate hover:underline",
                      "group-hover:text-foreground",
                    )}
                    title={sessionTitleShort(s)}
                  >
                    {sessionTitleShort(s)}
                  </Link>
                  <span className="text-muted-foreground mt-1 block truncate text-xs font-normal">
                    {s.repo} · <span className="font-mono">{s.branch}</span> ·{" "}
                    <span className="capitalize">{s.agentConfig.agent}</span>
                  </span>
                </TableCell>
                <TableCell>
                  <span className="inline-flex items-center gap-1.5">
                    <AgentSessionPhaseChip phase={s.phase} />
                    {s.isArchived ? (
                      <Badge variant="outline" className="gap-1">
                        <Archive className="size-3" />
                        {t("agentSessions.archivedBadge")}
                      </Badge>
                    ) : null}
                  </span>
                </TableCell>
                <TableCell>
                  <PrBadge session={s} />
                </TableCell>
                <TableCell className="text-muted-foreground hidden text-sm sm:table-cell">
                  <RelativeAge value={s.createdAt} />
                </TableCell>
                <TableCell className="text-right">
                  <ArchiveRowAction
                    session={s}
                    busy={busyId === s.id}
                    onToggle={(session) => void toggle(session)}
                  />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
