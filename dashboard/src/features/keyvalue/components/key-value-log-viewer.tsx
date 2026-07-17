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
import {
  type KeyValueLogFilters,
  type KeyValueLogRange,
  useKeyValueLogs,
} from "../hooks/use-key-value-logs";

const ALL_INSTANCES = "all";

export function KeyValueLogViewer({ resource }: { resource: string }) {
  const { t } = useTranslations();
  const [filters, setFilters] = useState<KeyValueLogFilters>({
    range: "1h",
    text: "",
    instance: "",
  });
  const debouncedText = useDebounce(filters.text, 300);
  const queryFilters = useMemo(
    () => ({ ...filters, text: debouncedText }),
    [filters, debouncedText],
  );
  const result = useKeyValueLogs(resource, queryFilters);
  const instances = useLogLabelValues(resource, "instance");

  let body: ReactNode;
  if (result.unavailable) {
    body = (
      <EmptyState
        iconName="Database"
        title={t("keyvalue.logsUnavailableTitle")}
        description={t("keyvalue.logsUnavailableBody")}
      />
    );
  } else if (result.unauthorized) {
    body = (
      <EmptyState
        iconName="LockKeyhole"
        title={t("keyvalue.logsUnauthorizedTitle")}
        description={t("keyvalue.logsUnauthorizedBody")}
      />
    );
  } else if (result.error) {
    body = (
      <EmptyState
        iconName="AlertCircle"
        title={t("keyvalue.logsErrorTitle")}
        description={result.error.message}
      />
    );
  } else if (result.loading && result.lines.length === 0) {
    body = (
      <div className="flex h-64 items-center justify-center rounded-md border text-muted-foreground">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        {t("keyvalue.logsLoading")}
      </div>
    );
  } else if (result.lines.length === 0) {
    body = (
      <EmptyState
        iconName="ScrollText"
        title={t("keyvalue.logsEmptyTitle")}
        description={
          filters.text || filters.instance
            ? t("keyvalue.logsEmptyFilteredBody")
            : t("keyvalue.logsEmptyBody")
        }
      />
    );
  } else {
    body = <LogLineList lines={result.lines} />;
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <Select
          value={filters.range}
          onValueChange={(range) =>
            setFilters((current) => ({
              ...current,
              range: range as KeyValueLogRange,
            }))
          }
        >
          <SelectTrigger
            className="w-40"
            size="sm"
            aria-label={t("keyvalue.logsRangeLabel")}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="1h">{t("keyvalue.logsRange1h")}</SelectItem>
            <SelectItem value="6h">{t("keyvalue.logsRange6h")}</SelectItem>
            <SelectItem value="24h">{t("keyvalue.logsRange24h")}</SelectItem>
          </SelectContent>
        </Select>

        <Select
          value={filters.instance || ALL_INSTANCES}
          onValueChange={(instance) =>
            setFilters((current) => ({
              ...current,
              instance: instance === ALL_INSTANCES ? "" : instance,
            }))
          }
        >
          <SelectTrigger
            className="w-56"
            size="sm"
            aria-label={t("keyvalue.logsInstanceLabel")}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_INSTANCES}>
              {t("keyvalue.logsAllInstances")}
            </SelectItem>
            {instances.map((instance) => (
              <SelectItem key={instance} value={instance}>
                {instance}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <div className="relative min-w-56 flex-1">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={filters.text}
            onChange={(event) =>
              setFilters((current) => ({
                ...current,
                text: event.target.value,
              }))
            }
            placeholder={t("keyvalue.logsSearchPlaceholder")}
            aria-label={t("keyvalue.logsSearchPlaceholder")}
            className="pl-8"
          />
        </div>
      </div>
      {body}
    </div>
  );
}
