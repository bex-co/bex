import { useMemo, useState, type ReactNode } from "react";
import { Loader2, Search } from "lucide-react";
import { EmptyState } from "@/common/components/empty-state";
import { Input } from "@/common/components/ui/input.tsx";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select.tsx";
import { useDebounce } from "@/common/hooks/use-debounce";
import { useTranslations } from "@/common/hooks/use-translations";
import { LogLineList } from "@/features/logs/components/log-line-list";
import { useLogLabelValues } from "@/features/logs/hooks/use-log-label-values";
import { RangeSelect } from "@/features/metrics/components/range-select";
import {
  DEFAULT_DATASTORE_LOG_RANGE,
  rangeWindow,
} from "@/features/logs/lib/datastore-log-range";
import { type RangeSelection } from "@/features/metrics/lib/range";
import { usePostgresLogs } from "../hooks/use-postgres-logs";

const ALL_INSTANCES = "all";

export function PostgresLogViewer({ resource }: { resource: string }) {
  const { t } = useTranslations();
  const [range, setRange] = useState<RangeSelection>(
    DEFAULT_DATASTORE_LOG_RANGE,
  );
  const [text, setText] = useState("");
  const [instance, setInstance] = useState("");
  const debouncedText = useDebounce(text, 300);
  const win = useMemo(() => rangeWindow(range), [range]);
  const queryFilters = useMemo(
    () => ({ ...win, text: debouncedText, instance }),
    [win, debouncedText, instance],
  );
  const result = usePostgresLogs(resource, queryFilters);
  const instances = useLogLabelValues(resource, "instance");

  let body: ReactNode;
  if (result.unavailable) {
    body = (
      <EmptyState
        iconName="Database"
        title={t("databases.logsUnavailableTitle")}
        description={t("databases.logsUnavailableBody")}
      />
    );
  } else if (result.unauthorized) {
    body = (
      <EmptyState
        iconName="LockKeyhole"
        title={t("databases.logsUnauthorizedTitle")}
        description={t("databases.logsUnauthorizedBody")}
      />
    );
  } else if (result.error) {
    body = (
      <EmptyState
        iconName="AlertCircle"
        title={t("databases.logsErrorTitle")}
        description={result.error.message}
      />
    );
  } else if (result.loading && result.lines.length === 0) {
    body = (
      <div className="flex h-64 items-center justify-center rounded-md border text-muted-foreground">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        {t("databases.logsLoading")}
      </div>
    );
  } else if (result.lines.length === 0) {
    body = (
      <EmptyState
        iconName="ScrollText"
        title={t("databases.logsEmptyTitle")}
        description={
          text || instance
            ? t("databases.logsEmptyFilteredBody")
            : t("databases.logsEmptyBody")
        }
      />
    );
  } else {
    body = <LogLineList lines={result.lines} />;
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <RangeSelect
          range={range}
          onRangeChange={setRange}
          ariaLabel={t("databases.logsRangeLabel")}
        />

        <Select
          value={instance || ALL_INSTANCES}
          onValueChange={(value) =>
            setInstance(value === ALL_INSTANCES ? "" : value)
          }
        >
          <SelectTrigger
            className="w-56"
            size="sm"
            aria-label={t("databases.logsInstanceLabel")}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_INSTANCES}>
              {t("databases.logsAllInstances")}
            </SelectItem>
            {instances.map((inst) => (
              <SelectItem key={inst} value={inst}>
                {inst}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <div className="relative min-w-56 flex-1">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={text}
            onChange={(event) => setText(event.target.value)}
            placeholder={t("databases.logsSearchPlaceholder")}
            aria-label={t("databases.logsSearchPlaceholder")}
            className="pl-8"
          />
        </div>
      </div>
      {body}
    </div>
  );
}
