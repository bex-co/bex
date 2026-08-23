import { useMemo, useState } from "react";
import { Link, useParams } from "@tanstack/react-router";
import { GitPullRequest, Search } from "lucide-react";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarInput,
  SidebarMenu,
  SidebarMenuAction,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/common/components/ui/sidebar.tsx";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentSessions } from "@/features/agent-sessions/hooks/use-agent-sessions";
import {
  agentSessionStatusPhraseKey,
  sessionTitleShort,
} from "@/features/agent-sessions/lib/mapper";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import { cn } from "@/common/lib/utils/utils";

/** Substring match for prose titles — avoids the mention picker's subsequence fallback. */
function sessionSearchMatch(query: string, candidate: string): boolean {
  const q = query.trim().toLowerCase();
  if (q === "") return true;
  return candidate.toLowerCase().includes(q);
}

/**
 * The agent-sessions section of the one dashboard rail (w5/m64) — Devin's
 * contextual list slot: global nav above, the section's working set below,
 * both inside the single `<Sidebar collapsible="icon">`. It renders only on
 * `/agents*` (see `DashboardSidebar`), mirroring Devin, whose slot carries
 * sessions on session routes, pull requests on `/review`, and nothing on
 * `/automations`/`/wiki`.
 *
 * Replaces the standalone `<aside>` w3/m45 t004 shipped, which made `/agents`
 * the only route in the dashboard rendering a second sidebar. Affordances:
 * search over title + repo, human status phrases, and the direct GitHub PR
 * link. (There is no "New session" or "View all" row: the global "Agents" nav
 * item opens the combined create + history page, so both would be redundant.)
 *
 * The whole group hides in icon mode — sessions have no meaningful icon
 * representation, which is Devin's own answer (its collapsed rail keeps nav
 * icons and drops the list).
 *
 * Below `lg` it rides `SidebarProvider`'s mobile Sheet, so sessions stay
 * reachable from the drawer. That is a deliberate improvement on the rail this
 * replaced, which set `hidden … lg:flex`; the combined main page also keeps the
 * full history available at every viewport size.
 */
export function AgentSessionsNavSection() {
  const { t } = useTranslations();
  const { agentSessionId } = useParams({ strict: false });
  const { sessions, loading } = useAgentSessions({
    limit: 20,
  });
  const [searchOpen, setSearchOpen] = useState(false);
  const [query, setQuery] = useState("");

  const filtered = useMemo(
    () =>
      sessions.filter(
        (s) =>
          sessionSearchMatch(query, sessionTitleShort(s)) ||
          sessionSearchMatch(query, s.repo),
      ),
    [sessions, query],
  );

  return (
    <SidebarGroup
      aria-label={t("agentSessions.sidebarLabel")}
      className="min-h-0 group-data-[collapsible=icon]:hidden"
    >
      <div className="flex items-center justify-between gap-1">
        <SidebarGroupLabel className="min-w-0 flex-1">
          {t("agentSessions.recentSessions")}
        </SidebarGroupLabel>
        <div className="flex shrink-0 items-center gap-0.5">
          <button
            type="button"
            aria-label={t("agentSessions.sidebarSearch")}
            aria-pressed={searchOpen}
            onClick={() => {
              setSearchOpen((open) => !open);
              setQuery("");
            }}
            className="text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground flex size-6 cursor-pointer items-center justify-center rounded-md"
          >
            <Search className="size-3.5" />
          </button>
        </div>
      </div>

      {searchOpen ? (
        <div className="pb-1">
          <SidebarInput
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            autoFocus
            aria-label={t("agentSessions.sidebarSearch")}
            placeholder={t("agentSessions.sidebarSearchPlaceholder")}
            className="h-7 text-sm"
          />
        </div>
      ) : null}

      <SidebarGroupContent className="min-h-0 overflow-y-auto">
        {loading && sessions.length === 0 ? (
          <div className="space-y-1.5 p-1">
            <Skeleton className="h-9 w-full" />
            <Skeleton className="h-9 w-full" />
            <Skeleton className="h-9 w-full" />
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
          <SidebarMenu>
            {filtered.map((s) => (
              <SessionRow
                key={s.id}
                session={s}
                active={s.id === agentSessionId}
              />
            ))}
          </SidebarMenu>
        )}
      </SidebarGroupContent>
    </SidebarGroup>
  );
}

/**
 * One recent-session row: a two-line menu button (title over the human status
 * phrase) that navigates to the session, with the PR number as a SIBLING
 * `SidebarMenuAction` — never nested inside the button's anchor, which would
 * be invalid HTML and would swallow the row's own navigation.
 */
function SessionRow({
  session,
  active,
}: {
  session: AgentSessionView;
  active: boolean;
}) {
  const { t } = useTranslations();
  const title = sessionTitleShort(session);
  const hasPrLink =
    session.prNumber != null &&
    session.prNumber !== 0 &&
    Boolean(session.prUrl);

  return (
    <SidebarMenuItem>
      <SidebarMenuButton
        asChild
        isActive={active}
        tooltip={title}
        className="h-auto flex-col items-start gap-0.5 py-1.5"
      >
        <Link
          to="/agents/$agentSessionId"
          params={{ agentSessionId: session.id }}
        >
          <span className="w-full truncate text-sm font-medium">{title}</span>
          <span className="text-muted-foreground flex w-full items-center gap-1.5 text-xs">
            <span className="truncate">
              {t(agentSessionStatusPhraseKey(session))}
            </span>
            {session.prNumber != null &&
            session.prNumber !== 0 &&
            !hasPrLink ? (
              <span className="inline-flex shrink-0 items-center gap-0.5">
                <GitPullRequest className="size-3" aria-hidden />#
                {session.prNumber}
              </span>
            ) : null}
          </span>
        </Link>
      </SidebarMenuButton>
      {hasPrLink ? (
        // Pinned to the STATUS line (bottom), not the title line — Devin puts
        // the PR marker beside the status phrase ("PR is ready · ⑂1"), and the
        // primitive's default `top-1.5` would collide with the title text.
        <SidebarMenuAction
          asChild
          className="peer-data-[size=default]/menu-button:top-auto bottom-1.5 aspect-auto w-auto px-1"
        >
          <a
            href={session.prUrl ?? undefined}
            target="_blank"
            rel="noreferrer"
            className="text-muted-foreground hover:text-foreground inline-flex items-center gap-0.5 text-xs"
          >
            <GitPullRequest className="size-3" aria-hidden />#{session.prNumber}
          </a>
        </SidebarMenuAction>
      ) : null}
    </SidebarMenuItem>
  );
}
