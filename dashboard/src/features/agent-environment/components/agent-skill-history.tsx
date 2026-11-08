import { useState } from "react";
import { Loader2 } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils.ts";
import {
  useAgentSkillVersion,
  useAgentSkillVersions,
  type AgentSkillVersionSummary,
} from "@/features/agent-environment/hooks/use-agent-skill-versions";
import {
  AgentSkillHistoryFrame,
  AgentSkillHistoryPanelHeader,
  AgentSkillHistorySkeleton,
  VERSION_ROW_CLASS,
} from "@/features/agent-environment/components/agent-skill-history-skeleton";
import {
  parseSkillDiff,
  skillContentLines,
  type SkillDiffRow,
} from "@/features/agent-environment/lib/skill-diff";

interface AgentSkillHistoryProps {
  ownerId: string;
  skillId: string;
  /** Admins only: whether "Load into editor" is enabled. */
  canLoad: boolean;
  onLoad: (content: string, version: number) => void;
}

type View = "changes" | "content";

function formatTime(iso: string): string {
  const date = new Date(iso);
  return Number.isNaN(date.getTime())
    ? iso
    : date.toLocaleString(undefined, {
        dateStyle: "medium",
        timeStyle: "short",
      });
}

function versionMeta(v: AgentSkillVersionSummary): string {
  const parts = [
    v.createdBy ?? "",
    v.createdAt ? formatTime(v.createdAt) : "",
    v.shortId,
  ];
  return parts.filter((part) => part !== "").join(" · ");
}

const ROW_STYLE: Record<SkillDiffRow["kind"], string> = {
  hunk: "bg-muted text-muted-foreground",
  context: "",
  add: "bg-green-500/15 text-green-900 dark:text-green-200",
  remove: "bg-red-500/15 text-red-900 dark:text-red-200",
};
const SIGN: Record<SkillDiffRow["kind"], string> = {
  hunk: "",
  context: " ",
  add: "+",
  remove: "-",
};

function LineRow({
  oldLine,
  newLine,
  sign,
  text,
  className,
  testId,
}: {
  oldLine: number | null;
  newLine: number | null;
  sign: string;
  text: string;
  className?: string;
  testId?: string;
}) {
  return (
    <div
      className={cn("flex min-w-max font-mono text-xs leading-5", className)}
      data-testid={testId}
    >
      <span className="text-muted-foreground w-10 shrink-0 pr-2 text-right select-none">
        {oldLine ?? ""}
      </span>
      <span className="text-muted-foreground w-10 shrink-0 pr-2 text-right select-none">
        {newLine ?? ""}
      </span>
      <span className="w-4 shrink-0 select-none">{sign}</span>
      <span className="pr-3 whitespace-pre">{text}</span>
    </div>
  );
}

/**
 * The skill editor's Version history tab (ADR099 D3): saved versions newest
 * first, and for the selected one either its line diff against the version
 * before it (Changes) or its whole SKILL.md (Content). "Load into editor" hands
 * that content to the Edit tab; saving it there writes a new version.
 */
export function AgentSkillHistory({
  ownerId,
  skillId,
  canLoad,
  onLoad,
}: AgentSkillHistoryProps) {
  const { t } = useTranslations();
  const list = useAgentSkillVersions(ownerId, skillId);
  const [picked, setPicked] = useState<number | null>(null);
  const [view, setView] = useState<View>("changes");
  const selected = picked ?? list.versions[0]?.version ?? null;
  const detail = useAgentSkillVersion(ownerId, skillId, selected);
  const summary = list.versions.find((v) => v.version === selected);

  if (list.loading) return <AgentSkillHistorySkeleton />;

  if (list.error && list.versions.length === 0) {
    return (
      <div
        className="flex h-44 flex-col items-start justify-center gap-3 md:h-96"
        role="alert"
      >
        <p className="text-destructive text-sm">
          {t("agentEnvironment.skills.history.loadFailed")}
        </p>
        <Button size="sm" variant="outline" onClick={list.retry}>
          {t("agentEnvironment.retry")}
        </Button>
      </div>
    );
  }

  const rows = view === "changes" ? parseSkillDiff(detail.version?.diff) : [];
  const lines =
    view === "content" ? skillContentLines(detail.version?.content) : [];

  let body;
  if (detail.error && !detail.version) {
    body = (
      <div className="flex flex-col items-start gap-3 p-3" role="alert">
        <p className="text-destructive text-sm">
          {t("agentEnvironment.skills.history.versionFailed")}
        </p>
        <Button size="sm" variant="outline" onClick={detail.retry}>
          {t("agentEnvironment.retry")}
        </Button>
      </div>
    );
  } else if (detail.loading || !detail.version) {
    body = (
      <div className="space-y-2 p-3" data-testid="agent-skill-version-loading">
        <Skeleton className="h-3 w-full" />
        <Skeleton className="h-3 w-5/6" />
        <Skeleton className="h-3 w-2/3" />
      </div>
    );
  } else if (view === "changes" && rows.length === 0) {
    body = (
      <p className="text-muted-foreground p-3 text-sm">
        {t("agentEnvironment.skills.history.noChanges")}
      </p>
    );
  } else if (view === "changes") {
    body = (
      <div className="py-1" data-testid="agent-skill-diff">
        {rows.map((row, i) => (
          <LineRow
            key={i}
            testId={`diff-${row.kind}`}
            className={ROW_STYLE[row.kind]}
            oldLine={row.kind === "hunk" ? null : row.oldLine}
            newLine={row.kind === "hunk" ? null : row.newLine}
            sign={SIGN[row.kind]}
            text={row.text}
          />
        ))}
      </div>
    );
  } else {
    body = (
      <div className="py-1" data-testid="agent-skill-content">
        {lines.map((line, i) => (
          <LineRow key={i} oldLine={null} newLine={i + 1} sign="" text={line} />
        ))}
      </div>
    );
  }

  return (
    <AgentSkillHistoryFrame
      list={
        <>
          <ul data-testid="agent-skill-versions">
            {list.versions.map((v) => (
              <li key={v.version} className="border-b">
                <button
                  type="button"
                  aria-current={v.version === selected}
                  data-testid={`version-${v.version}`}
                  onClick={() => setPicked(v.version)}
                  className={cn(
                    VERSION_ROW_CLASS,
                    "hover:bg-muted/60 gap-0.5 px-3 text-left",
                    v.version === selected && "bg-muted",
                  )}
                >
                  <span className="flex min-w-0 items-center gap-2">
                    <span className="truncate text-sm">{v.message}</span>
                    {v.current ? (
                      <Badge className="shrink-0">
                        {t("agentEnvironment.skills.history.current")}
                      </Badge>
                    ) : null}
                  </span>
                  <span className="text-muted-foreground truncate text-xs">
                    {versionMeta(v)}
                  </span>
                </button>
              </li>
            ))}
          </ul>
          {list.hasMore ? (
            <div className="p-2">
              {list.loadMoreFailed ? (
                <p className="text-destructive pb-2 text-xs" role="alert">
                  {t("agentEnvironment.skills.history.loadFailed")}
                </p>
              ) : null}
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="w-full"
                disabled={list.loadingMore}
                onClick={() => void list.loadMore()}
              >
                {list.loadingMore ? <Loader2 className="animate-spin" /> : null}
                {t("agentEnvironment.skills.history.loadMore")}
              </Button>
            </div>
          ) : null}
        </>
      }
      panel={
        <>
          <AgentSkillHistoryPanelHeader
            meta={
              summary ? (
                <>
                  <span className="truncate text-sm">{summary.message}</span>
                  <span className="text-muted-foreground truncate text-xs">
                    {versionMeta(summary)}
                  </span>
                </>
              ) : null
            }
            controls={
              <>
                <div className="flex gap-1" role="group">
                  {(["changes", "content"] as const).map((v) => (
                    <Button
                      key={v}
                      type="button"
                      size="sm"
                      variant={view === v ? "secondary" : "ghost"}
                      aria-pressed={view === v}
                      onClick={() => setView(v)}
                    >
                      {t(`agentEnvironment.skills.history.${v}`)}
                    </Button>
                  ))}
                </div>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={!canLoad || !detail.version?.content}
                  onClick={() => {
                    if (detail.version?.content != null)
                      onLoad(detail.version.content, detail.version.version);
                  }}
                >
                  {t("agentEnvironment.skills.history.load")}
                </Button>
              </>
            }
          />
          <div className="min-h-0 flex-1 overflow-auto">{body}</div>
        </>
      }
    />
  );
}
