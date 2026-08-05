import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import { GitPullRequest, MoreHorizontal, Plus, Search } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Skeleton } from "@/common/components/ui/skeleton";
import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentSessions } from "@/features/agent-sessions/hooks/use-agent-sessions";
import { agentSessionStatusPhrase } from "@/features/agent-sessions/lib/mapper";
import { requestAgentComposerFocus } from "@/features/agent-sessions/lib/composer-focus";
import type { AgentSessionView } from "@/features/agent-sessions/types";

export interface SessionSidebarProps {
  /** The session currently open in the chat view — highlighted in the list. */
  activeId: string;
}

/** True when a key event's target is an editable element (input-safe guard). */
function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return (
    target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName)
  );
}

/**
 * The left sessions rail on the `/agents*` routes (m44, polished in w3/m45
 * t004): a "New session" affordance carrying the `O` keyboard shortcut
 * (Devin's binding — a bare key so it can't collide with the app's ⌘K search
 * or ⌘B sidebar toggle, guarded to never fire while typing), plus a "Recent"
 * list with a client-side search filter and a More/view-all action reaching
 * the standalone list (`/agents?view=list`). Rows show the task title, a
 * human status phrase derived from phase + PR presence ("PR is ready" /
 * "Working…"), and the PR number as a direct GitHub link. Hidden below `lg`
 * (the header's back link and `/agents` remain the narrow-screen navigation).
 */
export function SessionSidebar({ activeId }: SessionSidebarProps) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { sessions, loading } = useAgentSessions();
  const [searchOpen, setSearchOpen] = useState(false);
  const [query, setQuery] = useState("");

  // Bare `O` (no modifiers, never while typing) opens + focuses New session.
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key.toLowerCase() !== "o") return;
      if (event.metaKey || event.ctrlKey || event.altKey) return;
      if (isEditableTarget(event.target)) return;
      event.preventDefault();
      void navigate({ to: "/agents" }).then(requestAgentComposerFocus);
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [navigate]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return sessions;
    return sessions.filter((s) => {
      const title = (s.agentConfig.task || s.id).toLowerCase();
      return title.includes(q) || s.repo.toLowerCase().includes(q);
    });
  }, [sessions, query]);

  return (
    <aside
      aria-label={t("agentSessions.sidebarLabel")}
      className="bg-muted/20 hidden w-72 shrink-0 flex-col border-r lg:flex"
    >
      <div className="border-b p-3">
        <Button asChild size="sm" className="w-full justify-start">
          <Link to="/agents">
            <Plus className="size-4" />
            {t("agentSessions.newSession")}
            <kbd
              aria-hidden
              className="bg-primary-foreground/20 ml-auto rounded px-1.5 py-0.5 font-sans text-[10px] font-medium"
            >
              O
            </kbd>
          </Link>
        </Button>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-2">
        <div className="flex items-center justify-between px-2 py-1">
          <p className="text-muted-foreground text-xs font-medium tracking-wide uppercase">
            {t("agentSessions.recentSessions")}
          </p>
          <div className="flex items-center gap-0.5">
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="text-muted-foreground hover:text-foreground size-6"
              aria-label={t("agentSessions.sidebarSearch")}
              aria-pressed={searchOpen}
              onClick={() => {
                setSearchOpen((open) => !open);
                setQuery("");
              }}
            >
              <Search className="size-3.5" />
            </Button>
            <Button
              asChild
              variant="ghost"
              size="icon"
              className="text-muted-foreground hover:text-foreground size-6"
            >
              <Link
                to="/agents"
                search={{ view: "list" }}
                aria-label={t("agentSessions.sidebarMore")}
              >
                <MoreHorizontal className="size-3.5" />
              </Link>
            </Button>
          </div>
        </div>

        {searchOpen ? (
          <div className="px-1 pb-2">
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              autoFocus
              aria-label={t("agentSessions.sidebarSearch")}
              placeholder={t("agentSessions.sidebarSearchPlaceholder")}
              className="h-7 text-sm"
            />
          </div>
        ) : null}

        {loading && sessions.length === 0 ? (
          <div className="space-y-1.5 p-1">
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
          </div>
        ) : sessions.length === 0 ? (
          <p className="text-muted-foreground px-2 py-3 text-sm">
            {t("agentSessions.sidebarEmpty")}
          </p>
        ) : filtered.length === 0 ? (
          <p className="text-muted-foreground px-2 py-3 text-sm">
            {t("agentSessions.sidebarNoMatches")}
          </p>
        ) : (
          <ul className="space-y-0.5">
            {filtered.map((s) => (
              <li key={s.id}>
                <SessionRow session={s} active={s.id === activeId} />
              </li>
            ))}
          </ul>
        )}
      </div>
    </aside>
  );
}

/**
 * One recent-session row: the whole row navigates to the session (stretched
 * link — never a nested anchor), while the PR number is a DIRECT external
 * GitHub link layered above it (`stopPropagation` keeps the row from also
 * navigating). The status line is the human phrase from
 * `agentSessionStatusPhrase` ("PR is ready" / "Working…" / …).
 */
function SessionRow({
  session,
  active,
}: {
  session: AgentSessionView;
  active: boolean;
}) {
  const { t } = useTranslations();
  const title = session.agentConfig.task || session.id;
  const phrase = t(
    `agentSessions.statusPhrase.${agentSessionStatusPhrase(session)}`,
  );
  return (
    <div
      className={cn(
        "hover:bg-muted relative rounded-lg px-2 py-2 transition-colors",
        active && "bg-muted",
      )}
    >
      <Link
        to="/agents/$agentSessionId"
        params={{ agentSessionId: session.id }}
        className="focus-visible:outline-none"
        title={title}
      >
        {/* Stretched hit area — makes the whole row the navigation target. */}
        <span aria-hidden className="absolute inset-0 rounded-lg" />
        <span className="text-foreground block truncate text-sm font-medium">
          {title}
        </span>
      </Link>
      <span className="text-muted-foreground mt-0.5 flex items-center gap-1.5 text-xs">
        <span>{phrase}</span>
        {session.prNumber != null ? (
          session.prUrl ? (
            <a
              href={session.prUrl}
              target="_blank"
              rel="noreferrer"
              onClick={(event) => event.stopPropagation()}
              className="hover:text-foreground relative z-10 inline-flex items-center gap-0.5 hover:underline"
            >
              <GitPullRequest className="size-3" aria-hidden />#
              {session.prNumber}
            </a>
          ) : (
            <span className="inline-flex items-center gap-0.5">
              <GitPullRequest className="size-3" aria-hidden />#
              {session.prNumber}
            </span>
          )
        ) : null}
      </span>
    </div>
  );
}
