import { useCallback, useEffect, useRef, useState } from "react";
import type { Terminal as XTerminal } from "@xterm/xterm";
import { RotateCw } from "lucide-react";
import "@xterm/xterm/css/xterm.css";
import { Button } from "@/common/components/ui/button";
import {
  type Translate,
  useTranslations,
} from "@/common/hooks/use-translations";
import { formatDuration } from "@/common/lib/format";
import { cn } from "@/common/lib/utils/utils.ts";

type Status =
  "connecting" | "starting" | "connected" | "closed" | "ended" | "error";

type CopyKey = Parameters<Translate>[0];

/** A minted Web Shell ticket: what the gateway WebSocket is opened with. */
export interface ShellTicket {
  /** The signed exec ticket to present to the gateway WebSocket. */
  ticket: string;
  /** The browser-reachable gateway WebSocket origin (e.g. wss://ssh.bex.co/shell). */
  url: string;
  /** The target is starting, and the gateway opens the shell once it is up. */
  starting?: boolean;
}

/**
 * The words a terminal uses, as catalog keys, so each surface keeps its own:
 * a service's Shell page speaks of instances, an agent session's Terminal of
 * its sandbox. `endReasons` names each reason the gateway ends a session or a
 * wait with (w4/m352, w10/m615); a reason missing there falls back to the
 * gateway's own sentence, so a newer gateway never shows a blank end.
 */
export interface ShellTerminalCopy {
  connecting: CopyKey;
  connected: CopyKey;
  closed: CopyKey;
  errorStatus: CopyKey;
  ended: CopyKey;
  reconnect: CopyKey;
  errorGeneric: CopyKey;
  endReasons: Record<string, CopyKey>;
  /** Only a target that can be starting needs these (w10/m615). */
  starting?: CopyKey;
  startingDetail?: CopyKey;
  /** Takes `{elapsed}`. */
  stillStarting?: CopyKey;
}

/**
 * The Browser Web Shell terminal (w2/m55, docs/ADR035-ssh.md § Browser Web
 * Shell), shared by a service's Shell page and an agent session's Terminal
 * (w10/m615). It mints a short-lived exec ticket from bex-api, opens the
 * isolated gateway's WebSocket with it, and streams an interactive TTY
 * through xterm.js. No SSH private key or Kubernetes credential touches the
 * browser — the ticket is the only capability, and the gateway (not bex-api)
 * holds pods/exec.
 *
 * A target that is starting (an agent session's sandbox bex-api's mint
 * woke) reads "starting" until the gateway says it is ready: the gateway
 * holds the WebSocket, says so when the wait begins and every 30 seconds,
 * then opens the shell.
 *
 * `mint` and `mintErrorMessage` are the surface's own; a new `mint` (another
 * target) opens a new session. xterm.js touches `window`/`document`, so it is
 * imported and instantiated only inside the client-side effect, never during
 * SSR.
 */
export function ShellTerminal({
  mint,
  mintErrorMessage,
  copy,
  fill = false,
}: {
  mint: () => Promise<ShellTicket>;
  mintErrorMessage: (err: unknown) => string;
  copy: ShellTerminalCopy;
  /** Fill the parent's height (a sheet) instead of a fixed 24rem. */
  fill?: boolean;
}) {
  const { t, i18n } = useTranslations();
  const language = i18n.language;
  const containerRef = useRef<HTMLDivElement>(null);
  const [status, setStatus] = useState<Status>("connecting");
  const [detail, setDetail] = useState<string>("");
  const [attempt, setAttempt] = useState(0);

  const reconnect = useCallback(() => setAttempt((n) => n + 1), []);

  // Any change to the connection effect's inputs below opens a new session, so
  // the previous one's status and detail no longer apply. They are reset during
  // render ("storing information from previous renders"), not from the effect
  // body; keep this list in step with that effect's dependencies.
  const [session, setSession] = useState({
    mint,
    mintErrorMessage,
    copy,
    attempt,
    t,
    language,
  });
  if (
    session.mint !== mint ||
    session.mintErrorMessage !== mintErrorMessage ||
    session.copy !== copy ||
    session.attempt !== attempt ||
    session.t !== t ||
    session.language !== language
  ) {
    setSession({ mint, mintErrorMessage, copy, attempt, t, language });
    setStatus("connecting");
    setDetail("");
  }

  useEffect(() => {
    let cancelled = false;
    let term: XTerminal | undefined;
    let ws: WebSocket | undefined;
    let observer: ResizeObserver | undefined;
    // Records that a terminal end-reason (exit or error control frame) was
    // already shown, so the socket's own close event doesn't overwrite it with a
    // generic "session closed".
    let finalized = false;
    // The target is starting: the gateway holds the WebSocket until it is up.
    let starting = false;

    const startingDetail = (elapsed?: number) =>
      elapsed && copy.stillStarting
        ? t(copy.stillStarting, {
            elapsed: formatDuration(elapsed * 1000, language),
          })
        : copy.startingDetail
          ? t(copy.startingDetail)
          : "";
    const markStarting = (elapsed?: number) => {
      starting = true;
      setStatus("starting");
      setDetail(startingDetail(elapsed));
    };

    void (async () => {
      const [{ Terminal }, { FitAddon }] = await Promise.all([
        import("@xterm/xterm"),
        import("@xterm/addon-fit"),
      ]);
      if (cancelled || !containerRef.current) return;

      term = new Terminal({
        cursorBlink: true,
        fontSize: 13,
        fontFamily:
          "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace",
        theme: { background: "#0a0a0a", foreground: "#e5e5e5" },
      });
      const fit = new FitAddon();
      term.loadAddon(fit);
      term.open(containerRef.current);
      fit.fit();

      let ticket: ShellTicket;
      try {
        ticket = await mint();
      } catch (err) {
        if (cancelled) return;
        finalized = true;
        setStatus("error");
        // A refusal names its own blocker (w4/506): the surface maps it.
        setDetail(mintErrorMessage(err));
        return;
      }
      if (cancelled) return;
      if (ticket.starting) markStarting();

      // The ticket rides a Sec-WebSocket-Protocol entry, never the URL — the
      // request line lands in the edge proxy's access log, headers don't
      // (w1/042 L8). The gateway selects the bare "bex.shell" marker, so the
      // credential isn't echoed in the handshake response either.
      ws = new WebSocket(ticket.url, [
        "bex.shell",
        `bex.ticket.${ticket.ticket}`,
      ]);
      ws.binaryType = "arraybuffer";

      const sendResize = () => {
        if (!term || ws?.readyState !== WebSocket.OPEN) return;
        fit.fit();
        ws.send(
          JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }),
        );
      };

      ws.onopen = () => {
        if (cancelled) return;
        if (!starting) setStatus("connected");
        sendResize();
      };
      ws.onmessage = (event) => {
        if (!term) return;
        if (typeof event.data === "string") {
          handleControl(event.data);
          return;
        }
        term.write(new Uint8Array(event.data as ArrayBuffer));
      };
      ws.onclose = () => {
        if (cancelled || finalized) return;
        setStatus("closed");
        setDetail("");
      };
      ws.onerror = () => {
        if (cancelled || finalized) return;
        finalized = true;
        setStatus("error");
        setDetail(t(copy.errorGeneric));
      };

      // Browser → gateway: keystrokes/paste as binary stdin frames; resize as a
      // JSON control frame. This matches the gateway's frame-type protocol.
      // Nothing is typed into a shell that has not opened yet.
      term.onData((data) => {
        if (ws?.readyState === WebSocket.OPEN && !starting) {
          ws.send(new TextEncoder().encode(data));
        }
      });

      observer = new ResizeObserver(() => sendResize());
      observer.observe(containerRef.current);

      function handleControl(raw: string) {
        let msg: {
          type?: string;
          code?: number;
          message?: string;
          reason?: string;
          elapsed?: number;
        };
        try {
          msg = JSON.parse(raw);
        } catch {
          return;
        }
        if (msg.type === "starting") {
          markStarting(msg.elapsed);
        } else if (msg.type === "ready") {
          starting = false;
          setStatus("connected");
          setDetail("");
        } else if (msg.type === "error") {
          // A reason means the server ended a live session, not a failure.
          const key =
            msg.reason && Object.hasOwn(copy.endReasons, msg.reason)
              ? copy.endReasons[msg.reason]
              : undefined;
          finalized = true;
          setStatus(msg.reason ? "ended" : "error");
          setDetail((key && t(key)) || msg.message || t(copy.errorGeneric));
        } else if (msg.type === "exit") {
          finalized = true;
          setStatus("closed");
          setDetail("");
        }
      }
    })();

    return () => {
      cancelled = true;
      observer?.disconnect();
      ws?.close();
      term?.dispose();
    };
  }, [mint, mintErrorMessage, copy, attempt, t, language]);

  return (
    <div className={cn("space-y-2", fill && "flex min-h-0 flex-1 flex-col")}>
      <div className="flex items-center justify-between gap-2">
        <StatusBadge status={status} copy={copy} />
        {(status === "closed" || status === "ended" || status === "error") && (
          <Button variant="outline" size="sm" onClick={reconnect}>
            <RotateCw />
            {t(copy.reconnect)}
          </Button>
        )}
      </div>
      <div
        className={cn(
          "bg-[#0a0a0a] overflow-hidden rounded-md border p-2",
          fill && "min-h-0 flex-1",
        )}
      >
        <div
          ref={containerRef}
          className={fill ? "h-full w-full" : "h-96 w-full"}
        />
      </div>
      {detail && <p className="text-muted-foreground text-sm">{detail}</p>}
    </div>
  );
}

function StatusBadge({
  status,
  copy,
}: {
  status: Status;
  copy: ShellTerminalCopy;
}) {
  const { t } = useTranslations();
  const label =
    status === "connecting"
      ? t(copy.connecting)
      : status === "starting"
        ? t(copy.starting ?? copy.connecting)
        : status === "connected"
          ? t(copy.connected)
          : status === "error"
            ? t(copy.errorStatus)
            : status === "ended"
              ? t(copy.ended)
              : t(copy.closed);
  const dot =
    status === "connected"
      ? "bg-emerald-500"
      : status === "connecting"
        ? "bg-amber-500"
        : status === "starting"
          ? "animate-pulse bg-amber-500"
          : status === "error"
            ? "bg-destructive"
            : "bg-muted-foreground";
  return (
    <span className="text-muted-foreground flex items-center gap-2 text-sm">
      <span className={`size-2 rounded-full ${dot}`} aria-hidden />
      {label}
    </span>
  );
}
