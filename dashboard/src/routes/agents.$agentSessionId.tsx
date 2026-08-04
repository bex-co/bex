import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { AlertCircle, ArrowLeft } from "lucide-react";
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
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentSession } from "@/features/agent-sessions/hooks/use-agent-session";
import { SessionDetailHeader } from "@/features/agent-sessions/components/session-detail-header";
import { PrCard } from "@/features/agent-sessions/components/pr-card";
import { EvidencePanel } from "@/features/agent-sessions/components/evidence-panel";
import { SteeringComposer } from "@/features/agent-sessions/components/steering-composer";
import { SessionConversation } from "@/features/agent-sessions/components/session-conversation";
import type { ConversationChatHandle } from "@/features/agent-sessions/components/session-conversation";
import type { AgentSessionView } from "@/features/agent-sessions/types";

// The agent-session detail page (ADR047 § D9): control-plane metadata (header +
// PR + evidence + failure) from t001's phase-aware GraphQL polling, t002's live
// conversation column as the primary narrative, and the single state-routed
// steering composer. Metadata and the conversation degrade independently — a
// down m43 stream shows the conversation's house callout while the header/PR/
// evidence keep rendering off polling.
export const Route = createFileRoute("/agents/$agentSessionId")({
  component: AgentSessionDetailPage,
  // Reuse the component as its own pending state (the detail-route convention —
  // the frame doubles as its loading skeleton, no white flash). Tolerates the
  // absent loader data because the page reads Apollo, not `useLoaderData`.
  pendingComponent: AgentSessionDetailPage,
  pendingMs: 0,
  beforeLoad: requireAuth(),
  head: ({ match }) => translatedTitleHead("agentSessions.detailTitle", match),
});

function AgentSessionDetailPage() {
  const { t } = useTranslations();
  const { agentSessionId } = Route.useParams();
  const { session, loading, error, refetch } = useAgentSession(agentSessionId);
  // Lifted from the conversation column so the steering composer can send a live
  // turn through the column's own useChat instance (null ⇒ live path disabled).
  const [chat, setChat] = useState<ConversationChatHandle | null>(null);

  return (
    <DashboardLayout>
      <div className="flex items-center gap-2 border-b px-4 py-3 sm:px-6">
        <Link
          to="/agents"
          className="text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5 text-sm"
        >
          <ArrowLeft className="size-4" />
          {t("agentSessions.backToList")}
        </Link>
      </div>

      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-6xl space-y-6">
          {loading && !session ? <DetailSkeleton /> : null}
          {!loading && !session && error ? (
            <LoadErrorState message={error.message} />
          ) : null}
          {session ? (
            <SessionDetailBody
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

function SessionDetailBody({
  session,
  chat,
  onChatStateChange,
  onChanged,
}: {
  session: AgentSessionView;
  chat: ConversationChatHandle | null;
  onChatStateChange: (handle: ConversationChatHandle | null) => void;
  onChanged: () => void;
}) {
  const { t } = useTranslations();

  return (
    <>
      <SessionDetailHeader session={session} onCanceled={onChanged} />

      <div className="grid gap-6 lg:grid-cols-3">
        {/* Center: the live transcript (primary narrative) + steering. */}
        <div className="space-y-6 lg:col-span-2">
          <Card className="flex h-[32rem] flex-col overflow-hidden py-0">
            <CardHeader className="border-b py-3">
              <CardTitle className="text-base">
                {t("agentSessions.conversationTitle")}
              </CardTitle>
            </CardHeader>
            <CardContent className="flex min-h-0 flex-1 flex-col p-0">
              <SessionConversation
                sessionId={session.id}
                isTerminal={session.isTerminal}
                onChatStateChange={onChatStateChange}
              />
            </CardContent>
          </Card>

          <SteeringComposer
            session={session}
            chat={chat}
            onSteered={onChanged}
          />
        </div>

        {/* Side: durable metadata cards. */}
        <div className="space-y-6">
          <PrCard session={session} />
          {session.phase === "failed" && session.failureReason ? (
            <Alert variant="destructive">
              <AlertCircle />
              <AlertTitle>{t("agentSessions.failureTitle")}</AlertTitle>
              <AlertDescription>{session.failureReason}</AlertDescription>
            </Alert>
          ) : null}
          <EvidencePanel evidence={session.evidence} />
        </div>
      </div>
    </>
  );
}

function DetailSkeleton() {
  return (
    <>
      <div className="flex items-center justify-between gap-4">
        <div className="space-y-2">
          <Skeleton className="h-7 w-64" />
          <Skeleton className="h-4 w-80" />
        </div>
        <Skeleton className="h-8 w-20" />
      </div>
      <div className="grid gap-6 lg:grid-cols-3">
        <Skeleton className="h-[32rem] w-full lg:col-span-2" />
        <div className="space-y-6">
          <Skeleton className="h-40 w-full" />
          <Skeleton className="h-60 w-full" />
        </div>
      </div>
    </>
  );
}

function LoadErrorState({ message }: { message: string }) {
  const { t } = useTranslations();
  return (
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
  );
}
