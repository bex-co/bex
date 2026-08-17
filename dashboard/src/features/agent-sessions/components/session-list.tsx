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
import { formatRelativeAge } from "@/features/services/lib/format";
import type {
  AgentSessionPhase,
  AgentSessionView,
} from "@/features/agent-sessions/types";
import { AgentSessionsEmptyState } from "@/features/agent-sessions/components/empty-state";

type ChipVariant = "success" | "destructive" | "secondary" | "outline";

// The blue/active treatment for the still-converging phases — mirrors the
// `badge.tsx` `success` variant's shape (soft tinted background + accessible
// foreground in both themes) since there is no built-in "info" variant.
const ACTIVE_CHIP_CLASS =
  "border-transparent bg-blue-600/15 text-blue-700 dark:text-blue-400";

// Phase → chip style. Terminal phases carry their own semantic color
// (completed=green, failed=red, canceled=muted); every still-converging phase
// shares the active/blue treatment (ADR047 D9). Keyed on the stable `phase`
// enum, never the free-text status.
const PHASE_CHIP: Record<
  AgentSessionPhase,
  { variant: ChipVariant; className?: string }
> = {
  completed: { variant: "success" },
  failed: { variant: "destructive" },
  canceled: { variant: "secondary" },
  canceling: { variant: "secondary" },
  // Hibernated is idle-but-resumable (ADR059 D2) — a distinct muted treatment,
  // not the active/blue of a converging phase; hibernating shares active.
  hibernated: { variant: "secondary" },
  hibernating: { variant: "outline", className: ACTIVE_CHIP_CLASS },
  creating: { variant: "outline", className: ACTIVE_CHIP_CLASS },
  running: { variant: "outline", className: ACTIVE_CHIP_CLASS },
  resuming: { variant: "outline", className: ACTIVE_CHIP_CLASS },
  redispatching: { variant: "outline", className: ACTIVE_CHIP_CLASS },
};

/**
 * The lifecycle chip: color-keyed on `phase` (completed=green, failed=red,
 * canceled/canceling=muted, everything still converging=blue/active). Shared by
 * the list rows and (later) the detail header so the color mapping can't drift.
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

/** The draft-PR badge — `#{number}` linking `prUrl` in a new tab when present. */
function PrBadge({ session }: { session: AgentSessionView }) {
  const { t } = useTranslations();
  if (session.prNumber == null) {
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
  /** Re-run the backing list after a row archive/unarchive (ADR065). */
  onChanged?: () => void;
}

/**
 * The per-row archive/unarchive toggle (ADR065 D1): one icon button that flips
 * the session's working-set membership. Presentational — the list holds the
 * single `useArchiveToggle` instance and passes it down, so 50+ rows don't
 * each instantiate the mutations hook. Its own trailing cell keeps the row's
 * navigation link untouched.
 */
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

/**
 * The workspace's agent sessions as a compact table. Repository, branch, and
 * driver sit under the task instead of occupying three competing columns;
 * lifecycle, pull request, and age stay scannable at the right. Clicking a
 * task opens its detail page (`/agents/{id}`).
 */
export function SessionList({
  sessions,
  loading,
  error,
  onChanged,
}: SessionListProps) {
  const { t } = useTranslations();
  const { toggle, busyId } = useArchiveToggle(onChanged);

  if (loading && sessions.length === 0) {
    return <Skeleton className="h-40 w-full" />;
  }
  if (error && sessions.length === 0) {
    return (
      <div className="py-8 text-center">
        <p className="font-medium">{t("agentSessions.errorTitle")}</p>
      </div>
    );
  }
  if (sessions.length === 0) {
    return <AgentSessionsEmptyState />;
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
                    className={cn(
                      "block truncate hover:underline",
                      "group-hover:text-foreground",
                    )}
                    title={s.agentConfig.task}
                  >
                    {s.agentConfig.task || s.id}
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
                  {formatRelativeAge(s.createdAt)}
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
