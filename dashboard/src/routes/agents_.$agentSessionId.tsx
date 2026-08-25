import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { requireAgentsFeature } from "@/common/lib/growthbook/require-agents-feature";
import { translatedTitleHead } from "@/common/lib/document-head";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentSession } from "@/features/agent-sessions/hooks/use-agent-session";
import { SessionChatColumn } from "@/features/agent-sessions/components/session-chat-column";
import { ConversationSkeleton } from "@/features/agent-sessions/components/conversation-skeleton";
import type { ConversationChatHandle } from "@/features/agent-sessions/components/session-conversation";
import {
  AGENT_SESSION_PHASES,
  type AgentSessionArchivedFilter,
  type AgentSessionPhase,
} from "@/features/agent-sessions/types";

interface AgentSessionDetailSearch {
  fromArchived?: AgentSessionArchivedFilter;
  fromPhase?: AgentSessionPhase;
}

// The agent-session detail page (ADR047 § D9), restructured into ONE full-page
// Devin-style chat (w3/m44): a left sessions sidebar + a chat column whose whole
// main pane is the scrollable conversation, with the header on top and the
// state-routed composer docked at the bottom. The conversation IS the page
// (w5/m65) — the evidence side panel and the inline PR card were removed, and a
// session's draft PR (when one was requested) is the header's `#N` badge.
// Metadata (header/PR/failure) comes from phase-aware GraphQL polling and the
// conversation rides the m43 stream — the two degrade independently.
export const Route = createFileRoute("/agents_/$agentSessionId")({
  staticData: { chrome: true },
  component: AgentSessionDetailPage,
  // A DEDICATED pending component (layout shell + skeleton, NO conversation
  // column) — the frame still shows during the blocking title load with no white
  // flash, but the `useChat` stream column must NOT mount during pending: reusing
  // the full page as its own pendingComponent mounted the column twice (pending +
  // real), firing two resume stream GETs and doubling the transcript (w3/m44).
  pendingComponent: AgentSessionDetailPending,
  pendingMs: 0,
  beforeLoad: ({ context, location }) => {
    requireAuth()( { context, location });
    requireAgentsFeature()({ context });
  },
  validateSearch: (
    search: Record<string, unknown>,
  ): AgentSessionDetailSearch => {
    const out: AgentSessionDetailSearch = {};
    if (search.fromArchived === "archived" || search.fromArchived === "all") {
      out.fromArchived = search.fromArchived as AgentSessionArchivedFilter;
    } else if (search.fromArchived === "true") {
      out.fromArchived = "archived";
    }
    if (
      typeof search.fromPhase === "string" &&
      AGENT_SESSION_PHASES.includes(search.fromPhase as AgentSessionPhase)
    ) {
      out.fromPhase = search.fromPhase as AgentSessionPhase;
    }
    return out;
  },
  head: ({ match }) => translatedTitleHead("agentSessions.detailTitle", match),
});

function AgentSessionDetailPage() {
  const { agentSessionId } = Route.useParams();
  const { fromArchived, fromPhase } = Route.useSearch();
  const { session, loading, error, refetch } = useAgentSession(agentSessionId);
  // Lifted from the conversation column so the steering composer can send a live
  // turn through the column's own useChat instance (null ⇒ live path disabled).
  const [chat, setChat] = useState<ConversationChatHandle | null>(null);

  return (
    <DashboardLayout>
      <div className="flex min-h-0 flex-1">
        <div className="flex min-w-0 flex-1 flex-col">
          {loading && !session ? <DetailSkeleton /> : null}
          {!loading && !session && error ? (
            <LoadErrorState message={error.message} />
          ) : null}
          {session ? (
            <SessionChatColumn
              session={session}
              chat={chat}
              onChatStateChange={setChat}
              onChanged={() => refetch()}
              backSearch={{ archived: fromArchived, phase: fromPhase }}
            />
          ) : null}
        </div>
      </div>
    </DashboardLayout>
  );
}

// The pending frame: layout shell (the rail comes from DashboardLayout) +
// skeleton, but deliberately NO SessionConversation (no useChat/stream mount)
// so navigating in doesn't fire a duplicate resume stream fetch before the
// real page mounts (w3/m44).
function AgentSessionDetailPending() {
  return (
    <DashboardLayout>
      <div className="flex min-w-0 min-h-0 flex-1 flex-col">
        <DetailSkeleton />
      </div>
    </DashboardLayout>
  );
}

function DetailSkeleton() {
  return (
    <div
      aria-hidden="true"
      className="flex min-h-0 flex-1 flex-col"
      data-route-skeleton="agent-session-detail"
      data-testid="agent-session-detail-skeleton"
    >
      {/* Keep this frame aligned with SessionDetailHeader's spacing: compact
          two-line metadata at the left, actions at the right, and the
          mobile-only back control. */}
      <div
        data-skeleton-region="session-header"
        className="bg-background/95 supports-backdrop-filter:bg-background/60 flex shrink-0 items-center gap-3 border-b px-4 py-2 backdrop-blur"
      >
        <Skeleton className="size-9 shrink-0 rounded-md lg:hidden" />
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <Skeleton className="h-5 w-20 rounded-md" />
            <Skeleton className="h-4 w-40 max-w-[45vw]" />
          </div>
          <div className="mt-0.5 flex items-center gap-3">
            <Skeleton className="h-3 w-24" />
            <Skeleton className="h-3 w-16" />
            <Skeleton className="h-3 w-12" />
          </div>
        </div>
        <Skeleton className="h-8 w-20 shrink-0 rounded-md" />
      </div>

      {/* The same ConversationSkeleton every transient state between here and
          the real transcript reuses (session-conversation.tsx,
          session-chat-column.tsx, session-conversation-impl.tsx). */}
      <div
        data-skeleton-region="conversation"
        className="min-h-0 flex-1 overflow-hidden"
      >
        <div className="mx-auto w-full max-w-3xl px-4 py-3">
          <ConversationSkeleton />
        </div>
      </div>

      {/* Match SteeringComposer's dock, including the bordered input shell,
          send button, and the hint line beneath it. */}
      <div
        data-skeleton-region="composer"
        className="bg-background shrink-0 border-t"
      >
        <div className="mx-auto w-full max-w-3xl space-y-1.5 px-4 py-2">
          <div className="border-input bg-background flex items-end gap-2 rounded-xl border px-2.5 py-1.5 shadow-xs">
            <Skeleton className="my-1.5 h-4 flex-1" />
            <Skeleton className="h-8 w-20 shrink-0 rounded-xl" />
          </div>
          <Skeleton className="mx-1 h-3 w-52 max-w-[70%]" />
        </div>
      </div>
    </div>
  );
}

function LoadErrorState({ message }: { message: string }) {
  const { t } = useTranslations();
  return (
    <div className="flex-1 overflow-auto p-4 sm:p-6">
      <div className="mx-auto w-full max-w-3xl space-y-4">
        <Link
          to="/agents"
          className="text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5 text-sm"
        >
          <ArrowLeft className="size-4" />
          {t("agentSessions.backToList")}
        </Link>
        <Card>
          <CardHeader>
            <CardTitle className="text-base">
              {t("agentSessions.detailErrorTitle")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-muted-foreground text-sm">{message}</p>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
