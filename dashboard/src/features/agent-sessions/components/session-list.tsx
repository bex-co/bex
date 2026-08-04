import { Link } from "@tanstack/react-router";
import { GitPullRequest } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
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
}

/**
 * The workspace's agent sessions as a table: each row shows a phase chip, the
 * task prompt + repo/branch, the driver agent, a draft-PR badge, and the
 * relative created age. Clicking a row opens its detail page (`/agents/{id}`).
 */
export function SessionList({ sessions, loading, error }: SessionListProps) {
  const { t } = useTranslations();

  if (loading && sessions.length === 0) {
    return <Skeleton className="h-40 w-full" />;
  }
  if (error && sessions.length === 0) {
    return (
      <div className="py-10 text-center">
        <p className="font-medium">{t("agentSessions.errorTitle")}</p>
      </div>
    );
  }
  if (sessions.length === 0) {
    return <AgentSessionsEmptyState />;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("agentSessions.listTitle")}</CardTitle>
      </CardHeader>
      <CardContent className="p-0">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("agentSessions.colTask")}</TableHead>
              <TableHead>{t("agentSessions.colRepo")}</TableHead>
              <TableHead>{t("agentSessions.colAgent")}</TableHead>
              <TableHead>{t("agentSessions.colPhase")}</TableHead>
              <TableHead>{t("agentSessions.colPr")}</TableHead>
              <TableHead>{t("agentSessions.colCreated")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sessions.map((s) => (
              <TableRow key={s.id} className="group">
                <TableCell className="max-w-[280px] font-medium">
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
                </TableCell>
                <TableCell className="max-w-[200px] text-sm text-muted-foreground">
                  <span className="block truncate">{s.repo}</span>
                  <span className="block truncate font-mono text-xs">
                    {s.branch}
                  </span>
                </TableCell>
                <TableCell className="text-sm capitalize">
                  {s.agentConfig.agent}
                </TableCell>
                <TableCell>
                  <AgentSessionPhaseChip phase={s.phase} />
                </TableCell>
                <TableCell>
                  <PrBadge session={s} />
                </TableCell>
                <TableCell className="text-sm text-muted-foreground">
                  {formatRelativeAge(s.createdAt)}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
