import { useCallback, useEffect, useRef, useState } from "react";
import type { Terminal as XTerminal } from "@xterm/xterm";
import { RotateCw } from "lucide-react";
import "@xterm/xterm/css/xterm.css";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  ShellUnavailableError,
  useShellSession,
} from "@/features/services/hooks/use-shell-session";

type Status = "connecting" | "connected" | "closed" | "error";

/**
 * The Browser Web Shell terminal (w2/m55, docs/ADR035-ssh.md § Browser Web
 * Shell). It mints a short-lived exec ticket from bex-api, opens the isolated
 * gateway's WebSocket with it, and streams an interactive TTY through xterm.js.
 * No SSH private key or Kubernetes credential touches the browser — the ticket
 * is the only capability, and the gateway (not bex-api) holds pods/exec.
 *
 * xterm.js touches `window`/`document`, so it is imported and instantiated only
 * inside the client-side effect, never during SSR.
 */
export function WebShellTerminal({
  serviceId,
  instanceId,
}: {
  serviceId: string;
  instanceId?: string;
}) {
  const { t } = useTranslations();
  const { createShellSession } = useShellSession();
  const containerRef = useRef<HTMLDivElement>(null);
  const [status, setStatus] = useState<Status>("connecting");
  const [detail, setDetail] = useState<string>("");
  const [attempt, setAttempt] = useState(0);

  const reconnect = useCallback(() => setAttempt((n) => n + 1), []);

  useEffect(() => {
    let cancelled = false;
    let term: XTerminal | undefined;
    let ws: WebSocket | undefined;
    let observer: ResizeObserver | undefined;
    // Records that a terminal end-reason (exit or error control frame) was
    // already shown, so the socket's own close event doesn't overwrite it with a
    // generic "session closed".
    let finalized = false;

    setStatus("connecting");
    setDetail("");

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

      let session;
      try {
        session = await createShellSession(serviceId, instanceId);
      } catch (err) {
        if (cancelled) return;
        finalized = true;
        if (err instanceof ShellUnavailableError) {
          setStatus("error");
          setDetail(t("services.shellErrorUnavailable"));
        } else {
          setStatus("error");
          setDetail(t("services.shellErrorGeneric"));
        }
        return;
      }
      if (cancelled) return;

      // The ticket rides a Sec-WebSocket-Protocol entry, never the URL — the
      // request line lands in the edge proxy's access log, headers don't
      // (w1/042 L8). The gateway selects the bare "bex.shell" marker, so the
      // credential isn't echoed in the handshake response either.
      ws = new WebSocket(session.url, [
        "bex.shell",
        `bex.ticket.${session.ticket}`,
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
        setStatus("connected");
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
        setDetail(t("services.shellErrorGeneric"));
      };

      // Browser → gateway: keystrokes/paste as binary stdin frames; resize as a
      // JSON control frame. This matches the gateway's frame-type protocol.
      term.onData((data) => {
        if (ws?.readyState === WebSocket.OPEN) {
          ws.send(new TextEncoder().encode(data));
        }
      });

      observer = new ResizeObserver(() => sendResize());
      observer.observe(containerRef.current);

      function handleControl(raw: string) {
        let msg: { type?: string; code?: number; message?: string };
        try {
          msg = JSON.parse(raw);
        } catch {
          return;
        }
        if (msg.type === "error") {
          finalized = true;
          setStatus("error");
          setDetail(msg.message ?? t("services.shellErrorGeneric"));
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
  }, [serviceId, instanceId, attempt, createShellSession, t]);

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-2">
        <StatusBadge status={status} />
        {(status === "closed" || status === "error") && (
          <Button variant="outline" size="sm" onClick={reconnect}>
            <RotateCw />
            {t("services.shellReconnect")}
          </Button>
        )}
      </div>
      <div className="bg-[#0a0a0a] overflow-hidden rounded-md border p-2">
        <div ref={containerRef} className="h-96 w-full" />
      </div>
      {detail && <p className="text-muted-foreground text-sm">{detail}</p>}
    </div>
  );
}

function StatusBadge({ status }: { status: Status }) {
  const { t } = useTranslations();
  const label =
    status === "connecting"
      ? t("services.shellConnecting")
      : status === "connected"
        ? t("services.shellConnected")
        : status === "error"
          ? t("services.shellErrorStatus")
          : t("services.shellClosed");
  const dot =
    status === "connected"
      ? "bg-emerald-500"
      : status === "connecting"
        ? "bg-amber-500"
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
