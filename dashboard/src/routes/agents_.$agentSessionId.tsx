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
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/common/components/ui/sheet";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAgentSession } from "@/features/agent-sessions/hooks/use-agent-session";
import { SessionDetailHeader } from "@/features/agent-sessions/components/session-detail-header";
import { SessionSidebar } from "@/features/agent-sessions/components/session-sidebar";
import { PrCard } from "@/features/agent-sessions/components/pr-card";
import { EvidencePanel } from "@/features/agent-sessions/components/evidence-panel";
import { SteeringComposer } from "@/features/agent-sessions/components/steering-composer";
import { SessionConversation } from "@/features/agent-sessions/components/session-conversation";
import type { ConversationChatHandle } from "@/features/agent-sessions/components/session-conversation";
import type { AgentSessionView } from "@/features/agent-sessions/types";

// The agent-session detail page (ADR047 § D9), restructured into ONE full-page
// Devin-style chat (w3/m44): a left sessions sidebar + a chat column whose whole
// main pane is the scrollable conversation, with the header on top and the
// state-routed composer docked at the bottom. The draft PR renders inline in the
// transcript; evidence lives behind a header-toggled side panel. Metadata
// (header/PR/evidence/failure) comes from phase-aware GraphQL polling and the
// conversation rides the m43 stream — the two degrade independently.
export const Route = createFileRoute("/agents_/$agentSessionId")({
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
  // Evidence relocated behind a header-toggled side panel (t006), closed by default.
  const [evidenceOpen, setEvidenceOpen] = useState(false);

  return (
    <DashboardLayout>
      <div className="flex min-h-0 flex-1">
        <SessionSidebar activeId={agentSessionId} />

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
              onOpenEvidence={() => setEvidenceOpen(true)}
            />
          ) : null}
        </div>
      </div>

      {session ? (
        <EvidenceSidePanel
          session={session}
          open={evidenceOpen}
          onOpenChange={setEvidenceOpen}
        />
      ) : null}
    </DashboardLayout>
  );
}

/** The chat column: header (top) + scrollable transcript + docked composer. */
function SessionChatColumn({
  session,
  chat,
  onChatStateChange,
  onChanged,
  onOpenEvidence,
}: {
  session: AgentSessionView;
  chat: ConversationChatHandle | null;
  onChatStateChange: (handle: ConversationChatHandle | null) => void;
  onChanged: () => void;
  onOpenEvidence: () => void;
}) {
  const { t } = useTranslations();

  // Phase-derived terminus label ("went to sleep" / error / canceled) — only a
  // terminal session shows it (the impl gates on isTerminal + settled).
  const terminalLabel =
    session.phase === "completed"
      ? t("agentSessions.terminalStatus.completed")
      : session.phase === "failed"
        ? t("agentSessions.terminalStatus.failed")
        : session.phase === "canceled"
          ? t("agentSessions.terminalStatus.canceled")
          : undefined;

  // Inline transcript footer: the failure callout (if any) then the draft-PR
  // card, so both read as part of the conversation flow rather than side cards.
  const footer = (
    <>
      {session.phase === "failed" && session.failureReason ? (
        <Alert variant="destructive">
          <AlertCircle />
          <AlertTitle>{t("agentSessions.failureTitle")}</AlertTitle>
          <AlertDescription>{session.failureReason}</AlertDescription>
        </Alert>
      ) : null}
      <PrCard session={session} />
    </>
  );

  return (
    <>
      <SessionDetailHeader
        session={session}
        onCanceled={onChanged}
        onOpenEvidence={onOpenEvidence}
      />

      <div className="min-h-0 flex-1">
        <SessionConversation
          sessionId={session.id}
          isTerminal={session.isTerminal}
          onChatStateChange={onChatStateChange}
          footer={footer}
          terminalLabel={terminalLabel}
        />
      </div>

      <SteeringComposer session={session} chat={chat} onSteered={onChanged} />
    </>
  );
}

/** Evidence as a right-side sheet opened from the header (t006). */
function EvidenceSidePanel({
  session,
  open,
  onOpenChange,
}: {
  session: AgentSessionView;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslations();
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full gap-0 overflow-y-auto sm:max-w-md">
        <SheetHeader>
          <SheetTitle>{t("agentSessions.evidenceTitle")}</SheetTitle>
        </SheetHeader>
        <div className="px-4 pb-6">
          <EvidencePanel evidence={session.evidence} />
        </div>
      </SheetContent>
    </Sheet>
  );
}

// The pending frame: layout shell + sidebar + skeleton, but deliberately NO
// SessionConversation (no useChat/stream mount) so navigating in doesn't fire a
// duplicate resume stream fetch before the real page mounts (w3/m44).
function AgentSessionDetailPending() {
  return (
    <DashboardLayout>
      <div className="flex min-h-0 flex-1">
        <SessionSidebar activeId="" />
        <div className="flex min-w-0 flex-1 flex-col">
          <DetailSkeleton />
        </div>
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
