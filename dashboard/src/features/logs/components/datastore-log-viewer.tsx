import { useMemo, useState, type ReactNode } from "react";
import { Loader2, Search } from "lucide-react";
import { EmptyState } from "@/common/components/empty-state";
import { isForbiddenError } from "@/common/lib/graphql-error";
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
import { LogEmptyState } from "./log-empty-state";
import { LogLineList } from "./log-line-list";
import { LogTruncationNotice } from "./log-truncation-notice";
import { useLogLabelValues } from "../hooks/use-log-label-values";
import { RangeSelect } from "@/features/metrics/components/range-select";
import {
  DEFAULT_DATASTORE_LOG_RANGE,
  rangeWindow,
} from "../lib/datastore-log-range";
import { type RangeSelection } from "@/features/metrics/lib/range";
import { useLogHistory } from "../hooks/use-log-history";
import { EMPTY_LOG_FILTERS, LOG_PAGE_SIZE } from "../types";

const ALL_INSTANCES = "all";

// The Logs tab of a managed datastore — Postgres (`dpg-`) or Key Value (`red-`).
// The two are one viewer: the same window/text/instance filters (the datastore
// logs contract answers service-only filters with a named 400,
// docs/render-artifacts/keyvalue-logs.md), the same paging, and copy that
// differs only by namespace.
export type DatastoreLogKind = "databases" | "keyvalue";

type Translate = ReturnType<typeof useTranslations>["t"];

type DatastoreLogCopy = Record<
  | "unavailableTitle"
  | "unavailableBody"
  | "unauthorizedTitle"
  | "unauthorizedBody"
  | "errorTitle"
  | "loading"
  | "emptyTitle"
  | "emptyBody"
  | "rangeLabel"
  | "instanceLabel"
  | "allInstances"
  | "searchPlaceholder",
  string
>;

// One literal t("…") per key, so translation-keys-exist checks every one.
function datastoreLogCopy(
  kind: DatastoreLogKind,
  t: Translate,
): DatastoreLogCopy {
  return kind === "databases"
    ? {
        unavailableTitle: t("databases.logsUnavailableTitle"),
        unavailableBody: t("databases.logsUnavailableBody"),
        unauthorizedTitle: t("databases.logsUnauthorizedTitle"),
        unauthorizedBody: t("databases.logsUnauthorizedBody"),
        errorTitle: t("databases.logsErrorTitle"),
        loading: t("databases.logsLoading"),
        emptyTitle: t("databases.logsEmptyTitle"),
        emptyBody: t("databases.logsEmptyBody"),
        rangeLabel: t("databases.logsRangeLabel"),
        instanceLabel: t("databases.logsInstanceLabel"),
        allInstances: t("databases.logsAllInstances"),
        searchPlaceholder: t("databases.logsSearchPlaceholder"),
      }
    : {
        unavailableTitle: t("keyvalue.logsUnavailableTitle"),
        unavailableBody: t("keyvalue.logsUnavailableBody"),
        unauthorizedTitle: t("keyvalue.logsUnauthorizedTitle"),
        unauthorizedBody: t("keyvalue.logsUnauthorizedBody"),
        errorTitle: t("logs.errorTitle"),
        loading: t("keyvalue.logsLoading"),
        emptyTitle: t("keyvalue.logsEmptyTitle"),
        emptyBody: t("keyvalue.logsEmptyBody"),
        rangeLabel: t("keyvalue.logsRangeLabel"),
        instanceLabel: t("logs.instanceLabel"),
        allInstances: t("keyvalue.logsAllInstances"),
        searchPlaceholder: t("keyvalue.logsSearchPlaceholder"),
      };
}

// `range`/`onRangeChange` are the URL-persisted selection threaded down from
// the hosting route (`databases.$databaseId` / `keyvalue.$keyValueId`, w6/065)
// — this is a component, not a route, so persistence has to come from its
// host. A standalone mount (unit tests) falls back to local state.
export function DatastoreLogViewer({
  kind,
  resource,
  range: rangeProp,
  onRangeChange,
}: {
  kind: DatastoreLogKind;
  resource: string;
  range?: RangeSelection;
  onRangeChange?: (range: RangeSelection) => void;
}) {
  const { t } = useTranslations();
  const copy = datastoreLogCopy(kind, t);
  const [localRange, setLocalRange] = useState<RangeSelection>(
    DEFAULT_DATASTORE_LOG_RANGE,
  );
  const range = rangeProp ?? localRange;
  const setRange = onRangeChange ?? setLocalRange;
  const [text, setText] = useState("");
  const [instance, setInstance] = useState("");
  const debouncedText = useDebounce(text, 300);
  const win = useMemo(() => rangeWindow(range), [range]);
  const queryFilters = useMemo(
    () => ({ ...EMPTY_LOG_FILTERS, text: debouncedText, instance }),
    [debouncedText, instance],
  );
  // Paged exactly like the service Logs tab (w4/m107): scrolling to the top
  // loads older pages until `hasMore` is false (w4/m136).
  const history = useLogHistory(resource, queryFilters, win);
  // bex-api's no-source refusal carries no code yet.
  const unavailable =
    history.error?.message.includes("logs source not configured") ?? false;
  const unauthorized = isForbiddenError(history.error);
  const instances = useLogLabelValues(resource, "instance");

  let body: ReactNode;
  if (unavailable) {
    body = (
      <EmptyState
        iconName="Database"
        title={copy.unavailableTitle}
        description={copy.unavailableBody}
      />
    );
  } else if (unauthorized) {
    body = (
      <EmptyState
        iconName="LockKeyhole"
        title={copy.unauthorizedTitle}
        description={copy.unauthorizedBody}
      />
    );
  } else if (history.timedOut) {
    body = <LogEmptyState reason="timeout" />;
  } else if (history.error) {
    body = (
      <LogEmptyState
        reason="failed"
        title={copy.errorTitle}
        message={history.error.message}
      />
    );
  } else if (history.loading) {
    body = (
      <div className="flex h-64 items-center justify-center rounded-md border text-muted-foreground">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        {copy.loading}
      </div>
    );
  } else if (history.lines.length === 0) {
    body = (
      <LogEmptyState
        reason="empty"
        filtered={Boolean(text || instance)}
        emptyTitle={copy.emptyTitle}
        emptyBody={copy.emptyBody}
        pages={history}
      />
    );
  } else {
    body = (
      <div className="space-y-2">
        {history.hasMore ? (
          <LogTruncationNotice
            partial={history.lines.length < LOG_PAGE_SIZE}
            loadingOlder={history.loadingOlder}
            onLoadOlder={history.loadOlder}
          />
        ) : null}
        <LogLineList
          lines={history.lines}
          hasMore={history.hasMore}
          loadingOlder={history.loadingOlder}
          onLoadOlder={history.loadOlder}
        />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <RangeSelect
          range={range}
          onRangeChange={setRange}
          ariaLabel={copy.rangeLabel}
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
            aria-label={copy.instanceLabel}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_INSTANCES}>{copy.allInstances}</SelectItem>
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
            placeholder={copy.searchPlaceholder}
            aria-label={copy.searchPlaceholder}
            className="pl-8"
          />
        </div>
      </div>
      {body}
    </div>
  );
}
