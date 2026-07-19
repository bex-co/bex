import { createFileRoute } from "@tanstack/react-router";
import { LogViewer } from "@/features/logs/components/log-viewer";
import { NonStaticRoute } from "@/features/services/components/non-static-route";
import type { RangePreset } from "@/features/metrics/lib/range";
import {
  CLEARED_LOG_SEARCH,
  logFiltersFromSearch,
  logFiltersToSearch,
  logRangeFromSearch,
  logSearchEquals,
  parseLogSearch,
  type LogSearch,
} from "@/features/logs/lib/log-search";
import type { LogFilters } from "@/features/logs/types";

export const Route = createFileRoute("/services/$serviceId/logs")({
  component: RouteComponent,
  validateSearch: parseLogSearch,
  // NOTE: no `search.middlewares` alias-strip here, deliberately — a
  // middleware rewrites the URL during hydration for alias-carrying deep
  // links, which re-enters TanStack Start's full-document reload path
  // (observed live, w7/m42). The aliases instead leave the URL on the first
  // interaction, via `write`'s CLEARED_LOG_SEARCH spread below.
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  // One writer for both handlers (w7/m42): the whole filter state mirrors
  // into the URL so a filtered view is shareable and reload-stable. The
  // CLEARED spread explicitly unsets every owned key + the t/r aliases (the
  // router retains raw params an update merely omits), then `next` writes the
  // non-default values back — a cleared filter actually leaves the URL.
  // Writing is skipped when the URL already encodes the state: there is
  // nothing to write, and navigating with an unchanged target during
  // hydration re-enters a full document reload loop under TanStack Start
  // (observed live).
  const write = (next: LogSearch) => {
    if (logSearchEquals(search, next)) return;
    // Functional form, deliberately: its return value is committed verbatim,
    // so the explicit undefineds actually clear; a plain search object treats
    // undefined keys as "no opinion" and the raw params survive (both
    // observed live, w7/m42).
    void navigate({
      search: () => ({ ...CLEARED_LOG_SEARCH, ...next }),
      replace: true,
    });
  };
  return (
    // holdWhileLoading: the viewer fires its history query + SSE tail on
    // mount — don't spend them while the type could still be static_site.
    <NonStaticRoute serviceId={serviceId} holdWhileLoading>
      <ServiceLogsPage
        serviceId={serviceId}
        range={logRangeFromSearch(search)}
        onRangeChange={(preset) => write({ ...search, range: preset.id })}
        initialFilters={logFiltersFromSearch(search)}
        initialLive={search.live !== 0}
        onFiltersChange={(filters, live) =>
          write({ range: search.range, ...logFiltersToSearch(filters, live) })
        }
      />
    </NonStaticRoute>
  );
}

// The Logs tab — bex-api's historical logs query + SSE live tail, laid out like
// Render's Logs viewer (w5/m6). The service chrome (header + nav + content
// container) comes from the `services.$serviceId` layout route; this renders
// only the viewer into the shared `<Outlet/>`. Exported taking `serviceId` as a
// prop (like the Overview page) so the routing test can mount it under a
// reconstructed router without the file Route's param context. A pure
// passthrough — the optional props default inside LogViewer.
export function ServiceLogsPage({
  serviceId,
  range,
  onRangeChange,
  initialFilters,
  initialLive,
  onFiltersChange,
}: {
  serviceId: string;
  range?: RangePreset;
  onRangeChange?: (range: RangePreset) => void;
  initialFilters?: LogFilters;
  initialLive?: boolean;
  onFiltersChange?: (filters: LogFilters, live: boolean) => void;
}) {
  return (
    <LogViewer
      resource={serviceId}
      range={range}
      onRangeChange={onRangeChange}
      initialFilters={initialFilters}
      initialLive={initialLive}
      onFiltersChange={onFiltersChange}
    />
  );
}
