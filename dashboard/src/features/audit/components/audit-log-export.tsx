import { useEffect, useId, useRef, useState } from "react";
import { Download, Loader2 } from "lucide-react";
import { useApolloClient } from "@apollo/client/react";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { useTranslations } from "@/common/hooks/use-translations";
import { downloadTextFile } from "@/common/lib/download-file";
import {
  AuditExportTooLargeError,
  MAX_EXPORT_DAYS,
  auditEventsToCSV,
  exportRangeDays,
  fetchAuditRange,
} from "@/features/audit/lib/audit-csv";

/** "YYYY-MM-DD" of a date in UTC, the days the export reads. */
function utcDay(date: Date): string {
  return date.toISOString().slice(0, 10);
}

/**
 * Workspace Settings → Audit Logs → Export as CSV (w3/m305): Render's export,
 * a start and end date at most 90 days apart. The enclosing panel renders it
 * only for an admin on a plan with audit logs, as Render allows.
 */
export function AuditLogExport({ ownerId }: { ownerId: string }) {
  const { t } = useTranslations();
  const client = useApolloClient();
  const startId = useId();
  const endId = useId();
  const messageId = useId();
  // A switch of workspace or leaving the page abandons an export in flight.
  const inFlight = useRef<AbortController | null>(null);
  useEffect(() => () => inFlight.current?.abort(), [ownerId]);
  const today = utcDay(new Date());
  const [start, setStart] = useState(() =>
    utcDay(new Date(Date.now() - 29 * 86_400_000)),
  );
  const [end, setEnd] = useState(today);
  const [exporting, setExporting] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);

  const days = start && end ? exportRangeDays(start, end) : 0;
  const invalid =
    !start || !end
      ? t("audit.exportRangeMissing")
      : days < 1
        ? t("audit.exportRangeOrder")
        : days > MAX_EXPORT_DAYS
          ? t("audit.exportRangeTooLong", { days: MAX_EXPORT_DAYS })
          : null;

  async function exportCSV() {
    const controller = new AbortController();
    inFlight.current = controller;
    setExporting(true);
    setFailure(null);
    try {
      const events = await fetchAuditRange(
        client,
        ownerId,
        start,
        end,
        controller.signal,
      );
      downloadTextFile(
        `audit-logs-${ownerId}-${start}-${end}.csv`,
        auditEventsToCSV(events),
        "text/csv;charset=utf-8",
      );
    } catch (err) {
      if (controller.signal.aborted) return;
      setFailure(
        err instanceof AuditExportTooLargeError
          ? t("audit.exportTooLarge")
          : t("audit.exportFailed"),
      );
    } finally {
      if (!controller.signal.aborted) setExporting(false);
    }
  }

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-end gap-3">
        <div className="space-y-1">
          <Label htmlFor={startId}>{t("audit.exportStart")}</Label>
          <Input
            id={startId}
            type="date"
            aria-invalid={invalid !== null}
            aria-describedby={messageId}
            value={start}
            max={end || today}
            onChange={(e) => setStart(e.target.value)}
            className="w-40"
          />
        </div>
        <div className="space-y-1">
          <Label htmlFor={endId}>{t("audit.exportEnd")}</Label>
          <Input
            id={endId}
            type="date"
            aria-invalid={invalid !== null}
            aria-describedby={messageId}
            value={end}
            min={start}
            max={today}
            onChange={(e) => setEnd(e.target.value)}
            className="w-40"
          />
        </div>
        <Button
          variant="outline"
          disabled={exporting || invalid !== null}
          onClick={() => void exportCSV()}
        >
          {exporting ? <Loader2 className="animate-spin" /> : <Download />}
          {t("audit.exportCSV")}
        </Button>
      </div>
      {(invalid ?? failure) ? (
        <p id={messageId} role="alert" className="text-sm text-destructive">
          {invalid ?? failure}
        </p>
      ) : (
        <p id={messageId} className="text-xs text-muted-foreground">
          {t("audit.exportHint", { days: MAX_EXPORT_DAYS })}
        </p>
      )}
    </div>
  );
}
