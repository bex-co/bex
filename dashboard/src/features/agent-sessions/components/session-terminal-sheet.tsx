import { useCallback, useState } from "react";
import { Play, SquareTerminal } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/common/components/ui/sheet";
import {
  ShellTerminal,
  type ShellTerminalCopy,
} from "@/common/components/shell-terminal";
import {
  type Translate,
  useTranslations,
} from "@/common/hooks/use-translations";
import {
  hasGraphQLErrorCode,
  mutationErrorMessage,
} from "@/common/lib/graphql-error";
import { ShellUnavailableError } from "@/features/services/hooks/use-shell-session";
import { useAgentSessionTerminal } from "@/features/agent-sessions/hooks/use-agent-session-terminal";
import {
  AgentSessionError,
  agentSessionErrorMessage,
} from "@/features/agent-sessions/lib/errors";
import type { AgentSessionView } from "@/features/agent-sessions/types";

/** The Terminal's words: a sandbox that starts, and why a session ended. */
const SESSION_TERMINAL_COPY: ShellTerminalCopy = {
  connecting: "agentSessions.terminalConnecting",
  connected: "agentSessions.terminalConnected",
  closed: "agentSessions.terminalClosed",
  errorStatus: "agentSessions.terminalErrorStatus",
  ended: "agentSessions.terminalEnded",
  reconnect: "agentSessions.terminalReconnect",
  errorGeneric: "agentSessions.terminalErrorGeneric",
  starting: "agentSessions.terminalStarting",
  startingDetail: "agentSessions.terminalStartingDetail",
  stillStarting: "agentSessions.terminalStillStarting",
  endReasons: {
    revoked: "agentSessions.terminalEndRevoked",
    instance_gone: "agentSessions.terminalEndSandboxGone",
    timeout: "agentSessions.terminalEndTimeout",
    unverified: "agentSessions.terminalEndUnverified",
    no_sandbox: "agentSessions.terminalEndNoSandbox",
    wake_timeout: "agentSessions.terminalEndWakeTimeout",
  },
};

/** Phases whose session has no sandbox to open, ever again. */
const CANCELED = new Set(["canceling", "canceled"]);

/**
 * The session's Terminal (w10/m615, docs/ADR054-open-in-zed.md D10): a shell
 * in its sandbox, in /workspace, from the browser, as Devin's session
 * workspace has "a new Terminal tab for running commands on Devin's machine".
 * The detail page is one full-page conversation, so it opens in a sheet
 * beside it, as Files does (ADR095 D10), full width on a phone. Opening it
 * on a session whose sandbox closed starts the sandbox, which "opening Files
 * or Terminal wakes Devin" also does: the terminal says the sandbox is
 * starting until the gateway opens the shell. Closing the sheet ends the
 * shell; a canceled or archived session says why it has none.
 *
 * The page remembers per session whether the sheet was open (w10/060), as
 * Devin's workspace pane does, and a session never opened starts with it
 * closed. A sheet the page reopened from that memory (`restored`) opens its
 * shell at once while the sandbox is up, and asks before it starts one that
 * closed, as a sleeping Devin's Desktop tab offers "Wake up Devin" instead of
 * waking it: a page load never starts metered sandbox compute on its own.
 *
 * Its header button shows from 640 px up. Below that the header row has no
 * room for it beside Files, Connect, Stop and Cancel, so the header's "…"
 * menu opens the same sheet (`open`, `onOpenChange`).
 */
export function SessionTerminalSheet({
  session,
  open,
  restored = false,
  onOpenChange,
}: {
  session: AgentSessionView;
  open: boolean;
  /** The page reopened the sheet from an earlier visit; nobody clicked. */
  restored?: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslations();
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          className="hidden shrink-0 gap-1.5 sm:inline-flex"
        >
          <SquareTerminal className="size-4" />
          {t("agentSessions.terminalButton")}
        </Button>
      </SheetTrigger>
      <SheetContent side="right" className="w-full gap-0 sm:max-w-3xl">
        <SheetHeader>
          <SheetTitle>{t("agentSessions.terminalTitle")}</SheetTitle>
          <SheetDescription>
            {t("agentSessions.terminalDescription")}
          </SheetDescription>
        </SheetHeader>
        <div className="flex min-h-0 flex-1 flex-col px-4 pb-4">
          {open ? (
            <SessionTerminal session={session} restored={restored} />
          ) : null}
        </div>
      </SheetContent>
    </Sheet>
  );
}

/**
 * Whether opening a shell would start the session's sandbox: an idle session
 * (completed or failed, not archived) whose sandbox has closed. bex-api's
 * mint starts exactly these (`terminalSandbox`); every other session's
 * sandbox is up, already starting, or never coming back.
 */
function sandboxClosed(session: AgentSessionView): boolean {
  return session.isSteerable && !session.sandboxId;
}

/**
 * The sheet's body: why there is no Terminal, the question a restored sheet
 * asks before it starts a closed sandbox, or the Terminal itself.
 */
function SessionTerminal({
  session,
  restored,
}: {
  session: AgentSessionView;
  restored: boolean;
}) {
  const { t } = useTranslations();
  const closed = sandboxClosed(session);
  // Decided when the sheet opens: a restored sheet holds a closed sandbox's
  // start until the person asks for it. Once the shell opens it stays, and a
  // sandbox that came up meanwhile (a turn started) needs no start.
  const [held, setHeld] = useState(restored && closed);
  if (held && !closed) setHeld(false);
  const none = CANCELED.has(session.phase)
    ? t("agentSessions.terminalCanceled")
    : session.isArchived
      ? t("agentSessions.terminalArchived")
      : null;
  if (none) {
    return (
      <p className="text-muted-foreground py-6 text-sm" role="status">
        {none}
      </p>
    );
  }
  if (held) {
    return (
      <div className="flex flex-col items-start gap-3 py-6">
        <p className="text-muted-foreground text-sm" role="status">
          {t("agentSessions.terminalSandboxClosed")}
        </p>
        <Button size="sm" className="gap-1.5" onClick={() => setHeld(false)}>
          <Play className="size-4" />
          {t("agentSessions.terminalStartSandbox")}
        </Button>
      </div>
    );
  }
  return <SessionShell sessionId={session.id} />;
}

function SessionShell({ sessionId }: { sessionId: string }) {
  const { t } = useTranslations();
  const { createTerminal } = useAgentSessionTerminal();
  const mint = useCallback(
    () => createTerminal(sessionId),
    [createTerminal, sessionId],
  );
  const mintErrorMessage = useCallback(
    (err: unknown) => terminalErrorMessage(err, t),
    [t],
  );
  return (
    <ShellTerminal
      fill
      mint={mint}
      mintErrorMessage={mintErrorMessage}
      copy={SESSION_TERMINAL_COPY}
    />
  );
}

/**
 * Why a Terminal's ticket was refused, in the reader's language: no browser
 * terminal on this deployment; a closed sandbox the caller may view but not
 * start, in the SSH banner's words (w10/043); a canceled or archived
 * session; or the start's own refusal (payment method, billing, the
 * live-sandbox limit) after "The sandbox couldn't start".
 */
function terminalErrorMessage(err: unknown, t: Translate): string {
  if (err instanceof ShellUnavailableError) {
    return t("agentSessions.terminalUnavailable");
  }
  if (err instanceof AgentSessionError) {
    if (err.code === "AGENT_SESSION_NOT_ATTACHABLE") {
      if (err.params.sandbox === "closed") {
        return t("agentSessions.terminalCannotStart");
      }
      if (CANCELED.has(String(err.params.phase ?? ""))) {
        return t("agentSessions.terminalCanceled");
      }
      return err.message || t("agentSessions.terminalErrorGeneric");
    }
    if (err.code === "AGENT_SESSION_ARCHIVED") {
      return t("agentSessions.terminalArchived");
    }
    return t("agentSessions.terminalStartRefused", {
      reason: agentSessionErrorMessage(err, t),
    });
  }
  if (START_REFUSALS.some((code) => hasGraphQLErrorCode(err, code))) {
    return t("agentSessions.terminalStartRefused", {
      reason: mutationErrorMessage(
        err,
        t("agentSessions.terminalErrorGeneric"),
      ),
    });
  }
  return mutationErrorMessage(err, t("agentSessions.terminalErrorGeneric"));
}

/** Admission's refusals of a start that are not the session's own codes. */
const START_REFUSALS = [
  "PAYMENT_REQUIRED",
  "BILLING_ENFORCED",
  "BILLING_BALANCE_DUE",
];
