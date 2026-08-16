import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
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
import type { ConversationChatHandle } from "@/features/agent-sessions/components/session-conversation";

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
  beforeLoad: requireAuth(),
  head: ({ match }) => translatedTitleHead("agentSessions.detailTitle", match),
});

function AgentSessionDetailPage() {
  const { agentSessionId } = Route.useParams();
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
              onChanged={() => void refetch()}
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
    <>
      <div className="flex items-center gap-3 border-b px-4 py-3">
        <Skeleton className="h-6 w-16" />
        <Skeleton className="h-6 w-48" />
        <div className="flex-1" />
        <Skeleton className="h-8 w-20" />
      </div>
      <div className="min-h-0 flex-1 space-y-6 p-6">
        <Skeleton className="h-24 w-full max-w-3xl" />
        <Skeleton className="ml-auto h-16 w-64" />
        <Skeleton className="h-40 w-full max-w-3xl" />
      </div>
      <div className="border-t p-4">
        <Skeleton className="h-14 w-full max-w-3xl" />
      </div>
    </>
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
