import { useCallback, useMemo } from "react";
import { serviceTabHead } from "@/features/services/lib/service-tab-head";
import { createFileRoute } from "@tanstack/react-router";
import { LogViewer } from "@/features/logs/components/log-viewer";
import { NonStaticRoute } from "@/features/services/components/non-static-route";
import {
  CLEARED_LOG_SEARCH,
  logFiltersFromSearch,
  logFiltersToSearch,
  logSearchEquals,
  parseLogSearch,
  serviceLineShell,
  serviceLogContextHref,
  serviceLogRange,
  type LogSearch,
} from "@/features/logs/lib/log-search";
import {
  logRangeToSearch,
  type LogRange,
} from "@/features/logs/lib/log-range";
import type { LogFilters, LogLine } from "@/features/logs/types";
import { useServer } from "@/features/services/hooks/use-server";
import { useServiceInstances } from "@/features/services/hooks/use-service-instances";
import { publiclyRoutable } from "@/features/services/lib/service-type";
import {
  ServiceLogsBodySkeleton,
  ServiceLogsSkeleton,
} from "@/common/components/route-skeletons";

export const Route = createFileRoute("/services/$serviceId/logs")({
  head: serviceTabHead("services.navLogs", "/services/$serviceId"),
  component: RouteComponent,
  pendingComponent: ServiceLogsSkeleton,
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
    // mount — don't spend them while the type could still be static_site. A
    // log-shaped skeleton holds the space so the tab never flashes blank (w9/m63 t003).
    <NonStaticRoute
      serviceId={serviceId}
      holdWhileLoading
      loadingFallback={<ServiceLogsBodySkeleton />}
    >
      <ServiceLogsPage
        serviceId={serviceId}
        range={serviceLogRange(search)}
        onRangeChange={(range) =>
          write({
            ...search,
            ...logRangeToSearch(range),
            // A View in context mark belongs to the range it opened with.
            contextLine: undefined,
          })
        }
        contextLine={search.contextLine}
        urlFilters={logFiltersFromSearch(search)}
        onFiltersChange={(filters) => {
          const next: LogSearch = {
            // A filter change preserves whatever range is active — Live tail,
            // a preset id or the custom window's start/end (w5/m56).
            range: search.range,
            rangeStart: search.rangeStart,
            rangeEnd: search.rangeEnd,
            ...logFiltersToSearch(filters),
          };
          // A View in context mark (w3/m528) leaves with the first change to
          // what the view shows; a report of the state the URL already names
          // keeps it.
          const marked = { ...next, contextLine: search.contextLine };
          write(logSearchEquals(search, marked) ? marked : next);
        }}
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
// passthrough apart from the service type, which decides whether request
// logs are offered; the optional props default inside LogViewer.
export function ServiceLogsPage({
  serviceId,
  range,
  onRangeChange,
  urlFilters,
  onFiltersChange,
  contextLine,
}: {
  serviceId: string;
  range?: LogRange;
  onRangeChange?: (range: LogRange) => void;
  urlFilters?: LogFilters;
  onFiltersChange?: (filters: LogFilters) => void;
  contextLine?: string;
}) {
  // A worker, cron job or private service receives no public HTTP requests,
  // so it is offered no request logs, as on Render (w4/613); a stale
  // `?type=request` link reads its application logs. The layout mounts this
  // tab once the service is loaded.
  const { service } = useServer(serviceId, { poll: false });
  // Render's SSH into instance (w3/m528): on a service whose Shell can open
  // (`sshAddress`, as the Shell page decides), a line's instance opens there
  // while it is one of the running instances.
  const shellEligible = Boolean(service?.sshAddress);
  const { instances } = useServiceInstances(serviceId, {
    skip: !shellEligible,
  });
  // One value per set of instances, so a poll that finds the same ones
  // keeps the rows' menu items (and the memoized rows) as they are.
  const runningIds = instances.map((instance) => instance.id).join(" ");
  const shell = useMemo(
    () =>
      shellEligible
        ? serviceLineShell(serviceId, new Set(runningIds.split(" ")))
        : undefined,
    [shellEligible, runningIds, serviceId],
  );
  const contextHref = useCallback(
    (line: LogLine) => serviceLogContextHref(serviceId, line),
    [serviceId],
  );
  return (
    <LogViewer
      resource={serviceId}
      range={range}
      onRangeChange={onRangeChange}
      urlFilters={urlFilters}
      onFiltersChange={onFiltersChange}
      servesRequests={service == null || publiclyRoutable(service.type)}
      contextLine={contextLine}
      contextHref={contextHref}
      shell={shell}
    />
  );
}
