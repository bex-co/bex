import type { ReactNode } from "react";
import { Navigate } from "@tanstack/react-router";
import { useServer } from "@/features/services/hooks/use-server";
import { isStaticSite } from "@/features/services/lib/service-type";

/**
 * Guards a service tab a static_site doesn't have — Logs, Shell, Scaling, Plan
 * (w5/m48/t001, Render parity): a static site serves from the object store via
 * the shared static-server (docs/ADR029-static-sites.md), so there is no pod to
 * tail, exec into, scale, or size. The sidebar already hides these entries; this
 * guard covers direct URLs, redirecting a static site to its canonical Events
 * landing under /static (w5/m57) instead of rendering runtime UI for instances
 * that don't exist. Children render for every other type, and
 * while the type is still loading (a legit web service must not flash empty) —
 * except with `holdWhileLoading`, for query-heavy children (the Logs page opens
 * an SSE tail on mount, which counts against BEX_MAX_SSE_CONNS) that shouldn't
 * fire and immediately tear down when a static site's direct URL redirects.
 */
export function NonStaticRoute({
  serviceId,
  holdWhileLoading = false,
  children,
}: {
  serviceId: string;
  holdWhileLoading?: boolean;
  children: ReactNode;
}) {
  const { service, loading } = useServer(serviceId);
  if (service && isStaticSite(service)) {
    return (
      <Navigate to="/static/$serviceId/events" params={{ serviceId }} replace />
    );
  }
  if (holdWhileLoading && !service && loading) {
    return null;
  }
  return <>{children}</>;
}
