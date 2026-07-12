import { useMemo, useState, type ReactNode } from "react";
import { AlertCircle, Loader2, WifiOff } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { useDebounce } from "@/common/hooks/use-debounce";
import { EmptyState } from "@/common/components/empty-state";
import { useLogHistory } from "../hooks/use-log-history";
import {
  useLiveLogs,
  type EventSourceFactory,
  type LiveStatus,
} from "../hooks/use-live-logs";
import { mergeLogLines } from "../lib/map";
import { LOG_TYPE_ALL, type LogTypeFilter } from "../types";
import { LogFilterBar } from "./log-filter-bar";
import { LogLineList } from "./log-line-list";

interface LogViewerProps {
  resource: string;
  /** Injectable SSE factory — threaded through for tests. */
  createEventSource?: EventSourceFactory;
}

/**
 * The service page's Logs tab: bex-api's historical `logs(...)` query plus an
 * SSE live tail (docs/ADR010-observability.md), laid out like Render's Logs viewer
 * (filter bar + line list). Owns the filter state; the data layer (history hook
 * + live hook) is presentation-free.
 */
export function LogViewer({ resource, createEventSource }: LogViewerProps) {
  const { t } = useTranslations();
  const [type, setType] = useState<LogTypeFilter>(LOG_TYPE_ALL);
  const [text, setText] = useState("");
  const [live, setLive] = useState(true);

  // Debounce the search so a keystroke doesn't refetch history and reopen the
  // stream on every character.
  const debouncedText = useDebounce(text, 300);

  const history = useLogHistory({ resource, type, text: debouncedText });
  const stream = useLiveLogs({
    resource,
    enabled: live,
    type,
    text: debouncedText,
    createEventSource,
  });

  // Live lines append after the historical page, deduped against it.
  const lines = useMemo(
    () => mergeLogLines(history.lines, stream.lines),
    [history.lines, stream.lines],
  );

  // One body per state (error / loading-first / empty / list) — resolved to a
  // single node so the render stays flat.
  let body: ReactNode;
  if (history.error) {
    body = (
      <EmptyState
        iconName="AlertCircle"
        title={t("logs.errorTitle")}
        description={history.error.message}
      />
    );
  } else if (history.loading && history.lines.length === 0) {
    body = (
      <div className="flex h-64 items-center justify-center rounded-md border text-muted-foreground">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        {t("logs.loading")}
      </div>
    );
  } else if (lines.length === 0) {
    body = (
      <EmptyState
        iconName="ScrollText"
        title={t("logs.emptyTitle")}
        description={
          type === LOG_TYPE_ALL
            ? t("logs.emptyBody")
            : t("logs.emptyFilteredBody")
        }
      />
    );
  } else {
    body = (
      <div className="space-y-2">
        {live && stream.status === "error" ? (
          <div className="flex items-center gap-2 rounded-md border border-dashed px-3 py-2 text-sm text-muted-foreground">
            <WifiOff className="h-4 w-4" />
            {t("logs.disconnected")}
          </div>
        ) : null}
        <LogLineList lines={lines} />
        <StreamStatus live={live} status={stream.status} />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <LogFilterBar
        type={type}
        onTypeChange={setType}
        text={text}
        onTextChange={setText}
        live={live}
        onLiveChange={setLive}
      />
      {body}
    </div>
  );
}

// The footer under the log list: a live pulse while streaming, a paused note
// when the tail is off, nothing in between (connecting/error already surface
// above the list).
function StreamStatus({ live, status }: { live: boolean; status: LiveStatus }) {
  const { t } = useTranslations();
  return (
    <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
      {live && status === "open" ? (
        <>
          <span className="relative flex h-2 w-2">
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-green-500 opacity-75" />
            <span className="relative inline-flex h-2 w-2 rounded-full bg-green-500" />
          </span>
          {t("logs.streaming")}
        </>
      ) : !live ? (
        <>
          <AlertCircle className="h-3.5 w-3.5" />
          {t("logs.paused")}
        </>
      ) : null}
    </p>
  );
}
